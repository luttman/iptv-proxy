# Iptv Proxy

[![Actions Status](https://github.com/pierre-emmanuelJ/iptv-proxy/workflows/CI/badge.svg)](https://github.com/pierre-emmanuelJ/iptv-proxy/actions?query=workflow%3ACI)

## Description

Iptv-Proxy proxies one or more Xtream-codes IPTV backends, hiding
their real credentials behind proxy logins of your own choosing.

It's multi-user: each proxy login is assigned to exactly one upstream
Xtream-code backend, and both users and backends are managed through a
built-in web admin UI — no redeploying with new flags every time
someone is added.

Supports live, VOD, series, and full EPG.

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

1. Add an **xtream code** (a name, the upstream base URL, and the
   upstream Xtream username/password).
2. Add a **user** (a proxy-facing username/password) and assign it to
   that xtream code.

Give that user's proxy username/password to their IPTV player instead
of the real upstream credentials. They can point their player at:

```
http://proxyexample.com:8080/get.php?username=<proxy-user>&password=<proxy-pass>&type=m3u_plus&output=ts
```

— or use `player_api.php`/`xmltv.php`/the `/live`, `/movie`, `/series`
endpoints exactly like a normal Xtream-codes server, all proxied
behind their assigned backend.

Two users can be assigned to two different upstream providers and
will each only ever see their own backend — neither sees the other's
upstream credentials.

## CLI flags

| Flag | Env var | Description |
|---|---|---|
| `--port` | `PORT` | Listening port (default `8080`) |
| `--advertised-port` | `ADVERTISED_PORT` | Port advertised in proxied URLs, e.g. behind a reverse proxy (defaults to `--port`) |
| `--hostname` | `HOSTNAME` | Hostname/IP advertised in proxied URLs |
| `--https` | `HTTPS` | Use `https://` in proxied URLs |
| `--db-path` | `DB_PATH` | SQLite database file for users and xtream codes (default `./iptv-proxy.db`) |
| `--admin-user` | `ADMIN_USER` | Admin username for `/admin` (**required**) |
| `--admin-password` | `ADMIN_PASSWORD` | Admin password for `/admin` (**required**) |
| `--m3u-file-name` | `M3U_FILE_NAME` | Filename used in the `Content-Disposition` header of generated M3U files |
| `--custom-endpoint` | `CUSTOM_ENDPOINT` | Optional path prefix for all routes |
| `--m3u-cache-expiration` | `M3U_CACHE_EXPIRATION` | Hours to cache a generated M3U file before regenerating it |
| `--xtream-api-get` | `XTREAM_API_GET` | Generate `get.php` from the Xtream API instead of proxying the upstream `get.php` directly |

Admin sessions are signed with a secret generated at process start —
restarting the process logs the admin out, but doesn't affect proxy
users.

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
% docker-compose up -d
```

Then visit `http://localhost:8080/admin` to add backends and users.

## TLS - https with traefik

Put files and folders of `./traekik` folder in root repo:
```Shell
$ cp -r ./traekik/* .
```

```Shell
$ mkdir config \
        && mkdir -p Traefik/etc/traefik \
        && mkdir -p Traefik/log
```

`docker-compose` sample with traefik:
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
      # Iptv-Proxy listening port
      PORT: 8080
      # Port to expose for Xtream endpoints behind traefik
      ADVERTISED_PORT: 443
      # Hostname or IP to expose the IPTVs endpoints (for machine not for docker)
      HOSTNAME: iptv.proxyexample.xyz
      GIN_MODE: release
      # Important to activate https protocol on proxy links
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

Replace `iptv.proxyexample.xyz` in `docker-compose.yml` with your desired domain.

```Shell
$ docker-compose up -d
```

## Installation

Download latest [release](https://github.com/pierre-emmanuelJ/iptv-proxy/releases)

Or

`% go install` in root repository

**ENJOY!**

## Powered by

- [cobra](https://github.com/spf13/cobra)
- [go.xtream-codes](https://github.com/tellytv/go.xtream-codes)
- [gin](https://github.com/gin-gonic/gin)
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure-Go SQLite, no cgo)

Grab me a beer 🍻

[![paypal](https://www.paypalobjects.com/en_US/i/btn/btn_donate_LG.gif)](https://www.paypal.com/donate?hosted_button_id=WQAAMQWJPKHUN)
