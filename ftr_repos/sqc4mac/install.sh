#!/bin/bash

# Why are you trying to read this? You trying to pirate my app?

set -e

APP_NAME="Squ1dCalc"
APP_CMD_NAME="squ1dcalc"
ENTRY_SCRIPT="main.py"
ICON_FILE="icon.icns"
VENV_DIR=".venv"
APP_BUNDLE="./${APP_NAME}.app"
PYTHON_PATH="/opt/homebrew/bin/python3"

echo "Checking environment..."

# Check for Homebrew Python
if ! [ -x "$PYTHON_PATH" ]; then
    echo "Python not found at $PYTHON_PATH"
    echo "Install Homebrew Python first: 'brew install python3'"
    exit 1
fi

# Detect current logged-in user (not root)
CURRENT_USER=$(stat -f '%Su' /dev/console)

# Run brew commands as that user
function brew_user() {
  sudo -u "$CURRENT_USER" brew "$@"
}

echo "Checking Homebrew dependencies..."

if brew_user list python-tk >/dev/null 2>&1 && brew_user list python-yq >/dev/null 2>&1; then
  echo "Required Homebrew packages already installed."
else
  echo "Installing missing packages via Homebrew..."
  brew_user install python-tk python-yq
fi

echo "Creating virtual environment..."
arch -arm64 "$PYTHON_PATH" -m venv "$VENV_DIR"
source "$VENV_DIR/bin/activate"

echo "Installing Python dependencies..."
pip install --upgrade pip
pip install pyinstaller tkmacosx pillow sympy

echo "Building single-file macOS app..."
pyinstaller \
  --onefile \
  --windowed \
  --name "$APP_CMD_NAME" \
  --icon "$ICON_FILE" \
  --hidden-import=tkmacosx \
  --collect-all tkmacosx \
  --collect-all PIL \
  --collect-submodules sympy \
  "$ENTRY_SCRIPT"

echo "Creating Squ1dCalc.app structure..."
mkdir -p "${APP_BUNDLE}/Contents/MacOS"
mkdir -p "${APP_BUNDLE}/Contents/Resources"

cp "dist/$APP_CMD_NAME" "${APP_BUNDLE}/Contents/MacOS/"
cp "$ICON_FILE" "${APP_BUNDLE}/Contents/Resources/"

echo "Writing Info.plist..."
cat > "${APP_BUNDLE}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
 "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_CMD_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>$(basename "$ICON_FILE")</string>
    <key>CFBundleIdentifier</key>
    <string>com.squ1dcalc.app</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
</dict>
</plist>
EOF

echo "Installing app to /Applications..."
cp -r "$APP_BUNDLE" /Applications

xattr -cr /Applications/Squ1dCalc.app
echo "Signing Squ1dCalc.app locally..."
sudo codesign --force --deep --sign - /Applications/Squ1dCalc.app

echo "SQC4MAC (Squ1dCalc for macOS) installed successfully!"
echo "Removing evidence..."
sudo rm -rf /tmp/fsdl/
