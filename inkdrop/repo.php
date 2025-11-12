<?php
include "guard.php";
include "loguserinfo.php";

$repo = $_GET["name"] ?? null;
$user = $_GET["user"] ?? ($_SESSION["name"] ?? null);

if (!$repo || !$user) {
    echo "Repository or user is not specified. Please proceed to <a href='index.php'>the main page</a>.";
    exit();
}

$repoPath = __DIR__ . "/repos/$user/$repo";

// Handle repository creation
$isOwner = ($_SESSION["name"] ?? null) === $user;

if (!is_dir($repoPath)) {
    // Only the owner can create a new repository
    if ($isOwner && $_SERVER["REQUEST_METHOD"] === "POST") {
        if (!mkdir($repoPath, 0755, true)) {
            echo "Failed to create repository.";
            exit();
        }
    } else {
        echo "The repository is not found. Please proceed to <a href='index.php'>the main page</a>.";
        exit();
    }
}

// Handle file delection, but only available to the owner of the repository
if ($isOwner && isset($_GET["delete"])) {
    $fileToDelete = basename($_GET["delete"]);
    $filePath = $repoPath . DIRECTORY_SEPARATOR . $fileToDelete;

    if (is_file($filePath)) {
        unlink($filePath);
        header("Location: repo.php?name=" . urlencode($repo));
        exit();
    }
}

// Handle file upload, but only available to the owner of the repository

if (
    $isOwner &&
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_FILES["upload"])
) {
    $file = $_FILES["upload"];
    $target = $repoPath . DIRECTORY_SEPARATOR . basename($file["name"]);

    if (move_uploaded_file($file["tmp_name"], $target)) {
        // Check if this is an API request (from FtR CLI)
        if (isset($_GET["api"]) && $_GET["api"] === "1") {
            // Return JSON response for CLI tools
            header("Content-Type: application/json");
            header("X-API-Response: true");
            echo json_encode([
                "success" => true,
                "message" => "File uploaded successfully",
                "filename" => basename($file["name"])
            ]);
            exit();
        }
        
        // HTML response for web client
        $message =
            "<b style='color: #0f0'>Uploaded " .
            htmlspecialchars($file["name"]) .
            ".</b>";
    } else {
        // Check if this is an API request (from FtR CLI)
        if (isset($_GET["api"]) && $_GET["api"] === "1") {
            // Return JSON error response
            header("Content-Type: application/json");
            header("X-API-Response: true");
            http_response_code(400);
            echo json_encode([
                "success" => false,
                "message" => "Upload failed"
            ]);
            exit();
        }
        
        // HTML response for web client
        $message = "<b style='color: red'>Upload failed.</b>";
    }
} elseif ($isOwner && $_SERVER["REQUEST_METHOD"] === "POST" && isset($_GET["api"]) && $_GET["api"] === "1") {
    // API request but no file uploaded
    header("Content-Type: application/json");
    header("X-API-Response: true");
    http_response_code(400);
    echo json_encode([
        "success" => false,
        "message" => "No file provided in upload"
    ]);
    exit();
} elseif (!$isOwner && $_SERVER["REQUEST_METHOD"] === "POST" && isset($_GET["api"]) && $_GET["api"] === "1") {
    // API request but not authorized
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

    $finfo = finfo_open(FILEINFO_MIME_TYPE);
    $mime_type = finfo_file($finfo, $previewPath);
    finfo_close($finfo);

    if (is_file($previewPath)) {
        $finfo = finfo_open(FILEINFO_MIME_TYPE); // Return MIME type
        $mime_type = finfo_file($finfo, $previewPath);
        finfo_close($finfo);

        if ($mime_type !== false) {
            $main_type = strtok($mime_type, "/"); // Get the part before the '/' in the MIME type

            // Prepare web-accessible URL for media files
            $mediaUrl =
                "/inkdrop/repos/$user/$repo/" . rawurlencode($previewFile);

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
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<audio controls><source src='$mediaUrl' type='$mime_type'>Your browser does not support the audio element.</audio>";
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
                    $content = file_get_contents($previewPath);
                    $previewContent =
                        "<h3>Preview of " .
                        htmlspecialchars($previewFile) .
                        "</h3>" .
                        "<br><br>" .
                        "<pre>" .
                        htmlspecialchars($content) .
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
                <form
                    action="repo.php?name=<?= urlencode($repo) ?>"
                    method="POST" enctype="multipart/form-data"
                    id="uploadForm"
                >
                    <input type="file" name="upload" required />
                    <button type="submit" class="select">Upload File</button>
                    <div id="progressContainer" style="display: none;">
                        <div id="progressBar"></div>
                        <div id="progressStatus">0%</div>
                    </div>
                </form>
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
                        if ($file === "." || $file === "..") {
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

                        // Use web paths for downloads and previews
                        $downloadLink =
                            "/inkdrop/repos/$user/$repo/" . rawurlencode($file);
                        $previewLink =
                            "/inkdrop/repo.php?name=" .
                            urlencode($repo) .
                            "&user=" .
                            urlencode($user) .
                            "&preview=" .
                            urlencode($file);
                        echo "<li><code>$file</code> ($size, $modified) ";
                        echo "<a href='$downloadLink' download><button class='select small'>Download</button></a>";
                        echo "<a href='$previewLink'><button class='select small'>Preview</button></a>";
                        if ($isOwner) {
                            $deleteLink =
                                "repo.php?name=" .
                                urlencode($repo) .
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
    </script>
</html>
