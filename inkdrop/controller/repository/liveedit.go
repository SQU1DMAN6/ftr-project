package repository

import (
	"encoding/json"
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
	"time"

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
	StreamURL   string
	SourceURL   string
	AceMode     string
	FilePath    string
	FileSize    int64
}

type liveEditMutationRequest struct {
	Op        string             `json:"op"`
	ClientID  string             `json:"clientId"`
	Content   string             `json:"content"`
	Version   int64              `json:"version"`
	Selection *liveEditSelection `json:"selection,omitempty"`
	SavePath  string             `json:"savePath,omitempty"`
	Overwrite bool               `json:"overwrite,omitempty"`
}

type liveEditMutationResponse struct {
	Success   bool               `json:"success"`
	Version   int64              `json:"version,omitempty"`
	Path      string             `json:"path,omitempty"`
	Mode      string             `json:"mode,omitempty"`
	SavedAt   int64              `json:"savedAt,omitempty"`
	Presence  []liveEditPresence `json:"presence,omitempty"`
	EditURL   string             `json:"editURL,omitempty"`
	Message   string             `json:"message,omitempty"`
	Error     string             `json:"error,omitempty"`
	Timestamp int64              `json:"timestamp,omitempty"`
}

func RepositoryLiveEditTextFile(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	sessionUser := SS.GetString(r.Context(), "name")
	target := newLiveEditTarget(r)

	if !isLoggedIn || sessionUser == "" {
		if r.Method == http.MethodGet && !isLiveEditAPIRequest(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"error":   "authentication required",
		})
		return
	}

	if useDocumentEditor(target, sessionUser) {
		switch r.Method {
		case http.MethodGet:
			handleDocumentEditGet(w, r, target, sessionUser)
		case http.MethodPost:
			handleDocumentEditMutation(w, r, target, sessionUser)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		handleLiveEditGet(w, r, sessionUser)
	case http.MethodPost:
		handleLiveEditMutation(w, r, sessionUser)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func useDocumentEditor(target *liveEditTarget, sessionUser string) bool {
	if target == nil {
		return false
	}

	if err := hydrateLiveEditTarget(target, sessionUser); err == nil {
		kind, detectErr := repoStore.DetectEditableFileKind(target.FilePath)
		if detectErr == nil {
			return kind == repoStore.EditableFileKindDocument
		}
	}

	return isDocumentEditorTarget(target)
}

func handleLiveEditGet(w http.ResponseWriter, r *http.Request, sessionUser string) {
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

	if r.URL.Query().Get("events") == "1" {
		if err != nil {
			http.Error(w, err.Error(), statusCode)
			return
		}
		streamLiveEditEvents(w, r, target, sessionUser)
		return
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

	var initialContent string
	var initialVersion int64
	var hasInitialContent bool
	var snapshotErr error
	if err == nil && target.FileSize <= liveEditMaxFileSize {
		var snapshot liveEditEvent
		snapshot, snapshotErr = liveEditSessions.Snapshot(target)
		if snapshotErr == nil {
			initialContent = snapshot.Content
			initialVersion = snapshot.Version
			hasInitialContent = true
		}
	}

	params := viewBackend.FrontEndParams{
		Title:                   fmt.Sprintf("Live Edit - %s", target.FileName),
		Name:                    sessionUser,
		Error:                   make(map[string]string),
		Path:                    target.WorkingDir,
		EditorFileName:          target.FileName,
		EditorFilePath:          target.DisplayPath,
		EditorRepoOwner:         target.RepoOwner,
		EditorRepoName:          target.RepoName,
		EditorBackURL:           target.BackURL,
		EditorLoadURL:           target.LoadURL,
		EditorSyncURL:           target.EditURL,
		EditorStreamURL:         target.StreamURL,
		EditorMode:              target.AceMode,
		EditorFileSize:          target.FileSize,
		EditorFileSizeLimit:     liveEditMaxFileSize,
		EditorEditable:          err == nil && snapshotErr == nil && target.FileSize <= liveEditMaxFileSize,
		EditorHasInitialContent: hasInitialContent,
		EditorInitialContent:    initialContent,
		EditorInitialVersion:    initialVersion,
	}

	if err != nil {
		params.Error["general"] = err.Error()
	} else if errors.Is(snapshotErr, errLiveEditNonText) {
		params.Error["general"] = "This file is not UTF-8 text, so it cannot be opened in the live editor yet."
	} else if snapshotErr != nil {
		params.Error["general"] = fmt.Sprintf("Failed to open the editor document: %s", snapshotErr)
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

func handleLiveEditMutation(w http.ResponseWriter, r *http.Request, sessionUser string) {
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
		writeJSON(w, statusCode, liveEditMutationResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, liveEditMaxFileSize+(1<<20))
	var req liveEditMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, liveEditMutationResponse{
			Success: false,
			Error:   "invalid live-edit request payload",
		})
		return
	}

	req.Op = strings.ToLower(strings.TrimSpace(req.Op))
	req.ClientID = strings.TrimSpace(req.ClientID)
	if req.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, liveEditMutationResponse{
			Success: false,
			Error:   "clientId is required",
		})
		return
	}

	if req.Op == "" {
		req.Op = "sync"
	}

	switch req.Op {
	case "presence":
		event, err := liveEditSessions.UpdatePresence(target, req.ClientID, sessionUser, req.Selection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, liveEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, liveEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Presence:  event.Presence,
			Timestamp: event.Timestamp,
		})
	case "leave":
		event := liveEditSessions.Leave(target, req.ClientID)
		writeJSON(w, http.StatusOK, liveEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Presence:  event.Presence,
			Timestamp: event.Timestamp,
		})
	case "sync", "save":
		if int64(len([]byte(req.Content))) > liveEditMaxFileSize {
			writeJSON(w, http.StatusRequestEntityTooLarge, liveEditMutationResponse{
				Success: false,
				Error:   fmt.Sprintf("file exceeds the %d byte live-edit limit", liveEditMaxFileSize),
			})
			return
		}

		event, err := liveEditSessions.UpdateContent(target, req.ClientID, sessionUser, req.Content, req.Selection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, liveEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, liveEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Path:      event.Path,
			Mode:      event.Mode,
			SavedAt:   event.SavedAt,
			Presence:  event.Presence,
			Message:   "saved",
			Timestamp: event.Timestamp,
		})
	case "save_as":
		if int64(len([]byte(req.Content))) > liveEditMaxFileSize {
			writeJSON(w, http.StatusRequestEntityTooLarge, liveEditMutationResponse{
				Success: false,
				Error:   fmt.Sprintf("file exceeds the %d byte live-edit limit", liveEditMaxFileSize),
			})
			return
		}

		savePath := normalizeBrowserPath(req.SavePath)
		if savePath == "/" {
			writeJSON(w, http.StatusBadRequest, liveEditMutationResponse{
				Success: false,
				Error:   "save path is required",
			})
			return
		}

		if savePath == target.DisplayPath {
			event, err := liveEditSessions.UpdateContent(target, req.ClientID, sessionUser, req.Content, req.Selection)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, liveEditMutationResponse{
					Success: false,
					Error:   err.Error(),
				})
				return
			}
			writeJSON(w, http.StatusOK, liveEditMutationResponse{
				Success:   true,
				Version:   event.Version,
				Path:      event.Path,
				Mode:      event.Mode,
				SavedAt:   event.SavedAt,
				Presence:  event.Presence,
				EditURL:   target.EditURL,
				Message:   "saved",
				Timestamp: event.Timestamp,
			})
			return
		}

		filePath, err := repoStore.WriteTextFileAtRepoPath(target.RepoOwner, target.RepoName, savePath, []byte(req.Content), req.Overwrite)
		if err != nil {
			if errors.Is(err, repoStore.ErrItemExists) {
				writeJSON(w, http.StatusConflict, liveEditMutationResponse{
					Success: false,
					Error:   "a file already exists at that path",
				})
				return
			}
			writeJSON(w, http.StatusBadRequest, liveEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		savedTarget := buildLiveEditTargetForRepoPath(target.RepoOwner, target.RepoName, savePath, filePath)
		event, err := liveEditSessions.UpdateContent(savedTarget, req.ClientID, sessionUser, req.Content, req.Selection)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, liveEditMutationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, liveEditMutationResponse{
			Success:   true,
			Version:   event.Version,
			Path:      event.Path,
			Mode:      event.Mode,
			SavedAt:   event.SavedAt,
			Presence:  event.Presence,
			EditURL:   savedTarget.EditURL,
			Message:   "saved_as",
			Timestamp: event.Timestamp,
		})
	default:
		writeJSON(w, http.StatusBadRequest, liveEditMutationResponse{
			Success: false,
			Error:   "unsupported live-edit operation",
		})
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
		StreamURL:   editURL + "?events=1",
		SourceURL:   buildPreviewRoutePath(repoOwner, repoName, workingDir, fileName),
		AceMode:     detectAceMode(fileName),
	}
}

func buildLiveEditTargetForRepoPath(repoOwner string, repoName string, repoPath string, filePath string) *liveEditTarget {
	displayPath := normalizeBrowserPath(repoPath)
	fileName := path.Base(displayPath)
	workingDir := normalizeBrowserPath(path.Dir(displayPath))
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
		StreamURL:   editURL + "?events=1",
		SourceURL:   buildPreviewRoutePath(repoOwner, repoName, workingDir, fileName),
		AceMode:     detectAceMode(fileName),
		FilePath:    filePath,
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

	fileInfo, err := os.Stat(filePath)
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
	if !fileInfo.Mode().IsRegular() {
		return liveEditHTTPError{
			StatusCode: http.StatusBadRequest,
			Err:        fmt.Errorf("Only regular files can be opened in live edit."),
		}
	}

	target.FilePath = filePath
	target.FileSize = fileInfo.Size()

	return nil
}

func streamLiveEditEvents(w http.ResponseWriter, r *http.Request, target *liveEditTarget, sessionUser string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming is not supported in this environment.", http.StatusInternalServerError)
		return
	}

	clientID := strings.TrimSpace(r.URL.Query().Get("client"))
	if clientID == "" {
		http.Error(w, "client query parameter is required", http.StatusBadRequest)
		return
	}

	_, events, cancel, err := liveEditSessions.Subscribe(target, clientID, sessionUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cancel()

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func serveLiveEditContent(w http.ResponseWriter, target *liveEditTarget) {
	if target.FileSize > liveEditMaxFileSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("File exceeds the %d byte live-edit limit.", liveEditMaxFileSize),
		})
		return
	}

	snapshot, err := liveEditSessions.Snapshot(target)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, errLiveEditNonText) {
			statusCode = http.StatusUnsupportedMediaType
		}
		writeJSON(w, statusCode, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"content": snapshot.Content,
		"mode":    snapshot.Mode,
		"path":    snapshot.Path,
		"size":    len([]byte(snapshot.Content)),
		"version": snapshot.Version,
		"savedAt": snapshot.SavedAt,
	})
}

func isLiveEditAPIRequest(r *http.Request) bool {
	return r.Method != http.MethodGet ||
		r.URL.Query().Get("content") == "1" ||
		r.URL.Query().Get("events") == "1"
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

func buildPreviewRoutePath(userName string, repoName string, workingDir string, itemName string) string {
	query := url.Values{}
	query.Set("user", userName)
	query.Set("repository", repoName)
	query.Set("working-directory", normalizeBrowserPath(workingDir))
	query.Set("itemName", itemName)
	return "/preview?" + query.Encode()
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
