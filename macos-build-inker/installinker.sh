#!/bin/bash

set -e

echo "Your sudo password may be prompted. If so, please enter it."
sudo echo "Installing Inker for macOS..."

echo "Removing temporary build directory..."
sudo rm -rf /tmp/fsdl

echo "Creating new temporary build directory..."
mkdir -p /tmp/fsdl

echo "Changing working directory to /tmp/fsdl..."
cd /tmp/fsdl

echo "Downloading FtR Inker..."
curl -sSL https://quanthai.net/inkdrop/repos/qchef/inker4mac/inker4mac.fsdl -o /tmp/fsdl/inker4mac.fsdl

echo "Extracting inker4mac.fsdl..."
unzip -oqq inker4mac.fsdl

echo "Copying app file..."
cp -r 'FtR Inker.app' /Applications

echo "Signing app locally..."
sudo xattr -cr '/Applications/FtR Inker.app'
echo "Signing app locally..."
sudo codesign --force --deep --sign - '/Applications/FtR Inker.app'

echo "Making app executable..."
sudo chmod -R +x '/Applications/FtR Inker.app/Contents/MacOS/inker'

echo "FtR Inker installed successfully."
echo "Cleaning up..."

rm -rf /tmp/fsdl