package app

import (
	routes "inkdrop"
	"inkdrop/config"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func BootApp() {
	r := chi.NewRouter()
	config.ConnectDatabase()
	config.InitSession()

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

	http.ListenAndServe(":6767", top)
}
