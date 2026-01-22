<?php
session_start();

if (!isset($_SESSION["login"]) || !isset($_SESSION["name"])) {
    echo "false";
} else {
    echo "true";    
}
?>