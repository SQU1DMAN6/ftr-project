package repository

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	viewBackend "inkdrop/view/connector"
	"net/http"
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
	}
	repoList, _ := repository.ListUserRepositories(userName)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form entry: %s", err), http.StatusBadRequest)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("reponame"))

	if repoName == "" {
		http.Error(w, "Repository name is required, but not provided", http.StatusBadRequest)
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
	}

	repoName := chi.URLParam(r, "reponame")
	userName := chi.URLParam(r, "user")
	path := chi.URLParam(r, "path")
	fmt.Println(repoName, userName, path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

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
		paramData.Message = "You do not own this repository. You cannot upload or make edits."
		paramData.UserOwnsRepository = true
	}

	directoryListing, err := repository.GetDirectoryListing(userName, repoName, path)
	if err != nil {
		paramData.Error["general"] = fmt.Sprintf("Failed to get directory listing of %s/%s%s: %s", userName, repoName, path, err)
	}
	if directoryListing == nil {
		paramData.Error["general"] = "The repository is empty. If you are the owner, consider uploading files."
	}
	paramData.RepoList = directoryListing

	fmt.Printf("User %s tried to access repository %s/%s%s", name, userName, repoName, path)
	viewBackend.IndexMainBrowseRepository(w, paramData)
}
