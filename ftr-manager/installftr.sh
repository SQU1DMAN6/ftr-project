#!/bin/bash

set -e
echo "You sudo password may be prompted. If so, please provide your sudo password. This is necessary for the FtR installation."
sudo echo "Installing FtR package manager..."

sudo rm -rf /tmp/fsdl/
mkdir -p /tmp/fsdl/
cd /tmp/fsdl/

echo "Extracting source code..."
curl --silent https://quanthai.net/ftr-manager-3.0.0-all-linux.fsdl -o ftr-manager.fsdl
sudo unzip -qq ftr-manager.fsdl
sudo chown -R $(whoami):$(whoami) /tmp/fsdl

if ! command -v go >/dev/null 2>&1; then
	"Error: Please install Golang to use the 'go' command necessary to install FtR."
	exit 1
fi

echo "Please sign in to your InkDrop account to use FtR."
go run . get qchef/ftr-manager

echo "FtR package manager has been installed successfully. Use 'ftr' as shell command to use it."
echo "ftr --help"
ftr --help

echo "Removing evidence..."
sudo ftr clear
