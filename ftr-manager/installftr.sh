#!/bin/bash

set -e
echo "You sudo password may be prompted. If so, please provide your sudo password. This is necessary for the FtR installation."
sudo echo "Installing FtR package manager..."

sudo rm -rf /tmp/fsdl/
mkdir -p /tmp/fsdl/
cd /tmp/fsdl/

curl --silent https://quanthai.net/ftr-manager.fsdl -o ftr-manager.fsdl
sudo unzip -qq ftr-manager.fsdl

if ! command -v go >/dev/null 2>&1; then
	"Error: Please install Golang to use the 'go' command necessary to install FtR."
	exit 1
fi

go build -o ftr .

chmod 755 ./ftr

sudo cp ./ftr /usr/local/bin/ftr

echo "FtR package manager has been installed successfully. Use 'ftr' as shell command to use it."
echo "ftr --help"
ftr --help

echo "Removing evidence..."
sudo ftr clear
