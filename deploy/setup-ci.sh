#!/usr/bin/env bash
# One-time setup for automatic deploys (.github/workflows/deploy.yml). Run on the VPS:
#
#   bash deploy/setup-ci.sh <server-ip>
#
# Creates a dedicated SSH key for GitHub (or reuses it), lets it log in, and prints the four
# values to paste into GitHub -> azaman-engine -> Settings -> Secrets and variables -> Actions.
# Run it again any time to see the values; delete ~/.ssh/github_deploy* and rerun to rotate.
set -euo pipefail

ip="${1:?usage: bash deploy/setup-ci.sh <server-ip>}"
key="$HOME/.ssh/github_deploy"

install -m 700 -d "$HOME/.ssh"
if [[ ! -f "$key" ]]; then
  ssh-keygen -q -t ed25519 -f "$key" -N "" -C "github-deploy"
fi
touch "$HOME/.ssh/authorized_keys"
chmod 600 "$HOME/.ssh/authorized_keys"
grep -qxF "$(cat "$key.pub")" "$HOME/.ssh/authorized_keys" || cat "$key.pub" >> "$HOME/.ssh/authorized_keys"

known=$(ssh-keyscan -t ed25519 "$ip" 2>/dev/null)
[[ -n "$known" ]] || { echo "ssh-keyscan got no answer from $ip: is that this server's public IP?"; exit 1; }

cat <<EOF

Add these four secrets in GitHub (Settings -> Secrets and variables -> Actions -> New repository secret):

VPS_HOST
$ip

VPS_USER
$(whoami)

VPS_KNOWN_HOSTS
$known

VPS_SSH_KEY  (everything between the lines, including BEGIN and END)
-----------------------------------------------------------------------
$(cat "$key")
-----------------------------------------------------------------------

The private key above is a password to this server: paste it into GitHub only, never into a
file in the repo or a chat.
EOF
