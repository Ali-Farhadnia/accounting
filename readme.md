# Personal accounting (web UI)

Small Go app with a web interface: **rial** and **gold (mg)** assets, transactions, reports, loans, and a **Talasea** gold spot cache. Data is stored in **SQLite** (file on disk).

## Requirements

- **Go 1.22+** (to build from source)
- Optional: **Docker** + **Compose** to run in a container

## Configuration

All settings use the **`ACCOUNTING_`** prefix. Copy `env.example` to `.env` and edit, or export variables in your shell / **systemd** / Docker.

| Variable | Meaning | Default |
|----------|---------|---------|
| `ACCOUNTING_HTTP_ADDR` | Listen address (`:8080` = all interfaces, port 8080) | `:8080` |
| `ACCOUNTING_DB` | Path to SQLite database file | `data/app.db` |
| `ACCOUNTING_PASSWORD` | Web login password | `changeme` (logs a warning if unset in production) |
| `ACCOUNTING_AUTH_KEY` | Secret for signing the session cookie (long random string; not the login password) | dev default if empty (not for production) |
| `ACCOUNTING_TZ` | IANA time zone for displaying dates/times (DB stores UTC) | `Asia/Tehran` |
| `ACCOUNTING_LANG` | Default UI language before the user picks EN/FA (`en` or `fa`) | `fa` |
| `ACCOUNTING_GOLD_API_URL` | Override gold price JSON API (empty = built-in Talasea URL) | *(empty)* |
| `ACCOUNTING_GOLD_RIAL_PER_TOMAN` | Rial per Toman for gold valuation (Iran: **10**) | `10` |
| `ACCOUNTING_GOLD_PRICE_SCALE` | Extra multiplier on gold Rial totals (rarely needed) | `1` |
| `ACCOUNTING_GOLD_GRAM_RIAL_SCALE` | Deprecated; used only if `ACCOUNTING_GOLD_PRICE_SCALE` is unset | `1` |

**Gold spot:** the default API value is treated as **Toman per milligram**; Rial equivalent is approximately `mg × Toman/mg × ACCOUNTING_GOLD_RIAL_PER_TOMAN` (and `ACCOUNTING_GOLD_PRICE_SCALE` if set).

## Run without Docker

```bash
cd /path/to/accounting
export ACCOUNTING_PASSWORD='your-password'
export ACCOUNTING_AUTH_KEY="$(openssl rand -hex 32)"
export ACCOUNTING_DB="$PWD/data/app.db"
go run .
# or: go build -o accounting . && ./accounting
# (use go build -mod=vendor only if you run go mod vendor first)
```

Open `http://127.0.0.1:8080` (or your host IP on port **8080**). Open the port in the host firewall / cloud security group if needed.

### Command-line overrides

- `-listen` — overrides `ACCOUNTING_HTTP_ADDR`
- `-db` — overrides `ACCOUNTING_DB`

Example: `./accounting -listen :9090 -db /var/lib/accounting/app.db`

## Run with Docker Compose

From the repository root (where `docker-compose.yml` lives):

1. Create a `.env` file next to `docker-compose.yml` (Compose loads it automatically):

   ```env
   ACCOUNTING_PASSWORD=your-strong-password
   ACCOUNTING_AUTH_KEY=paste-a-long-random-hex-string
   ```

   Generate a key: `openssl rand -hex 32`

2. Start:

   ```bash
   docker compose up -d --build
   ```

3. Open `http://SERVER_IP:8080`.

The SQLite file is stored in a **named volume** mounted at `/data` in the container (`ACCOUNTING_DB=/data/app.db`). Data survives container restarts; **do not** use `docker compose down -v` unless you intend to delete the volume. On Linux, Docker usually stores volumes under `/var/lib/docker/volumes/…` (see `docker volume ls` and `docker volume inspect`).

### Change password or other env

Edit `.env`, then recreate the container so new variables apply:

```bash
docker compose up -d --force-recreate
```

## Cross-build for Linux (deploy without compiling on the server)

On a **64-bit x86 Linux** server (`uname -m` → `x86_64`), build on your dev machine:

```bash
./scripts/deploy-linux-amd64.sh
```

This writes `build/accounting`. Optional: copy in one step:

```bash
./scripts/deploy-linux-amd64.sh ubuntu@your.server.ip
```

Defaults: remote directory `~/accounting`. A second argument overrides it (e.g. `/home/ubuntu/accounting`).

Uploads use **one `rsync` over SSH** (one password prompt with password-based login). For no prompts, use an SSH key: `ssh-keygen` then `ssh-copy-id user@server`.

The script copies:

- `build/accounting` — the binary  
- `scripts/server-run.sh` → remote `run.sh` — loads `.env` from the same directory and runs the binary  
- `.env` — **only if** `DEPLOY_ENV_FILE=/path/to/.env` is set, or if `REPO_ROOT/.env` exists locally  

**On the server**, after deploy:

```bash
cd ~/accounting
./run.sh
```

Put `ACCOUNTING_PASSWORD`, `ACCOUNTING_AUTH_KEY`, etc. in `~/accounting/.env` (copy from `env.example`), or create that file on the server by hand. To pass flags through the launcher: `./run.sh -listen :9090`.

**Run in the background and on boot (systemd):** the deployed `run.sh` can install a system service (needs `sudo` once):

```bash
cd ~/accounting
./run.sh install-service
```

Then use `sudo systemctl status accounting`, `sudo systemctl restart accounting` (e.g. after editing `.env`), and `sudo journalctl -u accounting -f` for logs. The unit reads **`EnvironmentFile=…/.env`**; use `KEY=value` lines like `env.example` (do not prefix with `export` if the service fails to start). To remove: `./run.sh uninstall-service`.

## Development notes

- **`vendor/`** is not committed; run **`go mod vendor`** if you want offline builds, then **`go build -mod=vendor`**. Docker builds run **`go mod download`** from `go.sum`.
- After changing `ACCOUNTING_PASSWORD`, users simply use the new password. Changing `ACCOUNTING_AUTH_KEY` invalidates existing sessions (cookies).
