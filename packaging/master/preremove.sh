#!/bin/sh
# Stops the master when the package is removed, but not when it is updated:
# dpkg passes "upgrade", rpm the number of versions left (1).
set -e
case "$1" in
upgrade | 1) ;;
*) [ ! -d /run/systemd/system ] || systemctl disable --now noryx-master.service noryx-master-update.path ;;
esac
