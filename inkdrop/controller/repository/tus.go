package repository

import (
	"encoding/base64"
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
		uploadDir := filepath.Join(repository.TempDir, "tus")
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

		if r.Method == http.MethodPost {
			metaHeader := r.Header.Get("Upload-Metadata")
			if metaHeader != "" {
				metaMap := parseUploadMetadata(metaHeader)
				owner := metaMap["username"]
				repoName := metaMap["repository"]
				if owner == "" || repoName == "" {
					http.Error(w, "Invalid upload metadata: owner and repository required", http.StatusBadRequest)
					return
				}

				repoPath := filepath.Join(repository.RepoDir, owner, repoName)
				if ok, _ := repository.DirExists(repoPath); !ok {
					http.Error(w, fmt.Sprintf("Repository %s/%s not found", owner, repoName), http.StatusNotFound)
					return
				}

				sessionUser := SS.GetString(r.Context(), "name")
				if sessionUser != owner {
					if meta, err := repository.LoadRepoMeta(owner, repoName); err == nil && meta != nil {
						allowed := false
						for _, o := range meta.Owners {
							if o == sessionUser {
								allowed = true
								break
							}
						}
						if !allowed {
							http.Error(w, "Forbidden - you are not listed as an owner for this repository", http.StatusForbidden)
							return
						}
					} else {
						http.Error(w, "Forbidden - not the repository owner", http.StatusForbidden)
						return
					}
				}
			}
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
		if tusErr != nil {
			http.Error(w, tusErr.Error(), http.StatusInternalServerError)
			return
		}
		tusHandler.ServeHTTP(w, r)
	})
}

func parseUploadMetadata(header string) map[string]string {
	out := map[string]string{}
	parts := strings.Split(header, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, " ", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		valB64 := strings.TrimSpace(kv[1])
		if valB64 == "" {
			out[key] = ""
			continue
		}
		if decoded, err := base64.StdEncoding.DecodeString(valB64); err == nil {
			out[key] = string(decoded)
		} else {
			out[key] = ""
		}
	}
	return out
}
