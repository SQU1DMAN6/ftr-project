#!/bin/bash

set -e

echo "Your sudo password may be prompted. If so, please enter it."
sudo echo "Installing Inker for macOS..."

echo "Copying app file..."
cp -r 'FtR Inker.app' /Applications

echo "Signing app locally..."
xattr -cr '/Applications/FtR Inker.app'
echo "Signing Squ1dCalc.app locally..."
sudo codesign --force --deep --sign - '/Applications/FtR Inker.app'

echo "FtR Inker installed successfully."
echo "Removing evidence..."

rm -rf /tmp/fsdl
