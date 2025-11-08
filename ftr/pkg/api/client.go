package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
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
	BaseURL = "https://quanthai.net"
	RepoURL = BaseURL + "/inkdrop/repos"
)

type Client struct {
	http      *http.Client
	sessionID string
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
			Jar: jar,
			// Keep a simple redirect limiter. Let the default transport and cookie jar
			// handle Set-Cookie headers so cookies get stored with proper domain/path
			// attributes.
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
		jar.SetCookies(baseURLParsed, []*http.Cookie{{
			Name:   "PHPSESSID",
			Value:  client.sessionID,
			Path:   "/",
			Domain: baseURLParsed.Hostname(),
		}})
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
	c.sessionID = string(data)
	return nil
}

func (c *Client) saveSession() error {
	sessionFile := filepath.Join(c.configDir, "session")
	return os.WriteFile(sessionFile, []byte(c.sessionID), 0600)
}

func (c *Client) Login(email, password string) error {
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
		return fmt.Errorf("invalid credentials")
	}

	// Parse Set-Cookie headers explicitly and normalize attributes before
	// storing into the cookie jar. This ensures Domain/Path/Secure are set so
	// the cookie will be sent on subsequent requests.
	foundSession := false
	// Parse Set-Cookie header strings manually. We only need to find the
	// PHPSESSID cookie and extract Domain/Path/Secure/HttpOnly if present.
	for _, sc := range resp.Header["Set-Cookie"] {
		parts := strings.Split(sc, ";")
		if len(parts) == 0 {
			continue
		}
		// first part should be NAME=VALUE
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
		foundSession = true
		break
	}

	// If not found in the Set-Cookie headers, check the cookie jar where the
	// transport may have stored cookies for redirects.
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

	// Verify session by accessing index.php
	// Build a verification request and explicitly include the PHPSESSID cookie
	// as a header to ensure it is sent to the server (this helps isolate
	// whether the cookie jar matching is the issue).
	verifyReq, err := http.NewRequest("GET", BaseURL+"/index.php", nil)
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
	verifyBody, err := io.ReadAll(verifyResp.Body)
	verifyResp.Body.Close()

	if bytes.Contains(verifyBody, []byte("Login with an existing InkDrop account")) {
		for k, v := range verifyResp.Header {
			fmt.Printf("  %s: %v\n", k, v)
		}
		// Print a short snippet of the body
		snippet := string(verifyBody)
		if len(snippet) > 1024 {
			snippet = snippet[:1024]
		}

		return fmt.Errorf("session verification failed")
	}

	// Set expiration time of session cookie to 90 days
	for _, cookie := range c.http.Jar.Cookies(baseURLParsed) {
		if cookie.Name == "PHPSESSID" {
			cookie.MaxAge = 60 * 60 * 24 * 90 // 90 days in seconds
			cookie.Expires = time.Now().Add(time.Hour * 24 * 90)
			c.http.Jar.SetCookies(baseURLParsed, []*http.Cookie{cookie})
			break
		}
	}

	return nil
}

func (c *Client) CreateRepo(user, repoName string) error {
	// The repository will be created automatically when we try to upload
	// Just verify we have the right permissions
	if user != os.Getenv("USER") {
		return fmt.Errorf("cannot create repository - not authorized")
	}
	return nil
}

func (c *Client) UploadFile(repoPath string, fileName string, reader io.Reader) error {
	if c.sessionID == "" {
		return fmt.Errorf("not logged in")
	}

	// Split user/repo
	parts := strings.Split(repoPath, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository path. Must be in format user/repo")
	}
	user, repoName := parts[0], parts[1]

	// First verify our session is still valid
	resp, err := c.http.Get(BaseURL + "/index.php")
	if err != nil {
		return fmt.Errorf("session verification failed: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	// If we got redirected to login, our session is invalid
	if bytes.Contains(body, []byte("Login with an existing InkDrop account")) {
		return fmt.Errorf("session expired - please login again")
	}

	// Now try to access or create the repo
	resp, err = c.http.Get(fmt.Sprintf("%s/repo.php?name=%s&user=%s", BaseURL, url.QueryEscape(repoName), url.QueryEscape(user)))
	if err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}

	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()

	// If repo doesn't exist, create it with our first upload
	if bytes.Contains(body, []byte("repository is not found")) {
		if os.Getenv("USER") != user {
			return fmt.Errorf("repository does not exist and you are not the owner")
		}
		// Repository will be created automatically by the upload
	}

	// Create multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add file
	fw, err := w.CreateFormFile("upload", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(fw, reader); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}
	w.Close()

	// Create request to repo.php with appropriate query parameters
	uploadURL := fmt.Sprintf("%s/repo.php?name=%s&user=%s", BaseURL, url.QueryEscape(repoName), url.QueryEscape(user))
	req, err := http.NewRequest("POST", uploadURL, &b)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Send request
	resp, err = c.http.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response to check for success/failure message
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Look for the success message in the response
	if bytes.Contains(body, []byte("color: #0f0")) && bytes.Contains(body, []byte("Uploaded")) {
		return nil // Success case
	}

	// Error cases
	if bytes.Contains(body, []byte("Failed to create repository")) {
		return fmt.Errorf("failed to create repository - permission denied")
	}

	if bytes.Contains(body, []byte("Upload failed")) || bytes.Contains(body, []byte("color: red")) {
		return fmt.Errorf("upload failed - server rejected the file")
	}

	if bytes.Contains(body, []byte("cannot upload")) || !bytes.Contains(body, []byte("uploadForm")) {
		return fmt.Errorf("upload failed - not authorized to upload to this repository")
	}

	return fmt.Errorf("upload failed - unexpected response from server")
}

func (c *Client) DownloadFile(downloadURL string, fileName string) (io.ReadCloser, error) {
	// Download from /inkdrop/repos/USER/REPO/filename

	resp, err := c.http.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status: %s", resp.Status)
	}

	return resp.Body, nil
}
