#!/bin/bash

echo "Removing FtR installation..."
ftr remove qchef/ftr-manager
sudo rm -f /usr/local/bin/ftr
sudo rm -rf /usr/local/share/ftr
sudo rm -r /home/$(whoami)/.config/ftr
echo "Done."