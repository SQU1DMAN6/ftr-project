package repository

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	"net/http"
	"os"
	"sync"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

var (
	tusOnce    sync.Once
	tusHandler *tusd.Handler
	tusErr     error
)

func TUSHandler() http.Handler {
	tusOnce.Do(func() {
		uploadDir := fmt.Sprintf("%s/tus_temp", repository.GlobalInkDropRepoDirMac)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			tusErr = err
			return
		}

		store := filestore.New(uploadDir)
		locker := filelocker.New(uploadDir)
		composer := tusd.NewStoreComposer()
		store.UseIn(composer)
		locker.UseIn(composer)

		h, err := tusd.NewHandler(tusd.Config{
			BasePath:                "/upload/",
			StoreComposer:           composer,
			NotifyCompleteUploads:   true,
			RespectForwardedHeaders: true,
		})
		if err != nil {
			tusErr = err
			return
		}

		go func() {
			for event := range h.CompleteUploads {
				fmt.Printf("Upload complete: ID=%s Filename=%s\n",
					event.Upload.ID,
					event.Upload.MetaData["filename"],
				)
			}
		}()

		tusHandler = h
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SS := config.GetSessionManager()
		if !SS.GetBool(r.Context(), "isLoggedIn") || SS.GetString(r.Context(), "name") == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if tusErr != nil {
			http.Error(w, tusErr.Error(), http.StatusInternalServerError)
			return
		}
		tusHandler.ServeHTTP(w, r)
	})
}
