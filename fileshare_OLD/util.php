<?php
function isValidEmail($email)
{
  // Regular expression pattern for validating email
  $pattern = '/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/';

  // Check if the email matches the regex
  return preg_match($pattern, $email) === 1;
}
?>