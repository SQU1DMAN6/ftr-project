<?php
include "guard.php";
?>
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="utf-8">
    <link rel="stylesheet" href="root.css" />
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
        href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
        rel="stylesheet" />
    <title>Login</title>
</head>
<style>
    * {
        scrollbar-width: none;
    }

    main {
        background-image: linear-gradient(
            to bottom,
            var(--primary),
            var(--secondary),
            var(--light-secondary)
        );
        opacity: 1;
        color: white;
        height: 100vh;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        text-align: center;
        margin: 0px;
        padding: 0px;
        width: 100%;
        height: 100vh;
    }

    .login-box {
        padding: 60px;
        background-image: linear-gradient(
            to bottom,
            var(--grey),
            var(--dark)
        );
        color: white;
        border: 2px solid #fff;
        border-radius: 15px;
        box-shadow: 0px 0px 15px 5px var(--light-light-blue);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;
    }
</style>
<body>
    <main>
        <div class="login-box">
            <h1 class="intro">Login to access FtR services</h1>
            <br><hr class="linebreaker" /><br>
            <form method="POST" name="login" action="login.php">
                <input class="details" type="email" name="email" required placeholder="EMAIL" /><br /><br />
                <input class="details" type="password" name="password" required placeholder="PASSWORD" /><br /><br />
                <button type="submit" class="redirect">Login</button><br /><br />
            </form>
            <br /><hr class="linebreaker" /><br />
            <a href="register.php"><button class="redirect">Register for a new account</button></a><br><br>
            <?php
            include "inkdrop/connect.php";

            if (isset($_POST["email"]) && isset($_POST["password"])) {
                $email = trim($_POST["email"]);
                $password = $_POST["password"];

                $query = "SELECT * FROM fileshare.users WHERE email = $1";
                $result = pg_query_params($db_handle, $query, [$email]);

                if (!$result) {
                    echo "Query failed: " . pg_last_error($db_handle);
                    exit();
                }

                if (pg_num_rows($result) > 0) {
                    $row = pg_fetch_assoc($result);
                    if (password_verify($password, $row["password"])) {
                        // Set session cookie to last 90 days
                        $lifetime = 60 * 60 * 24 * 90; // 90 days in seconds
                        session_set_cookie_params($lifetime);
                        session_regenerate_id(true);
                        
                        $_SESSION["login"] = true;
                        $_SESSION["name"] = $row["name"];
                        $_SESSION["email"] = $row["email"];
                        
                        // Set last login time
                        $_SESSION["login_time"] = time();
                        
                        echo "<script>window.location.href = 'index.php';</script>";
                        exit();
                    } else {
                        echo "<br><br><h2 style='color: #ff0000; background-color: black;'>Error logging in.</h2><br><br>";
                    }
                } else {
                    echo "<br><br><h2 style='color: #ff0000; background-color: black;'>Error logging in.</h2><br><br>";
                }
            }
            ?>
            </div>
    </main>
</body>

</html>