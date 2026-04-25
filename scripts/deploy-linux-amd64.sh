#!/usr/bin/env bash
# Cross-build a static Linux x86_64 binary and optionally copy it to a server.
# Run from any directory; the script locates the repo root (parent of scripts/).
#
# Usage:
#   ./scripts/deploy-linux-amd64.sh
#   ./scripts/deploy-linux-amd64.sh ubuntu@server.example
#   ./scripts/deploy-linux-amd64.sh ubuntu@server.example ~/accounting
#
# Optional: copy an env file to the server as .env (same vars as env.example / readme).
#   DEPLOY_ENV_FILE=/path/to/.env ./scripts/deploy-linux-amd64.sh user@host ~/accounting
# If DEPLOY_ENV_FILE is unset, defaults to a file named .env in the repo root (only if it exists).
#
# On the server, run the app with:  ~/accounting/run.sh
#   (run.sh sources .env and execs the binary; create .env on the server or use DEPLOY_ENV_FILE)
#
# Password prompts: with password-based SSH, each separate ssh/scp connection asks again.
# This script uses a single rsync to upload (one connection). For no password at all:
#   ssh-copy-id user@server
#
# Environment (optional):
#   OUT_NAME         output filename (default: accounting)
#   OUT_DIR          build output directory under repo (default: build)
#   DEPLOY_ENV_FILE  file to copy to REMOTE_DIR/.env (default: $REPO_ROOT/.env if present)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

OUT_NAME="${OUT_NAME:-accounting}"
OUT_DIR="${OUT_DIR:-build}"
mkdir -p "$OUT_DIR"
OUT_FILE="$OUT_DIR/$OUT_NAME"

echo "Building $OUT_FILE (linux/amd64, static)…"
BUILD_FLAGS=(-trimpath -ldflags="-s -w" -o "$OUT_FILE" .)
if [[ -f "$REPO_ROOT/vendor/modules.txt" ]]; then
	BUILD_FLAGS=(-mod=vendor "${BUILD_FLAGS[@]}")
fi
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build "${BUILD_FLAGS[@]}"

ls -la "$OUT_FILE"
echo "OK: $REPO_ROOT/$OUT_FILE"

if [[ -n "${1:-}" ]]; then
	SSH_TARGET="$1"
	REMOTE_DIR="${2:-~/accounting}"
	RUN_SH_SRC="$REPO_ROOT/scripts/server-run.sh"
	STAGE="$(mktemp -d "${TMPDIR:-/tmp}/accounting-deploy.XXXXXX")"
	trap 'rm -rf "$STAGE"' EXIT
	cp -f "$OUT_FILE" "$STAGE/$OUT_NAME"
	cp -f "$RUN_SH_SRC" "$STAGE/run.sh"
	chmod +x "$STAGE/$OUT_NAME" "$STAGE/run.sh"

	ENV_TO_COPY="${DEPLOY_ENV_FILE-}"
	if [[ -z "$ENV_TO_COPY" && -f "$REPO_ROOT/.env" ]]; then
		ENV_TO_COPY="$REPO_ROOT/.env"
	fi
	if [[ -n "$ENV_TO_COPY" ]]; then
		if [[ ! -f "$ENV_TO_COPY" ]]; then
			echo "error: DEPLOY_ENV_FILE or .env not found: $ENV_TO_COPY" >&2
			exit 1
		fi
		cp -f "$ENV_TO_COPY" "$STAGE/.env"
		echo "Syncing to ${SSH_TARGET}:${REMOTE_DIR}/ (binary, run.sh, .env) — one SSH connection…"
	else
		echo "Syncing to ${SSH_TARGET}:${REMOTE_DIR}/ (binary, run.sh; add repo .env or DEPLOY_ENV_FILE to include .env) — one SSH connection…"
	fi

	RSYNC_CMD=(rsync -avz -e ssh)
	if rsync --help 2>&1 | grep -q -- --mkpath; then
		RSYNC_CMD+=(--mkpath)
	else
		ssh "$SSH_TARGET" "mkdir -p $REMOTE_DIR"
	fi
	"${RSYNC_CMD[@]}" "$STAGE"/ "$SSH_TARGET:$REMOTE_DIR/"
	echo "Done. On the server: cd $REMOTE_DIR && ./run.sh"
fi
