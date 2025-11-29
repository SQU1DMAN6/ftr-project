package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

const (
	BaseURL     = "https://quanthai.net"
	RepoURL     = BaseURL + "/inkdrop/repos"
	InkDropPath = "/inkdrop"
)

type Client struct {
	http      *http.Client
	sessionID string
	email     string
	username  string
	configDir string
}

func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	if os.Geteuid() == 0 {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			if u, err := user.Lookup(sudoUser); err == nil {
				home = u.HomeDir
			}
		}
	}
	configDir := filepath.Join(home, ".config", "ftr")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	client := &Client{
		http: &http.Client{
			Jar:     jar,
			Timeout: 120 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		configDir: configDir,
	}

	// Try to load existing session
	if err := client.loadSession(); err == nil {
		// Pre-populate cookie jar with saved session
		baseURLParsed, err := url.Parse(BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse base URL: %w", err)
		}

		// Session loaded

		jar.SetCookies(baseURLParsed, []*http.Cookie{{
			Name:     "PHPSESSID",
			Value:    client.sessionID,
			Path:     "/",
			Domain:   baseURLParsed.Hostname(),
			Secure:   true,
			HttpOnly: true,
		}})

		// Also load user info if available
		_ = client.loadUserInfo()

		// Cookies and user info restored into jar

		return client, nil
	}

	return client, nil
}

func (c *Client) loadSession() error {
	sessionFile := filepath.Join(c.configDir, "session")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return err
	}
	c.sessionID = strings.TrimSpace(string(data))
	return nil
}

func (c *Client) saveSession() error {
	sessionFile := filepath.Join(c.configDir, "session")
	return os.WriteFile(sessionFile, []byte(c.sessionID), 0600)
}

func (c *Client) saveUserInfo(email, username string) error {
	c.email = email
	c.username = username

	emailFile := filepath.Join(c.configDir, "email")
	if err := os.WriteFile(emailFile, []byte(email), 0600); err != nil {
		return err
	}

	usernameFile := filepath.Join(c.configDir, "username")
	if err := os.WriteFile(usernameFile, []byte(username), 0600); err != nil {
		return err
	}

	return nil
}

func (c *Client) loadUserInfo() error {
	emailFile := filepath.Join(c.configDir, "email")
	email, err := os.ReadFile(emailFile)
	if err != nil {
		return err
	}
	c.email = strings.TrimSpace(string(email))

	usernameFile := filepath.Join(c.configDir, "username")
	username, err := os.ReadFile(usernameFile)
	if err != nil {
		return err
	}
	c.username = strings.TrimSpace(string(username))

	return nil
}

func (c *Client) SearchRepos(query string) ([]map[string]string, error) {
	searchURL := fmt.Sprintf("%s%s/index.php?search=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(query))
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	// Identify as FtR CLI so server may return API JSON
	req.Header.Set("X-FTR-CLIENT", "FtR-CLI")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s - %s", resp.Status, string(body))
	}

	var apiResp map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}
	// If server returned HTML (likely the login page), return a helpful error
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && trimmed[0] == '<' {
		snippet := string(trimmed)
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return nil, fmt.Errorf("search API not available: server returned HTML (likely login page). Try logging in or upgrade the server. Response snippet: %s", snippet)
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		snippet := string(body)
		if len(snippet) > 1024 {
			snippet = snippet[:1024]
		}
		return nil, fmt.Errorf("failed to parse search response: %w - response snippet: %s", err, snippet)
	}

	out := []map[string]string{}
	if ok, _ := apiResp["success"].(bool); !ok {
		return out, nil
	}
	if matches, ok := apiResp["matches"].([]interface{}); ok {
		for _, m := range matches {
			if mm, ok := m.(map[string]interface{}); ok {
				item := make(map[string]string)
				if u, ok := mm["user"].(string); ok {
					item["user"] = u
				}
				if r, ok := mm["repo"].(string); ok {
					item["repo"] = r
				}
				if d, ok := mm["description"].(string); ok {
					item["description"] = d
				}
				out = append(out, item)
			}
		}
	}
	return out, nil
}

func (c *Client) IsLoggedIn() bool {
	return c.sessionID != ""
}

func (c *Client) GetSessionInfo() (email, username string) {
	return c.email, c.username
}

func (c *Client) Login(email, password string) error {
	log.Printf("Attempting to log in as %s", email)
	// Initialize base URL for cookies
	baseURLParsed, err := url.Parse(BaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Send login credentials using an explicit request so we can set headers
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)

	loginURL := BaseURL + "/login.php"
	req, err := http.NewRequest("POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	// Typical browser-like headers to avoid server-side filtering
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.121 Safari/537.36")
	req.Header.Set("Referer", BaseURL+"/login.php")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body to check for errors
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if login failed by looking for error message in HTML
	if bytes.Contains(body, []byte("Error logging in")) {
		log.Println("Login failed: server response contained 'Error logging in'.")
		return fmt.Errorf("invalid credentials")
	}

	// Parse Set-Cookie headers explicitly and normalize attributes before
	// storing into the cookie jar. This ensures Domain/Path/Secure are set so
	// the cookie will be sent on subsequent requests.
	foundSession := false
	for _, sc := range resp.Header["Set-Cookie"] {
		parts := strings.Split(sc, ";")
		if len(parts) == 0 {
			continue
		}
		nv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
		if len(nv) != 2 {
			continue
		}
		name := nv[0]
		value := nv[1]
		if name != "PHPSESSID" {
			continue
		}
		cookie := &http.Cookie{Name: name, Value: value}
		for _, attr := range parts[1:] {
			attr = strings.TrimSpace(attr)
			if strings.EqualFold(attr, "secure") {
				cookie.Secure = true
				continue
			}
			if strings.EqualFold(attr, "httponly") {
				cookie.HttpOnly = true
				continue
			}
			if strings.HasPrefix(strings.ToLower(attr), "domain=") {
				cookie.Domain = strings.TrimPrefix(attr, "Domain=")
				cookie.Domain = strings.TrimPrefix(cookie.Domain, "domain=")
				cookie.Domain = strings.TrimSpace(cookie.Domain)
				continue
			}
			if strings.HasPrefix(strings.ToLower(attr), "path=") {
				cookie.Path = strings.TrimPrefix(attr, "Path=")
				cookie.Path = strings.TrimPrefix(cookie.Path, "path=")
				cookie.Path = strings.TrimSpace(cookie.Path)
				continue
			}
		}
		if cookie.Domain == "" {
			cookie.Domain = baseURLParsed.Hostname()
		}
		if cookie.Path == "" {
			cookie.Path = "/"
		}
		if !cookie.Secure && baseURLParsed.Scheme == "https" {
			cookie.Secure = true
		}
		c.http.Jar.SetCookies(baseURLParsed, []*http.Cookie{cookie})
		c.sessionID = cookie.Value
		if err := c.saveSession(); err != nil {
			fmt.Println("Warning: Failed to save session")
		}
		log.Printf("Found and saved PHPSESSID: %s", c.sessionID)
		foundSession = true
		break
	}

	if !foundSession {
		for _, cookie := range c.http.Jar.Cookies(baseURLParsed) {
			if cookie.Name == "PHPSESSID" {
				if cookie.Domain == "" {
					cookie.Domain = baseURLParsed.Hostname()
				}
				if cookie.Path == "" {
					cookie.Path = "/"
				}
				c.sessionID = cookie.Value
				if err := c.saveSession(); err != nil {
					fmt.Println("Warning: Failed to save session")
				}
				foundSession = true
				break
			}
		}
	}

	// Verify session by accessing index.php in inkdrop
	log.Println("Verifying new session by accessing index.php...")
	verifyReq, err := http.NewRequest("GET", BaseURL+InkDropPath+"/index.php", nil)
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}
	if c.sessionID != "" {
		cookieHeader := "PHPSESSID=" + c.sessionID
		verifyReq.Header.Set("Cookie", cookieHeader)
	}

	verifyResp, err := c.http.Do(verifyReq)
	if err != nil {
		return fmt.Errorf("failed to verify session: %w", err)
	}
	verifyBody, _ := io.ReadAll(verifyResp.Body)
	verifyResp.Body.Close()

	if bytes.Contains(verifyBody, []byte("Login to access FtR services")) {
		log.Println("Session verification failed: server redirected to login page.")
		return fmt.Errorf("session verification failed")
	}

	// Extract username from "Logged in as <b>username</b>" in the response
	var username string
	if idx := bytes.Index(verifyBody, []byte("Logged in as")); idx != -1 {
		start := idx + len("Logged in as")
		if bidx := bytes.Index(verifyBody[start:], []byte("<b>")); bidx != -1 {
			bstart := start + bidx + len("<b>")
			if bidx2 := bytes.Index(verifyBody[bstart:], []byte("</b>")); bidx2 != -1 {
				username = string(verifyBody[bstart : bstart+bidx2])
				username = strings.TrimSpace(username)
				log.Printf("Successfully extracted username from page: %s", username)
			}
		}
	}

	// Save user info (email and username). Always persist to avoid transient missing session info.
	if err := c.saveUserInfo(email, username); err != nil {
		log.Printf("Warning: Failed to save user info: %v", err)
		fmt.Println("Warning: Failed to save user info")
	}

	// Set expiration time of session cookie to 90 days
	for _, cookie := range c.http.Jar.Cookies(baseURLParsed) {
		if cookie.Name == "PHPSESSID" {
			cookie.MaxAge = 60 * 60 * 24 * 90
			cookie.Expires = time.Now().Add(time.Hour * 24 * 90)
			c.http.Jar.SetCookies(baseURLParsed, []*http.Cookie{cookie})
			break
		}
	}

	return nil
}

func (c *Client) clearSession() error {
	log.Println("Clearing local session data...")
	c.sessionID = ""
	c.email = ""
	c.username = ""
	_ = os.Remove(filepath.Join(c.configDir, "session"))
	_ = os.Remove(filepath.Join(c.configDir, "email"))
	_ = os.Remove(filepath.Join(c.configDir, "username"))
	return nil
}

func (c *Client) Logout() error {
	log.Println("Logging out.")
	return c.clearSession()
}
