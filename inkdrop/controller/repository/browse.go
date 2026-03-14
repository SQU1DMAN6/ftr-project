package repository

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	viewBackend "inkdrop/view/connector"
	"net/http"
	"path"
	"regexp"
	"slices"
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
	if r.Method != http.MethodGet {
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

	repoName := chi.URLParam(r, "reponame")
	err := repository.DeleteUserRepository(userName, repoName)
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

	fmt.Printf("User %s tried to access repository %s/%s%s", name, userName, repoName, path)
	viewBackend.IndexMainBrowseRepository(w, paramData)
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
	itemName := strings.TrimSpace(r.FormValue("itemName"))
	if itemName == "" || itemName == ".." {
		http.Error(w, "Invalid item for deletion.", http.StatusBadRequest)
		return
	}

	err = repository.DeleteItem(userName, repoName, workingDir, itemName)
	if err != nil {
		http.Error(w, "Delete failed. Try again later.", http.StatusServiceUnavailable)
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
