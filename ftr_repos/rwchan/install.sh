#!/usr/bin/env bash
set -e

APP_NAME="rwchan"
BIN_DIR="/usr/local/bin"
SHARE_DIR="/usr/local/share/$APP_NAME"
DESKTOP_DIR="/usr/local/share/applications"
PYTHON_BIN="$BIN_DIR/$APP_NAME"

DEPS=("portaudio" "python3-devel" "python3-venv" "python3-tk")

echo "Installing $APP_NAME..."

# Detect package manager
if command -v apt >/dev/null 2>&1; then
    PKG_MGR="apt"
elif command -v dnf >/dev/null 2>&1; then
    PKG_MGR="dnf"
elif command -v yum >/dev/null 2>&1; then
    PKG_MGR="yum"
elif command -v pacman >/dev/null 2>&1; then
    PKG_MGR="pacman"
else
    echo "No supported package manager found."
    exit 1
fi

# Install missing dependencies
echo "Checking dependencies..."
for dep in "${DEPS[@]}"; do
    case "$PKG_MGR" in
        apt)
            if ! dpkg -s "$dep" >/dev/null 2>&1; then
                echo "Installing $dep..."
                sudo apt update
                sudo apt install -y "$dep"
            fi
            ;;
        dnf|yum)
            if ! rpm -q "$dep" >/dev/null 2>&1; then
                echo "Installing $dep..."
                sudo $PKG_MGR install -y "$dep" --skip-unavailable
            fi
            ;;
        pacman)
            if ! pacman -Qi "$dep" >/dev/null 2>&1; then
                echo "Installing $dep..."
                sudo pacman -Sy --noconfirm "$dep"
            fi
            ;;
    esac
done

echo "Making venv..."
python3 -m venv .venv
echo "Activating venv..."
source ./.venv/bin/activate

pip install -q pyaudio

echo "Building binary..."
pyinstaller --noconsole --onefile main.py --name "$APP_NAME" \
    --hidden-import=pyaudio \
    --hidden-import=tkinter

echo "Installing binary to $BIN_DIR..."
sudo cp "dist/$APP_NAME" "$PYTHON_BIN"
sudo chmod +x "$PYTHON_BIN"

# Install .desktop entry
if [ -f "$APP_NAME.desktop" ]; then
    echo "Installing desktop entry..."
    sudo cp "$APP_NAME.desktop" "$DESKTOP_DIR/"
    sudo chmod 644 "$DESKTOP_DIR/$APP_NAME.desktop"
else
    echo "$APP_NAME.desktop not found, skipping menu entry."
fi

if [ -f "$APP_NAME.png" ]; then
    sudo mkdir -p "$SHARE_DIR"
    sudo cp "$APP_NAME.png" "$SHARE_DIR/"
fi

echo "$APP_NAME installed successfully! Run $APP_NAME as shell command."
