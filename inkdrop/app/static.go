package app

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func RegisterStatic(r *chi.Mux) {

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	assetsPath := filepath.Join(workDir, "assets")

	fmt.Println("assetsPath", assetsPath)

	checkDirExists(assetsPath, "assets")

	fileServer := func(path string) http.Handler {
		fs := http.FileServer(http.Dir(path))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// if appconfig.InProduction {
			// 	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			// 	// Prevent cookies on static assets <-- only uses this option for /assets and /public, so no need to set cookies
			// 	// This is to prevent cookies from being sent with static file requests
			// 	// which can help with performance and security.
			// 	// Note: This is not a security measure, but rather a performance optimization.
			// 	w.Header().Del("Set-Cookie")
			// 	w.Header().Del("Vary")
			// 	w.Header().Set("Vary", "Accept-Encoding") // keep only encoding
			// } else {
			// 	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			// }
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			fs.ServeHTTP(w, r)
		})
	}

	// Handle static files in two folders
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer(assetsPath)))

}

func checkDirExists(path string, name string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("Warning: %s directory not found at %s", name, path)
	}
}
