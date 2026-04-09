#!/bin/bash

echo "Removing FtR installation..."
sudo rm -f /usr/local/bin/ftr
sudo rm -rf /usr/local/share/ftr
sudo rm -r /home/$(whoami)/.config/ftr
echo "Done."