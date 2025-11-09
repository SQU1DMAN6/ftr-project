<?php
$filename = "connections.txt";

function getUserIpAddress()
{
    $ipAddress = "";
    if (!empty($_SERVER["HTTP_CLIENT_IP"])) {
        $ipAddress = $_SERVER["HTTP_CLIENT_IP"];
    } elseif (!empty($_SERVER["HTTP_X_FORWARDED_FOR"])) {
        $ipAddress = $_SERVER["HTTP_X_FORWARDED_FOR"];
    } else {
        $ipAddress = $_SERVER["REMOTE_ADDR"];
    }
    return $ipAddress;
}

$userIP = getUserIpAddress();
$userSession = print_r($_SESSION, true);

$data_to_write = "===\r\nUser session: $userSession\r\nUser IP: $userIP\r\n===";

$file_handle = fopen($filename, "a");

if ($file_handle) {
    fwrite($file_handle, $data_to_write);
    fclose($file_handle);
}
?>
