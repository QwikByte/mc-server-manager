#!/bin/sh
# Forgets the removed unit. Data and configuration stay, see the README to delete them.
[ ! -d /run/systemd/system ] || systemctl daemon-reload
