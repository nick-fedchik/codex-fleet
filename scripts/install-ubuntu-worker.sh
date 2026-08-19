#!/usr/bin/env bash

set -Eeuo pipefail

readonly SCRIPT_NAME="$(basename "$0")"
readonly MIN_UBUNTU_VERSION="24.04"
readonly CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/codex-fleet"
readonly CONFIG_FILE="$CONFIG_DIR/worker.env"
readonly SSH_DIR="$HOME/.ssh"
readonly AUTHORIZED_KEYS="$SSH_DIR/authorized_keys"
readonly WORKER_KEY="${CODEX_FLEET_WORKER_KEY:-$SSH_DIR/id_ed25519_codex_fleet_worker}"
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_MASTER_KEY_FILE="$SCRIPT_DIR/../config/master.pub"
readonly MASTER_KEY_URL="${CODEX_FLEET_MASTER_KEY_URL:-https://raw.githubusercontent.com/nick-fedchik/codex-fleet/main/config/master.pub}"

usage() {
    cat <<EOF
Usage:
  $SCRIPT_NAME MASTER_HOST

MASTER_HOST is the master's DNS name or IP address. Run this script as the
already-created worker user, not as root. The user must have sudo access.

The script reads the master's public SSH key from config/master.pub when run
from a clone. It can also fetch the canonical key from GitHub, use
CODEX_FLEET_MASTER_PUBLIC_KEY, or ask for the key interactively.
EOF
}

die() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

warn() {
    printf 'warning: %s\n' "$*" >&2
}

log() {
    printf '[codex-fleet] %s\n' "$*"
}

if [[ ${1:-} == "-h" || ${1:-} == "--help" ]]; then
    usage
    exit 0
fi

if [[ $# -ne 1 ]]; then
    usage >&2
    exit 2
fi

MASTER_HOST=$1

if [[ $EUID -eq 0 ]]; then
    die "run as the pre-created worker user, not as root"
fi

if [[ $MASTER_HOST == *$'\n'* || $MASTER_HOST == *$'\r'* || $MASTER_HOST == *[[:space:]]* ]]; then
    die "MASTER_HOST must be a single hostname or IP address"
fi

if [[ ! -r /etc/os-release ]]; then
    die "/etc/os-release is missing; Ubuntu cannot be detected"
fi

# shellcheck disable=SC1091
source /etc/os-release

[[ ${ID:-} == "ubuntu" ]] || die "this installer supports Ubuntu only"
command -v dpkg >/dev/null 2>&1 || die "dpkg is required"
dpkg --compare-versions "${VERSION_ID:-0}" ge "$MIN_UBUNTU_VERSION" || \
    die "Ubuntu $MIN_UBUNTU_VERSION or newer is required (found ${VERSION_ID:-unknown})"

for command_name in apt-get getent sudo systemctl; do
    command -v "$command_name" >/dev/null 2>&1 || die "required command not found: $command_name"
done

sudo -v || die "sudo authorization is required"

WORKER_USER=$(id -un)
WORKER_HOSTNAME=$(hostname)

log "worker user: $WORKER_USER"
log "worker hostname: $WORKER_HOSTNAME"
log "master host: $MASTER_HOST"

if ! getent ahosts "$MASTER_HOST" >/dev/null 2>&1; then
    warn "MASTER_HOST does not currently resolve; saving it for later use"
fi

log "installing system dependencies"
sudo apt-get update
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    curl \
    openssh-client \
    openssh-server

if ! command -v ollama >/dev/null 2>&1; then
    log "installing Ollama"
    curl -fsSL https://ollama.com/install.sh | sh
fi

command -v ollama >/dev/null 2>&1 || die "Ollama installation did not provide the ollama command"

log "enabling SSH server"
sudo systemctl enable --now ssh

if systemctl list-unit-files ollama.service >/dev/null 2>&1; then
    log "enabling Ollama service"
    sudo systemctl enable --now ollama
else
    warn "ollama.service was not found; start Ollama using the installation's documented method"
fi

umask 077
mkdir -p "$SSH_DIR" "$CONFIG_DIR"
chmod 700 "$SSH_DIR" "$CONFIG_DIR"

MASTER_PUBLIC_KEY=${CODEX_FLEET_MASTER_PUBLIC_KEY:-}
if [[ -z "$MASTER_PUBLIC_KEY" && -f "$REPO_MASTER_KEY_FILE" ]]; then
    MASTER_PUBLIC_KEY=$(awk 'NF { if (++n > 1) exit 2; print } END { if (n != 1) exit 2 }' "$REPO_MASTER_KEY_FILE") || \
        die "invalid master public key file: $REPO_MASTER_KEY_FILE"
fi
if [[ -z "$MASTER_PUBLIC_KEY" ]]; then
    MASTER_PUBLIC_KEY=$(curl --silent --fail --show-error --max-time 10 "$MASTER_KEY_URL" 2>/dev/null || true)
fi
if [[ -z "$MASTER_PUBLIC_KEY" && -t 0 ]]; then
    printf '%s\n' 'Paste the master public SSH key (press Enter to skip):'
    IFS= read -r MASTER_PUBLIC_KEY
fi

if [[ -n "$MASTER_PUBLIC_KEY" ]]; then
    [[ $MASTER_PUBLIC_KEY != *$'\n'* && $MASTER_PUBLIC_KEY != *$'\r'* ]] || \
        die "master public key must be one line"
    printf '%s\n' "$MASTER_PUBLIC_KEY" | ssh-keygen -lf - >/dev/null 2>&1 || \
        die "master public key is not a valid OpenSSH public key"
    AUTHORIZED_KEY_ENTRY="restrict $MASTER_PUBLIC_KEY"
    touch "$AUTHORIZED_KEYS"
    chmod 600 "$AUTHORIZED_KEYS"
    if grep -Fqx -- "$AUTHORIZED_KEY_ENTRY" "$AUTHORIZED_KEYS"; then
        log "master public key is already authorized"
    else
        printf '%s\n' "$AUTHORIZED_KEY_ENTRY" >>"$AUTHORIZED_KEYS"
        log "restricted master public key added to $AUTHORIZED_KEYS"
    fi
else
    warn "master public key was not supplied; passwordless master SSH is not configured"
fi

if [[ ! -e "$WORKER_KEY" ]]; then
    log "creating worker SSH key"
    ssh-keygen -t ed25519 -f "$WORKER_KEY" -C "codex-fleet@$WORKER_HOSTNAME" -N ""
elif [[ ! -f "$WORKER_KEY" ]]; then
    die "worker key path exists but is not a regular file: $WORKER_KEY"
else
    log "keeping existing worker SSH key: $WORKER_KEY"
fi

chmod 600 "$WORKER_KEY"
chmod 644 "$WORKER_KEY.pub"

log "writing worker configuration"
{
    printf '# Generated by %s. Do not add private keys here.\n' "$SCRIPT_NAME"
    printf 'CODEX_FLEET_MASTER_HOST=%s\n' "$MASTER_HOST"
    printf 'CODEX_FLEET_WORKER_USER=%s\n' "$WORKER_USER"
    printf 'CODEX_FLEET_WORKER_HOSTNAME=%s\n' "$WORKER_HOSTNAME"
    printf 'CODEX_FLEET_WORKER_KEY=%s\n' "$WORKER_KEY"
    printf 'OLLAMA_URL=http://127.0.0.1:11434\n'
} >"$CONFIG_FILE"
chmod 600 "$CONFIG_FILE"

if ! curl --silent --fail --show-error --max-time 5 http://127.0.0.1:11434/api/tags >/dev/null; then
    warn "Ollama API is not responding yet; inspect with: sudo systemctl status ollama"
fi

printf '\nWorker installation completed.\n'
printf 'Configuration: %s\n' "$CONFIG_FILE"
printf 'Worker public key: %s.pub\n' "$WORKER_KEY"
printf '\nNext steps:\n'
printf '  1. Load a model: ollama pull MODEL_NAME\n'
printf '  2. Give the master operator this worker hostname/IP and SSH login: %s\n' "$WORKER_USER"
printf '  3. Register the worker on the master with: codex-fleet worker add ... --check\n'
