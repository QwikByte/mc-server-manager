#!/usr/bin/env bash
# Installs or updates MC Server Manager from its GitHub releases on Linux with systemd.
#
#   install.sh master [--public-host <host>] [--admin <name>]   the panel and control plane
#   install.sh agent [--join <token>] [--install-docker]         a node that runs Minecraft servers
#   install.sh all [...]                                         both on this machine, connected to each other
#   install.sh update [--only master|agent]                      updates what is installed
#
# Options: --version <vX.Y.Z> installs that release instead of the one this script belongs to.
# Packages: .deb (apt), .rpm (dnf, yum, zypper) and Arch Linux (pacman), for x86_64 and arm64.
set -Eeuo pipefail

# The release workflow replaces "latest" with the version of the release.
version=latest
readonly REPO=https://github.com/QwikByte/mc-server-manager
readonly ENROLL_PORT=9443 AGENT_PORT=7443

public_host="" admin=admin join="" install_docker=no only="" local_agent=no fresh_master=no password=""

log() { printf '\n\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mWarning:\033[0m %s\n' "$*" >&2; }
die() { printf '\033[1;31mError:\033[0m %s\n' "$*" >&2; exit 1; }
# A download that can't connect within 20 seconds is tried again, rather than hanging.
fetch() { curl --proto '=https' --tlsv1.2 -fsSL --connect-timeout 20 --retry 3 "$@"; }
has_tty() { (: </dev/tty) 2>/dev/null; }
installed() { command -v "$1" >/dev/null; }

usage() {
  die "Usage: install.sh master|agent|all|update [--version <vX.Y.Z>] [--public-host <host>] [--admin <name>] [--join <token>] [--install-docker] [--only master|agent]"
}

# ask asks on the terminal and prints the answer, or the default for an empty one. Without a
# terminal it prints nothing.
ask() {
  local answer=""
  has_tty || return 0
  read -r -p "$1 " answer </dev/tty
  printf '%s' "${answer:-$2}"
}

parse_args() {
  while (($#)); do
    case $1 in
      --version) version=${2:?--version needs a value}; shift ;;
      --public-host) public_host=${2:?--public-host needs a value}; shift ;;
      --admin) admin=${2:?--admin needs a value}; shift ;;
      --join) join=${2:?--join needs a value}; shift ;;
      --install-docker) install_docker=yes ;;
      --only) only=${2:?--only needs a value}; shift ;;
      *) usage ;;
    esac
    shift
  done
}

check_system() {
  [ "$(id -u)" -eq 0 ] || die "Run the installer as root, e.g. with sudo."
  [ -d /run/systemd/system ] || die "MC Server Manager needs a Linux system running systemd."
  case $(uname -m) in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) die "There are no packages for $(uname -m), only for x86_64 and arm64." ;;
  esac
  if installed apt-get; then format=deb
  elif installed dnf || installed yum || installed zypper; then format=rpm
  elif installed pacman; then format=pkg.tar.zst
  else die "No supported package manager found (apt, dnf, yum, zypper or pacman). Install from the archives of a release instead."
  fi
  if [ "$version" = latest ]; then
    # GitHub redirects to the tag of the latest release.
    version=$(curl --proto '=https' --tlsv1.2 -fsS -o /dev/null -w '%{redirect_url}' "$REPO/releases/latest") || die "GitHub is not reachable."
    [[ "$version" == */releases/tag/* ]] || die "No release found, see $REPO/releases"
    version=${version##*/}
  fi
  [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || die "Invalid version: $version"
  tmp=$(mktemp -d)
  chmod 755 "$tmp" # apt reads local packages as its own user
  trap 'rm -rf "$tmp"' EXIT
  fetch -o "$tmp/checksums.txt" "$REPO/releases/download/$version/checksums.txt" || die "Release $version not found."
}

# wait_for_packages waits up to 10 minutes while another installation of packages runs, e.g.
# the automatic updates of a new server or an update of the master on the same machine, and
# says so. unattended-upgrade-shutdown always runs on Ubuntu and installs nothing.
wait_for_packages() {
  local i
  installed pgrep || return 0
  for ((i = 0; i < 120; i++)); do
    pgrep -x 'apt|apt-get|aptitude|dpkg|dnf|yum|zypper|pacman' >/dev/null ||
      pgrep -f '^/usr/bin/python3[.0-9]* /usr/bin/unattended-upgrade( |$)' >/dev/null || break
    ((i)) || printf 'Waiting for another installation of packages to finish, e.g. automatic updates'
    printf .
    sleep 5
  done
  ((i == 0)) || printf '\n'
}

# install_package downloads a package of the release, verifies its checksum and installs or updates it.
install_package() {
  local file="$1_${version#v}_linux_${arch}.$format"
  log "Installing $1 $version"
  fetch -o "$tmp/$file" "$REPO/releases/download/$version/$file"
  (cd "$tmp" && grep -E "^[0-9a-f]{64}  $file\$" checksums.txt | sha256sum -c --strict --quiet) ||
    die "The checksum of $file does not match, the download is broken."
  wait_for_packages
  # The package managers wait for an installation that is still running on their own; apt for
  # up to 5 more minutes.
  if installed apt-get; then
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --allow-downgrades -o DPkg::Lock::Timeout=300 \
      -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold "$tmp/$file" >/dev/null
  elif installed dnf; then dnf install -y -q "$tmp/$file" >/dev/null
  elif installed yum; then yum install -y -q "$tmp/$file" >/dev/null
  elif installed zypper; then ZYPP_LOCK_TIMEOUT=300 zypper --non-interactive -q install --allow-unsigned-rpm "$tmp/$file" >/dev/null
  else
    local i
    for ((i = 0; i < 300; i++)); do [ -e /var/lib/pacman/db.lck ] || break; sleep 1; done
    pacman -U --noconfirm --needed "$tmp/$file" >/dev/null
  fi
}

wait_for_port() {
  local i
  for ((i = 0; i < 30; i++)); do
    (: <"/dev/tcp/127.0.0.1/$1") 2>/dev/null && return
    sleep 1
  done
  return 1
}

setup_master() {
  if [ ! -e /var/lib/mcsm-master/master.db ]; then
    fresh_master=yes
    [ -n "$public_host" ] || public_host=$(ask "Host name or IP address under which your nodes reach this machine [$(uname -n)]:" "")
    # A host name, an IPv4 address or an IPv6 address, which has at least two colons.
    [[ "$public_host" =~ ^[A-Za-z0-9.-]*$|^[0-9A-Fa-f.]*:[0-9A-Fa-f.:]*:[0-9A-Fa-f.:]*$ ]] ||
      die "Enter a host name or IP address without port: $public_host"
    [[ "$public_host" != *:* ]] || public_host="[$public_host]"
  fi
  install_package mcsm-master
  if [ "$fresh_master" = yes ]; then
    [ -z "$public_host" ] || sed -i "s|^MCSM_MASTER_OPTS=\"|&--public-enroll-addr $public_host:$ENROLL_PORT |" /etc/mcsm/master.env
    log "Creating the administrator $admin"
    install -d -o mcsm -g mcsm -m 0700 /var/lib/mcsm-master
    if has_tty; then
      printf 'Choose the password for %s, with at least 12 characters. What you type is not shown.\n' "$admin"
      runuser -u mcsm -- mcsm-master user add "$admin" </dev/tty
    else
      password=$(head -c 18 /dev/urandom | base64)
      printf '%s\n' "$password" | runuser -u mcsm -- mcsm-master user add "$admin"
    fi
  fi
  systemctl enable --now --quiet mcsm-master
}

setup_docker() {
  if ! installed docker; then
    [ "$install_docker" = yes ] ||
      [[ $(ask "The agent runs servers in Docker, which is not installed. Install it with Docker's script from https://get.docker.com? [Y/n]" y) =~ ^[yYjJ] ]] ||
      { warn "Without Docker the node can't run servers. Install it and restart the agent: systemctl restart mcsm-agent"; return; }
    log "Installing Docker, which takes a few minutes"
    fetch -o "$tmp/get-docker.sh" https://get.docker.com
    sh "$tmp/get-docker.sh" >/dev/null
  fi
  systemctl enable --now --quiet docker || warn "Docker could not be started, see: journalctl -u docker"
}

setup_agent() {
  setup_docker
  install_package mcsm-agent
  if [ "$local_agent" = yes ]; then
    sed -i "s|^MCSM_AGENT_OPTS=.*|MCSM_AGENT_OPTS=\"--listen 127.0.0.1:$AGENT_PORT\"|" /etc/mcsm/agent.env
    wait_for_port "$ENROLL_PORT" || die "The master did not start, see: journalctl -u mcsm-master"
  fi
  if [ -n "$join" ]; then
    log "Connecting the agent to the master"
    mcsm-agent enroll "$join"
    systemctl enable --quiet mcsm-agent
    systemctl restart mcsm-agent
  elif [ -e /var/lib/mcsm-agent/pki/ca.crt ]; then
    systemctl enable --now --quiet mcsm-agent
  fi
}

# setup_all registers this machine as node of the local master and connects its agent, which
# only accepts connections from this machine.
setup_all() {
  setup_master
  if [ ! -e /var/lib/mcsm-agent/pki/ca.crt ] && [ -z "$join" ]; then
    join=$(runuser -u mcsm -- mcsm-master node add "$(uname -n)" "127.0.0.1:$AGENT_PORT" --public-enroll-addr "127.0.0.1:$ENROLL_PORT")
    local_agent=yes
  fi
  setup_agent
}

summary_master() {
  printf '\nThe master is running.\n\n'
  if [ "$fresh_master" = yes ]; then
    printf '  Sign in     as %s%s\n' "$admin" "${password:+ with the password $password, then change it in the panel}"
  fi
  cat <<EOF
  Panel       http://127.0.0.1:8080
              Serve it over HTTPS with a reverse proxy, e.g. with Caddy and this Caddyfile:
                panel.example.com {
                    reverse_proxy 127.0.0.1:8080
                }
              Until then, reach it through an SSH tunnel: ssh -L 8080:127.0.0.1:8080 root@<this machine>
  Nodes       enroll on port $ENROLL_PORT, open it for them. Add them in the panel under Nodes.
              The address they use is set in /etc/mcsm/master.env or the panel's settings.
  Settings    /etc/mcsm/master.env, then: systemctl restart mcsm-master
  Log         journalctl -u mcsm-master, or: sudo -u mcsm mcsm-master logs
EOF
}

summary_agent() {
  if [ ! -e /var/lib/mcsm-agent/pki/ca.crt ]; then
    printf '\nThe agent is installed. Add this node in the panel and run the command it shows.\n'
    return
  fi
  cat <<EOF

The agent is running.

  Firewall    allow port $AGENT_PORT only from the master's IP address
  Status      mcsm-agent status
  Settings    /etc/mcsm/agent.env, then: systemctl restart mcsm-agent
  Log         journalctl -u mcsm-agent, or: mcsm-agent logs
EOF
}

main() {
  local mode=${1:-}
  (($#)) && shift
  parse_args "$@"
  case $mode:$only in
    master: | agent: | all: | update: | update:master | update:agent) ;;
    *) usage ;;
  esac
  check_system
  # Steps run one per line: bash ignores set -e in functions called within && lists.
  case $mode in
    master)
      setup_master
      summary_master
      ;;
    agent)
      setup_agent
      summary_agent
      ;;
    all)
      setup_all
      summary_master
      summary_agent
      ;;
    update)
      local program updated=()
      for program in mcsm-master mcsm-agent; do
        if [[ -z "$only" || "$program" == "mcsm-$only" ]] && installed "$program"; then updated+=("$program"); fi
      done
      ((${#updated[@]})) || die "Nothing to update: ${only:+mcsm-$only is }not installed."
      for program in "${updated[@]}"; do install_package "$program"; done
      log "Updated to $version. Running services were restarted."
      ;;
  esac
}

# Everything runs from here, so a script cut off while downloading does nothing.
main "$@"
