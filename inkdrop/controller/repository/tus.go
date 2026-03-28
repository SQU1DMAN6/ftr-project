package repository

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
		uploadDir := fmt.Sprintf("%s/tus_temp", repository.RepoDir)
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
				id := event.Upload.ID
				meta := event.Upload.MetaData
				filename := meta["filename"]
				userName := meta["username"]
				repoName := meta["repository"]
				workingDir := meta["workingDirectory"]

				log.Printf("Upload complete: ID=%s Filename=%s User=%s Repo=%s Dir=%s\n",
					id, filename, userName, repoName, workingDir)

				if userName == "" || repoName == "" || filename == "" {
					log.Printf("ERROR: missing metadata for upload %s (user=%q repo=%q file=%q), skipping move",
						id, userName, repoName, filename)
					continue
				}

				tusFilePath := filepath.Join(uploadDir, id)
				err := repository.MoveUploadedFile(userName, repoName, workingDir, filename, tusFilePath)
				if err != nil {
					log.Printf("ERROR: failed to move upload %s to %s/%s%s/%s: %s",
						id, userName, repoName, workingDir, filename, err)
				} else {
					log.Printf("Moved upload %s -> %s/%s%s/%s",
						id, userName, repoName, workingDir, filename)
				}
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
