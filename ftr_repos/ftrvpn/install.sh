#!/bin/bash

echo "Your sudo password may be prompted. Sudo is necessary for the FtRVPN installation."

sudo echo "Installing FtRVPN..."

sudo chmod 101 ftrvpn
sudo chmod 400 vpnkey

sudo mkdir /usr/local/share/ftrvpn
sudo cp ftrvpn /usr/local/share/ftrvpn
sudo cp ftrvpn /usr/local/bin/ftrvpn
sudo cp vpnkey /usr/local/share/ftrvpn

echo "FtRVPN installed successfully. Run 'ftrvpn' as shell command."