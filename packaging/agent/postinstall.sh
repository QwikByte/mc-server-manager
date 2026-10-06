#!/bin/sh
# Restarts a running agent after an update. Servers keep running meanwhile. The private
# network of the nodes comes up at boot, if the node is part of it.
set -e
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload
	systemctl enable --quiet noryx-overlay.service
	systemctl try-restart noryx-agent.service
fi
