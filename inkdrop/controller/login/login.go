package login

import (
	viewBackend "inkdrop/view/connector"
	"net/http"
)

func LoginMain(w http.ResponseWriter, r *http.Request) {
	p := viewBackend.FrontEndParams{
		Title:   "Login",
		Message: "Log in to an existing FtR account",
		Error:   make(map[string]string),
	}

	viewBackend.LoginMain(w, p)
}
