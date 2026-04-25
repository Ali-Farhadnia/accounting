#!/usr/bin/env bash
# On the server: keep next to the `accounting` binary, with .env in the same directory.
#
# Foreground (Ctrl+C to stop):
#   ./run.sh
#   ./run.sh -listen :9090
#
# Install as a systemd service (start on boot, restart on failure, run in background):
#   ./run.sh install-service
# Then:  sudo systemctl status accounting
# After changing .env:  sudo systemctl restart accounting
#
# Remove the service:
#   ./run.sh uninstall-service
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"
BIN="$DIR/accounting"
UNIT="accounting"
UNIT_PATH="/etc/systemd/system/${UNIT}.service"

case "${1:-}" in
-h|--help|help)
	sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'
	;;
install-service|install)
	if [[ ! -f "$BIN" ]]; then
		echo "error: missing $BIN" >&2
		exit 1
	fi
	chmod +x "$BIN" 2>/dev/null || true
	if [[ ! -f "$DIR/.env" ]]; then
		echo "error: create $DIR/.env first (see env.example)" >&2
		exit 1
	fi
	# User that should own the process: directory owner (works when you deploy as ubuntu).
	SVC_USER="$(stat -c '%U' "$DIR" 2>/dev/null || id -un)"
	SVC_GROUP="$(stat -c '%G' "$DIR" 2>/dev/null || id -gn)"
	if [[ -z "$SVC_USER" || "$SVC_USER" == "root" ]]; then
		echo "error: $DIR should be owned by a normal user, not root" >&2
		exit 1
	fi
	if ! command -v systemctl &>/dev/null; then
		echo "error: systemctl not found (need systemd, e.g. Ubuntu Server)" >&2
		exit 1
	fi
	# Systemd EnvironmentFile: KEY=VALUE lines only; avoid 'export' in .env
	tmp="$(mktemp)"
	{
		echo "[Unit]"
		echo "Description=Personal accounting web"
		echo "After=network-online.target"
		echo "Wants=network-online.target"
		echo ""
		echo "[Service]"
		echo "Type=simple"
		echo "User=$SVC_USER"
		echo "Group=$SVC_GROUP"
		echo "WorkingDirectory=$DIR"
		echo "EnvironmentFile=$DIR/.env"
		echo "ExecStart=$BIN"
		echo "Restart=on-failure"
		echo "RestartSec=5"
		echo ""
		echo "[Install]"
		echo "WantedBy=multi-user.target"
	} >"$tmp"
	sudo install -m 0644 -T "$tmp" "$UNIT_PATH"
	rm -f "$tmp"
	sudo systemctl daemon-reload
	sudo systemctl enable --now "$UNIT"
	echo "Service installed. Commands:"
	echo "  sudo systemctl status $UNIT"
	echo "  sudo systemctl restart $UNIT   # after editing .env"
	;;
uninstall-service|uninstall)
	if ! command -v systemctl &>/dev/null; then
		exit 0
	fi
	if [[ -f "$UNIT_PATH" ]]; then
		sudo systemctl disable --now "$UNIT" 2>/dev/null || true
		sudo rm -f "$UNIT_PATH"
		sudo systemctl daemon-reload
		echo "Removed $UNIT_PATH"
	else
		echo "No unit at $UNIT_PATH (nothing to do)"
	fi
	;;
*)
	if [[ -f .env ]]; then
		set -a
		# shellcheck disable=SC1091
		source .env
		set +a
	fi
	exec "$BIN" "$@"
	;;
esac
