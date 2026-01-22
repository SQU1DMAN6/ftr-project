#!/bin/bash

echo "You may be prompted for your sudo password. If so, please enter it. It is necessary for the FtR Inker installation."

set -e

APP_NAME="inker"
BIN_DIR="/usr/local/bin"
SHARE_DIR="/usr/local/share/$APP_NAME"
DESKTOP_DIR="/usr/share/applications"
ICON_PATH="/usr/local/share/$APP_NAME/$APP_NAME.png"

sudo echo "Installing $APP_NAME..."

# Create share directory
echo "Creating share directory at $SHARE_DIR..."
sudo mkdir -p "$SHARE_DIR"

echo "Copying application files..."
sudo cp -r ./* "$SHARE_DIR"

echo "Building..."
go build -o "$APP_NAME" . > /dev/null

echo "Installing binary to $BIN_DIR..."
sudo cp "$APP_NAME" "$BIN_DIR/$APP_NAME"
sudo chmod 755 "$BIN_DIR/$APP_NAME"

# Desktop entry
if [ -f "$APP_NAME.desktop" ]; then
	echo "Installing desktop entry..."
	sudo cp "$APP_NAME.desktop" "$DESKTOP_DIR"
	sudo chmod 644 "$DESKTOP_DIR/$APP_NAME.desktop"
else
	echo "Warning: no desktop entry found to install"
fi

# Icon
if [ -f "$APP_NAME.png" ]; then
	echo "Installing app icon..."
	sudo cp "$APP_NAME.png" "$ICON_PATH"
fi

echo "Done! $APP_NAME installed!"
