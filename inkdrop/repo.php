<?php
include "guard.php";
include "loguserinfo.php";

// ============ Repo metadata and encryption functions ============

/**
 * Get the metadata file path for a repository
 */
function getMetaBaseDir() {
    // Allow overriding via env var for deployment to a secure location
    $env = getenv('INKDROP_META_DIR');
    if ($env && strlen($env) > 0) {
        return rtrim($env, DIRECTORY_SEPARATOR) . DIRECTORY_SEPARATOR . 'repos';
    }

    // Default fallback outside the web-accessible folder (one level up)
    $fallback = __DIR__ . DIRECTORY_SEPARATOR . '..' . DIRECTORY_SEPARATOR . '.inkdrop_meta' . DIRECTORY_SEPARATOR . 'repos';
    if (!is_dir($fallback)) {
        @mkdir($fallback, 0700, true);
    }
    return $fallback;
}

function getRepoMetaPath($repoPath) {
    // Attempt to extract user and repo from the repoPath which should be
    // something like /.../inkdrop/repos/{user}/{repo}
    $parts = explode(DIRECTORY_SEPARATOR, trim($repoPath, DIRECTORY_SEPARATOR));
    $repo = array_pop($parts);
    $user = array_pop($parts);

    $base = getMetaBaseDir();
    $userDir = $base . DIRECTORY_SEPARATOR . $user;
    if (!is_dir($userDir)) {
        @mkdir($userDir, 0700, true);
    }
    return $userDir . DIRECTORY_SEPARATOR . $repo . '.json';
}

/**
 * Load repository metadata
 */
function loadRepoMeta($repoPath) {
    $metaPath = getRepoMetaPath($repoPath);
    // If metadata exists in the secure store, use it
    if (is_file($metaPath)) {
        $data = json_decode(file_get_contents($metaPath), true);
        if (is_array($data)) return $data;
    }

    // Legacy location (inside repo) migration: if .repo_meta.json exists in repoPath,
    // move it to the secure store and remove the legacy file.
    $legacy = $repoPath . DIRECTORY_SEPARATOR . '.repo_meta.json';
    if (is_file($legacy)) {
        $data = json_decode(file_get_contents($legacy), true);
        if (is_array($data)) {
            // Ensure destination dir exists
            $metaDir = dirname($metaPath);
            if (!is_dir($metaDir)) @mkdir($metaDir, 0700, true);
            file_put_contents($metaPath, json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES));
            @chmod($metaPath, 0600);
            @unlink($legacy);
            return $data;
        }
    }

    // Default metadata for new repositories
    return [
        'type' => 'generic_public_readonly',
        'encrypted' => true,
        'description' => '',
        'created_at' => time(),
        'modified_at' => time(),
        'files' => [],
    ];
}

/**
 * Save repository metadata
 */
function saveRepoMeta($repoPath, $meta) {
    $metaPath = getRepoMetaPath($repoPath);
    $metaDir = dirname($metaPath);
    if (!is_dir($metaDir)) @mkdir($metaDir, 0700, true);
    file_put_contents($metaPath, json_encode($meta, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES));
    @chmod($metaPath, 0600);
}

/**
 * Generate an AES-256 encryption key (for file storage)
 */
function generateAESKey() {
    return bin2hex(random_bytes(32)); // 256-bit key in hex
}

/**
 * Check if repo is accessible by current user
 */
function canAccessRepo($repoType, $isOwner) {
    if ($isOwner) return true;
    
    return in_array($repoType, [
        'generic_public_readonly',
        'generic_opensource',
        'software_public',
        'software_opensource',
    ]);
}

/**
 * Check if current user can edit repo
 */
function canEditRepo($repoType, $isOwner) {
    if ($isOwner) return true;
    
    return in_array($repoType, [
        'generic_opensource',
        'software_opensource',
    ]);
}

/**
 * Check if repo can be fetched via API (FtR)
 */
function canFetchViaAPI($repoType) {
    // Only Software repos are accessible via the FtR API
    return in_array($repoType, [
        'software_public',
        'software_opensource',
    ]);
}

/**
 * Compute SHA256 hash of raw data
 */
function computeDataHash($data) {
    return hash('sha256', $data);
}

/**
 * Simple malware/content check for an in-memory payload
 */
function checkForMalwareContent($content, $fileName) {
    $ext = strtolower(pathinfo($fileName, PATHINFO_EXTENSION));
    $dangerousExts = ['exe', 'bat', 'cmd', 'scr', 'vbs', 'dll', 'sys', 'drv', 'pif', 'com', 'msi', 'ps1'];
    if (in_array($ext, $dangerousExts)) {
        return "Potentially malicious file extension detected: .$ext";
    }

    // Check for dangerous patterns in all files (especially in packages)
    $suspiciousPatterns = [
        'shell_exec(',
        'exec(',
        'system(',
        'passthru(',
        'eval(',
        'assert(',
        'create_function(',
        'base64_decode(',
        'proc_open(',
        'proc_exec(',
        'popen(',
        'pcntl_exec(',
    ];

    foreach ($suspiciousPatterns as $pattern) {
        if (stripos($content, $pattern) !== false) {
            return "Suspicious code pattern detected: $pattern";
        }
    }

    return null;
}

/**
 * Compute SHA256 hash of a file
 */
function computeFileHash($filePath) {
    return hash_file('sha256', $filePath);
}

/**
 * Compute HMAC signature for a file (for package verification)
 */
function computeFileSignature($filePath, $key) {
    $fileContent = file_get_contents($filePath);
    return hash_hmac('sha256', $fileContent, $key);
}

/**
 * Simple malware signature check (extension + content patterns)
 */
function checkForMalware($filePath, $fileName) {
    $dangerousExts = ['exe', 'bat', 'cmd', 'scr', 'vbs', 'dll', 'sys', 'drv', 'pif', 'com', 'msi', 'ps1'];
    $ext = strtolower(pathinfo($fileName, PATHINFO_EXTENSION));
    
    if (in_array($ext, $dangerousExts)) {
        return "Potentially malicious file extension detected: .$ext";
    }
    
    // Check for dangerous patterns in all files (especially in packages)
    $content = file_get_contents($filePath);
    $suspiciousPatterns = [
        'shell_exec(',
        'exec(',
        'system(',
        'passthru(',
        'eval(',
        'assert(',
        'create_function(',
        'base64_decode(',
        'proc_open(',
        'proc_exec(',
        'popen(',
        'pcntl_exec(',
    ];
    
    foreach ($suspiciousPatterns as $pattern) {
        if (stripos($content, $pattern) !== false) {
            return "Suspicious code pattern detected: $pattern";
        }
    }
    
    return null; // No malware detected
}

/**
 * Encrypt file using AES-256 (OpenSSL)
 */
function encryptFile($filePath, $encryptionKey) {
    $plaintext = file_get_contents($filePath);
    $iv = openssl_random_pseudo_bytes(openssl_cipher_iv_length('aes-256-cbc'));
    $encrypted = openssl_encrypt($plaintext, 'aes-256-cbc', hex2bin($encryptionKey), OPENSSL_RAW_DATA, $iv);
    
    // Store IV with encrypted data: IV:encrypted
    return bin2hex($iv) . ':' . bin2hex($encrypted);
}

/**
 * Decrypt file using AES-256 (OpenSSL)
 */
function decryptFile($encryptedData, $encryptionKey) {
    list($ivHex, $encryptedHex) = explode(':', $encryptedData, 2);
    $iv = hex2bin($ivHex);
    $encrypted = hex2bin($encryptedHex);
    
    return openssl_decrypt($encrypted, 'aes-256-cbc', hex2bin($encryptionKey), OPENSSL_RAW_DATA, $iv);
}

// ============ Main logic ============

$repo = $_GET["name"] ?? null;
$user = $_GET["user"] ?? ($_SESSION["name"] ?? null);

if (!$repo || !$user) {
    echo "Repository or user is not specified. Please proceed to <a href='index.php'>the main page</a>.";
    exit();
}

$repoPath = __DIR__ . "/repos/$user/$repo";
$isOwner = ($_SESSION["name"] ?? null) === $user;

// Load metadata
$repoMeta = [];
$repoType = 'generic_private';
$isEncrypted = false;
$encryptionKey = null;

// Handle repository creation
if (!is_dir($repoPath)) {
    // Only the owner can create a new repository
    if ($isOwner && $_SERVER["REQUEST_METHOD"] === "POST" && isset($_POST['action']) && $_POST['action'] === 'create') {
        $selectedType = $_POST['repo_type'] ?? 'generic_private';
        $enableEncryption = isset($_POST['encrypt']) && $_POST['encrypt'] === '1';
        
        // Validate repo type
        $validTypes = [
            'generic_private',
            'generic_public_readonly',
            'generic_opensource',
            'software_public',
            'software_opensource',
        ];
        
        if (!in_array($selectedType, $validTypes)) {
            echo "Invalid repository type specified.";
            exit();
        }
        
        if (!mkdir($repoPath, 0755, true)) {
            echo "Failed to create repository.";
            exit();
        }
        
        // Create metadata file
        $repoMeta = [
            'type' => $selectedType,
            'encrypted' => $enableEncryption,
            'encryption_key' => $enableEncryption ? generateAESKey() : null,
            'description' => trim($_POST['description'] ?? ''),
            'created_at' => time(),
            'modified_at' => time(),
            'files' => [],
        ];
        saveRepoMeta($repoPath, $repoMeta);
    } else {
        // Check if showing creation form
        if ($isOwner && $_SERVER["REQUEST_METHOD"] === "GET") {
            // Show repo creation form
            include_repo_creation_form($repo, $user);
            exit();
        }
        
        echo "The repository is not found. Please proceed to <a href='index.php'>the main page</a>.";
        exit();
    }
} else {
    $repoMeta = loadRepoMeta($repoPath);
    $repoType = $repoMeta['type'] ?? 'generic_private';
    $isEncrypted = $repoMeta['encrypted'] ?? false;
    $encryptionKey = $repoMeta['encryption_key'] ?? null;
    
    // Handle API file metadata request (used by FtR to get expected hashes/signatures)
    if (isset($_GET['filemeta']) && $_GET['filemeta'] === '1' && isset($_GET['file']) && isset($_GET['api']) && $_GET['api'] === '1') {
        header('Content-Type: application/json');
        header('X-API-Response: true');

        // Only allow metadata queries for repositories the caller is allowed to access.
        // FtR clients (and logged-in users) may request metadata for public/non-private
        // repositories or when they are the owner. This allows authorized FtR clients
        // to retrieve per-file encryption keys for decryption.
        if (!canAccessRepo($repoType, $isOwner)) {
            http_response_code(403);
            echo json_encode(["success" => false, "message" => "File metadata not available for this repository"]);
            exit();
        }

        $qfile = basename($_GET['file']);
        $entry = $repoMeta['files'][$qfile] ?? null;
        if (!$entry) {
            http_response_code(404);
            echo json_encode(["success" => false, "message" => "File not found"]);
            exit();
        }

        // Determine whether we should include the encryption key in the response.
        // Treat clients that present the FtR header as FtR clients regardless of HTTP method;
        // metadata requests may be GETs from CLI clients and should still be recognized.
        $isFtRClient = (isset($_SERVER['HTTP_X_FTR_CLIENT']) && $_SERVER['HTTP_X_FTR_CLIENT'] === 'FtR-CLI');
        $isFromRepoPage = isset($_SERVER['HTTP_REFERER']) && strpos($_SERVER['HTTP_REFERER'], 'repo.php') !== false;

        $resp = [
            'success' => true,
            'file' => $qfile,
            'hash' => $entry['hash'] ?? null,
            'signature' => $entry['signature'] ?? null,
            'encrypted' => $entry['encrypted'] ?? false,
            'flagged' => $entry['flagged'] ?? false,
            'flagged_note' => $entry['flagged_note'] ?? null,
        ];

        // Only reveal the per-file encryption key to FtR CLI clients (who must be
        // logged in) or when requested from the repository browser (the web UI).
        // This allows any authenticated API user to get the key for decryption.
        if (isset($_SESSION['name']) && isset($entry['encryption_key'])) {
            $resp['encryption_key'] = $entry['encryption_key'];
        }

        echo json_encode($resp);
        exit();
    }

    // API: list all files in repository (recursively) for FtR clients
    if (isset($_GET['list']) && $_GET['list'] === '1' && isset($_GET['api']) && $_GET['api'] === '1') {
        header('Content-Type: application/json');

        $filesOut = [];
        $baseLen = strlen($repoPath) + 1;
        $it = new RecursiveIteratorIterator(new RecursiveDirectoryIterator($repoPath, RecursiveDirectoryIterator::SKIP_DOTS));
        foreach ($it as $fileinfo) {
            if ($fileinfo->isFile()) {
                $rel = substr($fileinfo->getRealPath(), $baseLen);
                if ($rel === '.repo_meta.json') continue;
                $entry = [
                    'path' => str_replace(DIRECTORY_SEPARATOR, '/', $rel),
                    'size' => $fileinfo->getSize(),
                    'modified' => $fileinfo->getMTime(),
                ];
                $metaEntry = $repoMeta['files'][basename($rel)] ?? null;
                if ($metaEntry) {
                    $entry['hash'] = $metaEntry['hash'] ?? null;
                    $entry['signature'] = $metaEntry['signature'] ?? null;
                    $entry['encrypted'] = $metaEntry['encrypted'] ?? false;
                } else {
                    $entry['hash'] = computeFileHash($fileinfo->getRealPath());
                    $entry['encrypted'] = false;
                }
                $filesOut[] = $entry;
            }
        }

        echo json_encode(['success' => true, 'files' => $filesOut]);
        exit();
    }
    
    // Check access permissions
    if (!canAccessRepo($repoType, $isOwner)) {
        http_response_code(403);
        echo "You do not have permission to access this repository.";
        exit();
    }
}

// Handle settings changes (repo type and encryption) - owner only
if ($isOwner && $_SERVER["REQUEST_METHOD"] === "POST" && isset($_POST['action']) && $_POST['action'] === 'update_settings') {
    $newType = $_POST['repo_type'] ?? $repoType;
    $validTypes = [
        'generic_private',
        'generic_public_readonly',
        'generic_opensource',
        'software_public',
        'software_opensource',
    ];
    
    if (in_array($newType, $validTypes)) {
        $repoMeta['type'] = $newType;
        $repoMeta['modified_at'] = time();
            // Update description if provided
            if (isset($_POST['description'])) {
                $repoMeta['description'] = trim($_POST['description']);
            }
        
        // Handle encryption toggle
        $enableEncryption = isset($_POST['encrypt']) && $_POST['encrypt'] === '1';
        if ($enableEncryption && !$repoMeta['encrypted']) {
            // Enable encryption - generate new key
            $repoMeta['encrypted'] = true;
            $repoMeta['encryption_key'] = generateAESKey();
        } elseif (!$enableEncryption) {
            // Disable encryption
            $repoMeta['encrypted'] = false;
            $repoMeta['encryption_key'] = null;
        }
        
        saveRepoMeta($repoPath, $repoMeta);
        $isEncrypted = $repoMeta['encrypted'];
        $encryptionKey = $repoMeta['encryption_key'];
        $settingsMessage = "<b style='color: #0f0'>Repository settings updated successfully.</b>";
    }
}

// Handle API delete request for non-owner
if (isset($_GET["delete"]) && isset($_GET['api']) && $_GET['api'] === '1' && !$isOwner) {
    http_response_code(403);
    header('Content-Type: application/json');
    echo json_encode(['success' => false, 'message' => 'You do not have permission to delete files from this repository.']);
    exit();
}

// Handle file deletion, but only available to the owner of the repository
if ($isOwner && isset($_GET["delete"])) {
    $fileToDelete = basename($_GET["delete"]);
    $filePath = $repoPath . DIRECTORY_SEPARATOR . $fileToDelete;

    if (is_file($filePath)) {
        unlink($filePath);
        
        // Remove from metadata
        if (isset($repoMeta['files'][$fileToDelete])) {
            unset($repoMeta['files'][$fileToDelete]);
            saveRepoMeta($repoPath, $repoMeta);
        }
        
        // API response for FtR clients
        if (isset($_GET['api']) && $_GET['api'] === '1') {
            header('Content-Type: application/json');
            echo json_encode([
                'success' => true,
                'message' => 'File deleted successfully',
                'file' => $fileToDelete,
            ]);
            exit();
        }

        header("Location: repo.php?name=" . urlencode($repo) . "&user=" . urlencode($user));
        exit();
    } elseif ($isOwner && isset($_GET['delete']) && isset($_GET['api']) && $_GET['api'] === '1') {
        // API request to delete a file that does not exist
        http_response_code(404);
        header('Content-Type: application/json');
        echo json_encode(['success' => false, 'message' => 'File not found']);
        exit();
    }
}

// Handle file download with permission checks and decryption
if (isset($_GET["download"])) {
    $fileToDownload = basename($_GET["download"]);
    $filePath = $repoPath . DIRECTORY_SEPARATOR . $fileToDownload;

    $isAPIRequest = isset($_GET['api']) && $_GET['api'] === '1';
    
    if (is_file($filePath)) {
        // Read stored content (may be encrypted at rest)
        $fileContent = file_get_contents($filePath);

        // File metadata (may include per-file encryption key)
        $fileMeta = $repoMeta['files'][$fileToDownload] ?? null;

        // If this is an API request, only serve the stored blob to clients that
        // present the FtR handshake (POST + X-FTR-Client header) or requests coming
        // from the InkDrop repo page (referer includes repo.php). This prevents
        // direct raw GETs from revealing the stored blob.
        if ($isAPIRequest) {
            $isFtRClient = ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_SERVER['HTTP_X_FTR_CLIENT']) && $_SERVER['HTTP_X_FTR_CLIENT'] === 'FtR-CLI');
            $isFromRepoPage = isset($_SERVER['HTTP_REFERER']) && strpos($_SERVER['HTTP_REFERER'], 'repo.php') !== false;
            if (!($isFtRClient || $isFromRepoPage)) {
                http_response_code(403);
                header('Content-Type: application/json');
                echo json_encode(["success" => false, "message" => "API downloads must use the FtR client or the repository browser"]);
                exit();
            }

            // Include helpful headers for CLI clients
            $fileMeta = $repoMeta['files'][$fileToDownload] ?? null;
            if ($fileMeta && isset($fileMeta['hash'])) {
                header('X-File-Hash: ' . $fileMeta['hash']);
            }
            if ($fileMeta && isset($fileMeta['signature'])) {
                header('X-File-Signature: ' . $fileMeta['signature']);
            }
            header('X-File-Encrypted: ' . (($fileMeta['encrypted'] ?? false) ? '1' : '0'));
            if ($fileMeta && !empty($fileMeta['flagged'])) {
                header('X-File-Flagged: 1');
                header('X-File-Flagged-Note: ' . ($fileMeta['flagged_note'] ?? ''));
            }

            // Support HTTP Range requests so browsers can seek within the file
            $filesize = strlen($fileContent);
            header('Accept-Ranges: bytes');

            $start = 0;
            $end = $filesize - 1;
            $length = $filesize;

            if (isset($_SERVER['HTTP_RANGE'])) {
                // Example: "Range: bytes=0-499"
                if (preg_match('/bytes\s*=\s*(\d*)-(\d*)/', $_SERVER['HTTP_RANGE'], $matches)) {
                    if ($matches[1] !== '') $start = intval($matches[1]);
                    if ($matches[2] !== '') $end = intval($matches[2]);
                    if ($start > $end || $start < 0 || $end >= $filesize) {
                        header('HTTP/1.1 416 Requested Range Not Satisfiable');
                        header("Content-Range: bytes */$filesize");
                        exit();
                    }
                    $length = $end - $start + 1;
                    header('HTTP/1.1 206 Partial Content');
                    header("Content-Range: bytes $start-$end/$filesize");
                }
            }

            header('Content-Type: application/octet-stream');
            header('Content-Disposition: attachment; filename="' . basename($fileToDownload) . '"');
            header('Content-Length: ' . $length);
            header('Cache-Control: no-cache, no-store, must-revalidate');

            // Output the requested byte range (or full content)
            if ($length === $filesize && $start === 0) {
                echo $fileContent;
            } else {
                echo substr($fileContent, $start, $length);
            }
            exit();
        }
        // Non-API downloads: handle encrypted files carefully. Only allow decrypted
        // downloads when coming from the repository browser (browser UI). FtR clients
        // should use the API endpoint which is handled above.
        $isFileEncrypted = ($fileMeta['encrypted'] ?? false) || isset($fileMeta['encryption_key']);

        if ($isFileEncrypted) {
            $isFromRepoPage = isset($_SERVER['HTTP_REFERER']) && strpos($_SERVER['HTTP_REFERER'], 'repo.php') !== false;
            if (! $isFromRepoPage) {
                http_response_code(403);
                echo "Encrypted files may only be downloaded via the repository browser or the FtR client.";
                exit();
            }

            // Decrypt before sending to the UI (use per-file key if present, otherwise repo key)
            $decKey = $fileMeta['encryption_key'] ?? $encryptionKey ?? null;
            if (! $decKey) {
                http_response_code(500);
                echo "Encrypted file key missing on server.";
                exit();
            }

            $fileContent = decryptFile($fileContent, $decKey);
            // Determine MIME type from the decrypted buffer
            $finfo = finfo_open(FILEINFO_MIME_TYPE);
            $mime_type = finfo_buffer($finfo, $fileContent) ?: 'application/octet-stream';
            finfo_close($finfo);
            $fileSize = strlen($fileContent);
        } else {
            // Not encrypted: determine MIME type from the stored file
            $fileSize = strlen($fileContent);
            $finfo = finfo_open(FILEINFO_MIME_TYPE);
            $mime_type = finfo_file($finfo, $filePath) ?: 'application/octet-stream';
            finfo_close($finfo);
        }

        // Send file (decrypted if we performed decryption above)
        // Support HTTP Range requests for seekable playback
        header('Accept-Ranges: bytes');

        $filesize = strlen($fileContent);
        $start = 0;
        $end = $filesize - 1;
        $length = $filesize;

        if (isset($_SERVER['HTTP_RANGE'])) {
            if (preg_match('/bytes\s*=\s*(\d*)-(\d*)/', $_SERVER['HTTP_RANGE'], $matches)) {
                if ($matches[1] !== '') $start = intval($matches[1]);
                if ($matches[2] !== '') $end = intval($matches[2]);
                if ($start > $end || $start < 0 || $end >= $filesize) {
                    header('HTTP/1.1 416 Requested Range Not Satisfiable');
                    header("Content-Range: bytes */$filesize");
                    exit();
                }
                $length = $end - $start + 1;
                header('HTTP/1.1 206 Partial Content');
                header("Content-Range: bytes $start-$end/$filesize");
            }
        }

        header("Content-Type: $mime_type");
        header("Content-Disposition: attachment; filename=\"" . basename($fileToDownload) . "\"");
        header("Content-Length: $length");
        header("Cache-Control: no-cache, no-store, must-revalidate");

        if ($length === $filesize && $start === 0) {
            echo $fileContent;
        } else {
            echo substr($fileContent, $start, $length);
        }
        exit();
    } else {
        http_response_code(404);
        if ($isAPIRequest) {
            header("Content-Type: application/json");
            echo json_encode(["success" => false, "message" => "File not found"]);
        } else {
            echo "File not found.";
        }
        exit();
    }
}

// Handle directory creation (owner or editable repos)
if (
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_POST['action']) && $_POST['action'] === 'mkdir' &&
    ( $isOwner || canEditRepo($repoType, $isOwner) )
) {
    $dirName = trim($_POST['dir'] ?? '');
    // Validate directory name: cannot start or end with a dot, and no .. components
    if ($dirName === '' || strpos($dirName, '..') !== false || str_starts_with($dirName, '.') || str_ends_with($dirName, '.')) {
        echo "Invalid directory name.";
        exit();
    }
    $targetDir = $repoPath . DIRECTORY_SEPARATOR . $dirName;
    if (!is_dir($targetDir)) {
        if (!mkdir($targetDir, 0755, true)) {
            echo "Failed to create directory.";
            exit();
        }
    }
    // Update metadata
    $repoMeta['modified_at'] = time();
    saveRepoMeta($repoPath, $repoMeta);
    header("Location: repo.php?name=" . urlencode($repo) . "&user=" . urlencode($user));
    exit();
}

// Handle file upload - available to the owner or repos that allow edits by others
if (
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_FILES["upload"]) &&
    ( $isOwner || canEditRepo($repoType, $isOwner) )
) {
    $file = $_FILES["upload"];
    $fileName = basename($file["name"]);
    $target = $repoPath . DIRECTORY_SEPARATOR . $fileName;
    $uploadSuccess = false;
    $uploadError = "Upload failed";

    // Check for malware; if found, we will still accept the upload but flag it in metadata
    $malwareCheck = checkForMalware($file["tmp_name"], $fileName);
    $flagged = false;
    $flaggedNote = null;

    if ($malwareCheck) {
        $flagged = true;
        $flaggedNote = $malwareCheck;
    }


    if (move_uploaded_file($file["tmp_name"], $target)) {
        $uploadSuccess = true;

        // Determine whether this upload should be encrypted at file level
        $encryptThis = false;
        $preEncrypted = false;
        // Web form field
        if (isset($_POST['encrypt_file']) && $_POST['encrypt_file'] === '1') {
            $encryptThis = true;
        }
        // API clients may send encrypt via form value or query param
        if (isset($_POST['encrypt']) && $_POST['encrypt'] === '1') {
            $encryptThis = true;
        }
        if (isset($_GET['encrypt']) && $_GET['encrypt'] === '1') {
            $encryptThis = true;
        }
        // Detect if client pre-encrypted the upload (FtR CLI does this when -E is used)
        if (isset($_POST['pre_encrypted']) && $_POST['pre_encrypted'] === '1') {
            $preEncrypted = true;
            $encryptThis = true;
        }

        // Determine file hash: if client provided plaintext hash, use it (pre-encrypted uploads)
        if ($preEncrypted && isset($_POST['plain_hash']) && strlen($_POST['plain_hash']) > 0) {
            $fileHash = $_POST['plain_hash'];
        } else {
            // compute hash of stored blob (plaintext or server-encrypted later)
            $fileHash = computeFileHash($target);
        }

        // Choose key for signature/encryption: if client pre-encrypted, server does not have key.
        // Otherwise, for server-side per-file encryption generate a key; fallback to repo-level key.
        $fileEncKey = null;
        if ($preEncrypted) {
            $fileEncKey = null; // key is held by client
        } elseif ($encryptThis) {
            $fileEncKey = generateAESKey();
        } elseif ($isEncrypted && $encryptionKey) {
            $fileEncKey = $encryptionKey;
        }

        $fileSignature = null;
        if ($fileEncKey) {
            $fileSignature = computeFileSignature($target, $fileEncKey);
        }

        // Encrypt file if we have a key to encrypt with and the client did not pre-encrypt
        if ($fileEncKey && !$preEncrypted) {
            $encryptedData = encryptFile($target, $fileEncKey);
            file_put_contents($target, $encryptedData);
        }

        // Update metadata; do not store per-file encryption key when client pre-encrypted
        $repoMeta['files'][$fileName] = [
            'hash' => $fileHash,
            'signature' => $fileSignature,
            'size' => filesize($target),
            'uploaded_at' => time(),
            'encrypted' => $encryptThis ? true : false,
        ];
        if (!$preEncrypted && $encryptThis && $fileEncKey) {
            // store per-file encryption key in secure metadata (meta dir is outside web root)
            $repoMeta['files'][$fileName]['encryption_key'] = $fileEncKey;
        } elseif ($preEncrypted && isset($_POST['file_key']) && strlen($_POST['file_key']) > 0) {
            // Client provided the per-file key (FtR -E). Store it securely in metadata so
            // authorized FtR/InkDrop clients can retrieve it for decryption on other devices.
            $repoMeta['files'][$fileName]['encryption_key'] = $_POST['file_key'];
        }

        if ($flagged) {
            $repoMeta['files'][$fileName]['flagged'] = true;
            $repoMeta['files'][$fileName]['flagged_note'] = $flaggedNote;
        }

        $repoMeta['modified_at'] = time();
        saveRepoMeta($repoPath, $repoMeta);

        $uploadError = null;
    }
    
    // Check if this is an API request (from FtR CLI)
    if (isset($_GET["api"]) && $_GET["api"] === "1") {
        header("Content-Type: application/json");
        header("X-API-Response: true");
        
        if ($uploadSuccess) {
            http_response_code(200);
            echo json_encode([
                "success" => true,
                "message" => "File uploaded successfully",
                "filename" => $fileName,
                "hash" => $repoMeta['files'][$fileName]['hash'] ?? null,
                "signature" => $repoMeta['files'][$fileName]['signature'] ?? null,
                "flagged" => $repoMeta['files'][$fileName]['flagged'] ?? false,
                "flagged_note" => $repoMeta['files'][$fileName]['flagged_note'] ?? null,
            ]);
        } else {
            http_response_code(400);
            echo json_encode([
                "success" => false,
                "message" => $uploadError
            ]);
        }
        exit();
    }
    
    // HTML response for web client
    if ($uploadSuccess) {
        $message = "<b style='color: #0f0'>Uploaded " . htmlspecialchars($fileName) . ".</b>";
    } else {
        $message = "<b style='color: red'>" . htmlspecialchars($uploadError) . "</b>";
    }
} elseif ($_SERVER["REQUEST_METHOD"] === "POST" && isset($_GET["api"]) && $_GET["api"] === "1" && !isset($_FILES["upload"])) {
    // API request but no file uploaded
    header("Content-Type: application/json");
    header("X-API-Response: true");
    http_response_code(400);
    echo json_encode([
        "success" => false,
        "message" => "No file provided in upload"
    ]);
    exit();
} elseif ($_SERVER["REQUEST_METHOD"] === "POST" && isset($_GET["api"]) && $_GET["api"] === "1" && !($isOwner || canEditRepo($repoType, $isOwner))) {
    // API request but not authorized (only owners or repos that allow edits may upload via API)
    header("Content-Type: application/json");
    header("X-API-Response: true");
    http_response_code(403);
    echo json_encode([
        "success" => false,
        "message" => "Not authorized to upload to this repository",
        "debug" => [
            "logged_in_as" => $_SESSION["name"] ?? "unknown",
            "repository_owner" => $user,
            "is_owner" => $isOwner
        ]
    ]);
    exit();
}

// Handle file preview, which is available to all users
$previewContent = "";

if (isset($_GET["preview"])) {
    $previewFile = basename($_GET["preview"]);
    $previewPath = $repoPath . DIRECTORY_SEPARATOR . $previewFile;

    if (is_file($previewPath)) {
        // Get file content (decrypt if needed)
        $fileContent = file_get_contents($previewPath);
        $fileMeta = $repoMeta['files'][$previewFile] ?? null;
        $isFileEncrypted = ($fileMeta['encrypted'] ?? false) || isset($fileMeta['encryption_key']);
        if ($isFileEncrypted) {
            $decKey = $fileMeta['encryption_key'] ?? $encryptionKey ?? null;
            if ($decKey) {
                $fileContent = decryptFile($fileContent, $decKey);
            } else {
                echo "<p style='color: orange'>Unable to preview encrypted file: key missing.</p>";
            }
        }
        
        // Determine MIME type
        $finfo = finfo_open(FILEINFO_MIME_TYPE);
        $mime_type = finfo_file($finfo, $previewPath) ?: 'application/octet-stream';
        finfo_close($finfo);

        if ($mime_type !== false) {
            $main_type = strtok($mime_type, "/"); // Get the part before the '/' in the MIME type

            // Use repo.php endpoint for media files (enforces permissions and decryption)
            $mediaUrl =
                "repo.php?name=" . urlencode($repo) .
                "&user=" . urlencode($user) .
                "&download=" . urlencode($previewFile);

            switch ($main_type) {
                case "video":
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<video width='90%' controls><source src='$mediaUrl' type='$mime_type'>Your browser does not support HTML video.</video>";
                    break;
                case "audio":
                    $audioId = 'aud_' . md5($previewFile);
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<div style='display:flex; align-items:center; gap:12px; max-width:90%;'>" .
                        "<button id='{$audioId}_play' class='select small'>Play</button>" .
                        "<div style='flex:1; display:flex; align-items:center; gap:8px;'>" .
                        "<input id='{$audioId}_seek' type='range' min='0' value='0' max='100' style='width:100%;'/>" .
                        "<span id='{$audioId}_time' style='min-width:80px; font-size:12pt; color:#ddd;'>0:00 / 0:00</span>" .
                        "</div>" .
                        "</div>" .
                        "<audio id='$audioId' preload='metadata' style='display:none;'><source src='$mediaUrl' type='$mime_type'></audio>" .
                        "<script> (function() {\n" .
                        "  var ctxTag = '[repo.php audio]';\n" .
                        "  var a = document.getElementById('{$audioId}');\n" .
                        "  var btn = document.getElementById('{$audioId}_play');\n" .
                        "  var seek = document.getElementById('{$audioId}_seek');\n" .
                        "  var time = document.getElementById('{$audioId}_time');\n" .
                        "  var seeking = false;\n" .
                        "  function formatTime(t) { if (!isFinite(t) || t < 0) return '0:00'; return Math.floor(t/60) + ':' + ('0'+Math.floor(t%60)).slice(-2); }\n" .
                        "  console.log(ctxTag, 'init', {audioId: '{$audioId}', src: '{$mediaUrl}'});\n" .
                        "  btn.addEventListener('click', function(){ console.log(ctxTag, 'play-btn-click', {paused: a.paused, currentTime: a.currentTime, duration: a.duration}); if (a.paused) { a.play(); btn.textContent = 'Pause'; } else { a.pause(); btn.textContent = 'Play'; } });\n" .
                        "  a.addEventListener('loadedmetadata', function(){ console.log(ctxTag, 'loadedmetadata', {duration: a.duration}); });\n" .
                        "  a.addEventListener('timeupdate', function(){ console.log(ctxTag, 'timeupdate', {seeking: seeking, currentTime: a.currentTime, duration: a.duration}); if (!seeking && a.duration && isFinite(a.duration) && a.duration > 0) { seek.value = (a.currentTime / a.duration) * 100; time.textContent = formatTime(a.currentTime) + ' / ' + formatTime(a.duration); } });\n" .
                        "  // Use pointer/touch/mouse handlers for broad compatibility and log events\n" .
                        "  seek.addEventListener('pointerdown', function(e){ seeking = true; console.log(ctxTag, 'pointerdown', {value: seek.value, event: e.type}); });\n" .
                        "  seek.addEventListener('touchstart', function(e){ seeking = true; console.log(ctxTag, 'touchstart', {value: seek.value, event: e.type}); }, { passive: true });\n" .
                        "  seek.addEventListener('mousedown', function(e){ seeking = true; console.log(ctxTag, 'mousedown', {value: seek.value, event: e.type}); });\n" .
                        "  function applySeekValue() { console.log(ctxTag, 'applySeekValue', {value: seek.value, duration: a.duration, currentTimeBefore: a.currentTime}); if (!a || !isFinite(a.duration) || a.duration <= 0) { console.log(ctxTag, 'applySeekValue-abort', {duration: a.duration}); return; } var val = parseFloat(seek.value); if (isNaN(val)) { console.log(ctxTag, 'applySeekValue-invalid', {val: seek.value}); return; } a.currentTime = (val/100) * a.duration; console.log(ctxTag, 'applied', {newCurrentTime: a.currentTime}); }\n" .
                        "  seek.addEventListener('input', function(e){ console.log(ctxTag, 'input', {value: seek.value}); if (a.duration && isFinite(a.duration) && a.duration > 0) { var newTime = (seek.value/100) * a.duration; time.textContent = formatTime(newTime) + ' / ' + formatTime(a.duration); } });\n" .
                        "  // Apply seek on pointerup/touchend/mouseup/change and clear seeking flag\n" .
                        "  seek.addEventListener('pointerup', function(e){ console.log(ctxTag, 'pointerup', {value: seek.value}); applySeekValue(); seeking = false; });\n" .
                        "  seek.addEventListener('mouseup', function(e){ console.log(ctxTag, 'mouseup', {value: seek.value}); applySeekValue(); seeking = false; });\n" .
                        "  seek.addEventListener('touchend', function(e){ console.log(ctxTag, 'touchend', {value: seek.value}); applySeekValue(); seeking = false; });\n" .
                        "  seek.addEventListener('change', function(e){ console.log(ctxTag, 'change', {value: seek.value}); applySeekValue(); seeking = false; });\n" .
                        "  a.addEventListener('ended', function(){ console.log(ctxTag, 'ended'); btn.textContent = 'Play'; seek.value = 0; });\n" .
                        "})(); </script>";
                    break;
                case "image":
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<img src='$mediaUrl' style='max-width: 90%; height: auto;'>";
                    break;
                case "text":
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<pre>" .
                        htmlspecialchars($fileContent) .
                        "</pre>";
                default:
                    $allowedExts = [
                        "txt",
                        "md",
                        "json",
                        "conf",
                        "sh",
                        "php",
                        "js",
                        "css",
                        "html",
                        "py",
                        "cpp",
                        "go",
                        "cs",
                        "xml",
                    ]; // Previewable file extensions
                    $ext = pathinfo($previewFile, PATHINFO_EXTENSION);

                    if (in_array($ext, $allowedExts)) {
                        $content = file_get_contents($previewPath);
                        $previewContent =
                            "<h3>Preview of " .
                            htmlspecialchars($previewFile) .
                            "</h3>" .
                            "<br><br>" .
                            "<pre>" .
                            htmlspecialchars($content) .
                            "</pre>";
                    } else {
                        echo "File is not previewable.";
                    }
                    break;
            }
        }
    } else {
        echo "<p style='color: orange'>File is not previewable.</p>";
    }
}

/**
 * Render repository creation form
 */
function include_repo_creation_form($repo, $user) {
    ?>
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8" />
        <link rel="stylesheet" href="root.css?version=1.2" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap" rel="stylesheet" />
        <title>Create Repository - InkDrop</title>
    </head>
    <body>
        <main style="justify-content: center;">
            <div style="background: rgba(0,0,0,0.3); padding: 40px; border-radius: 10px; max-width: 500px; margin: 20px;">
                <h2>Create New Repository: <code><?php echo htmlspecialchars($repo); ?></code></h2>
                <p>Select the repository type and encryption settings:</p>
                
                <form method="POST" action="repo.php?name=<?php echo urlencode($repo); ?>&user=<?php echo urlencode($user); ?>" style="margin-top: 20px;">
                    <input type="hidden" name="action" value="create" />
                    
                    <h3>Repository Type</h3>
                    
                    <div style="margin: 15px 0;">
                        <input type="radio" name="repo_type" value="generic_public_readonly" checked id="type2" />
                        <label for="type2"><b>Generic Public (InkDrop FileShare)</b> - Everyone can see, only you can edit</label>
                    </div>
                    
                    <div style="margin: 15px 0;">
                        <input type="radio" name="repo_type" value="generic_private" id="type1" />
                        <label for="type1"><b>Generic Private</b> - Only you can see and edit</label>
                    </div>
                    
                    <div style="margin: 15px 0;">
                        <input type="radio" name="repo_type" value="generic_opensource" id="type3" />
                        <label for="type3"><b>Generic Open-Source</b> - Everyone can see and edit</label>
                    </div>
                    
                    <div style="margin: 15px 0;">
                        <input type="radio" name="repo_type" value="software_public" id="type4" />
                        <label for="type4"><b>Software Public</b> - API-accessible software repository</label>
                    </div>
                    
                    <div style="margin: 15px 0;">
                        <input type="radio" name="repo_type" value="software_opensource" id="type5" />
                        <label for="type5"><b>Software Open-Source</b> - API-accessible and editable</label>
                    </div>
                    
                    <hr style="margin: 30px 0;" />
                    
                    <h3>Security Options</h3>
                    
                    <div style="margin: 15px 0;">
                        <input type="checkbox" name="encrypt" value="1" id="encrypt_check" checked />
                        <label for="encrypt_check"><b>Enable AES-256 Encryption</b> - Files will be encrypted at rest</label>
                    </div>
                    
                    <p style="font-size: 12px; color: #aaa; margin: 10px 0;">
                        Encrypted files cannot be decrypted without the encryption key. The key is stored with the repository.
                    </p>
                    
                    <div style="margin-top: 30px;">
                        <button type="submit" class="redirect" style="width: 100%; padding: 12px;">Create Repository</button>
                        <a href="index.php" style="display: block; text-align: center; margin-top: 10px;">Cancel</a>
                    </div>
                </form>
            </div>
        </main>
        <style>
            body {
                        background-image: linear-gradient(135deg, #0b1220 0%, #172433 100%);
                        color: #e6eef6;
                        font-family: "Inter", "Open Sans", system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial;
                        line-height: 1.45;
            }
            
            main {
                display: flex;
                flex-direction: column;
                min-height: 100vh;
                align-items: center;
                padding: 20px 10px;
                max-width: 1100px;
                margin: 0 auto;
            }
            
            label {
                margin-left: 8px;
                cursor: pointer;
            }
            
            h2, h3 {
                margin: 10px 0;
                color: #4fe1a6;
                font-weight: 600;
            }
            
            code {
                background: rgba(255,255,255,0.03);
                padding: 4px 8px;
                border-radius: 4px;
                color: #b7f5d6;
                font-family: "Source Code Pro", monospace;
            }
            
            hr {
                border: none;
                border-top: 1px solid #444;
            }
        </style>
    </body>
    </html>
    <?php
}
?>
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="UTF-8" />
        <link rel="stylesheet" href="root.css?version=1.2" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
            rel="stylesheet"
        />
        <title><?php echo htmlspecialchars($repo); ?> - InkDrop</title>
    </head>
    <body>
        <main>
            <h1 class="intro"><?php echo htmlspecialchars(
                $repo,
            ); ?> - InkDrop</h1>
            <?php if (!empty($repoMeta['description'])): ?>
                <p style="max-width: 900px; text-align: center; color: #eef; margin-top:6px;"><?php echo htmlspecialchars($repoMeta['description']); ?></p>
            <?php endif; ?>
            <br><hr class='linebreaker'><br />

            <div class="main main3">
                <p style="font-size: 20pt">Logged in as <b><?php echo htmlspecialchars(
                    $_SESSION["name"],
                ); ?></b></p>
                <?php if (!$isOwner): ?>
                    <p style="color: orange"><i>Note: You are viewing a repository owned by <b><?php echo htmlspecialchars(
                        $user,
                    ); ?></b>. You cannot upload or delete files.</i></p>
                <?php endif; ?>

                <div class="btn-row">
                    <a href="index.php"><button class="redirect">Back to main page</button></a>
                    <a href="logout.php"><button class="redirect">Logout</button></a>
                </div>
            </div>

            <div class="main main1">
                <?php if ($isOwner): ?>
                
                <!-- Repository Settings Panel -->
                <div style="margin: 15px 0;">
                    <button id="openSettingsBtn" class="select">Settings</button>

                    <div id="settingsModal" style="display:none; position: fixed; inset:0; background: rgba(0,0,0,0.6); align-items: center; justify-content: center;">
                        <div style="background: #fff; color: #222; padding: 20px; border-radius: 8px; width: 480px; max-width: 95%; margin: auto;">
                            <h3 style="margin-top: 0;">Repository Settings</h3>
                            <form id="settingsForm" method="POST" action="repo.php?name=<?php echo urlencode($repo); ?>&user=<?php echo urlencode($user); ?>" style="display: grid; gap: 12px;">
                                <input type="hidden" name="action" value="update_settings" />
                                <label for="repo_type_select"><b>Repository Type:</b></label>
                                <select name="repo_type" id="repo_type_select" style="padding: 8px; border-radius: 4px;">
                                    <option value="generic_private" <?php echo $repoType === 'generic_private' ? 'selected' : ''; ?>>Generic Private</option>
                                    <option value="generic_public_readonly" <?php echo $repoType === 'generic_public_readonly' ? 'selected' : ''; ?>>Generic Public</option>
                                    <option value="generic_opensource" <?php echo $repoType === 'generic_opensource' ? 'selected' : ''; ?>>Generic Open-Source</option>
                                    <option value="software_public" <?php echo $repoType === 'software_public' ? 'selected' : ''; ?>>Software Public (API)</option>
                                    <option value="software_opensource" <?php echo $repoType === 'software_opensource' ? 'selected' : ''; ?>>Software Open-Source (API)</option>
                                </select>

                                <div>
                                    <p style="font-size: 12px; color: #444; margin: 4px 0;"><b>Encryption:</b> repository-level encryption is <b><?php echo $isEncrypted ? 'ENABLED' : 'DISABLED'; ?></b>.</p>
                                </div>

                                <label for="description"><b>Description (optional):</b></label>
                                <textarea id="description" name="description" rows="3" style="width:100%; padding:8px;"><?php echo htmlspecialchars($repoMeta['description'] ?? ''); ?></textarea>

                                <p style="font-size: 12px; color: #666; margin: 0;">
                                    <b>Created:</b> <?php echo date('Y-m-d H:i:s', $repoMeta['created_at'] ?? time()); ?><br />
                                    <b>Last Modified:</b> <?php echo date('Y-m-d H:i:s', $repoMeta['modified_at'] ?? time()); ?><br />
                                    <b>Files:</b> <?php echo count(array_filter(scandir($repoPath) ?? [], function($f) { return $f !== '.' && $f !== '..' && $f !== '.repo_meta.json'; })); ?>
                                </p>

                                <div style="display:flex; gap:8px; justify-content:flex-end; margin-top:8px;">
                                    <button type="button" id="closeSettingsBtn" class="select" style="background:#888;">Cancel</button>
                                    <button id="updateSettingsBtn" type="submit" class="redirect">Update Settings</button>
                                </div>
                            </form>
                            <?php if (isset($settingsMessage)): ?>
                                <p style="margin-top: 10px; color: green"><?php echo $settingsMessage; ?></p>
                            <?php endif; ?>
                        </div>
                    </div>
                </div>
                
                <!-- File Upload Form -->
                <div style="display: flex; gap: 16px; align-items: flex-start;">
                    <form
                        action="repo.php?name=<?= urlencode($repo) ?>"
                        method="POST" enctype="multipart/form-data"
                        id="uploadForm"
                        style="flex: 1; background: rgba(255,255,255,0.02); padding: 12px; border-radius: 8px;"
                    >
                        <label style="display: block; margin-bottom: 8px;"><b>Upload File</b></label>
                        <input type="file" name="upload" required style="display:block; margin-bottom:10px;" />
                        <div style="margin-bottom:8px;">
                            <input type="checkbox" name="encrypt_file" value="1" id="encrypt_file_check" />
                            <label for="encrypt_file_check">Encrypt file at upload (AES-256)</label>
                        </div>
                        <button type="submit" class="select">Upload File</button>
                        <div id="progressContainer" style="display: none;">
                            <div id="progressBar"></div>
                            <div id="progressStatus">0%</div>
                        </div>
                    </form>
                </div>
                <?php if (isset($message)): ?>
                    <p><?php echo $message; ?></p>
                <?php endif; ?>
                <?php endif; ?>
            </div>
            <hr class="linebreaker" />
            <div class="main">
                <h2>Files in repo:</h2>
                <br /><br />
                <ul>
                    <?php
                    $files = scandir($repoPath);
                    foreach ($files as $file) {
                        if ($file === "." || $file === ".." || $file === ".repo_meta.json") {
                            continue;
                        }

                        $fullPath = $repoPath . DIRECTORY_SEPARATOR . $file;
                        // Handle file size with larger units if needed
                        $fileSize = filesize($fullPath);
                        if ($fileSize < 1024) {
                            $size = $fileSize . " B";
                        } elseif ($fileSize < 1024 * 1024) {
                            $size = round($fileSize / 1024, 1) . " KB";
                        } elseif ($fileSize < 1024 * 1024 * 1024) {
                            $size = round($fileSize / (1024 * 1024), 1) . " MB";
                        } else {
                            $size =
                                round($fileSize / (1024 * 1024 * 1024), 1) .
                                " GB";
                        }

                        // More detailed timestamp
                        $modified = date("Y-m-d H:i:s T", filemtime($fullPath));

                        // Get file metadata
                        $fileMeta = $repoMeta['files'][$file] ?? [];
                        $isFileEncrypted = $fileMeta['encrypted'] ?? false;
                        $fileHash = $fileMeta['hash'] ?? 'N/A';
                        $fileSignature = $fileMeta['signature'] ?? null;
                        $fileId = 'hash_' . md5($file);

                        // Use repo.php for downloads (to enforce permissions and decryption)
                        $downloadLink =
                            "repo.php?name=" .
                            urlencode($repo) .
                            "&user=" .
                            urlencode($user) .
                            "&download=" .
                            urlencode($file);
                        $previewLink =
                            "repo.php?name=" .
                            urlencode($repo) .
                            "&user=" .
                            urlencode($user) .
                            "&preview=" .
                            urlencode($file);
                        
                        $encryptionBadge = $isFileEncrypted ? ' <span style="background: #f00; padding: 2px 6px; border-radius: 3px; font-size: 10pt;">ENCRYPTED</span>' : '';
                        
                        echo "<li>";
                        echo "<code>$file</code> ($size, $modified)$encryptionBadge<br />";
                        echo "<span style='font-size: 11pt; color: #aaa;'>Hash: <code style='color: #0f0;'>" . substr($fileHash, 0, 16) . "...</code></span>";
                        // View Hash button to reveal full hash/signature
                        echo " <button type='button' class='select small' onclick=\"toggleHash('" . $fileId . "')\">View Hash</button>";
                        echo "<span id='" . $fileId . "' style='display: none; margin-left: 8px; font-size: 10pt; color: #ddd;'>Full Hash: <code style=\"color: #0f0;\">" . htmlspecialchars($fileHash) . "</code>";
                        if ($fileSignature) {
                            echo " Signature: <code style=\"color: #0f0;\">" . htmlspecialchars($fileSignature) . "</code>";
                        }
                        echo "</span>";
                        echo "<br />";
                        echo "<a href='$downloadLink' download><button class='select small'>Download</button></a>";
                        echo "<a href='$previewLink'><button class='select small'>Preview</button></a>";
                        if ($isOwner) {
                            $deleteLink =
                                "repo.php?name=" .
                                urlencode($repo) .
                                "&user=" .
                                urlencode($user) .
                                "&delete=" .
                                urlencode($file);
                            echo "<a href='$deleteLink' onclick=\"return confirm('Delete $file?')\"><button class='select small'>Delete</button></a>";
                        }

                        echo "</li>";
                    }
                    ?>
                </ul>

                <hr class='linebreaker' />
                <?php echo $previewContent; ?>
            </div>
        </main>
    </body>
    <style>
    * {
        scrollbar-width: none;
    }

    main {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--secondary)
        );
        color: white;
        display: flex;
        flex-direction: column;
        overflow: auto;
        scrollbar-width: none;
        width: 100%;
        height: 100vh;
        align-items: center;
    }

    .btn-row {
        margin: 10px 0;
    }

    input[type="file"] {
        margin: 8px;
        madding: 8px;
        border: 1px solid white;
        color: white;
        background-color: var(--dark);
        border-radius: 5px;
    }

    ul {
        padding-left: 20px;
        list-style-type: square;
    }

    li {
        margin-bottom: 3px;
        font-size: 14pt;
    }

    a {
        color: cyan;
        text-decoration: none;
        margin-left: 2px;
    }

    a:hover {
        text-decoration: underline;
    }

    pre {
        background-color: #222;
        color: #ddd;
        padding: 10px;
        border: 1px solid #333;
        border-radius: 4px;
        white-space: pre-wrap;
        overflow-y: auto;
        width: 97%
    }

    #progressContainer {
        width: 100%;
        max-width: 400px;
        margin: 10px 0;
        padding: 5px;
        background: rgba(0, 0, 0, 0.1);
        border-radius: 4px;
    }

    #progressBar {
        width: 0%;
        height: 20px;
        background: #00ff00;
        border-radius: 4px;
        transition: width 0.3s ease;
    }

    #progressStatus {
        text-align: center;
        margin-top: 5px;
        color: white;
    }
    </style>
    <script>
    function toggleHash(id) {
        var el = document.getElementById(id);
        if (!el) return;
        el.style.display = (el.style.display === 'none' || el.style.display === '') ? 'inline-block' : 'none';
    }
    document.getElementById('uploadForm')?.addEventListener('submit', function(e) {
        e.preventDefault();

        const form = e.target;
        const formData = new FormData(form);
        const progressContainer = document.getElementById('progressContainer');
        const progressBar = document.getElementById('progressBar');
        const progressStatus = document.getElementById('progressStatus');

        progressContainer.style.display = 'block';

        const xhr = new XMLHttpRequest();
        xhr.open('POST', form.action, true);

        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                const percentComplete = (e.loaded / e.total) * 100;
                progressBar.style.width = percentComplete + '%';
                progressStatus.textContent = Math.round(percentComplete) + '%';
            }
        };

        xhr.onload = function() {
            if (xhr.status === 200) {
                window.location.reload();
            } else {
                alert('Upload failed. Please try again.');
                progressContainer.style.display = 'none';
            }
        };

        xhr.onerror = function() {
            alert('Upload failed. Please try again.');
            progressContainer.style.display = 'none';
        };

        xhr.send(formData);
    });
    // Prevent double-click on Update Settings by disabling the button once submitted
    // Use a small timeout when disabling the button to avoid races where
    // the browser may cancel the submit if the button is disabled too early.
    document.getElementById('settingsForm')?.addEventListener('submit', function(e) {
        const btn = document.getElementById('updateSettingsBtn');
        if (btn) {
            setTimeout(function() {
                btn.disabled = true;
                btn.textContent = 'Updating...';
            }, 50);
        }
    });

    // Modal open/close handlers
    document.getElementById('openSettingsBtn')?.addEventListener('click', function() {
        const modal = document.getElementById('settingsModal');
        if (modal) modal.style.display = 'flex';
    });
    document.getElementById('closeSettingsBtn')?.addEventListener('click', function() {
        const modal = document.getElementById('settingsModal');
        if (modal) modal.style.display = 'none';
    });
    </script>
</html>
