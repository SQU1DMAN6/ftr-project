package config

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

var sessionManager *scs.SessionManager

func InitSession() {
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * 90 * time.Hour
	sessionManager.Cookie.Name = "PHPSESSID"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.SameSite = http.SameSiteStrictMode
	sessionManager.Cookie.Secure = false
}

func GetSessionManager() *scs.SessionManager {
	return sessionManager
}
