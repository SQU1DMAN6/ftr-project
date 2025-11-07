<?php
ini_set("session.gc_maxlifetime", 60 * 60 * 24 * 90);
ini_set("session.cookie_lifetime", 60 * 60 * 24 * 90);
session_start();

if (empty($_SERVER["HTTPS"]) || $_SERVER["HTTPS"] === "off") {
    header(
        "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"],
    );
    exit();
}
?>
