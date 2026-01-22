<?php
include "guard.php";

$repo = $_GET["name"] ?? null;
$user = $_GET["user"] ?? ($_SESSION["name"] ?? null);

if (!$repo || !$user) {
    echo "Repo or user not specified.";
    exit();
}

$repoPath = __DIR__ . "/repos/$user/$repo/";

if (!is_dir($repoPath)) {
    echo "Repo not found.";
    exit();
}

$isOwner = ($_SESSION["name"] ?? null) === $user;

// Handle file deletion (owner only)
if ($isOwner && isset($_GET["delete"])) {
    $toDelete = basename($_GET["delete"]);
    $filePath = $repoPath . $toDelete;
    if (is_file($filePath)) {
        unlink($filePath);
        header("Location: repo.php?name=" . urlencode($repo));
        exit();
    }
}

// Handle file upload (owner only)
$message = "";
if ($isOwner && $_SERVER["REQUEST_METHOD"] === "POST" && isset($_FILES["upload"])) {
    $file = $_FILES["upload"];
    $target = $repoPath . basename($file["name"]);

    if (move_uploaded_file($file["tmp_name"], $target)) {
        $message = "<b style='color: #0F0;'>Uploaded " . htmlspecialchars($file["name"]) . ".</b>";
    } else {
        $message = "<b style='color: red;'>Upload failed.</b>";
    }
}

// Handle preview
$previewContent = "";
if (isset($_GET["preview"])) {
    $previewFile = basename($_GET["preview"]);
    $previewPath = $repoPath . $previewFile;
    $allowedExts = ["txt", "md", "json", "conf", "sh", "php", "js", "css", "html", "py", "cpp", "go", "cs", "xml"]; // Previewable file extensions

    $ext = pathinfo($previewFile, PATHINFO_EXTENSION);
    if (in_array($ext, $allowedExts) && is_file($previewPath)) {
        $content = file_get_contents($previewPath);
        $previewContent = "<h3>Preview of " . htmlspecialchars($previewFile) . "</h3><br><br><pre>" . htmlspecialchars($content) . "</pre>";
    } else {
        $previewContent = "<p style='color: orange;'>File not previewable.</p>";
    }
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title><?= htmlspecialchars($repo) ?> – FileShare</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body>
<main>
    <h1><?= htmlspecialchars($repo) ?></h1>
    <p style="font-size: small;">Logged in as <b><?= htmlspecialchars($_SESSION["name"]) ?></b></p>
    <?php if (!$isOwner): ?>
        <p style="color: orange;"><i>Note: You're viewing a repo owned by <b><?= htmlspecialchars($user) ?></b>. You can't upload or delete files.</i></p>
    <?php endif; ?>

    <div class="btn-row">
        <a href="index.php"><button>Back</button></a>
        <a href="logout.php"><button>Logout</button></a>
    </div>

    <?php if ($isOwner): ?>
    <form action="repo.php?name=<?= urlencode($repo) ?>" method="POST" enctype="multipart/form-data">
        <input type="file" name="upload" required>
        <button type="submit">Upload File</button>
    </form>
    <?= $message ? "<p>$message</p>" : "" ?>
    <?php endif; ?>

    <hr>
    <h2>Files in repo:</h2>
    <ul>
        <?php
        $files = scandir($repoPath);
        foreach ($files as $file) {
            if ($file === "." || $file === "..") continue;

            $fullPath = $repoPath . $file;
            $size = round(filesize($fullPath) / 1024, 1); // KB
            $modified = date("Y-m-d H:i", filemtime($fullPath));
            $downloadLink = "repos/$user/$repo/" . rawurlencode($file);
            $previewLink = "repo.php?name=" . urlencode($repo) . "&user=" . urlencode($user) . "&preview=" . urlencode($file);

            echo "<li><code>$file</code> ($size KB, $modified) ";
            echo "<a href='$downloadLink' download>[Download]</a> ";
            echo "<a href='$previewLink'>[Preview]</a> ";

            if ($isOwner) {
                $deleteLink = "repo.php?name=" . urlencode($repo) . "&delete=" . urlencode($file);
                echo "<a href='$deleteLink' onclick=\"return confirm('Delete $file?');\">[Delete]</a>";
            }

            echo "</li>";
        }
        ?>
    </ul>

    <hr>
    <?= $previewContent ?>
</main>
</body>

<style>
    * {
        margin: 0;
        padding: 0;
    }

    body {
        font-family: monospace;
        background-color: #666;
        color: white;
    }

    main {
        padding: 15px;
        width: 100%;
        background-color: #666;
        height: 100vh;
        overflow: auto;
    }

    .btn-row {
        margin: 10px 0;
    }

    input[type="file"] {
        margin-right: 10px;
        padding: 5px;
        border: 1px solid #333;
        background-color: #454545;
        color: white;
        border-radius: 4px;
    }

    button {
        padding: 3px;
        background-color: #333333;
        color: white;
        border: 1px solid white;
        border-radius: 5px;
    }

    ul {
        padding-left: 20px;
        list-style-type: square;
    }

    li {
        margin-bottom: 6px;
    }

    a {
        color: cyan;
        text-decoration: none;
        margin-left: 5px;
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
</style>
</html>
