<?php
ini_set("session.gc_maxlifetime", 60 * 60 * 24 * 90);
ini_set("session.cookie_lifetime", 60 * 60 * 24 * 90);
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

$message = "";
$message2 = "";
?>

<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="UTF-8"/>
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <link rel="stylesheet" href="root.css?version=1.2" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
            rel="stylesheet"
        />
        <title>Register</title>
    </head>
    <body>
        <main>
            <h1 class="intro">Register for FtR Services</h1>
            <br/><hr class="linebreaker"/><br/>
            <div class="register-box">
                <?php if (!$store): ?>
                <form action="register.php" method="POST" name="register">
                    <input class="details" type="text" name="name" placeholder="USERNAME" required/><br />
                    <input class="details" type="email" name="email" placeholder="EMAIL" required/><br />
                    <input class="details" type="password" name="password" placeholder="PASSWORD" required/><br />
                    <button type="submit" class="redirect">Register</button><br />
                </form>
                <br />
                <h2>If you already have an account, please proceed to <a href="login.php"><button class="redirect">Login</button></a></h2><br/><br/>
                <?php endif; ?>
                <?php
                include "inkdrop/connect.php";
                include "inkdrop/util.php";

                $store = false;

                if (
                    isset($_POST["name"]) &&
                    isset($_POST["email"]) &&
                    isset($_POST["password"])
                ) {
                    $nam = $_POST["name"];
                    $name = str_replace(" ", "_", $nam);

                    $email = strtolower(trim($_POST["email"]));
                    if (!isValidEmail($email)) {
                        echo "Your email address is not correct.";
                        return;
                    }

                    $password_raw = $_POST["password"];
                    $password_cooked = password_hash(
                        $password_raw,
                        PASSWORD_DEFAULT,
                    );
                    $store = true;
                }

                if ($store) {
                    if (!preg_match("/^[a-zA-Z0-9_-]{1,21}$/", $name)) {
                        echo "Your name contains invalid characters or is too long.";
                        $store = false;
                    }
                    $checkQuery =
                        "SELECT * FROM fileshare.users WHERE name = \$1 OR email = \$2";
                    $checkResult = pg_query_params($db_handle, $checkQuery, [
                        $name,
                        $email,
                    ]);

                    if (pg_num_rows($checkResult) > 0) {
                        echo "<b style='color: orange'>ERROR: Username or email already exists.</b>";
                        $store = false;
                    } else {
                        $query =
                            "INSERT INTO fileshare.users (name, email, password) VALUES (\$1, \$2, \$3)";
                        $result = pg_query_params($db_handle, $query, [
                            $name,
                            $email,
                            $password_cooked,
                        ]);

                        if ($result) {
                            $message =
                                "<b style='color: green;'>New record created successfully.</b><br><br><p>Please proceed to the login page: <a href='login.php'><button class='redirect'>Login</button></a></p>";
                        } else {
                            echo "<b style='color: orange'>Error registering.</b>";
                            $store = false;
                        }
                    }
                }
                ?>
            </div>
            <?php
            if ($store) {
                echo "
                <div class='main main1'>
                    $message
                    Hello, $name<br><br>
                    Your email is $email<br><br>
                </div>
                ";
            }
            $store = false;
            ?>
        </main>
    </body>
    <style>
        * {
            scrollbar-width: none;
        }

        main {
            width: 100%;
            height: 100vh;
            color: #fff;
            background-color: var(--secondary);
            display: flex;
            flex-direction: column;
            overflow: auto;
            scrollbar-width: none;
            align-items: center;
            text-align: center;
        }

        div.register-box {
            display: flex;
            flex-direction: column;
            background-image: linear-gradient(
                to bottom,
                var(--primary),
                var(--light-blue)
            );
            align-items: center;
            justify-content: center;
            padding: 40px;
            border: 1px solid white;
            border-radius: 20px;
            width: 90%;
        }
    </style>
</html>
