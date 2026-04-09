package repository

import (
	"bytes"
	"errors"
	"fmt"
	"inkdrop/config"
	repoStore "inkdrop/repository"
	viewBackend "inkdrop/view/connector"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

const liveEditMaxFileSize int64 = 5 * 1024 * 1024

type liveEditTarget struct {
	FileName    string
	RepoOwner   string
	RepoName    string
	WorkingDir  string
	DisplayPath string
	BackURL     string
	EditURL     string
	LoadURL     string
	AceMode     string
	FilePath    string
	FileSize    int64
}

func RepositoryLiveEditTextFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	sessionUser := SS.GetString(r.Context(), "name")

	if !isLoggedIn || sessionUser == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	target := newLiveEditTarget(r)
	statusCode := http.StatusOK
	err := hydrateLiveEditTarget(target, sessionUser)
	if err != nil {
		if status, ok := err.(liveEditHTTPError); ok {
			statusCode = status.StatusCode
			err = status.Err
		} else {
			statusCode = http.StatusInternalServerError
		}
	}

	if r.URL.Query().Get("content") == "1" {
		if err != nil {
			writeJSON(w, statusCode, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		serveLiveEditContent(w, target)
		return
	}

	params := viewBackend.FrontEndParams{
		Title:               fmt.Sprintf("Live Edit - %s", target.FileName),
		Name:                sessionUser,
		Error:               make(map[string]string),
		Path:                target.WorkingDir,
		EditorFileName:      target.FileName,
		EditorFilePath:      target.DisplayPath,
		EditorRepoOwner:     target.RepoOwner,
		EditorRepoName:      target.RepoName,
		EditorBackURL:       target.BackURL,
		EditorLoadURL:       target.LoadURL,
		EditorMode:          target.AceMode,
		EditorFileSize:      target.FileSize,
		EditorFileSizeLimit: liveEditMaxFileSize,
		EditorEditable:      err == nil && target.FileSize <= liveEditMaxFileSize,
	}

	if err != nil {
		params.Error["general"] = err.Error()
	} else if target.FileSize > liveEditMaxFileSize {
		params.Error["general"] = fmt.Sprintf(
			"This file is %d bytes. Live editing currently supports files up to %d bytes (5 MB).",
			target.FileSize,
			liveEditMaxFileSize,
		)
	}

	if len(params.Error) > 0 {
		w.WriteHeader(statusCode)
	}

	if renderErr := viewBackend.LiveEditTextFile(w, params); renderErr != nil {
		http.Error(w, fmt.Sprintf("Failed to render live editor: %s", renderErr), http.StatusInternalServerError)
	}
}

type liveEditHTTPError struct {
	StatusCode int
	Err        error
}

func (e liveEditHTTPError) Error() string {
	return e.Err.Error()
}

func newLiveEditTarget(r *http.Request) *liveEditTarget {
	fileName := strings.TrimSpace(chi.URLParam(r, "filename"))
	repoOwner := strings.TrimSpace(chi.URLParam(r, "user"))
	repoName := strings.TrimSpace(chi.URLParam(r, "reponame"))
	workingDir := normalizeBrowserPath(chi.URLParam(r, "*"))
	displayPath := workingDir
	if fileName != "" {
		displayPath = normalizeBrowserPath(path.Join(workingDir, fileName))
	}
	editURL := buildEditRoutePath(fileName, repoOwner, repoName, workingDir)

	return &liveEditTarget{
		FileName:    fileName,
		RepoOwner:   repoOwner,
		RepoName:    repoName,
		WorkingDir:  workingDir,
		DisplayPath: displayPath,
		BackURL:     buildBrowseRoutePath(repoOwner, repoName, workingDir),
		EditURL:     editURL,
		LoadURL:     editURL + "?content=1",
		AceMode:     detectAceMode(fileName),
	}
}

func hydrateLiveEditTarget(target *liveEditTarget, sessionUser string) error {
	if target.RepoOwner == "" || target.RepoName == "" || target.FileName == "" {
		return liveEditHTTPError{
			StatusCode: http.StatusBadRequest,
			Err:        fmt.Errorf("Repository owner, repository name, and file name are required."),
		}
	}

	if target.RepoOwner != sessionUser {
		meta, err := repoStore.LoadRepoMeta(target.RepoOwner, target.RepoName)
		if err != nil {
			return liveEditHTTPError{
				StatusCode: http.StatusInternalServerError,
				Err:        fmt.Errorf("Failed to verify edit access: %w", err),
			}
		}

		allowed := false
		if meta != nil {
			for _, owner := range meta.Owners {
				if owner == sessionUser {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			return liveEditHTTPError{
				StatusCode: http.StatusForbidden,
				Err:        fmt.Errorf("You do not have permission to edit this file."),
			}
		}
	}

	filePath, err := repoStore.GetItemPath(target.RepoOwner, target.RepoName, target.WorkingDir, target.FileName)
	if err != nil {
		return liveEditHTTPError{
			StatusCode: http.StatusBadRequest,
			Err:        fmt.Errorf("Invalid file path requested."),
		}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return liveEditHTTPError{
				StatusCode: http.StatusNotFound,
				Err:        fmt.Errorf("The requested file could not be found."),
			}
		}
		return liveEditHTTPError{
			StatusCode: http.StatusInternalServerError,
			Err:        fmt.Errorf("Failed to open file metadata: %w", err),
		}
	}

	if !info.Mode().IsRegular() {
		return liveEditHTTPError{
			StatusCode: http.StatusBadRequest,
			Err:        fmt.Errorf("Only regular files can be opened in live edit."),
		}
	}

	target.FilePath = filePath
	target.FileSize = info.Size()

	return nil
}

func serveLiveEditContent(w http.ResponseWriter, target *liveEditTarget) {
	if target.FileSize > liveEditMaxFileSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("File exceeds the %d byte live-edit limit.", liveEditMaxFileSize),
		})
		return
	}

	data, err := os.ReadFile(target.FilePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to read file: %s", err),
		})
		return
	}

	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]interface{}{
			"success": false,
			"error":   "This file is not UTF-8 text, so it cannot be opened in the text editor yet.",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"content": string(data),
		"mode":    target.AceMode,
		"path":    target.DisplayPath,
		"size":    target.FileSize,
	})
}

func buildEditRoutePath(fileName string, userName string, repoName string, rawPath string) string {
	base := fmt.Sprintf(
		"/edit/%s/%s/%s",
		url.PathEscape(fileName),
		url.PathEscape(userName),
		url.PathEscape(repoName),
	)
	safePath := normalizeBrowserPath(rawPath)
	if safePath == "/" {
		return base
	}

	segments := strings.Split(strings.Trim(safePath, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return base + "/" + strings.Join(segments, "/")
}

func detectAceMode(fileName string) string {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	switch lowerName {
	case "dockerfile":
		return "dockerfile"
	case "makefile", "gnumakefile":
		return "makefile"
	}

	if strings.HasPrefix(lowerName, ".env") {
		return "dotenv"
	}
	if strings.HasPrefix(lowerName, ".gitignore") {
		return "gitignore"
	}

	switch strings.ToLower(path.Ext(lowerName)) {
	case ".go":
		return "golang"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".jsx":
		return "jsx"
	case ".html", ".htm", ".xhtml", ".tmpl":
		return "html"
	case ".css", ".scss", ".sass":
		return "css"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".py":
		return "python"
	case ".sh", ".bash", ".zsh":
		return "sh"
	case ".yml", ".yaml":
		return "yaml"
	case ".xml", ".svg":
		return "xml"
	case ".sql":
		return "sql"
	case ".toml":
		return "toml"
	case ".ini", ".cfg", ".conf":
		return "ini"
	case ".java":
		return "java"
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp":
		return "c_cpp"
	case ".rs":
		return "rust"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".lua":
		return "lua"
	default:
		return "text"
	}
}
