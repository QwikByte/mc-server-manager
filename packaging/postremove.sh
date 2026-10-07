#!/bin/sh
# Forgets the removed unit. Data and configuration stay, see "Removing" in docs/installation.md to delete them.
[ ! -d /run/systemd/system ] || systemctl daemon-reload
