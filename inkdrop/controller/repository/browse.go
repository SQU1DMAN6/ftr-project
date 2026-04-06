package repository

import (
	"encoding/json"
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	viewBackend "inkdrop/view/connector"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func IndexMain(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repoList, err := repository.ListUserRepositories(userName)

	p := viewBackend.FrontEndParams{
		Title:           "InkDrop Browser",
		Message:         "Browse the InkDrop machine",
		Name:            userName,
		IsViewingPublic: false,
		RepoList:        repoList,
		Error:           make(map[string]string),
	}

	if err != nil {
		p.Error["general"] = fmt.Sprintf("Failed to get repository listing: %s", err)
	}

	viewBackend.IndexMain(w, p)
}

func IndexMainPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	repoList, _ := repository.ListUserRepositories(userName)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form entry: %s", err), http.StatusBadRequest)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("reponame"))

	if repoName == "" {
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        repoList,
			Error:           make(map[string]string),
		}
		paramData.Error["general"] = "Repository name is required."
		viewBackend.IndexMain(w, paramData)
		return
	}

	repoNameCooked, err := repository.ProcessRepoName(repoName)
	if err != nil {
		fmt.Println("Error processing repository name:", err)
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        repoList,
			Error:           make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Error creating repository: %s\n", err)

		viewBackend.IndexMain(w, paramData)
		return
	}

	err = repository.CreateNewUserRepository(userName, repoNameCooked)
	if err != nil {
		fmt.Println("Error creating new user repository:", err)
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        repoList,
			Error:           make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Error creating repository: %s\n", err)

		viewBackend.IndexMain(w, paramData)
		return
	}

	fmt.Printf("New Repository Created: %s/%s\n", userName, repoNameCooked)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func DeleteRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	if isLoggedIn != true {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse repo delete form", http.StatusServiceUnavailable)
		return
	}
	repoName := r.FormValue("reponame")
	err = repository.DeleteUserRepository(userName, repoName)
	if err != nil {
		repoList, _ := repository.ListUserRepositories(userName)
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        repoList,
			Error:           make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Failed to delete repository %s/%s: %s", userName, repoName, err)
		fmt.Printf("Failed to delete repository %s/%s: %s", userName, repoName, err)
		viewBackend.IndexMain(w, paramData)
		return
	}

	fmt.Printf("Successfully removed %s/%s\n", userName, repoName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func IndexMainBrowseRepository(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	var userOwnsRepo bool

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	name := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || name == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repoName := chi.URLParam(r, "reponame")
	userName := chi.URLParam(r, "user")
	path := chi.URLParam(r, "*")
	fmt.Println(repoName, userName, path)
	path = normalizeBrowserPath(path)

	if name == userName {
		userOwnsRepo = true
	} else {
		userOwnsRepo = false
	}

	repoListing, err := repository.ListUserRepositories(userName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get repository listing of user %s: %s", userName, err), http.StatusServiceUnavailable)
		fmt.Printf("Failed to get directory listing of user %s: %s\n", userName, err)
		return
	}

	if !slices.Contains(repoListing, repoName) {
		fmt.Printf("User %s tried to access repository %s/%s, but was inaccessible.\n", name, userName, repoName)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	paramData := viewBackend.FrontEndParams{
		Title:    fmt.Sprintf("%s/%s - InkDrop", userName, repoName),
		Name:     name,
		Error:    make(map[string]string),
		Message2: userName,
		Message3: repoName,
		Path:     path,
	}

	if userOwnsRepo == true {
		paramData.Message = fmt.Sprintf("Browsing your repository '%s'", repoName)
		paramData.UserOwnsRepository = true
	} else {
		paramData.Message = "You are viewing this repository in read-only mode."
		paramData.UserOwnsRepository = false
	}

	directoryListing, err := repository.GetDirectoryListing(userName, repoName, path)
	if err != nil {
		paramData.Error["general"] = fmt.Sprintf("Failed to get directory listing of %s/%s%s: %s", userName, repoName, path, err)
	}
	if directoryListing == nil {
		if path == "/" {
			paramData.Error["general"] = "The repository is empty. If you are the owner, consider uploading files."
		}
	}

	if path != "/" && err == nil {
		if directoryListing == nil {
			directoryListing = []string{}
		}
		directoryListing = append([]string{".."}, directoryListing...)
	}
	paramData.RepoList = directoryListing

	// Load repository metadata if present
	if meta, err := repository.LoadRepoMeta(userName, repoName); err == nil && meta != nil {
		paramData.RepoDescription = meta.Description
		paramData.RepoOwners = strings.Join(meta.Owners, ",")
		paramData.RepoPublic = meta.Public
	}

	fmt.Printf("User %s tried to access repository %s/%s%s", name, userName, repoName, path)
	viewBackend.IndexMainBrowseRepository(w, paramData)
}

// RepositorySettings handles updates to repository metadata (description, owners, permissions)
func RepositorySettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")
	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("repository"))
	if repoName == "" {
		http.Error(w, "Repository required", http.StatusBadRequest)
		return
	}

	// load existing meta and enforce ownership
	meta, _ := repository.LoadRepoMeta(userName, repoName)
	// if meta not present, treat the user who owns the repo (path owner) as authorized
	authorized := false
	if meta != nil {
		for _, o := range meta.Owners {
			if o == userName {
				authorized = true
				break
			}
		}
	} else {
		// fallback: if current session user equals repo owner (path owner), allow
		repoOwner := chi.URLParam(r, "user")
		if repoOwner == "" {
			repoOwner = userName
		}
		if repoOwner == userName {
			authorized = true
		}
	}

	if !authorized {
		http.Error(w, "Forbidden - not an owner", http.StatusForbidden)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	ownersRaw := strings.TrimSpace(r.FormValue("owners"))
	publicFlag := r.FormValue("public") == "1" || r.FormValue("public") == "on"

	// Validate owners; if none valid, keep existing owners
	owners, err := repository.ValidateOwners(ownersRaw)
	if err != nil {
		if meta != nil && len(meta.Owners) > 0 {
			owners = meta.Owners
		} else {
			// as last resort, make current user the owner
			owners = []string{userName}
		}
	}

	newMeta := &repository.RepoMeta{
		Owners:      owners,
		Description: description,
		Public:      publicFlag,
	}
	if err := repository.SaveRepoMeta(userName, repoName, newMeta); err != nil {
		http.Error(w, "Failed to save metadata", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/%s/%s", userName, repoName), http.StatusSeeOther)
}

func RepositoryCreateNewDirectory(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse user form. Try again.", http.StatusBadRequest)
		return
	}

	folderName := r.FormValue("folderName")
	repoName := r.FormValue("repository")
	workingDir := r.FormValue("working-directory")
	workingDir = normalizeBrowserPath(workingDir)
	folderName = strings.TrimSpace(folderName)
	folderName = strings.ReplaceAll(folderName, " ", "_")

	if pass, _ := regexp.MatchString("^[A-Za-z0-9_-]+$", folderName); !pass {
		http.Error(w, "Invalid folder name. Only letters, numbers, underscores and hyphens allowed.", http.StatusBadRequest)
		return
	}

	err = repository.CreateNewDirectory(userName, repoName, workingDir, folderName)
	if err != nil {
		http.Error(w, "Failed to create new folder. Go back and try again later.", http.StatusServiceUnavailable)
		return
	}

	fmt.Printf("User %s created a new folder: '%s' at '%s' at '%s'\n", userName, folderName, repoName, workingDir)

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	if wd == "" {
		http.Redirect(w, r, fmt.Sprintf("/%s/%s", userName, repoName), http.StatusSeeOther)
		return
	} else {
		http.Redirect(w, r, fmt.Sprintf("/%s/%s/%s", userName, repoName, wd), http.StatusSeeOther)
		return
	}
}

func RepositoryRenameItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	repoName := r.FormValue("repository")
	workingDir := r.FormValue("working-directory")
	oldName := r.FormValue("oldName")
	newName := r.FormValue("newName")
	workingDir = normalizeBrowserPath(workingDir)

	if oldName == "" || oldName == ".." {
		http.Error(w, "Invalid source name.", http.StatusBadRequest)
		return
	}
	if !isValidMovePath(newName) {
		http.Error(w, "Invalid destination path.", http.StatusBadRequest)
		return
	}

	err = repository.RenameItem(userName, repoName, workingDir, oldName, newName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Rename failed: %s", err), http.StatusServiceUnavailable)
		return
	}

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	var redirectPath string
	if wd == "" {
		redirectPath = fmt.Sprintf("/%s/%s", userName, repoName)
	} else {
		redirectPath = fmt.Sprintf("/%s/%s/%s", userName, repoName, wd)
	}
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

func RepositoryDeleteItem(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")
	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	repoName := r.FormValue("repository")
	workingDir := normalizeBrowserPath(r.FormValue("working-directory"))
	itemNames := r.Form["itemName"]
	if len(itemNames) == 0 {
		trimmed := strings.TrimSpace(r.FormValue("itemName"))
		if trimmed != "" {
			itemNames = []string{trimmed}
		}
	}
	if len(itemNames) == 0 {
		http.Error(w, "Invalid item for deletion.", http.StatusBadRequest)
		return
	}

	for _, itemName := range itemNames {
		itemName = strings.TrimSpace(itemName)
		if itemName == "" || itemName == ".." {
			http.Error(w, "Invalid item for deletion.", http.StatusBadRequest)
			return
		}
		err = repository.DeleteItem(userName, repoName, workingDir, itemName)
		if err != nil {
			http.Error(w, "Delete failed. Try again later.", http.StatusServiceUnavailable)
			return
		}
	}

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	var redirectPath string
	if wd == "" {
		redirectPath = fmt.Sprintf("/%s/%s", userName, repoName)
	} else {
		redirectPath = fmt.Sprintf("/%s/%s/%s", userName, repoName, wd)
	}
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

// func RepositoryUploadFiles(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	SS := config.GetSessionManager()
// 	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
// 	userName := SS.GetString(r.Context(), "name")
// 	if isLoggedIn != true || userName == "" {
// 		http.Redirect(w, r, "/", http.StatusSeeOther)
// 		return
// 	}

// 	err := r.ParseMultipartForm(5000 << 20)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Failed to parse multipart form: %s", err), http.StatusBadRequest)
// 		return
// 	}
// 	defer r.MultipartForm.RemoveAll()

// 	repoName := r.FormValue("repository")
// 	workingDir := r.FormValue("working-directory")

// 	files := r.MultipartForm.File["fileToUpload"]
// 	//WIP : because we upload multiple files, files is a slice of FileHeader

// 	// Creat /tmp folder and save files to it

// 	// for _, fileHeader := range files {
// 	// 	ext := filepath.Ext(fileHeader.Filename)
// 	// 	dst, err := os.CreateTemp("/tmp", "files-*"+ext)
// 	// 	if err != nil {
// 	// 		http.Error(w, fmt.Sprintf("Failed to create temp file: %s", err), http.StatusInternalServerError)
// 	// 		return
// 	// 	}
// 	// 	defer dst.Close()
// 	// 	src, err := fileHeader.Open()
// 	// 	if err != nil {
// 	// 		http.Error(w, fmt.Sprintf("Failed to open file: %s", err), http.StatusInternalServerError)
// 	// 		return
// 	// 	}
// 	// 	defer src.Close()
// 	// 	_, err = io.Copy(dst, src)
// 	// 	if err != nil {
// 	// 		http.Error(w, fmt.Sprintf("Failed to copy file: %s", err), http.StatusInternalServerError)
// 	// 		return
// 	// 	}
// 	// 	fmt.Printf("Saved %s to %s\n", fileHeader.Filename, dst.Name())
// 	// }

// 	for _, fileHeader := range files {
// 		fname := fileHeader.Filename
// 		var destination string = fmt.Sprintf("%s/%s/%s%s", repository.GlobalInkDropRepoDir, userName, repoName, workingDir)
// 		fmt.Println("Destination to copy files is", destination)
// 		dst, err := os.Create(destination + "/" + fname)
// 		if err != nil {
// 			http.Error(w, fmt.Sprintf("Failed to create temporary file: %s", err), http.StatusServiceUnavailable)
// 			return
// 		}
// 		defer dst.Close()
// 		src, err := fileHeader.Open()
// 		if err != nil {
// 			http.Error(w, fmt.Sprintf("Failed to open file %s: %s", fname, err), http.StatusServiceUnavailable)
// 			return
// 		}
// 		defer src.Close()
// 		_, err = io.Copy(dst, src)
// 		if err != nil {
// 			http.Error(w, fmt.Sprintf("Failed to copy file %s: %s", fname, err), http.StatusServiceUnavailable)
// 			return
// 		}

// 		fmt.Printf("Saved %s to %s\n", fname, dst.Name())
// 	}

// 	fmt.Printf("User '%s' upload to [ %s/%s%s ]\n", userName, userName, repoName, workingDir)
// 	fmt.Printf("files=[%v]\n", files)

// 	http.Redirect(w, r, fmt.Sprintf("/%s/%s%s", userName, repoName, workingDir), http.StatusSeeOther)
// }

func normalizeBrowserPath(raw string) string {
	if raw == "" {
		return "/"
	}
	clean := path.Clean("/" + raw)
	if clean == "." || clean == "" {
		return "/"
	}
	return clean
}

func isValidMovePath(raw string) bool {
	candidate := strings.TrimSpace(raw)
	if candidate == "" || strings.HasPrefix(candidate, "/") {
		return false
	}
	// if strings.Contains(candidate, "..") {
	// 	return false
	// }

	parts := strings.Split(candidate, "/")
	// for _, part := range parts {
	// 	if part == "" {
	// 		return false
	// 	}
	// 	// if pass, _ := regexp.MatchString("^[A-Za-z0-9_-]+$", part); !pass {
	// 	// 	return false
	// 	// }
	// }
	if slices.Contains(parts, "") {
		return false
	}
	return true
}

func RepositoryDownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse download form: %s", err), http.StatusServiceUnavailable)
		return
	}

	repoName := r.FormValue("repository")
	workingDir := r.FormValue("working-directory")
	itemName := r.FormValue("itemName")

	fmt.Printf("User %s tried to download %s at %s at %s\n", userName, itemName, workingDir, repoName)
	fileToDownload, err := repository.GetItemPath(userName, repoName, workingDir, itemName)
	if err != nil {
		http.Error(w, "Invalid file requested.", http.StatusBadRequest)
		return
	}

	file, err := os.Open(fileToDownload)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %s", err), http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		http.Error(w, "Requested item is not a downloadable file.", http.StatusBadRequest)
		return
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Unable to read file.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(itemName))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}

func RepositoryPreviewFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")

	if isLoggedIn != true || userName == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query()
	repoName := query.Get("repository")
	workingDir := query.Get("working-directory")
	itemName := query.Get("itemName")

	if itemName == "" {
		http.Error(w, "Invalid file requested.", http.StatusBadRequest)
		return
	}

	fileToPreview, err := repository.GetItemPath(userName, repoName, workingDir, itemName)
	if err != nil {
		http.Error(w, "Invalid file requested.", http.StatusBadRequest)
		return
	}

	file, err := os.Open(fileToPreview)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %s", err), http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		http.Error(w, "Requested item is not a previewable file.", http.StatusBadRequest)
		return
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Unable to read file.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func SessionConfirm(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	if SS.GetBool(r.Context(), "isLoggedIn") {
		w.Write([]byte("true"))
		return
	}
	w.Write([]byte("false"))
}

func RepositoryIndex(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	searchQuery := strings.TrimSpace(query.Get("search"))
	apiMode := query.Get("api") == "1"
	if apiMode && searchQuery != "" {
		matches, err := repository.SearchRepositories(searchQuery)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "matches": matches})
		return
	}
	IndexMain(w, r)
}

func RepositoryAPI(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	repoName := query.Get("name")
	userName := query.Get("user")
	if repoName == "" || userName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "user and repo are required"})
		return
	}

	if query.Get("list") == "1" {
		files, err := repository.ListRepositoryFilesRecursive(userName, repoName)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "files": files})
		return
	}

	if query.Get("filemeta") == "1" {
		fileName := query.Get("file")
		if fileName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "file parameter is required"})
			return
		}
		filePath, err := repository.GetItemPath(userName, repoName, "/", fileName)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid file path"})
			return
		}
		hash, err := repository.CalculateFileHash(filePath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to compute file metadata"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "hash": hash, "flagged": "0", "encrypted": "0", "signature": ""})
		return
	}

	if query.Get("meta") == "1" {
		m, err := repository.LoadRepoMeta(userName, repoName)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		if m == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "meta": map[string]interface{}{}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "meta": m})
		return
	}

	if downloadName := query.Get("download"); downloadName != "" {
		// allow both POST and GET for compatibility
		filePath, err := repository.GetItemPath(userName, repoName, "/", downloadName)
		if err != nil {
			http.Error(w, "Invalid file requested.", http.StatusBadRequest)
			return
		}
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "Failed to open file.", http.StatusNotFound)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil || !stat.Mode().IsRegular() {
			http.Error(w, "Requested item is not a downloadable file.", http.StatusBadRequest)
			return
		}

		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		contentType := http.DetectContentType(buf[:n])
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "Unable to read file.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(downloadName))
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(500 << 20); err == nil {
			file, header, err := r.FormFile("upload")
			if err == nil {
				defer file.Close()
				destDir := fmt.Sprintf("%s/%s/%s", repository.RepoDir, userName, repoName)
				if ok, _ := repository.DirExists(destDir); !ok {
					writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "repository does not exist"})
					return
				}
				destPath := fmt.Sprintf("%s/%s", destDir, path.Base(header.Filename))
				if !strings.HasPrefix(path.Clean(destPath), path.Clean(destDir)) {
					writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid file name"})
					return
				}
				out, err := os.Create(destPath)
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to save file"})
					return
				}
				defer out.Close()
				if _, err := io.Copy(out, file); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to save file"})
					return
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "upload complete"})
				return
			}
		}
	}

	writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "unsupported api operation"})
}

func RepositoryDownloadRepositoryAsSQAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repoName := chi.URLParam(r, "reponame")

	repoToDownload := fmt.Sprintf("%s/%s/%s", repository.RepoDir, userName, repoName)
	fmt.Printf("User %s tried to download repository at %s\n", userName, repoToDownload)

	archive, err := repository.PackSQAR(repoToDownload, userName, repoName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to pack archive into SQAR file: %s", err), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(repoName+".sqar"))
	http.ServeFile(w, r, archive)
	err = os.Remove(archive)
	if err != nil {
		fmt.Printf("Failed to cleanup file %s: %s\n", archive, err)
	}
}
