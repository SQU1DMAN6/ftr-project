<?php
ini_set("session.gc_maxlifetime", 7776000);
ini_set("session.cookie_lifetime", 7776000);
session_set_cookie_params(7776000, "/");
session_start();

if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}
?>
