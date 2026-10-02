#!/bin/sh
# Stops the agent when the package is removed, but not when it is updated:
# dpkg passes "upgrade", rpm the number of versions left (1).
set -e
case "$1" in
upgrade | 1) ;;
*) [ ! -d /run/systemd/system ] || systemctl disable --now mcsm-agent.service ;;
esac
