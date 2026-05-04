package api

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
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

	// On Windows, use %APPDATA%\FtR for config
	configDir := filepath.Join(os.Getenv("APPDATA"), "FtR")
	if configDir == "FtR" { // APPDATA not set
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, "AppData", "Roaming", "FtR")
	}
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
		return nil, fmt.Errorf("search API not available: server returned HTML (likely login page). Try logging in")
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

func (c *Client) getFileMeta(user, repo, fileName string) (map[string]string, error) {
	metaURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&filemeta=1&file=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user), url.QueryEscape(fileName))
	req, err := http.NewRequest("GET", metaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata request: %w", err)
	}

	req.Header.Set("X-FTR-CLIENT", "FtR-CLI")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
		return nil, nil
	}

	out := make(map[string]string)
	if h, ok := apiResp["hash"].(string); ok {
		out["hash"] = h
	}
	if s, ok := apiResp["signature"].(string); ok {
		out["signature"] = s
	}
	if f, ok := apiResp["flagged"]; ok {
		switch v := f.(type) {
		case bool:
			if v {
				out["flagged"] = "1"
			} else {
				out["flagged"] = "0"
			}
		case string:
			out["flagged"] = v
		}
	}
	if fn, ok := apiResp["flagged_note"].(string); ok {
		out["flagged_note"] = fn
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
	if ek, ok := apiResp["encryption_key"].(string); ok {
		out["encryption_key"] = ek
	}
	return out, nil
}

func (c *Client) DownloadAndVerify(user string, repo string, fileName string, destPath string, progress func(float64)) error {
	meta, _ := c.getFileMeta(user, repo, fileName)
	expectedHash := ""
	if meta != nil {
		expectedHash = meta["hash"]
	}

	downloadURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&download=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user), url.QueryEscape(fileName))
	// If metadata indicates the file was flagged during upload, warn the user.
	if meta != nil {
		if note, ok := meta["flagged_note"]; ok && note != "" {
			return fmt.Errorf("the file %s was flagged on upload. This means it is potentially malicious. Consider using the FtR CLI client if you truly want to download it. File was flagged: %s", fileName, note)
		}
	}

	req, err := http.NewRequest("POST", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	req.Header.Set("X-FTR-Client", "FtR-CLI")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var apiResp map[string]interface{}
		if err := json.Unmarshal(body, &apiResp); err == nil {
			if msg, ok := apiResp["message"].(string); ok {
				if strings.Contains(msg, "Suspicious") || strings.Contains(msg, "Malicious") {
					return fmt.Errorf("the file %s contains potentially malicious code. Consider using the FtR CLI client if you truly want to download it", fileName)
				} else {
					if strings.Contains(msg, "File not found") {
						return fmt.Errorf("file not found: %s", fileName)
					}
					return fmt.Errorf("server error: %s", msg)
				}
			}
		}
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	tmpPath := destPath + ".part"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	if resp.Body == nil {
		outFile.Close()
		return fmt.Errorf("download failed: empty response body")
	}

	size := resp.ContentLength
	progressReader := NewProgressReader(resp.Body, size, progress)

	if _, err := io.Copy(outFile, progressReader); err != nil {
		outFile.Close()
		return fmt.Errorf("failed to save downloaded file: %w", err)
	}
	// Close the file explicitly before further operations
	outFile.Close()

	encrypted := false
	if meta != nil {
		if val, ok := meta["encrypted"]; ok && val == "1" {
			encrypted = true
		}
	}

	if encrypted {
		keysDir := filepath.Join(c.configDir, "keys")
		// Try per-file key first, then repo-level key
		perFileKey := filepath.Join(keysDir, fmt.Sprintf("%s_%s_%s.key", user, repo, fileName))
		repoKey := filepath.Join(keysDir, fmt.Sprintf("%s_%s.key", user, repo))
		keyHex := ""
		if data, err := os.ReadFile(perFileKey); err == nil {
			keyHex = strings.TrimSpace(string(data))
		} else if data, err := os.ReadFile(repoKey); err == nil {
			keyHex = strings.TrimSpace(string(data))
		} else {
			// If server provided an encryption_key in metadata, persist it locally for next time
			if meta != nil {
				if ek, ok := meta["encryption_key"]; ok && ek != "" {
					keyHex = ek
					// write to perFileKey path
					_ = os.MkdirAll(keysDir, 0700)
					_ = os.WriteFile(perFileKey, []byte(keyHex), 0600)
				}
			}
		}
		if keyHex == "" {
			return fmt.Errorf("repository content is encrypted; no decryption key found at %s or %s. Ask the repo owner to provide the key or place it there", perFileKey, repoKey)
		}

		encData, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to read downloaded encrypted file: %w", err)
		}

		plaintext, err := c.decryptHexPayload(string(encData), keyHex)
		if err != nil {
			return fmt.Errorf("failed to decrypt payload: %w", err)
		}

		if expectedHash != "" {
			computed := fmt.Sprintf("%x", sha256.Sum256(plaintext))
			if !strings.EqualFold(expectedHash, computed) {
				return fmt.Errorf("integrity check failed after decryption: expected %s computed %s", expectedHash, computed)
			}
		}

		if err := os.WriteFile(destPath, plaintext, 0644); err != nil {
			return fmt.Errorf("failed to write decrypted file: %w", err)
		}
		os.Remove(tmpPath)
		return nil
	}

	// If not encrypted, verify hash if available
	if expectedHash != "" {
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to read downloaded file for integrity check: %w", err)
		}
		computed := fmt.Sprintf("%x", sha256.Sum256(data))
		if !strings.EqualFold(expectedHash, computed) {
			return fmt.Errorf("integrity check failed: expected %s computed %s", expectedHash, computed)
		}
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("failed to finalise downloaded file: %w", err)
	}

	return nil
}

func (c *Client) decryptHexPayload(s string, keyHex string) ([]byte, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid encrypted payload format")
	}
	ivHex := parts[0]
	encHex := parts[1]

	iv, err := hex.DecodeString(ivHex)
	if err != nil {
		return nil, fmt.Errorf("invalid iv hex: %w", err)
	}
	encrypted, err := hex.DecodeString(encHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted hex: %w", err)
	}

	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return nil, fmt.Errorf("invalid key hex: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	if len(encrypted)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	out := make([]byte, len(encrypted))
	mode.CryptBlocks(out, encrypted)

	// PKCS7 unpad
	if len(out) == 0 {
		return nil, errors.New("decryption resulted in empty payload")
	}
	pad := int(out[len(out)-1])
	if pad <= 0 || pad > aes.BlockSize {
		return nil, errors.New("invalid padding")
	}
	return out[:len(out)-pad], nil
}

// ProgressReader is a wrapper for io.Reader that reports progress.
type ProgressReader struct {
	io.Reader
	total    int64
	read     int64
	progress func(float64)
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	pr.read += int64(n)
	if pr.progress != nil && pr.total > 0 {
		pr.progress(float64(pr.read) / float64(pr.total))
	}
	return
}

func NewProgressReader(r io.Reader, size int64, progress func(float64)) *ProgressReader {
	return &ProgressReader{Reader: r, total: size, progress: progress}
}

// UploadFile uploads a file to a repository, with optional encryption and progress reporting.
func (c *Client) UploadFile(repoPath string, fileName string, reader io.Reader, size int64, encrypt bool, progress func(float64)) error {
	if c.sessionID == "" {
		return fmt.Errorf("not logged in")
	}

	parts := strings.Split(repoPath, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository path. Must be in format user/repo")
	}
	user, repoName := parts[0], parts[1]

	// First verify our session is still valid
	req, err := http.NewRequest("GET", BaseURL+InkDropPath+"/index.php", nil)
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}
	if c.sessionID != "" {
		req.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("session verification failed: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if bytes.Contains(body, []byte("Login with an existing InkDrop account")) {
		return fmt.Errorf("session expired - please login again")
	}

	// Build multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	progressReader := NewProgressReader(reader, size, progress)

	var computed string

	if encrypt {
		// Read full plaintext from the original reader, compute hash, encrypt, save key locally, and upload encrypted payload
		data, err := io.ReadAll(progressReader)
		if err != nil {
			return fmt.Errorf("failed to read file for encryption: %w", err)
		}

		// Compute plaintext hash
		h := sha256.Sum256(data)
		plaintextHash := fmt.Sprintf("%x", h)
		computed = plaintextHash

		// Generate AES-256 key
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("failed to generate encryption key: %w", err)
		}
		keyHex := hex.EncodeToString(key)

		// Encrypt with AES-256-CBC + PKCS7
		block, err := aes.NewCipher(key)
		if err != nil {
			return fmt.Errorf("failed to create cipher: %w", err)
		}
		iv := make([]byte, aes.BlockSize)
		if _, err := rand.Read(iv); err != nil {
			return fmt.Errorf("failed to create iv: %w", err)
		}

		pad := aes.BlockSize - (len(data) % aes.BlockSize)
		padded := append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
		encrypted := make([]byte, len(padded))
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(encrypted, padded)

		encPayload := hex.EncodeToString(iv) + ":" + hex.EncodeToString(encrypted)

		// Save per-file key locally
		keysDir := filepath.Join(c.configDir, "keys")
		_ = os.MkdirAll(keysDir, 0700)
		keyFile := filepath.Join(keysDir, fmt.Sprintf("%s_%s_%s.key", user, repoName, fileName))
		if err := os.WriteFile(keyFile, []byte(keyHex), 0600); err != nil {
			return fmt.Errorf("failed to save encryption key: %w", err)
		}

		// Write encrypted payload to form
		fw, err := w.CreateFormFile("upload", fileName)
		if err != nil {
			return fmt.Errorf("failed to create form file: %w", err)
		}
		// Create a new reader for the encrypted payload to be copied to the form.
		// The original reader has already been consumed.
		encryptedReader := strings.NewReader(encPayload)

		if _, err := io.Copy(fw, encryptedReader); err != nil {
			return fmt.Errorf("failed to copy encrypted file data: %w", err)
		}

		_ = w.WriteField("encrypt", "1")
		_ = w.WriteField("pre_encrypted", "1")
		_ = w.WriteField("plain_hash", plaintextHash)
		_ = w.WriteField("file_key", keyHex)

	} else {
		fw, err := w.CreateFormFile("upload", fileName)
		if err != nil {
			return fmt.Errorf("failed to create form file: %w", err)
		}

		hasher := sha256.New()
		teeReader := io.TeeReader(progressReader, hasher)

		if _, err := io.Copy(fw, teeReader); err != nil {
			return fmt.Errorf("failed to copy file data: %w", err)
		}

		computed = fmt.Sprintf("%x", hasher.Sum(nil))
	}

	w.Close()

	uploadURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repoName), url.QueryEscape(user))
	uploadReq, err := http.NewRequest("POST", uploadURL, &b)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	uploadReq.Header.Set("Content-Type", w.FormDataContentType())
	if c.sessionID != "" {
		uploadReq.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}
	uploadReq.Header.Set("X-FTR-CLIENT", "FtR-GUI") // Identify as GUI client

	resp, err = c.http.Do(uploadReq)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp map[string]interface{}
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if success, ok := apiResp["success"].(bool); ok {
			if success {
				serverHash := ""
				if h, ok := apiResp["hash"].(string); ok {
					serverHash = h
				}
				if serverHash != "" && computed != "" && !strings.EqualFold(serverHash, computed) {
					return fmt.Errorf("upload succeeded but integrity mismatch: server=%s local=%s", serverHash, computed)
				}
				return nil
			}
			errMsg := "upload failed - server error"
			if msg, ok := apiResp["message"].(string); ok {
				errMsg = msg
			}
			return fmt.Errorf("%s", errMsg)
		}
	}

	if bytes.Contains(body, []byte("color: #0f0")) && bytes.Contains(body, []byte("Uploaded")) {
		return nil
	}

	if bytes.Contains(body, []byte("Failed to create repository")) {
		return fmt.Errorf("failed to create repository - permission denied")
	}

	if bytes.Contains(body, []byte("Upload failed")) || bytes.Contains(body, []byte("color: red")) {
		return fmt.Errorf("upload failed - server rejected the file")
	}

	return fmt.Errorf("upload failed - unexpected response from server")
}

// ListRepoFiles returns a recursive list of files in a repository via the API
func (c *Client) ListRepoFiles(user, repo string) ([]map[string]interface{}, error) {
	listURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&list=1&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user))
	req, err := http.NewRequest("GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request: %w", err)
	}
	req.Header.Set("X-FTR-CLIENT", "FtR-CLI")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list failed: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list response: %w", err)
	}

	var apiResp map[string]interface{}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	out := []map[string]interface{}{}
	if ok, _ := apiResp["success"].(bool); !ok {
		return out, nil
	}
	if fl, ok := apiResp["files"].([]interface{}); ok {
		for _, f := range fl {
			if fm, ok := f.(map[string]interface{}); ok {
				out = append(out, fm)
			}
		}
	}
	return out, nil
}

// SessionConfirmed queries the sessionconfirm endpoint and returns true if
// the remote server considers the current session valid.
func (c *Client) SessionConfirmed() (bool, error) {
	req, err := http.NewRequest("GET", BaseURL+"/sessionconfirm", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create sessionconfirm request: %w", err)
	}
	req.Header.Set("X-FTR-CLIENT", "FtR-GUI")
	if c.sessionID != "" {
		req.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("sessionconfirm request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read sessionconfirm response: %w", err)
	}
	s := strings.TrimSpace(string(body))
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}
	return false, fmt.Errorf("unexpected sessionconfirm response: %s", s)
}

// DeleteRemoteFile sends a request to delete a file from a repository.
func (c *Client) DeleteRemoteFile(user, repo, fileName string) error {
	if !c.IsLoggedIn() {
		return errors.New("not logged in")
	}

	deleteURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&delete=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user), url.QueryEscape(fileName))

	req, err := http.NewRequest("GET", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	// Identify as a GUI client
	req.Header.Set("X-FTR-CLIENT", "FtR-GUI")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned error: %s - %s", resp.Status, string(body))
	}

	return nil
}

// DownloadFileContent downloads a file's content into memory, handling decryption.
func (c *Client) DownloadFileContent(user, repo, fileName string) ([]byte, error) {
	if !c.IsLoggedIn() {
		return nil, errors.New("not logged in")
	}

	meta, err := c.getFileMeta(user, repo, fileName)
	if err != nil {
		// This is not fatal, but we should log it. We can still attempt the download.
		log.Printf("Warning: could not get file metadata for %s/%s/%s: %v", user, repo, fileName, err)
	}

	downloadURL := fmt.Sprintf("%s%s/repo.php?name=%s&user=%s&download=%s&api=1", BaseURL, InkDropPath, url.QueryEscape(repo), url.QueryEscape(user), url.QueryEscape(fileName))

	req, err := http.NewRequest("POST", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}
	// The server expects this header for API downloads to differentiate from web requests.
	req.Header.Set("X-FTR-CLIENT", "FtR-GUI")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed with status %s: %s", resp.Status, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read download content: %w", err)
	}

	// Check if the file is encrypted and decrypt if necessary
	encrypted := false
	if meta != nil {
		if val, ok := meta["encrypted"]; ok && val == "1" {
			encrypted = true
		}
	}

	if encrypted {
		var keyHex string
		// If server provided an encryption_key in metadata, use it.
		if meta != nil {
			if ek, ok := meta["encryption_key"]; ok && ek != "" {
				keyHex = ek
			}
		}

		if keyHex == "" {
			// Fallback to local keys if server didn't provide one
			keysDir := filepath.Join(c.configDir, "keys")
			perFileKey := filepath.Join(keysDir, fmt.Sprintf("%s_%s_%s.key", user, repo, fileName))
			if data, err := os.ReadFile(perFileKey); err == nil {
				keyHex = strings.TrimSpace(string(data))
			}
		}

		if keyHex == "" {
			return nil, fmt.Errorf("file is encrypted, but no decryption key was found on the server or locally")
		}

		// The downloaded content is the raw encrypted blob, which is hex-encoded.
		plaintext, err := c.decryptHexPayload(string(content), keyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt file content: %w", err)
		}
		return plaintext, nil
	}

	// If not encrypted, return the content directly.
	return content, nil
}
