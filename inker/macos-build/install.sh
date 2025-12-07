#!/bin/bash

set -e

echo "Your sudo password may be prompted. If so, please enter it."
sudo echo "Installing Inker for macOS..."

echo "Copying app file..."
cp -r 'FtR Inker.app' /Applications

echo "Signing app locally..."
sudo xattr -cr '/Applications/FtR Inker.app'
echo "Signing app locally..."
sudo codesign --force --deep --sign - '/Applications/FtR Inker.app'

echo "Making app executable..."
sudo chmod -R +x '/Applications/FtR Inker.app/Contents/MacOS/inker'

echo "FtR Inker installed successfully."
echo "Removing evidence..."

rm -rf /tmp/fsdl
