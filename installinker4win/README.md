# installinker4win

A simple Windows GUI installer for Inker that automates setup.

## Features

- Downloads `inker.zip` from https://quanthai.net/inker.zip
- Extracts the archive automatically
- Copies the binary to `%USERPROFILE%\bin`
- Creates a Start Menu shortcut
- Initializes the FtR Windows registry with version info
- Updates the JSON registry file
- Logs all operations with timestamps for easy debugging

## Building

```bash
go build ./...
```

This will produce `installinker4win.exe`.

## Running

Double-click `installinker4win.exe` or run it from the command line. The GUI will appear with an "Install Inker" button.

Click the button and watch the verbose log output as the installer progresses. All operations, file paths, and errors are logged for transparency and debugging.
