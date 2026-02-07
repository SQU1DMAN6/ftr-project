package app

import (
	routes "inkdrop"
	"inkdrop/config"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func BootApp() {
	r := chi.NewRouter()
	config.ConnectDatabase()
	config.InitSession()

	r.Use(config.GetSessionManager().LoadAndSave)

	RegisterMiddleWares(r)
	routes.RegisterRoutes(r)

	http.ListenAndServe(":6767", r)
}
