<?php
session_start();
if (empty($_SERVER['HTTPS']) || $_SERVER['HTTPS'] === "off") {
    header("Location: https://" . $_SERVER['HTTP_HOST'] . $_SERVER['REQUEST_URI']);
    exit();
}
if (isset($_SESSION["login"])) {
    header("Location: index.php");
    exit();
}
?>
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FileShare Login</title>
</head>

<body>
    <main>
        <form method="POST" name="login" action="login.php">
            <input type="email" name="email" required placeholder="EMAIL"><br><br>
            <input type="password" name="password" required placeholder="PASSWORD"><br><br>
            <button type="submit">Login</button><br><br>
        </form>
        <br><br>
        <a href="register.php"><button>Register for FileShare account</button></a>

        <?php
        include "connect.php";

        if (isset($_POST["email"]) && isset($_POST["password"])) {
                $email = trim($_POST["email"]);
                $password = $_POST["password"];

                $query = "SELECT * FROM fileshare.users WHERE email = $1";
                $result = pg_query_params($db_handle, $query, array($email));

                if (!$result) {
                    echo "Query failed: " . pg_last_error($db_handle);
                    exit;
                }

            if (pg_num_rows($result) > 0) {
                $row = pg_fetch_assoc($result);
                if (password_verify($password, $row["password"])) {
                    $_SESSION["login"] = true;
					$_SESSION["name"] = $row["name"];
					$_SESSION["email"] = $row["email"];
					session_regenerate_id(true);
					echo "<script>window.location.href = 'index.php';</script>";
					exit();
                } else {
                    echo "<br><br><b style='color: #ff0000; background-color: black;'>Error logging in.</b><br><br>";
                }
            } else {
                echo "<br><br><b style='color: #ff0000; background-color: black;'>Error logging in.</b><br><br>";
            }
        }
        ?>
    </main>
</body>

<style>
    * {
        margin: 0px;
        padding: 0px;
    }

    body {
        font-family: monospace;
    }

    main {
        padding: 2px;
        background-color: #666666;
        color: #000;
        display: flex;
        flex-direction: column;
        position: relative;
        overflow: auto;
        width: 100%;
        height: 100vh;
    }

    button {
        padding: 3px;
        background-color: #333333;
        color: white;
        border: 1px solid white;
        border-radius: 5px;
    }
</style>

</html>