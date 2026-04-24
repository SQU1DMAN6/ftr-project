package config

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
)

var sessionManager *scs.SessionManager

func InitSession() error {
	store, err := NewDiskSessionStore(
		sessionDatabasePath(),
		time.Minute,
	)
	if err != nil {
		return err
	}

	sessionManager = scs.New()
	sessionManager.Store = store
	sessionManager.Lifetime = 24 * 90 * time.Hour
	sessionManager.Cookie.Name = "PHPSESSID"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.SameSite = http.SameSiteStrictMode
	sessionManager.Cookie.Secure = false
	return nil
}

func GetSessionManager() *scs.SessionManager {
	return sessionManager
}

func sessionDatabasePath() string {
	rootDir := strings.TrimSpace(os.Getenv("FTR_ROOT_DIR"))
	if rootDir == "" {
		rootDir = "/srv/ftr"
		if runtime.GOOS == "darwin" {
			rootDir = "/srv/ftr"
		}
	}
	return filepath.Join(filepath.Clean(rootDir), "sessions", "sessions.db")
}
