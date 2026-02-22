package repository

import (
	"inkdrop/config"
	viewBackend "inkdrop/view/connector"
	"net/http"
)

func IndexMain(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}

	p := viewBackend.FrontEndParams{
		Title:           "InkDrop Browser",
		Message:         "Browse the InkDrop machine",
		Name:            userName,
		IsViewingPublic: false,
		Error:           make(map[string]string),
	}

	viewBackend.IndexMain(w, p)
}
