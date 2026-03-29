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
	r.Post("/deleterepo", repository.DeleteRepository)
	r.Get("/{user}/{reponame}/*", repository.IndexMainBrowseRepository)
	r.Get("/{user}/{reponame}", repository.IndexMainBrowseRepository)
	r.Post("/new/dir", repository.RepositoryCreateNewDirectory)
	r.Post("/rename", repository.RepositoryRenameItem)
	r.Post("/delete/item", repository.RepositoryDeleteItem)
	r.Post("/download", repository.RepositoryDownloadFile)
	r.Get("/downloadrepo/{reponame}", repository.RepositoryDownloadRepositoryAsSQAR)
}

// NewTUSHandler returns the TUS upload handler wrapped with http.StripPrefix.
// It must be mounted outside the main chi router because the TUS protocol
// requires trailing slashes, which conflicts with StripSlashes middleware.
func NewTUSHandler() http.Handler {
	tusH := repository.TUSHandler()
	return http.StripPrefix("/upload", tusH)
}
