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
	r.Post("/", repository.IndexMainPost)
	r.Get("/login", login.LoginMain)
	r.Post("/login", login.LoginMainPost)
	r.Get("/register", register.RegisterMain)
	r.Post("/register", register.RegisterMainPost)
	r.Get("/logout", login.Logout)
	r.Get("/delete/{reponame}", repository.DeleteRepository)
	r.Get("/{user}/{reponame}/{path}", repository.IndexMainBrowseRepository)
	r.Get("/{user}/{reponame}", repository.IndexMainBrowseRepository)
	// r.Get("/successregister", register.SuccessRegister)
}
