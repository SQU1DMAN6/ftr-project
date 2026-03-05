package repository

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"inkdrop/config"
	repo "inkdrop/repository"

	"github.com/go-chi/chi/v5"
)

func prepareEnv(t *testing.T) {
	tmp := t.TempDir()
	repo.GlobalInkDropRepoDir = filepath.Join(tmp, "userRepositories")
	if err := os.MkdirAll(filepath.Join(repo.GlobalInkDropRepoDir, "testuser", "testrepo"), 0755); err != nil {
		t.Fatalf("failed to make repo directory: %v", err)
	}
	config.InitSession()
}

func TestRepositoryCreateNewDirectory_Redirect(t *testing.T) {
	prepareEnv(t)
	SS := config.GetSessionManager()

	form := url.Values{}
	form.Set("folderName", "abc")
	form.Set("repository", "testrepo")
	form.Set("working-directory", "/foo/")

	req := httptest.NewRequest(http.MethodPost, "/new/dir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx, _ := SS.Load(req.Context(), "")
	SS.Put(ctx, "isLoggedIn", true)
	SS.Put(ctx, "name", "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	RepositoryCreateNewDirectory(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/testuser/testrepo/foo" {
		t.Errorf("redirect location wrong: %s", loc)
	}
}

func TestRepositoryCreateNewDirectory_SpacesBecomeUnderscore(t *testing.T) {
	prepareEnv(t)
	SS := config.GetSessionManager()
	base := filepath.Join(repo.GlobalInkDropRepoDir, "testuser", "testrepo")

	form := url.Values{}
	form.Set("folderName", "my folder")
	form.Set("repository", "testrepo")
	form.Set("working-directory", "/")

	req := httptest.NewRequest(http.MethodPost, "/new/dir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx, _ := SS.Load(req.Context(), "")
	SS.Put(ctx, "isLoggedIn", true)
	SS.Put(ctx, "name", "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	RepositoryCreateNewDirectory(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 got %d", rr.Code)
	}
	if rr.Header().Get("Location") != "/testuser/testrepo" {
		t.Fatalf("expected redirect to working directory root, got %s", rr.Header().Get("Location"))
	}
	if _, err := os.Stat(filepath.Join(base, "my_folder")); err != nil {
		t.Fatalf("expected folder with underscore, got stat error: %v", err)
	}
}

func TestRepositoryRenameItem_Success(t *testing.T) {
	prepareEnv(t)
	base := filepath.Join(repo.GlobalInkDropRepoDir, "testuser", "testrepo")
	os.MkdirAll(filepath.Join(base, "oldname"), 0755)

	SS := config.GetSessionManager()
	form := url.Values{}
	form.Set("oldName", "oldname")
	form.Set("newName", "newname")
	form.Set("repository", "testrepo")
	form.Set("working-directory", "/")

	req := httptest.NewRequest(http.MethodPost, "/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx, _ := SS.Load(req.Context(), "")
	SS.Put(ctx, "isLoggedIn", true)
	SS.Put(ctx, "name", "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	RepositoryRenameItem(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on rename, got %d", rr.Code)
	}
	if _, err := os.Stat(filepath.Join(base, "newname")); err != nil {
		t.Errorf("rename did not occur: %v", err)
	}
}

func TestRepositoryDeleteItem_Success(t *testing.T) {
	prepareEnv(t)
	base := filepath.Join(repo.GlobalInkDropRepoDir, "testuser", "testrepo")
	target := filepath.Join(base, "deleteme")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("failed to create test folder: %v", err)
	}

	SS := config.GetSessionManager()
	form := url.Values{}
	form.Set("itemName", "deleteme")
	form.Set("repository", "testrepo")
	form.Set("working-directory", "/")

	req := httptest.NewRequest(http.MethodPost, "/delete/item", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx, _ := SS.Load(req.Context(), "")
	SS.Put(ctx, "isLoggedIn", true)
	SS.Put(ctx, "name", "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	RepositoryDeleteItem(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on delete, got %d", rr.Code)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("folder still exists after delete")
	}
}

func TestRepositoryRenameItem_RejectsTraversal(t *testing.T) {
	prepareEnv(t)
	base := filepath.Join(repo.GlobalInkDropRepoDir, "testuser", "testrepo")
	if err := os.MkdirAll(filepath.Join(base, "safe"), 0755); err != nil {
		t.Fatalf("failed to make source folder: %v", err)
	}

	SS := config.GetSessionManager()
	form := url.Values{}
	form.Set("oldName", "safe")
	form.Set("newName", "../escape")
	form.Set("repository", "testrepo")
	form.Set("working-directory", "/")

	req := httptest.NewRequest(http.MethodPost, "/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx, _ := SS.Load(req.Context(), "")
	SS.Put(ctx, "isLoggedIn", true)
	SS.Put(ctx, "name", "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	RepositoryRenameItem(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for traversal rename, got %d", rr.Code)
	}
}

func TestBrowseShowsParentEntryOutsideRoot(t *testing.T) {
	prepareEnv(t)
	base := filepath.Join(repo.GlobalInkDropRepoDir, "testuser", "testrepo", "folderA")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("failed to make nested folder: %v", err)
	}

	router := chi.NewRouter()
	router.Get("/{user}/{reponame}/*", IndexMainBrowseRepository)
	router.Get("/{user}/{reponame}", IndexMainBrowseRepository)

	SS := config.GetSessionManager()
	req := httptest.NewRequest(http.MethodGet, "/testuser/testrepo/folderA", nil)
	ctx, _ := SS.Load(req.Context(), "")
	SS.Put(ctx, "isLoggedIn", true)
	SS.Put(ctx, "name", "testuser")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "data-kind=\"parent\"") {
		t.Fatalf("expected parent entry '..' in browse output")
	}
}
