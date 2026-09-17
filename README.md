# iptv-proxy

[![Actions Status](https://github.com/luttman/iptv-proxy/workflows/CI/badge.svg)](https://github.com/luttman/iptv-proxy/actions?query=workflow%3ACI)

A reverse proxy for Xtream-codes IPTV services. Create proxy logins for
players while keeping upstream credentials on the server.

- Manage providers, addresses, credentials, and users through `/admin`.
- Assign each user to an upstream account and set concurrent-stream limits.
- Proxy live TV, VOD, series, playlists, and EPG.
- Monitor active streams, bandwidth, provider health, and subscription expiry.
- Store configuration in SQLite, with optional encryption of upstream credentials.

## Dashboard

![Dashboard overview](docs/dashboard.png)

Use **Manage** to edit a provider's addresses and credentials.

![Manage popup](docs/manage-popup.png)

## About this fork

Based on [pierre-emmanuelJ/iptv-proxy](https://github.com/pierre-emmanuelJ/iptv-proxy),
with multiple users and providers, persistent storage, and a web admin UI.
The original plain M3U-passthrough mode is not supported.

## Quick start

```sh
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
   concurrent-stream limit) and assign it to a credential on that provider.

When a provider has multiple base URLs, enter one URL per line. Address
selection uses monitored TCP latency, keeps the current address when the
latency difference is small, and probes directly when no healthy monitoring
data is available.

Health checks run every 5 minutes. The dashboard shows the latest 60 checks
and 24-hour/7-day uptime. Check history is stored in SQLite and retained for
7 days across restarts.

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

The included [docker-compose.yml](docker-compose.yml) builds the app locally
and stores the database in `./data`. Set `HOSTNAME`, `ADMIN_USER`, and
`ADMIN_PASSWORD` before starting it:

```sh
docker compose up --build -d
```

Then visit `http://localhost:8080/admin` to add providers and users.

For public access, terminate TLS with a reverse proxy and set `HTTPS=1`,
`ADVERTISED_PORT=443`, and `HOSTNAME` to your domain. `HTTPS` changes generated
URLs and cookie settings; the app itself still listens over HTTP.
The [Traefik examples](traefik/) are optional and need your own domain and
certificate configuration. They reference Traefik v2.4; review the image
version before using them.

## Outbound proxy settings

Open the **Proxy settings** popup from the admin dashboard to configure an outbound
HTTP, HTTPS, or SOCKS5 proxy. Choose **Custom proxy**, enter a URL, enable it,
and save. Use **Test proxy** to check the selected proxy before saving: it
reports the exit IP and response time through ipify, without changing settings.
The check has a 10-second timeout and does not measure streaming bandwidth.
An authenticated URL can use `http://username:password@host:port`.

You can also choose **Environment variables** to use Go's
[standard environment variables](https://pkg.go.dev/net/http#ProxyFromEnvironment).
Set both variables to route requests to HTTP and HTTPS providers:

```sh
export HTTP_PROXY=http://proxy.example.com:3128
export HTTPS_PROXY=http://proxy.example.com:3128
iptv-proxy --hostname localhost --admin-user admin --admin-password change-me
```

For SOCKS5, use `socks5://proxy.example.com:1080` as the value of both
variables. Go also supports `socks5h://` with proxy-side DNS resolution.
An authenticated proxy URL can use `scheme://username:password@host:port`;
URL-encode special characters in the username and password.

With Docker, add these entries under the `iptv-proxy` service's `environment`
block, replacing the URL with your proxy address:

```yaml
HTTP_PROXY: socks5://proxy.example.com:1080
HTTPS_PROXY: socks5://proxy.example.com:1080
NO_PROXY: localhost,127.0.0.1
```

The popup shows environment proxy addresses with authentication hidden. A proxy
defaults to enabled when an environment proxy is configured; otherwise it is
disabled. Your saved enable/disable choice is preserved.
Its enable switch can bypass the environment proxy without changing those values.
UI settings are stored in SQLite and apply to new requests immediately; active
streams keep their connections. A saved custom URL overrides environment settings
when **Custom proxy** is selected. Leave the URL blank to keep the saved value.
Custom mode routes all provider HTTP requests through that proxy, without
`NO_PROXY` bypasses. Proxy settings are encrypted at rest when an encryption key
is configured; otherwise they are stored in plaintext.

Restart the process or recreate the container after changing environment variables.
In environment mode, `NO_PROXY` bypasses the proxy for listed hosts, as do
localhost and loopback addresses. `ALL_PROXY` is not used by this app's HTTP
transport. Settings apply across providers, with no per-provider toggle.

An outbound proxy is useful when you need another network route or public
exit IP. It adds a network hop and must support your streaming bandwidth;
SOCKS5 alone does not encrypt traffic.

**Limitation:** health checks and address selection still make direct TCP
connections. Their status and latency do not describe the proxied route.
A provider reachable only through the proxy can fail selection when multiple
addresses are enabled. Use one enabled address per provider in that case.
This setting does not route all server traffic through the proxy.

## Building from source

```sh
go build .
```

or, to also run the test suite:

```sh
go vet ./...
go test ./...
```

Requires Go 1.25+. The SQLite driver ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite))
is pure Go, so no cgo or C toolchain is needed to build or run this.

## Encrypting upstream credentials

Set `--encryption-key-file` (or `ENCRYPTION_KEY`) to a 32-byte AES-256
key (raw or base64) to encrypt upstream Xtream usernames/passwords at
rest instead of storing them in plaintext. Generate one with:

```sh
openssl rand -base64 32 > /run/secrets/iptv-proxy.key
```

For an existing database with plaintext credentials, run the
migration once (it backs up the database file to
`<db-path>.bak-<timestamp>` before touching anything, and applies the
change in a single transaction):

```sh
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
- Terminate HTTPS at a reverse proxy and
  pass `--https` so cookies are marked `Secure`.

## License

GPL-3.0, inherited from the upstream project. See [LICENSE](LICENSE).

## Credits

Built on top of [pierre-emmanuelJ/iptv-proxy](https://github.com/pierre-emmanuelJ/iptv-proxy).
Also uses [cobra](https://github.com/spf13/cobra), [gin](https://github.com/gin-gonic/gin),
[go.xtream-codes](https://github.com/tellytv/go.xtream-codes), and
[modernc.org/sqlite](https://gitlab.com/cznic/sqlite).
