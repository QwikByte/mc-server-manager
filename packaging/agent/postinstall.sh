#!/bin/sh
# Restarts a running agent after an update. Servers keep running meanwhile.
set -e
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload
	systemctl try-restart noryx-agent.service
fi
