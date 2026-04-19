#!/bin/bash

echo "You may be prompted for your sudo password. If so, please enter it."

set -e

sudo echo "Installing FtR for UNIX operating systems..."

if ! command -v go >/dev/null 2>&1; then
	"Error: Please install Golang to use the 'go' command necessary to install FtR."
	exit 1
fi

echo "Changing working directory to temporary build directory..."
cd /tmp/fsdl

echo "Creating share directory at /usr/local/share/ftr..."
sudo mkdir -p "/usr/local/share/ftr"

echo "Building FtR..."
go build -o ftr .
echo "Making binary executable..."
chmod 755 ./ftr

if [ -f "/usr/local/bin/ftr" ]; then
	echo "Renaming busy application file..."
	sudo mv /usr/local/bin/ftr /usr/local/bin/ftr.old
fi

echo "Copying application file..."
sudo cp ftr /usr/local/share/ftr
echo "Copying remove.sh..."
sudo cp remove.sh /usr/local/share/ftr
echo "Installing binary to /usr/local/bin/ftr..."
sudo cp ftr /usr/local/bin/ftr

ftr --help
echo "Cleaning build directory..."
sudo rm -rf /tmp/fsdl

if [ -f "/usr/local/bin/ftr.old" ]; then
	echo "Cleaning old files..."
	sudo rm -f /usr/local/bin/ftr.old
fi

ftr clear
echo "You're all set."