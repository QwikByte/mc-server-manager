#!/bin/sh
# Creates the master's user and restarts a running master after an update.
set -e
systemd-sysusers mcsm-master.conf
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload
	systemctl try-restart mcsm-master.service
fi
