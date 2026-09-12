# iptv-proxy

[![Actions Status](https://github.com/luttman/iptv-proxy/workflows/CI/badge.svg)](https://github.com/luttman/iptv-proxy/actions?query=workflow%3ACI)

A reverse proxy for Xtream-codes IPTV services. It sits between your
IPTV players and one or more upstream Xtream-codes providers, hiding
the real upstream credentials behind proxy logins you create and
manage yourself.

It is multi-tenant: any number of proxy users can be created, each
assigned to exactly one upstream backend, managed through a built-in
web admin UI. Two users assigned to two different backends are fully
isolated from each other — neither ever sees the other's upstream
credentials.

## About this fork

This is a fork of [pierre-emmanuelJ/iptv-proxy](https://github.com/pierre-emmanuelJ/iptv-proxy),
which is a single-user, single-backend proxy configured entirely
through CLI flags. This fork rebuilds that into a multi-user,
multi-backend service with persistent storage and an admin UI on top
of the original project's Xtream-codes proxying logic. The notable
differences from upstream:

- **Multi-user, multi-backend.** Users and upstream Xtream-code
  backends are stored in SQLite and managed at runtime through
  `/admin`, instead of one hardcoded user/backend pair set at
  startup via flags.
- **Admin web UI.** Server-rendered pages for creating, editing, and
  deleting users and backends, with a live view of currently active
  streams (per-user, per-backend, with duration) and the ability to
  forcibly stop a stuck or stale stream.
- **Per-user concurrent stream limits.** Each user can be capped to
  N simultaneous streams; exceeding it evicts that user's oldest
  stream automatically.
- **Security hardening.** CSRF protection on all admin mutations,
  per-IP rate limiting on both admin and proxy login attempts,
  bcrypt-hashed passwords, `SameSite`/`Secure` session cookies.
- **Operational hardening.** Graceful shutdown on `SIGTERM`/`SIGINT`,
  connection-pooled outbound HTTP client, request-scoped timeouts
  that don't cap long-running streams, structured logging.
- **Modernized toolchain.** Updated to current Go, refreshed
  dependencies, vendoring intact, unit and integration test coverage
  for the proxy, store, and admin packages (previously none).
- **Removed:** the plain M3U-passthrough mode from upstream. It
  doesn't fit a model where a user is assigned to one of several
  Xtream backends, and supporting both would mean two parallel auth
  paths. If you need that mode, use the upstream project instead.

The underlying Xtream-codes request/response handling (live, VOD,
series, EPG, HLS) is inherited from upstream and is otherwise
unchanged.

## Quick start

```Bash
iptv-proxy --port 8080 \
           --hostname proxyexample.com \
           --admin-user admin \
           --admin-password change-me \
           --db-path ./iptv-proxy.db
```

Then open `http://proxyexample.com:8080/admin`, log in with the admin
credentials above, and:

1. Add an **xtream code** (a name, one or more upstream base URLs, and the
   upstream Xtream username/password).
2. Add a **user** (a proxy-facing username/password, and optionally a
   concurrent-stream limit) and assign it to that xtream code.

When a backend has multiple base URLs, enter one URL per line. The proxy
checks their service ports in parallel for each new request and uses the
reachable URL with the lowest connection latency.

The admin dashboard checks every configured address every 5 minutes and
shows its current status, connection latency, and uptime over the latest
60 checks. This recent history resets when the proxy restarts.

Give that user's proxy username/password to their IPTV player instead
of the real upstream credentials. They can point their player at:

```
http://proxyexample.com:8080/get.php?username=<proxy-user>&password=<proxy-pass>&type=m3u_plus&output=ts
```

or use `player_api.php`, `xmltv.php`, or the `/live`, `/movie`,
`/series` endpoints exactly like a normal Xtream-codes server, all
proxied behind their assigned backend.

## CLI flags

| Flag | Env var | Description |
|---|---|---|
| `--port` | `PORT` | Listening port (default `8080`) |
| `--advertised-port` | `ADVERTISED_PORT` | Port advertised in proxied URLs, e.g. behind a reverse proxy (defaults to `--port`) |
| `--hostname` | `HOSTNAME` | Hostname/IP advertised in proxied URLs |
| `--https` | `HTTPS` | Use `https://` in proxied URLs, and mark cookies `Secure` |
| `--db-path` | `DB_PATH` | SQLite database file for users and xtream codes (default `./iptv-proxy.db`) |
| `--encryption-key-file` | `ENCRYPTION_KEY` | Path to a mounted secret file (or, via the env var, the key value directly) holding the 32-byte AES-256 key used to encrypt upstream credentials at rest. Omit to store them in plaintext. |
| `--admin-user` | `ADMIN_USER` | Admin username for `/admin` (required) |
| `--admin-password` | `ADMIN_PASSWORD` | Admin password for `/admin` (required) |
| `--m3u-file-name` | `M3U_FILE_NAME` | Filename used in the `Content-Disposition` header of generated M3U files |
| `--custom-endpoint` | `CUSTOM_ENDPOINT` | Optional path prefix for all routes |
| `--m3u-cache-expiration` | `M3U_CACHE_EXPIRATION` | Hours to cache a generated M3U file before regenerating it |
| `--xtream-api-get` | `XTREAM_API_GET` | Generate `get.php` from the Xtream API instead of proxying the upstream `get.php` directly |

Admin sessions are signed with a secret generated at process start, so
restarting the process logs the admin out (proxy users are
unaffected). Both the admin login and proxy login are rate-limited
per source IP.

## With Docker

```Yaml
volumes:
  # SQLite database holding users and xtream-code backends.
  # Mounted as a directory so the file survives container recreation.
  - ./data:/data
ports:
  - 8080:8080
environment:
  DB_PATH: /data/iptv-proxy.db
  PORT: 8080
  HOSTNAME: localhost
  GIN_MODE: release
  ADMIN_USER: admin
  ADMIN_PASSWORD: change-me
```

```Shell
docker compose up --build -d
```

Then visit `http://localhost:8080/admin` to add backends and users.

## TLS with Traefik

Copy the `./traefik` folder's contents into the repo root:

```Shell
cp -r ./traefik/* .
mkdir -p Traefik/etc/traefik Traefik/log
```

`docker-compose` sample with Traefik:

```Yaml
version: "3"
services:
  iptv-proxy:
    build:
      context: .
      dockerfile: Dockerfile
    volumes:
      - ./data:/data
    container_name: "iptv-proxy"
    restart: on-failure
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.iptv-proxy.rule=Host(`iptv.proxyexample.xyz`)"
      - "traefik.http.routers.iptv-proxy.entrypoints=websecure"
      - "traefik.http.routers.iptv-proxy.tls.certresolver=mydnschallenge"
      - "traefik.http.services.iptv-proxy.loadbalancer.server.port=8080"
    environment:
      DB_PATH: /data/iptv-proxy.db
      PORT: 8080
      ADVERTISED_PORT: 443
      HOSTNAME: iptv.proxyexample.xyz
      GIN_MODE: release
      HTTPS: 1
      ADMIN_USER: admin
      ADMIN_PASSWORD: change-me

  traefik:
    restart: always
    image: traefik:v2.4
    read_only: true
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./Traefik/traefik.yaml:/traefik.yaml:ro
      - ./Traefik/etc/traefik:/etc/traefik/
      - ./Traefik/log:/var/log/traefik/
```

Replace `iptv.proxyexample.xyz` with your own domain, then:

```Shell
docker compose up --build -d
```

## Building from source

```Shell
go build .
```

or, to also run the test suite:

```Shell
go vet ./...
go test ./...
```

Requires Go 1.25+. The SQLite driver ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite))
is pure Go, so no cgo or C toolchain is needed to build or run this.

## Encrypting upstream credentials

Set `--encryption-key-file` (or `ENCRYPTION_KEY`) to a 32-byte AES-256
key (raw or base64) to encrypt upstream Xtream usernames/passwords at
rest instead of storing them in plaintext. Generate one with:

```Shell
openssl rand -base64 32 > /run/secrets/iptv-proxy.key
```

For an existing database with plaintext credentials, run the
migration once (it backs up the database file to
`<db-path>.bak-<timestamp>` before touching anything, and applies the
change in a single transaction):

```Shell
iptv-proxy encrypt-credentials --db-path ./iptv-proxy.db --encryption-key-file /run/secrets/iptv-proxy.key
```

The process refuses to start if the database holds encrypted
credentials but the key is missing or wrong, rather than risk sending
garbage credentials upstream.

**Backup and recovery:** the encryption key is not stored in SQLite —
back it up separately from the database file. Restoring the database
without its matching key makes the Xtream credentials unrecoverable;
restoring the key without the database is useless on its own. Keep
both together in your backup process.

## Security notes

- Passwords (both admin and proxy users) are bcrypt-hashed; nothing is
  stored in plaintext except the upstream Xtream credentials, which
  must be readable to forward requests (unless encrypted at rest, see
  above).
- Admin and proxy logins are rate-limited per source IP; only failed
  attempts count against the limit, so a legitimate player hitting
  auth-protected endpoints repeatedly is never throttled.
- All admin state-changing requests require a matching CSRF token.
- Run behind HTTPS (directly or via a reverse proxy like Traefik) and
  pass `--https` so cookies are marked `Secure`.

## Project layout

- `cmd/` — CLI entry point (Cobra/Viper)
- `pkg/server/` — Xtream-codes proxying, routing, per-request auth
- `pkg/store/` — SQLite-backed users and xtream-code backends
- `pkg/admin/` — the `/admin` web UI
- `pkg/ratelimit/` — the per-IP failure limiter used by both admin and proxy auth
- `pkg/xtream-proxy/` — thin wrapper around the upstream Xtream-codes client library

## License

GPL-3.0, inherited from the upstream project. See [LICENSE](LICENSE).

## Credits

Built on top of [pierre-emmanuelJ/iptv-proxy](https://github.com/pierre-emmanuelJ/iptv-proxy).
Also uses [cobra](https://github.com/spf13/cobra), [gin](https://github.com/gin-gonic/gin),
[go.xtream-codes](https://github.com/tellytv/go.xtream-codes), and
[modernc.org/sqlite](https://gitlab.com/cznic/sqlite).
