package login

import (
	"fmt"
	"inkdrop/config"
	userModel "inkdrop/model"
	viewBackend "inkdrop/view/connector"
	"net/http"
	"strings"
)

func LoginMain(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	name := SS.GetString(r.Context(), "name")
	isloggedin := SS.GetBool(r.Context(), "isLoggedIn")
	if name != "" && isloggedin == true {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	p := viewBackend.FrontEndParams{
		Title:   "Login",
		Message: "Log in to an existing FtR account",
		Error:   make(map[string]string),
	}

	if err := viewBackend.LoginMain(w, p); err != nil {
		// if the template fails, propagate a generic error
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func LoginMainPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form entry: %s", err), http.StatusBadRequest)
		return
	}

	userEmail := strings.TrimSpace(r.FormValue("email"))
	userPassword := strings.TrimSpace(r.FormValue("password"))

	if userEmail == "" || userPassword == "" {
		paramData := viewBackend.FrontEndParams{
			Title:   "Login",
			Message: "Log into an existing FtR account",
			Error:   make(map[string]string),
		}

		paramData.Error["general"] = "Username or email and password are required."

		if err := viewBackend.LoginMain(w, paramData); err != nil {
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
		}
		return
	}

	//future work: csrf https://github.com/gorilla/csrf

	fmt.Println("User tried to log in:", userEmail)

	db := config.GetDB()

	user, err := userModel.CheckPassword(db, userEmail, userPassword)
	if err != nil {
		fmt.Println("Error:", err)
		paramData := viewBackend.FrontEndParams{
			Title:   "Login",
			Message: "Log into an existing FtR account",
			Error:   make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Error logging in: %s", err)

		if err2 := viewBackend.LoginMain(w, paramData); err2 != nil {
			http.Error(w, "Failed to render page", http.StatusInternalServerError)
		}
		return
	}

	fmt.Printf("\nUser: %v | ID: %v | Email: %v\n", user.Name, user.ID, user.Email)
	SS := config.GetSessionManager()

	SS.Put(r.Context(), "email", user.Email)
	SS.Put(r.Context(), "name", user.Name)
	SS.Put(r.Context(), "isLoggedIn", true)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func LoginLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionManager := config.GetSessionManager()
	err := sessionManager.Destroy(r.Context())
	if err != nil {
		http.Error(w, "Failed to log out", http.StatusServiceUnavailable)
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
