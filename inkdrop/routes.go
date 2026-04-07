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
	r.Get("/logout", login.LoginLogout)
	r.Get("/logout.php", login.LoginLogout)
	r.Post("/login.php", login.LoginMainPost)
	r.Get("/login.php", login.LoginMain)
	r.Post("/inkdrop/login.php", login.LoginMainPost)
	r.Get("/inkdrop/login.php", login.LoginMain)
	r.Get("/sessionconfirm", repository.SessionConfirm)
	r.Get("/inkdrop/sessionconfirm", repository.SessionConfirm)
	r.Get("/index.php", repository.RepositoryIndex)
	r.Get("/inkdrop/index.php", repository.RepositoryIndex)
	r.Get("/repo.php", repository.RepositoryAPI)
	r.Post("/repo.php", repository.RepositoryAPI)
	r.Get("/inkdrop/repo.php", repository.RepositoryAPI)
	r.Post("/inkdrop/repo.php", repository.RepositoryAPI)
	r.Post("/deleterepo", repository.DeleteRepository)
	r.Get("/{user}/{reponame}/*", repository.IndexMainBrowseRepository)
	r.Get("/{user}/{reponame}", repository.IndexMainBrowseRepository)
	r.Post("/new/dir", repository.RepositoryCreateNewDirectory)
	r.Post("/rename", repository.RepositoryRenameItem)
	r.Post("/delete/item", repository.RepositoryDeleteItem)
	r.Post("/repo/settings", repository.RepositorySettings)
	r.Post("/download", repository.RepositoryDownloadFile)
	r.Get("/preview", repository.RepositoryPreviewFile)
	r.Get("/downloadrepo/{user}/{reponame}", repository.RepositoryDownloadRepositoryAsSQAR)
}

func NewTUSHandler() http.Handler {
	tusH := repository.TUSHandler()
	return http.StripPrefix("/upload", tusH)
}
