package routes

import (
	"inkdrop/controller/login"
	"inkdrop/controller/register"
	"inkdrop/controller/repository"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router) {
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is up"))
	})

	r.Get("/", repository.IndexMain)
	r.Get("/login", login.LoginMain)
	r.Post("/login", login.LoginMainPost)
	r.Get("/register", register.RegisterMain)
	r.Post("/register", register.RegisterMainPost)
	// r.Get("/successregister", register.SuccessRegister)
}
