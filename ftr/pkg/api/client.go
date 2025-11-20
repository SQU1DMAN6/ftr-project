package api

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"ftr/pkg/screen"
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
	BaseURL     = "https://quanthai.net"
	RepoURL     = BaseURL + "/inkdrop/repos"
	InkDropPath = "/inkdrop"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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
			Timeout: 30 * time.Second,
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

		if os.Getenv("FTR_DEBUG") == "1" {
			fmt.Printf("DEBUG NewClient: Loaded sessionID=%s (len=%d)\n", client.sessionID, len(client.sessionID))
		}

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
	c.email = string(email)

	usernameFile := filepath.Join(c.configDir, "username")
	username, err := os.ReadFile(usernameFile)
	if err != nil {
		return err
	}
	c.username = string(username)

	return nil
}

func (c *Client) GetSessionInfo() (email, username string) {
	return c.email, c.username
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

	// Verify session by accessing index.php in inkdrop
	// Build a verification request and explicitly include the PHPSESSID cookie
	// as a header to ensure it is sent to the server (this helps isolate
	// whether the cookie jar matching is the issue).
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

	if bytes.Contains(verifyBody, []byte("Login with an existing InkDrop account")) {
		return fmt.Errorf("session verification failed")
	}

	// Extract username from "Logged in as <b>username</b>" in the response
	var username string
	if idx := bytes.Index(verifyBody, []byte("Logged in as")); idx != -1 {
		// Look for the pattern after "Logged in as"
		start := idx + len("Logged in as")
		// Find the opening <b> tag
		if bidx := bytes.Index(verifyBody[start:], []byte("<b>")); bidx != -1 {
			bstart := start + bidx + len("<b>")
			// Find the closing </b> tag
			if bidx2 := bytes.Index(verifyBody[bstart:], []byte("</b>")); bidx2 != -1 {
				username = string(verifyBody[bstart : bstart+bidx2])
				username = strings.TrimSpace(username)
			}
		}
	}

	// Save user info (email and username)
	if username != "" {
		if err := c.saveUserInfo(email, username); err != nil {
			fmt.Println("Warning: Failed to save user info")
		}
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

	// Debug: Print session info
	if os.Getenv("FTR_DEBUG") == "1" {
		fmt.Printf("DEBUG UploadFile: sessionID=%s (len=%d)\n", c.sessionID, len(c.sessionID))
		fmt.Printf("DEBUG UploadFile: email=%s, username=%s\n", c.email, c.username)
		fmt.Printf("DEBUG UploadFile: repoPath=%s, user=%s, repoName=%s\n", repoPath, user, repoName)
	}

	// Ensure cookie jar contains our session cookie so automatic requests
	// will include it. Some environments may not persist cookie attrs, so
	// re-add it to the jar before making any requests.
	baseURLParsed, _ := url.Parse(BaseURL)
	if c.sessionID != "" {
		c.http.Jar.SetCookies(baseURLParsed, []*http.Cookie{{
			Name:     "PHPSESSID",
			Value:    c.sessionID,
			Path:     "/",
			Domain:   baseURLParsed.Hostname(),
			Secure:   true,
			HttpOnly: true,
		}})
	}

	// First verify our session is still valid and get current user info
	req, err := http.NewRequest("GET", BaseURL+InkDropPath+"/index.php", nil)
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}
	// Also set Cookie header explicitly to be resilient in environments where
	// the cookie jar might not be consulted for some requests.
	if c.sessionID != "" {
		req.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("session verification failed: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Debug: Check verification response
	if os.Getenv("FTR_DEBUG") == "1" {
		if bytes.Contains(body, []byte("Logged in as")) {
			fmt.Printf("DEBUG: Verification passed - found 'Logged in as' in response\n")
		} else {
			fmt.Printf("DEBUG: Verification FAILED - 'Logged in as' NOT found in response\n")
			if bytes.Contains(body, []byte("Login with an existing")) {
				fmt.Printf("DEBUG: Got redirected to login page - session is invalid!\n")
			}
		}
	}

	// If we got redirected to login, our session is invalid
	if bytes.Contains(body, []byte("Login with an existing InkDrop account")) {
		return fmt.Errorf("session expired - please login again")
	}

	// Extract current logged-in user from the response for debugging
	// Look for "Logged in as: <username>" or similar pattern
	var loggedInUser string
	if idx := bytes.Index(body, []byte("Logged in as")); idx != -1 {
		// Try to extract the username (this is a best effort)
		start := idx + len("Logged in as")
		if end := bytes.Index(body[start:], []byte("<")); end != -1 {
			loggedInUser = strings.TrimSpace(string(body[start : start+end]))
			loggedInUser = strings.TrimPrefix(loggedInUser, ":")
			loggedInUser = strings.TrimSpace(loggedInUser)
		}
	}

	// Now try to access or create the repo
	repoReq, err := http.NewRequest("GET", fmt.Sprintf("%s%s/repo.php?name=%s&user=%s", BaseURL, InkDropPath, url.QueryEscape(repoName), url.QueryEscape(user)), nil)
	if err != nil {
		return fmt.Errorf("failed to create repo check request: %w", err)
	}
	if c.sessionID != "" {
		repoReq.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}
	resp, err = c.http.Do(repoReq)
	if err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}

	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	// If repo doesn't exist, create it with our first upload
	if bytes.Contains(body, []byte("repository is not found")) {
		if loggedInUser != "" && loggedInUser != user {
			return fmt.Errorf("repository does not exist and you are not the owner (logged in as '%s', trying to upload to '%s')", loggedInUser, user)
		}
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

	var fileSize int64
	if rs, ok := reader.(io.ReadSeeker); ok {
		cur, _ := rs.Seek(0, io.SeekCurrent)
		end, _ := rs.Seek(0, io.SeekEnd)
		fileSize = end - cur
		rs.Seek(cur, io.SeekStart)
	}

	pr := &screen.ProgressReader{
		R:     reader,
		Total: fileSize,
		Start: time.Now(),
	}

	// Compute SHA-256 while uploading
	hasher := sha256.New()
	tr := io.TeeReader(pr, hasher)

	if _, err := io.Copy(fw, tr); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	fmt.Println()
	screen.ClearProgressBar()

	w.Close()

	// Create request to repo.php with appropriate query parameters
	// Use api=1 to request JSON response from CLI uploads
	uploadURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repoName), url.QueryEscape(user))
	uploadReq, err := http.NewRequest("POST", uploadURL, &b)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())

	// Ensure PHP session cookie is sent even if the cookie jar did not
	// correctly attach it (some environments may not persist cookie attrs).
	if c.sessionID != "" {
		uploadReq.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}

	// Send request
	resp, err = c.http.Do(uploadReq)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response to check for success/failure message
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Debug: Print response for debugging
	if os.Getenv("FTR_DEBUG") == "1" {
		fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)
		fmt.Printf("DEBUG: Response body (first 500 chars): %s\n", string(body[:min(len(body), 500)]))
	}

	// Try to parse JSON response (API response)
	var apiResp map[string]interface{}
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if success, ok := apiResp["success"].(bool); ok {
			if success {
				// If server returned a hash, compare with our computed hash
				serverHash := ""
				if h, ok := apiResp["hash"].(string); ok {
					serverHash = h
				}
				computed := fmt.Sprintf("%x", hasher.Sum(nil))
				if serverHash != "" && !strings.EqualFold(serverHash, computed) {
					return fmt.Errorf("upload succeeded but integrity mismatch: server=%s local=%s", serverHash, computed)
				}
				return nil // Success case
			}
			// Error response
			errMsg := "upload failed - server error"
			if msg, ok := apiResp["message"].(string); ok {
				errMsg = msg
			}

			// Add debug info if available
			if debug, ok := apiResp["debug"].(map[string]interface{}); ok {
				dLogged := ""
				dOwner := ""
				if v, ok := debug["logged_in_as"].(string); ok {
					dLogged = v
				}
				if v, ok := debug["repository_owner"].(string); ok {
					dOwner = v
				}
				if dLogged != "" || dOwner != "" {
					errMsg = fmt.Sprintf("upload failed: %s (logged in as '%s', repository owner is '%s')", errMsg, dLogged, dOwner)
				}
			}

			// If debug is enabled, print full server response to help diagnose.
			if os.Getenv("FTR_DEBUG") == "1" {
				fmt.Printf("DEBUG: Server JSON response: %s\n", string(body))
			}

			return fmt.Errorf("%s", errMsg)
		}
	} else {
		if os.Getenv("FTR_DEBUG") == "1" {
			fmt.Printf("DEBUG: Non-JSON response from server: %s\n", string(body))
		}
	}

	// Fallback for HTML responses (shouldn't happen with api=1, but just in case)
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
		return fmt.Errorf("upload failed - not authorized to upload to this repository. Note your session may have expired, please login again")
	}

	return fmt.Errorf("upload failed - unexpected response from server")
}

func (c *Client) DownloadFile(downloadURL string, fileName string) (io.ReadCloser, error) {
	resp, err := c.http.Get(downloadURL)

	size := resp.ContentLength

	pr := &screen.ProgressReader{
		R:     resp.Body,
		Total: size,
		Start: time.Now(),
	}

	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status: %s", resp.Status)
	}

	screen.ClearProgressBar()

	return io.NopCloser(pr), nil
}

// GetFileMeta attempts to retrieve file metadata (hash/signature) via the API.
// This endpoint may not be implemented on the server; the function gracefully
// returns nil map if metadata is not available.
func (c *Client) GetFileMeta(user, repo, fileName string) (map[string]string, error) {
	metaURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&filemeta=1&file=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user), url.QueryEscape(fileName))
	resp, err := c.http.Get(metaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try parse JSON error
		body, _ := io.ReadAll(resp.Body)
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err == nil {
			if msg, ok := apiResp["message"].(string); ok {
				return nil, fmt.Errorf("server error: %s", msg)
			}
		}
		return nil, fmt.Errorf("metadata request failed: %s", resp.Status)
	}

	var apiResp map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata response: %w", err)
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		// Not JSON - return nil to indicate metadata not available
		return nil, nil
	}

	out := make(map[string]string)
	if h, ok := apiResp["hash"].(string); ok {
		out["hash"] = h
	}
	if s, ok := apiResp["signature"].(string); ok {
		out["signature"] = s
	}
	if e, ok := apiResp["encrypted"]; ok {
		switch v := e.(type) {
		case bool:
			if v {
				out["encrypted"] = "1"
			} else {
				out["encrypted"] = "0"
			}
		case string:
			out["encrypted"] = v
		case float64:
			if v != 0 {
				out["encrypted"] = "1"
			} else {
				out["encrypted"] = "0"
			}
		}
	}
	return out, nil
}

// DownloadAndVerify downloads a file from a repository via the repo.php API,
// optionally verifies the SHA-256 hash if metadata is available, and saves to destPath.
func (c *Client) DownloadAndVerify(repoPath string, fileName string, destPath string) error {
	parts := strings.Split(repoPath, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository path. Must be in format user/repo")
	}
	user, repo := parts[0], parts[1]

	// Try to get metadata first
	meta, _ := c.GetFileMeta(user, repo, fileName)
	expectedHash := ""
	if meta != nil {
		expectedHash = meta["hash"]
	}

	downloadURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&download=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user), url.QueryEscape(fileName))
	resp, err := c.http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try parse JSON error
		body, _ := io.ReadAll(resp.Body)
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err == nil {
			if msg, ok := apiResp["message"].(string); ok {
				// Check for malware detection error
				if strings.Contains(msg, "Suspicious") || strings.Contains(msg, "Malicious") {
					// Alert user to potentially malicious code
					fmt.Printf("\n[WARNING] This package contains potentially malicious code:\n")
					fmt.Printf("  %s\n\n", msg)
					fmt.Printf("Proceed with download anyway? [y/N]: ")

					var response string
					fmt.Scanln(&response)

					if strings.ToLower(response) != "y" {
						return fmt.Errorf("download cancelled by user")
					}

					// User chose 'y', but we still can't proceed if server rejected it
					// Return the original error (file couldn't be stored due to malware)
					return fmt.Errorf("server rejected file due to malware: %s", msg)
				} else {
					return fmt.Errorf("server error: %s", msg)
				}
			}
		}
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Prepare destination file (we may overwrite after decryption)
	tmpPath := destPath + ".part"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	// Stream download with progress
	size := resp.ContentLength
	pr := &screen.ProgressReader{R: resp.Body, Total: size, Start: time.Now()}

	if _, err := io.Copy(outFile, pr); err != nil {
		outFile.Close()
		return fmt.Errorf("failed to save downloaded file: %w", err)
	}
	outFile.Close()

	// If file is encrypted, attempt to decrypt using local key store
	encrypted := false
	if meta != nil {
		if val, ok := meta["encrypted"]; ok && val == "1" {
			encrypted = true
		}
	}

	if encrypted {
		// Locate key: ~/.config/ftr/keys/{user}_{repo}.key
		keysDir := filepath.Join(c.configDir, "keys")
		keyFile := filepath.Join(keysDir, fmt.Sprintf("%s_%s.key", user, repo))
		keyHex := ""
		if data, err := os.ReadFile(keyFile); err == nil {
			keyHex = strings.TrimSpace(string(data))
		} else {
			return fmt.Errorf("repository content is encrypted; no decryption key found at %s. Ask the repo owner to provide the key or place it there", keyFile)
		}

		// Read encrypted payload
		encData, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to read downloaded encrypted file: %w", err)
		}

		// decrypt format is hex(iv) ':' hex(ciphertext)
		plaintext, err := decryptHexPayload(encData, keyHex)
		if err != nil {
			return fmt.Errorf("failed to decrypt payload: %w", err)
		}

		// Verify hash against expectedHash if available
		if expectedHash != "" {
			computed := fmt.Sprintf("%x", sha256.Sum256(plaintext))
			if !strings.EqualFold(expectedHash, computed) {
				return fmt.Errorf("integrity check failed after decryption: expected %s computed %s", expectedHash, computed)
			}
		}

		// Write plaintext to final path
		if err := os.WriteFile(destPath, plaintext, 0644); err != nil {
			return fmt.Errorf("failed to write decrypted file: %w", err)
		}
		// Remove temp
		_ = os.Remove(tmpPath)
		return nil
	}

	// Not encrypted: verify hash of downloaded file
	fdata, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read downloaded file for verification: %w", err)
	}
	computed := fmt.Sprintf("%x", sha256.Sum256(fdata))
	if expectedHash != "" && !strings.EqualFold(expectedHash, computed) {
		return fmt.Errorf("integrity check failed: expected %s computed %s", expectedHash, computed)
	}
	// Move temp to dest
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to finalize downloaded file: %w", err)
	}
	return nil
}

// decryptHexPayload expects data like "<iv_hex>:<cipher_hex>" and a key in hex
func decryptHexPayload(payload []byte, keyHex string) ([]byte, error) {
	parts := strings.SplitN(string(payload), ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid encrypted payload format")
	}
	iv, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid IV encoding: %w", err)
	}
	cipherBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext encoding: %w", err)
	}
	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return nil, fmt.Errorf("invalid key encoding: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: expected 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	if len(cipherBytes)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}
	if len(iv) != aes.BlockSize {
		return nil, errors.New("invalid IV length")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	out := make([]byte, len(cipherBytes))
	mode.CryptBlocks(out, cipherBytes)
	// PKCS7 unpad
	if len(out) == 0 {
		return nil, errors.New("decrypted payload empty")
	}
	pad := int(out[len(out)-1])
	if pad <= 0 || pad > aes.BlockSize || pad > len(out) {
		return nil, errors.New("invalid padding")
	}
	for i := 0; i < pad; i++ {
		if out[len(out)-1-i] != byte(pad) {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return out[:len(out)-pad], nil
}
