<?php
function validate_session() {
    if (!isset($_SESSION["login"]) || !isset($_SESSION["login_time"])) {
        header("Location: /login.php");
        exit();
    }

    // Check if session has expired (90 days)
    $session_duration = 60 * 60 * 24 * 90; // 90 days in seconds
    if (time() - $_SESSION["login_time"] > $session_duration) {
        session_destroy();
        header("Location: /login.php");
        exit();
    }
}
?>