#!/usr/bin/env bash
# Sets up a complete MC Server Manager test environment on Ubuntu (desktop, server or WSL):
# installs Go, Node.js and Docker when missing, builds both programs, starts the master,
# enrolls a local agent and optionally creates a Paper test server.
#
#   scripts/ubuntu-test.sh [setup]   install, build and start everything (default)
#   scripts/ubuntu-test.sh start     start master and agent again, e.g. after a reboot
#   scripts/ubuntu-test.sh stop      stop master and agent; Minecraft servers keep running
#   scripts/ubuntu-test.sh reset     stop everything and delete all test data and servers
set -Eeuo pipefail

REPO_URL=https://github.com/QwikByte/mc-server-manager.git
BRANCH=claude/nifty-ritchie-dxkhyr
GO_MINOR=27
NODE_MAJOR=22
BASE=http://localhost:8080
AGENT_PORT=7443

SELF=scripts/ubuntu-test.sh
REPO_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DATA=$REPO_DIR/.data/test
JAR=$DATA/cookies.txt
SUDO=sudo
[ "$(id -u)" -ne 0 ] || SUDO=
export PATH=/usr/local/go/bin:$PATH

log() { printf '\n\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mWarning:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31mError:\033[0m %s\n' "$*" >&2; exit 1; }
py() {
  local code=$1
  shift
  python3 -c "import json, sys; d = json.load(sys.stdin); $code" "$@"
}
port_in_use() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }
docker_up() { $SUDO docker info >/dev/null 2>&1; }
is_wsl() { grep -qi microsoft /proc/version; }

wait_for() {
  local tries=$1 i
  shift
  for ((i = 0; i < tries; i++)); do
    if "$@"; then return 0; fi
    sleep 1
  done
  return 1
}

# api calls the master's REST API with the session cookie and prints error bodies.
api() {
  local out
  if ! out=$(curl -sS --fail-with-body -b "$JAR" -c "$JAR" -H 'Content-Type: application/json' "$@"); then
    echo "API request failed: $out" >&2
    return 1
  fi
  printf '%s' "$out"
}

install_packages() {
  log "Installing system packages"
  $SUDO apt-get update -qq
  $SUDO apt-get install -y -qq git curl ca-certificates make python3 xz-utils >/dev/null
}

# relocate re-runs the script from a clone when it was started on its own, or on WSL from a
# Windows drive: the agent needs Unix sockets and file permissions, which /mnt/c lacks.
relocate() {
  if [ -f "$REPO_DIR/go.mod" ] && ! { is_wsl && [[ "$REPO_DIR" == /mnt/* ]]; }; then return; fi
  local target=$HOME/mc-server-manager
  log "Using a clone in $target"
  if [ -d "$target/.git" ]; then
    git -C "$target" fetch -q origin "$BRANCH"
    git -C "$target" checkout -q "$BRANCH"
    git -C "$target" pull -q --ff-only
  else
    git clone -q --branch "$BRANCH" "$REPO_URL" "$target"
  fi
  exec bash "$target/$SELF" "$@"
}

install_go() {
  local minor=0 file sha
  # Checked outside the repository, whose go.mod would make an old Go refuse to run.
  command -v go >/dev/null && minor=$(cd / && GOTOOLCHAIN=local go env GOVERSION 2>/dev/null | cut -d. -f2)
  [ "${minor:-0}" -lt "$GO_MINOR" ] || return 0
  log "Installing Go"
  read -r file sha < <(curl -fsSL 'https://go.dev/dl/?mode=json' | py '
f = next(f for f in d[0]["files"] if f["os"] == "linux" and f["arch"] == sys.argv[1] and f["kind"] == "archive")
print(f["filename"], f["sha256"])' "$(dpkg --print-architecture)")
  curl -fsSLo "/tmp/$file" "https://go.dev/dl/$file"
  echo "$sha  /tmp/$file" | sha256sum -c --quiet
  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf "/tmp/$file"
  rm "/tmp/$file"
  hash -r
  # shellcheck disable=SC2016 # expanded when the profile is loaded
  grep -qs /usr/local/go/bin "$HOME/.profile" || echo 'export PATH=/usr/local/go/bin:$PATH' >>"$HOME/.profile"
}

install_node() {
  if command -v node >/dev/null && [ "$(node -p 'process.versions.node.split(".")[0]')" -ge "$NODE_MAJOR" ]; then return; fi
  log "Installing Node.js $NODE_MAJOR"
  local url=https://nodejs.org/dist/latest-v$NODE_MAJOR.x arch line file
  arch=$(dpkg --print-architecture)
  [ "$arch" != amd64 ] || arch=x64
  line=$(curl -fsSL "$url/SHASUMS256.txt" | grep -E " node-v[0-9.]+-linux-$arch\.tar\.xz$")
  file=${line##* }
  curl -fsSLo "/tmp/$file" "$url/$file"
  (cd /tmp && echo "$line" | sha256sum -c --quiet)
  $SUDO tar -C /usr/local --strip-components=1 --exclude='*.md' --exclude=LICENSE -xJf "/tmp/$file"
  rm "/tmp/$file"
  hash -r
}

install_docker() {
  local docker_bin
  docker_bin=$(command -v docker || true)
  if [[ -n "$docker_bin" && "$(readlink -f "$docker_bin")" == *docker-desktop* ]]; then
    docker info >/dev/null 2>&1 || die "Docker Desktop is installed but not reachable. Start it (with the WSL integration for this distribution) and run the script again."
    return
  fi
  [ -z "$docker_bin" ] || return 0
  log "Installing Docker Engine"
  # shellcheck source=/dev/null
  . /etc/os-release
  $SUDO install -m 0755 -d /etc/apt/keyrings
  $SUDO curl -fsSLo /etc/apt/keyrings/docker.asc "https://download.docker.com/linux/$ID/gpg"
  $SUDO chmod a+r /etc/apt/keyrings/docker.asc
  printf 'Types: deb\nURIs: https://download.docker.com/linux/%s\nSuites: %s\nComponents: stable\nSigned-By: /etc/apt/keyrings/docker.asc\n' \
    "$ID" "${UBUNTU_CODENAME:-$VERSION_CODENAME}" | $SUDO tee /etc/apt/sources.list.d/docker.sources >/dev/null
  $SUDO apt-get update -qq
  $SUDO apt-get install -y -qq docker-ce docker-ce-cli containerd.io >/dev/null
}

start_docker() {
  docker_up && return
  if command -v docker >/dev/null; then
    if [ "$(ps -p 1 -o comm=)" = systemd ]; then
      $SUDO systemctl enable --now docker >/dev/null 2>&1 || true
    else
      $SUDO service docker start >/dev/null 2>&1 || true
    fi
  fi
  wait_for 30 docker_up || warn "Docker is not running, so the node can't create servers."
}

running() { [ -f "$1" ] && $SUDO kill -0 "$(cat "$1")" 2>/dev/null; }
stopped() { ! $SUDO kill -0 "$1" 2>/dev/null; }

start_master() {
  running "$DATA/master.pid" && return
  ! port_in_use 8080 || die "Port 8080 is already in use. Stop the other mcsm-master first."
  nohup "$REPO_DIR/bin/mcsm-master" --data-dir "$DATA/master" serve --public-enroll-addr 127.0.0.1:9443 >>"$DATA/master.log" 2>&1 &
  echo $! >"$DATA/master.pid"
  wait_for 20 curl -so /dev/null "$BASE/api/auth/me" || die "The master did not start, see $DATA/master.log"
}

# The agent runs as root because access to Docker is equivalent to root anyway.
start_agent() {
  running "$DATA/agent.pid" && return
  ! port_in_use "$AGENT_PORT" || die "Port $AGENT_PORT is already in use. Stop the other mcsm-agent first."
  # shellcheck disable=SC2086 # $SUDO is empty when running as root
  nohup $SUDO "$REPO_DIR/bin/mcsm-agent" --data-dir "$DATA/agent" serve --listen "127.0.0.1:$AGENT_PORT" >>"$DATA/agent.log" 2>&1 &
  echo $! >"$DATA/agent.pid"
}

node_online() { [ "$(api "$BASE/api/nodes/$1" 2>/dev/null | py 'print(d["status"])' 2>/dev/null)" = online ]; }

initialise() {
  local password node_id token
  mkdir -p "$DATA"
  chmod 700 "$DATA"
  log "Creating the administrator account"
  password=$(python3 -c 'import secrets; print(secrets.token_hex(12))')
  printf '%s\n' "$password" | "$REPO_DIR/bin/mcsm-master" --data-dir "$DATA/master" user add admin
  printf '%s\n' "$password" >"$DATA/admin-password"
  chmod 600 "$DATA/admin-password"

  log "Starting the master"
  start_master
  api -o /dev/null -d "{\"username\":\"admin\",\"password\":\"$password\"}" "$BASE/api/auth/login"

  log "Registering this machine as node and enrolling its agent"
  read -r node_id token < <(api -d "{\"name\":\"$(hostname)\",\"address\":\"127.0.0.1:$AGENT_PORT\"}" "$BASE/api/nodes" |
    py 'print(d["node"]["id"], d["joinToken"])')
  $SUDO "$REPO_DIR/bin/mcsm-agent" --data-dir "$DATA/agent" enroll "$token"
  start_agent
  wait_for 30 node_online "$node_id" || die "The agent did not come online, see $DATA/agent.log"
  touch "$DATA/initialised"
  offer_test_server "$node_id"
}

offer_test_server() {
  local node_id=$1 answer server_id
  [ -t 0 ] || return 0
  if [ "$(api "$BASE/api/nodes/$node_id" | py 'print(d.get("info", {}).get("runtime", ""))')" = unavailable ]; then
    warn "The node can't reach Docker, skipping the test server."
    return
  fi
  read -r -p $'\nCreate and start a Paper test server on port 25565?\nThis accepts the Minecraft EULA (https://aka.ms/MinecraftEULA). [y/N] ' answer
  [[ "$answer" =~ ^[yYjJ] ]] || return 0
  log "Creating the test server (the first download of the server image takes a few minutes)"
  server_id=$(api -m 900 -d '{"name":"Lobby","type":"paper","version":"","memoryMb":2048,"port":25565,"acceptEula":true}' \
    "$BASE/api/nodes/$node_id/servers" | py 'print(d["id"])')
  api -m 120 -X POST -o /dev/null "$BASE/api/nodes/$node_id/servers/$server_id/start"
  echo "Started. On its first start the server downloads Paper; its lamp in the panel flickers until it is ready."
}

summary() {
  local hint="open it in your browser" minecraft=localhost:25565 ip
  if is_wsl; then
    hint="open it in your Windows browser"
  elif [ -n "${SSH_CONNECTION:-}" ]; then
    # The panel listens on localhost only; an SSH tunnel brings it to the operator's computer.
    read -r _ _ ip _ <<<"$SSH_CONNECTION"
    hint="on your computer run: ssh -L 8080:localhost:8080 $USER@$ip"
    minecraft=$ip:25565
    if [[ "$ip" == *:* ]]; then minecraft="[$ip]:25565"; fi
  fi
  cat <<EOF

MC Server Manager is running.

  Panel      $BASE  ($hint)
  Login      admin / $(cat "$DATA/admin-password")
  Minecraft  $minecraft
  Logs       $DATA/master.log
             $DATA/agent.log

  $SELF stop    stop master and agent
  $SELF start   start them again, e.g. after a reboot
  $SELF reset   delete all test data and servers
EOF
}

cmd_start() {
  [ -x "$REPO_DIR/bin/mcsm-master" ] || die "Nothing is built yet, run: $SELF setup"
  [ -z "$SUDO" ] || sudo -v
  start_docker
  if [ -f "$DATA/initialised" ]; then
    log "Starting master and agent"
    start_master
    start_agent
  elif [ -d "$DATA" ]; then
    die "A previous setup did not finish. Run: $SELF reset"
  else
    initialise
  fi
  summary
}

stop_process() {
  local pidfile=$1 pid
  if running "$pidfile"; then
    pid=$(cat "$pidfile")
    $SUDO kill "$pid"
    wait_for 15 stopped "$pid" || warn "Process $pid did not stop."
  fi
  rm -f "$pidfile"
}

cmd_stop() {
  stop_process "$DATA/agent.pid"
  stop_process "$DATA/master.pid"
  log "Stopped master and agent. Minecraft servers keep running in Docker."
}

cmd_reset() {
  [ -z "$SUDO" ] || sudo -v
  cmd_stop
  if docker_up; then
    # shellcheck disable=SC2086 # $SUDO is empty when running as root
    $SUDO docker ps -aq --filter label=io.mcsm.managed | xargs -r $SUDO docker rm -f >/dev/null
  fi
  $SUDO rm -rf -- "${DATA:?}"
  log "Deleted all test data and servers."
}

main() {
  trap 'warn "Something failed. The logs are in $DATA; \"$SELF reset\" starts over."' ERR
  case "${1:-setup}" in
    setup)
      command -v apt-get >/dev/null || die "This script needs Ubuntu or another Debian-based system."
      [ -z "$SUDO" ] || sudo -v
      install_packages
      relocate "$@"
      install_go
      install_node
      install_docker
      log "Building the panel and both programs"
      make -C "$REPO_DIR" build
      cmd_start
      ;;
    start) cmd_start ;;
    stop) cmd_stop ;;
    reset) cmd_reset ;;
    *) die "Usage: $SELF [setup|start|stop|reset]" ;;
  esac
}

main "$@"
