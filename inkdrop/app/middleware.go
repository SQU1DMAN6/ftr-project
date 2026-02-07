package app

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func RegisterMiddleWares(r *chi.Mux) {
	r.Use(middleware.Logger)
	r.Use(middleware.StripSlashes)
}
