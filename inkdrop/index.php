<?php
include "guard.php";
//you are cinl
include "loguserinfo.php";

$baseRepoDir = __DIR__ . "/repos/";
$username = $_SESSION["name"];

$viewPublic = ($_GET["view"] ?? "") === "public";
$publicUser = $_GET["user"] ?? null;
$userRepoDir = "$baseRepoDir/$username/";
if (!is_dir($userRepoDir)) {
    mkdir($userRepoDir, 0775, true);
}

$createMessage = "";

// Handle repository deletion
if (!$viewPublic && isset($_POST["delete_repo"])) {
    $repoToDelete = trim($_POST["delete_repo"]);
    if (preg_match("/^[a-zA-Z0-9_-]{1,50}$/", $repoToDelete)) {
        $repoPath = $userRepoDir . $repoToDelete;
        if (is_dir($repoPath)) {
            // Recursively delete the repository directory
            $files = new RecursiveIteratorIterator(
                new RecursiveDirectoryIterator(
                    $repoPath,
                    RecursiveDirectoryIterator::SKIP_DOTS,
                ),
                RecursiveIteratorIterator::CHILD_FIRST,
            );

            foreach ($files as $fileinfo) {
                if ($fileinfo->isDir()) {
                    rmdir($fileinfo->getRealPath());
                } else {
                    unlink($fileinfo->getRealPath());
                }
            }
            rmdir($repoPath);
            $createMessage =
                "<b style='color: #00FF00'>Repository deleted successfully.</b>";
        }
    }
}

if (
    !$viewPublic &&
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_POST["reponame"])
) {
    $reponame = trim($_POST["reponame"]);
    if (!preg_match("/^[a-zA-Z0-9_-]{1,50}$/", $reponame)) {
        $createMessage =
            "<b style='color: red'>Repository name contains invalid characters or is too long.</b>";
    } else {
        $fullRepoPath = "$userRepoDir$reponame/";
        if (is_dir($fullRepoPath)) {
            $createMessage =
                "<b style='color: red'>Repository already exists.</b>";
        } else {
            mkdir($fullRepoPath, 0755, true);
            $createMessage =
                "<b style='color: #00FF00'>Repository created successfully.</b>";
        }
    }
}
?>
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <link rel="stylesheet" href="root.css?version=1.2" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
            rel="stylesheet"
        />
        <title>InkDrop</title>
    </head>
    <body>
        <main>
            <h1 class="intro" style="color: white">Welcome to InkDrop, the file sharing platform</h1>
            <br /><hr class="linebreaker"/><br /><br>
            <?php
            echo "<p style='display: none'>";
            include "connect.php";
            echo "</p>";
            ?>
            <?php if (!$viewPublic): ?>
            <div class="main1 main">
                <p style='font-size: 25pt'>Logged in as <b><?php echo htmlspecialchars(
                    $username,
                ); ?></b></p>
                <br /><br />
                <nav>
                    <a href="?view=public"><button class="select">Browse Repositories</button></a>
                </nav>
                <br /><hr class="linebreaker"/><br />
                <form method="POST" action="index.php">
                    <input name="reponame" type="text" class="details" placeholder="Enter new repository name..." required/><br /><br />
                    <button type="submit" class="redirect">Create Repo</button>
                </form>

                <?php $createMessage ? "<br><br>$createMessage" : ""; ?>
            </div>
            <div class="main2 main">
                <?php if (
                    !empty(array_filter(glob($userRepoDir . "*"), "is_dir"))
                ) {
                    echo "<h2>Your Repositories:</h2><br />";
                } ?>
                <ul>
                    <?php
                    $repos = array_filter(glob($userRepoDir . "*"), "is_dir");
                    if (empty($repos)) {
                        echo "<li><b style='font-size: 24'>You do not have any repositories. Consider creating one to store files.</b></li>";
                    } else {
                        foreach ($repos as $repoPath) {
                            $repoName = basename($repoPath);
                            echo "<li>";
                            echo "<a href='repo.php?name=$repoName&user=$username' style='color: white; text-decoration: underline; font-size: 20pt'>$repoName</a>";
                            echo " <button onclick=\"deleteRepo('" .
                                htmlspecialchars($repoName, ENT_QUOTES) .
                                "')\" class='select small' style='margin-left: 10px; background: #ff3333'>Delete</button>";
                            echo "</li>";
                        }
                    }
                    ?>
                </ul>
                <br /><br />
                <a href="logout.php"><button class="redirect">Logout</button></a>
            </div>
            <?php else: ?>
            <a href="index.php"><button class="redirect">Back to main page</button></a>
            <br /><br />
            <div class="main3 main">
                <?php if (!$publicUser): ?>
                        <h2>All Users</h2>
                        <ul>
                            <?php
                            $users = array_filter(
                                glob($baseRepoDir . "*"),
                                "is_dir",
                            );
                            if (empty($users)) {
                                echo "<li>No users found.</li>";
                            } else {
                                foreach ($users as $userDir) {
                                    $user = basename($userDir);
                                    echo "<li><a href='?view=public&user=" .
                                        urlencode($user) .
                                        "'><button class='select'>" .
                                        htmlspecialchars($user) .
                                        "</button></a></li>";
                                }
                            }
                            ?>
                        </ul>
                <?php else: ?>
                <h2>Repositories of <?php echo htmlspecialchars(
                    $publicUser,
                ); ?></h2>
                <ul>
                    <?php
                    $publicUserDir = $baseRepoDir . $publicUser . "/";
                    if (!is_dir($publicUserDir)) {
                        echo "<li>User not found.</li>";
                    } else {
                        $repos = array_filter(
                            glob($publicUserDir . "*"),
                            "is_dir",
                        );
                        if (empty($repos)) {
                            echo "<li>This user does not have any repositories.</li>";
                        } else {
                            foreach ($repos as $repoPath) {
                                $repoName = basename($repoPath);
                                $link =
                                    "repo.php?name=" .
                                    urlencode($repoName) .
                                    "&user=" .
                                    urlencode($publicUser);
                                echo "<li><a href='$link'><button class='select'>" .
                                    htmlspecialchars($repoName) .
                                    "</button></a></li>";
                            }
                        }
                    }
                    ?>
                </ul>
                <?php endif; ?>
            </div>
            <?php endif; ?>
        </main>
    </body>
    <style>
    main {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--secondary)
        );
        width: 100%;
        height: 100vh;
        position: relative;
        display: flex;
        flex-direction: column;
        align-items: center;
        word-break: break-word;
        overflow: auto;
        scrollbar-width: none;
    }

    * {
        scrollbar-width: none;
    }
    </style>
    <script>
    function deleteRepo(repoName) {
        if (confirm('Are you sure you want to delete the repository "' + repoName + '"?\nThis action cannot be undone and all files will be permanently deleted.')) {
            const form = document.createElement('form');
            form.method = 'POST';
            form.style.display = 'none';

            const input = document.createElement('input');
            input.type = 'hidden';
            input.name = 'delete_repo';
            input.value = repoName;

            form.appendChild(input);
            document.body.appendChild(form);
            form.submit();
        }
    }
    </script>
</html>
