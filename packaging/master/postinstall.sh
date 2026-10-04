#!/bin/sh
# Creates the master's user and restarts a running master after an update.
set -e
systemd-sysusers noryx-master.conf
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload
	systemctl try-restart noryx-master.service
fi
