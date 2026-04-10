package app

import (
	routes "inkdrop"
	"inkdrop/config"
	"inkdrop/repository"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func BootApp() {
	r := chi.NewRouter()
	config.ConnectDatabase()
	config.InitSession()
	if err := repository.EnsureStorageLayout(); err != nil {
		log.Fatalf("failed to initialize InkDrop storage under %s: %v", repository.RootDir, err)
	}

	ss := config.GetSessionManager()
	r.Use(ss.LoadAndSave)

	RegisterMiddleWares(r)
	RegisterStatic(r)
	routes.RegisterRoutes(r)

	// The TUS upload handler is served outside the main chi router
	// because the TUS protocol requires trailing slashes in URLs,
	// which conflicts with the StripSlashes middleware on the main router.
	// We wrap it with the session manager so auth checks still work.
	tusHandler := ss.LoadAndSave(routes.NewTUSHandler())

	// Top-level dispatcher: route /upload* to the TUS handler,
	// everything else to the main chi router.
	top := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/upload") {
			tusHandler.ServeHTTP(w, req)
			return
		}
		r.ServeHTTP(w, req)
	})

	if err := http.ListenAndServe(":6767", top); err != nil {
		log.Fatal(err)
	}
}
