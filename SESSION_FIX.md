# Session Persistence Fix

## Problem
After running `ftr login`, the session information was not persisting for subsequent commands. Running `ftr session` would fail with "no active session found" error.

## Root Cause
The `login.php` script was using a JavaScript `window.location.href` redirect instead of an HTTP redirect. While browsers execute JavaScript and follow the redirect, the Go HTTP client does not process JavaScript. This caused the login flow to work incorrectly.

The flow was:
1. FtR POSTs credentials to `/login.php`
2. Server sets session and outputs HTML with JavaScript redirect to `index.php`
3. Go client receives HTML but doesn't execute the JavaScript
4. Go client still has the session cookie (set in the POST response headers)
5. Go client tries to extract username from the response body
6. But the response body is still the login form, not the `index.php` page
7. Username extraction fails, so `saveUserInfo()` is never called
8. Session files (email/username) are not created
9. On next command, `ftr session` has no user info to display

## Solution
Changed `/home/qchef/Documents/web-design/login.php` to use an HTTP redirect via PHP's `header()` function:

```php
// Old (JavaScript redirect - doesn't work with HTTP clients)
echo "<script>window.location.href = 'index.php';</script>";

// New (HTTP redirect - works with all HTTP clients)
header("Location: index.php", true, 302);
exit();
```

## How It Works Now
1. FtR POSTs credentials to `/login.php`
2. Server sets session and returns HTTP 302 redirect to `index.php`
3. Go HTTP client automatically follows the redirect (redirects are enabled)
4. FtR receives the actual `index.php` page containing "Logged in as <username>"
5. FtR extracts the username from the HTML response
6. FtR calls `saveUserInfo(email, username)` to persist session
7. Session files are created: `~/.config/ftr/email`, `~/.config/ftr/username`, `~/.config/ftr/session`
8. On next command, `ftr session` loads the user info and displays it correctly

## Files Modified
- `/home/qchef/Documents/web-design/login.php` - Changed JavaScript redirect to HTTP redirect
- `/home/qchef/Documents/web-design/ftr/pkg/api/client.go` - No changes needed (session persistence logic was already correct)

## Testing
To test the fix:
1. Run `ftr login` with your credentials
2. Run `ftr session` - should now show your email and username
3. Run other FtR commands - session should persist across commands
4. Verify session files are created: `ls -la ~/.config/ftr/`

Expected output after login:
```
$ ftr login
Email: your@email.com
Password: 
Successfully logged in

$ ftr session
Current Session Information:
    Email       your@email.com
    Username    your_username

$ ls -la ~/.config/ftr/
-rw------- email
-rw------- username
-rw------- session
```

## Session Persistence Architecture
The FtR client implements a complete session persistence system:

1. **Saving Session (after login)**
   - `saveSession()` - Saves PHPSESSID cookie to `~/.config/ftr/session`
   - `saveUserInfo(email, username)` - Saves email to `~/.config/ftr/email` and username to `~/.config/ftr/username`
   - Files created with 0600 permissions (read/write for owner only)

2. **Loading Session (on startup)**
   - `loadSession()` - Loads PHPSESSID from `~/.config/ftr/session`
   - `loadUserInfo()` - Loads email and username from files
   - Session restored automatically when FtR starts, avoiding need to re-login for each command

3. **Using Session**
   - `GetSessionInfo()` - Returns loaded email and username
   - HTTP cookies are restored automatically for subsequent API requests

This design allows:
- Single login per session
- Persistent commands without re-authentication
- Secure credential storage (file-based with restricted permissions)
- Cross-command session state preservation
