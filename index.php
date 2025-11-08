<?php
include "guard.php";

// Validate session duration (90 days)
if (isset($_SESSION["login_time"])) {
    $session_duration = 7776000; // 90 days in seconds
    if (time() - $_SESSION["login_time"] > $session_duration) {
        // Session expired
        session_destroy();
        header("Location: login.php");
        exit();
    }
}

if (isset($_SESSION["name"]) && isset($_SESSION["login"])) {
    $username = $_SESSION["name"];
    $login = true;
}
?>
<!doctype html>
<html lang="en">

<head>
    <meta charset="UTF-8" />
    <script type="text/javascript" src="js/3d.js" defer></script>
    <link rel="stylesheet" href="css/3d.css" />
    <link rel="stylesheet" href="root.css?version=0.3409584529" />
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
        href="https://fonts.googleapis.com/css2?family=Open+Sans:ital,wght@0,300..800;1,300..800&family=Source+Code+Pro:ital@0;1&display=swap"
        rel="stylesheet" />
    <title>SQU1D</title>
    <style>
        * {
            box-sizing: border-box;
        }

        body {
            margin: 0;
            padding: 0;
            overflow: hidden;
        }

        main {
            overflow-y: auto;
            overflow-x: hidden;
            width: 100%;
            height: 100vh;
            background-image: linear-gradient(to bottom,
                    var(--secondary) 1%,
                    var(--primary),
                    var(--light-blue));
            color: #fff;
        }

        /* Custom scrollbar */
        main::-webkit-scrollbar {
            width: 20px;
        }

        main::-webkit-scrollbar-track {
            background: rgba(255, 255, 255, 0.1);
        }

        main::-webkit-scrollbar-thumb {
            background: rgba(255, 255, 255, 0.3);
        }

        .section-block {
            display: flex;
            flex-direction: column;
            padding: 40px;
            width: 100%;
            max-width: 900px;
            margin: 0 auto;
            text-align: center;
        }

        .optioncontainer {
            display: flex;
            flex-wrap: wrap;
            justify-content: center;
            gap: 20px;
        }

        a.redirector {
            text-decoration: none;
            color: inherit;
        }

        /* Clean Atropos container sizing */
        .atropos {
            width: 200px;
            height: 200px;
        }

        .atropos-scale,
        .atropos-rotate,
        .atropos-inner {
            width: 100%;
            height: 100%;
        }

        .tile-inner {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            width: 100%;
            height: 100%;
        }

        .tile-inner h2 {
            margin: 0 0 10px 0;
            text-align: center;
            color: var(--secondary);
        }

        .tile-inner img {
            height: 140px;
            width: auto;
            pointer-events: none;
        }
    </style>
</head>

<body>
    <main>
        <h1 class="intro" style="text-align:center; padding-top:20px;">
            This is quanthai.net. A hub for anything.<br />Software included.
        </h1>

        <hr class="linebreaker" />

        <!-- Intro section -->
        <div class="section-block">
            <?php if ($login === true) {
                echo "<h2>Welcome, " . $username . ", to the FtR project.</h2>";
                echo "<br /><br /><a href='logout.php'><button class='redirect'>Logout</button></a>";
            } else {
                echo "<p>You might not be logged in. If so, please proceed to the <a href='login.php'><button class='redirect'>Login</button></a> page to access FtR's services.<br />If you don't have an account, please proceed to <a href='register.php'><button class='redirect'>Sign Up</button></a></p>";
            } ?>
        </div>

        <!-- SECTION 1 -->
        <div class="section-block">
            <h2>A place to install software. Only the best. With freedom.</h2>
            <br />
            <div class="optioncontainer">
                <a href="/squ1dcalc" class="redirector">
                    <div class="atropos" data-atropos>
                        <div class="atropos-scale">
                            <div class="atropos-rotate"
                                style="border:1px solid var(--light-blue); border-radius:6px; background:white;">
                                <div class="atropos-inner">
                                    <div class="tile-inner" data-atropos-offset="0">
                                        <h2>Squ1dCalc</h2>
                                        <img src="assets/img/squ1dcalc_logo.png" alt="Squ1dCalc" />
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </a>

                <a href="/ftr-manager" class="redirector">
                    <div class="atropos" data-atropos>
                        <div class="atropos-scale">
                            <div class="atropos-rotate"
                                style="border:1px solid var(--light-blue); border-radius:6px; background:#f9f9f9;">
                                <div class="atropos-inner">
                                    <div class="tile-inner" data-atropos-offset="0">
                                        <h2>FtR Apps</h2>
                                        <img src="assets/img/ftr_logo.png" alt="FtR Apps" />
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </a>
            </div>
        </div>

        <!-- SECTION 2 -->
        <div class="section-block">
            <h2>A store for your files. Never lose them. Never worry.</h2>
            <br />
            <div class="optioncontainer">
                <a href="/inkdrop" class="redirector">
                    <div class="atropos" data-atropos>
                        <div class="atropos-scale">
                            <div class="atropos-rotate"
                                style="border:1px solid var(--light-blue); border-radius:6px; background:white;">
                                <div class="atropos-inner">
                                    <div class="tile-inner" data-atropos-offset="0">
                                        <h2>InkDrop</h2>
                                        <img src="assets/img/inkdrop_logo.png" alt="InkDrop" />
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </a>
            </div>
        </div>

        <!-- SECTION 3 -->
        <div class="section-block">
            <h2>Join the community. Start a conversation. Connect.</h2>
            <br />
            <div class="optioncontainer">
                <a href="/ftrchat" class="redirector">
                    <div class="atropos" data-atropos>
                        <div class="atropos-scale">
                            <div class="atropos-rotate"
                                style="border:1px solid var(--light-blue); border-radius:6px; background:white;">
                                <div class="atropos-inner">
                                    <div class="tile-inner" data-atropos-offset="0">
                                        <h2>FtR Chat</h2>
                                        <img src="assets/img/ftrchat_logo.png" alt="FtR Chat" />
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </a>
            </div>
        </div>
    </main>

    <script>
        document.addEventListener('DOMContentLoaded', () => {
            document.querySelectorAll('.atropos').forEach(el => {
                new Atropos({
                    el,
                    shadowScale: 2,
                    alwaysActive: false,
                });
            });
        });
    </script>
</body>

</html>
