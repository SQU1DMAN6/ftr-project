#!/bin/sh
# Simple install script for the tooling example
echo "Tooling example install script ran"
# create a marker file to indicate install ran (no sudo required)
mkdir -p "$HOME/.local/share/tooling-example"
printf "installed" > "$HOME/.local/share/tooling-example/installed.txt"
exit 0
