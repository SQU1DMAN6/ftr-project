<?php
session_start();

if (isset($_SESSION["login"])) {
    header("Location: index.php");
    exit();
}

if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}
?>

<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FileShare Register</title>
</head>

<body>
    <main>
        <form action="/register.php" method="POST" name="login">
            <input type="text" name="name" placeholder="NAME" required><br>
            <input type="email" name="email" placeholder="EMAIL" required><br>
            <input type="password" name="password" placeholder="PASSWORD" required><br>
            <button type="submit">Register</button>
        </form>
        <br><br>

        <p>If you already have an account, then <a href="/login.php"><button>Login</button></a></p><br><br>

        <?php
        echo "<br><br>";

        include "connect.php";
        include "util.php";

        $store = false;

        // Displaying form results once submitted
        if (isset($_POST["name"])) {
            $name = $_POST["name"];

            $email = strtolower(trim($_POST["email"]));
            if (!isValidEmail($email)) {
                echo "Your email address is not correct!";
                return;
            }

            $password_raw = $_POST["password"];
            $password_cooked = password_hash($password_raw, PASSWORD_DEFAULT);
            echo "Hello, $name<br><br>";
            echo "Your email is: $email<br><br>";
            echo "Your password is: $password_raw<br>Encrypted to: $password_cooked<br><br>";
            $store = true;
        }

        // Validate information before we proceed. (for example use regex to validate email format)

        if ($store) {
            $checkQuery =
                "SELECT * FROM fileshare.users WHERE name = $1 OR email = $2";
            $checkResult = pg_query_params($db_handle, $checkQuery, [
                $name,
                $email,
            ]);

            if (pg_num_rows($checkResult) > 0) {
                echo "<br><b style='color: orange;'>Username or email already exists.</b><br>";
                $store = false;
            } else {
                $query =
                    "INSERT INTO fileshare.users (name, email, password) VALUES ($1, $2, $3)";
                $result = pg_query_params($db_handle, $query, [
                    $name,
                    $email,
                    $password_cooked,
                ]);

                if ($result) {
                    echo "<br><br><b style='color: green; background-color: black;'>New record created</b><br><br>";
                    echo "Please redirect to the login page: <a href='login.php'><button>Login</button></a>";
                } else {
                    echo "<br><br><b style='color: #ff0000; background-color: black;'>Error creating record</b><br><br>";
                }
            }
            $store = false;
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
        width: 100%;
        height: 100vh;
        overflow: auto;
        display: flex;
        flex-direction: column;
        position: relative;
        color: #000;
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
