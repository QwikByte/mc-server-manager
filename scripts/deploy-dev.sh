#!/usr/bin/env bash
# Builds the panel and both programs for Linux from this checkout and installs them over SSH on
# test machines with Noryx installed, replacing the programs of their packages:
#
#   scripts/deploy-dev.sh --master root@vm1 --agent root@vm1 --agent root@vm2
#
# Turn off "Check for updates" in the panel's settings, so that it doesn't offer the latest release
# as an update of the test build. ARCH (default amd64) and VERSION (default: the commit and -dev)
# can be set.
set -Eeuo pipefail

usage() {
  echo "Usage: $0 [--master user@host]... [--agent user@host]..." >&2
  exit 2
}

masters=()
agents=()
while (($#)); do
  case $1 in
    --master) [ $# -ge 2 ] || usage; masters+=("$2"); shift 2 ;;
    --agent) [ $# -ge 2 ] || usage; agents+=("$2"); shift 2 ;;
    *) usage ;;
  esac
done
[ $((${#masters[@]} + ${#agents[@]})) -gt 0 ] || usage

cd "$(dirname "${BASH_SOURCE[0]}")/.."
version=${VERSION:-$(git describe --tags --always --dirty)-dev}
export GOOS=linux GOARCH=${ARCH:-amd64}
if [ ${#masters[@]} -gt 0 ]; then make web master VERSION="$version"; fi
if [ ${#agents[@]} -gt 0 ]; then make agent VERSION="$version"; fi

# deploy copies a program to a host, installs it in place of the package's and restarts its
# service. Servers keep running while the agent restarts.
deploy() {
  local host=$1 name=$2
  printf '\n\033[1;32m==>\033[0m %s %s on %s\n' "$name" "$version" "$host"
  scp -q "bin/$name" "$host:/tmp/$name.new"
  # shellcheck disable=SC2029 # name expands here on purpose
  ssh "$host" "sudo=sudo; [ \"\$(id -u)\" -ne 0 ] || sudo=; \
    \$sudo install -m 0755 /tmp/$name.new /usr/bin/$name && rm -f /tmp/$name.new && \$sudo systemctl restart $name"
}
for host in ${masters[@]+"${masters[@]}"}; do deploy "$host" noryx-master; done
for host in ${agents[@]+"${agents[@]}"}; do deploy "$host" noryx-agent; done
