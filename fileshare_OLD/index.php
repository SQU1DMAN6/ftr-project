<?php
include "guard.php";

$baseRepoDir = __DIR__ . "/repos/";
$username = $_SESSION["name"];

$viewPublic = ($_GET["view"] ?? "") === "public";
$publicUser = $_GET["user"] ?? null;

$userRepoDir = $baseRepoDir . $username . "/";
if (!is_dir($userRepoDir)) {
    mkdir($userRepoDir, 0775, true);
}

$createMessage = "";

if (
    !$viewPublic &&
    $_SERVER["REQUEST_METHOD"] === "POST" &&
    isset($_POST["reponame"])
) {
    $reponame = trim($_POST["reponame"]);

    if (!preg_match("/^[a-zA-Z0-9_-]+$/", $reponame)) {
        $createMessage = "<b style='color: red;'>Invalid repo name.</b>";
    } else {
        $fullRepoPath = $userRepoDir . $reponame . "/";
        if (is_dir($fullRepoPath)) {
            $createMessage = "<b style='color: red;'>Repo already exists.</b>";
        } else {
            mkdir($fullRepoPath, 0775, true);
            $createMessage = "<b style='color: green; background-color: black;'>Repo '$reponame' created successfully.</b>";
        }
    }
}
?>

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>FileShare</title>
</head>
<body>
<main>
    <h1>FileShare</h1>
    <br>
    <?php include "connect.php"; ?>
    <br>

    <?php if (!$viewPublic): ?>
        <p>Logged in as <b><?= htmlspecialchars($username) ?></b></p>

        <nav>
            <a href="?view=public"><button>Browse Repos</button></a>
        </nav>

        <br>
        <form method="POST" action="index.php">
            <input type="text" name="reponame" placeholder="Enter new repo name" required>
            <br>
            <button type="submit">Create Repo</button>
        </form>

        <?= $createMessage ? "<br><br>$createMessage" : "" ?>

        <br><hr><br>

        <h2>Your Repos:</h2>
        <ul>
            <?php
            $repos = array_filter(glob($userRepoDir . "*"), "is_dir");
            if (empty($repos)) {
                echo "<li>No repos yet.</li>";
            } else {
                foreach ($repos as $repoPath) {
                    $repoName = basename($repoPath);
                    echo "<li><a href='repo.php?name=$repoName&user=$username'>$repoName</a></li>";
                }
            }
            ?>
        </ul>

        <br><br>
        <a href="logout.php"><button>Logout</button></a>

    <?php else: ?>
        <nav>
            <a href="index.php"><button>Back to main</button></a>
        </nav>

        <?php if (!$publicUser): ?>
            <h2>All Users</h2>
            <ul>
                <?php
                $users = array_filter(glob($baseRepoDir . "*"), "is_dir");
                if (empty($users)) {
                    echo "<li>No users found.</li>";
                } else {
                    foreach ($users as $userDir) {
                        $user = basename($userDir);
                        echo "<li><a href='?view=public&user=" .
                            urlencode($user) .
                            "'>" .
                            htmlspecialchars($user) .
                            "</a></li>";
                    }
                }
                ?>
            </ul>

        <?php else: ?>
            <h2>Repos of <?= htmlspecialchars($publicUser) ?></h2>
            <ul>
                <?php
                $publicUserDir = $baseRepoDir . $publicUser . "/";
                if (!is_dir($publicUserDir)) {
                    echo "<li>User not found.</li>";
                } else {
                    $repos = array_filter(glob($publicUserDir . "*"), "is_dir");
                    if (empty($repos)) {
                        echo "<li>No repos found.</li>";
                    } else {
                        foreach ($repos as $repoPath) {
                            $repoName = basename($repoPath);
                            $link =
                                "repo.php?name=" .
                                urlencode($repoName) .
                                "&user=" .
                                urlencode($publicUser);
                            echo "<li><a href='$link'>" .
                                htmlspecialchars($repoName) .
                                "</a></li>";
                        }
                    }
                }
                ?>
            </ul>
        <?php endif; ?>
    <?php endif; ?>
</main>
</body>

<style>
    * {
        margin: 0px;
        padding: 0px;
    }

    body {
        font-family: monospace;
        background-color: #666666;
        color: #000;
    }

    main {
        padding: 2px;
        display: flex;
        flex-direction: column;
        position: relative;
        overflow: auto;
        width: 100%;
        height: 100vh;
        background-color: #666;
    }

    input[type="text"] {
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
        padding-left: 15px;
    }

    li {
        margin-bottom: 5px;
    }

    a {
        color: blue;
        text-decoration: none;
    }

    a:hover {
        text-decoration: underline;
    }

    nav {
        margin-bottom: 15px;
    }
</style>
</html>
