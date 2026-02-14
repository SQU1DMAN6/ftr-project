package listrepos

import (
	"inkdrop/config"
	"net/http"
)

func IndexListUserRepostitories(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
