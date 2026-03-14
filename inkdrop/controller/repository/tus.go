package repository

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/repository"
	"net/http"
	"os"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

func RepositoryUploadFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// if !strings.HasPrefix(workingDir, "/") {
	// 	workingDir = "/" + workingDir
	// }

	// Handle TUS Upload For FtR (TUFF)

	// var uploadDirRemote string = fmt.Sprintf("%s/%s/%s%s", repository.GlobalInkDropRepoDir, userName, repoName, workingDir)
	uploadDirRemote := fmt.Sprintf("%s/%s/Test_Repository/", repository.GlobalInkDropRepoDir, userName)
	fmt.Println("debuging, folder to upload: ", uploadDirRemote)

	err := os.MkdirAll(uploadDirRemote, 0755)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create destination upload directory :%s", err), http.StatusServiceUnavailable)
		fmt.Printf("Failed to create destination upload directory: %s\n", err)
		return
	}

	store := filestore.New(uploadDirRemote)
	locker := filelocker.New(uploadDirRemote)

	composer := tusd.NewStoreComposer()
	store.UseIn(composer)
	locker.UseIn(composer)

	tusHandler, err := tusd.NewHandler(tusd.Config{
		BasePath:                "/upload/",
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		RespectForwardedHeaders: true,
	})

	fmt.Println("debuging, tus handler created")

	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to craete TUS handler: %v", err), http.StatusServiceUnavailable)
		fmt.Printf("Unable to create TUS handler: %v\n", err)
		return
	}

	go func() {
		for event := range tusHandler.CompleteUploads {
			fmt.Printf("Upload complete: ID '%s', Size '%d bytes', Filename '%s'",
				event.Upload.ID,
				event.Upload.Size,
				event.Upload.MetaData["filename"],
			)

		}
		fmt.Println("debugging, upload complete.")
	}()
}
