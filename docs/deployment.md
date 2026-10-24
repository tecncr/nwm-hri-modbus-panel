# Deployment

nwm-panel is a single static binary (dashboard included) with no runtime
dependencies besides the serial ports it talks to and, optionally,
TimescaleDB.

## Behind a reverse proxy

Run nwm-panel on loopback and let the proxy terminate TLS. The defaults
(`server.listen: 127.0.0.1:8080`) are already safe for this.

What makes it proxy-friendly:

- **Client IPs** are only taken from `X-Forwarded-For` (or `server.proxy_header`)
  when the connection comes from `server.trusted_proxies`. Otherwise the
  header is ignored, so clients cannot spoof their IP to evade the
  authentication-failure limiter.
- **Path prefixes** work in both styles: the dashboard uses relative URLs
  everywhere and hash-based routing, so it needs no rewrites.
- **Security headers** are set by the app itself: a strict same-origin
  `Content-Security-Policy`, `X-Content-Type-Options: nosniff`,
  `X-Frame-Options`/`frame-ancestors 'none'`, `Referrer-Policy: no-referrer`,
  and a restrictive `Permissions-Policy`. Add HSTS at the proxy.
- **Request limits**: body size, read/write/idle timeouts, per-IP rate
  limiting, bounded database queries.
- Responses are compressed by the app; the proxy does not need to.

### Caddy: dedicated host name

```caddyfile
nwm.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

```yaml
server:
  listen: 127.0.0.1:8080
  trusted_proxies: [loopback]
```

### Caddy: sub-path, prefix stripped (recommended for sub-paths)

```caddyfile
example.com {
	redir /nwm /nwm/
	handle_path /nwm/* {
		reverse_proxy 127.0.0.1:8080
	}
}
```

Leave `server.base_path` empty: the proxy removes `/nwm` before forwarding.
The `redir` matters — the dashboard must be opened with a trailing slash so its
relative asset URLs resolve under `/nwm/`.

### Caddy: sub-path, prefix kept

```caddyfile
example.com {
	handle /nwm* {
		reverse_proxy 127.0.0.1:8080
	}
}
```

```yaml
server:
  base_path: /nwm
  trusted_proxies: [loopback]
```

With `base_path` set, every route (API and UI) is mounted under `/nwm`, and
`/nwm` redirects to `/nwm/` by itself.

### Caddy: Unix socket

```yaml
server:
  listen: unix:/run/nwm-panel/nwm-panel.sock
  trusted_proxies: [unix]
```

```caddyfile
nwm.example.com {
	reverse_proxy unix//run/nwm-panel/nwm-panel.sock
}
```

The socket is created with mode 0660; give the proxy's user group access.

### Other proxies

Any proxy works the same way. For nginx, forward `X-Forwarded-For` and use
`location /nwm/ { proxy_pass http://127.0.0.1:8080/; }` for a stripped prefix.
If the proxy runs in a container, list its network address (or `private`) in
`trusted_proxies`.

### Cross-origin clients

The dashboard is same-origin and needs no CORS. Only list origins in
`server.cors_origins` if a browser application on another origin must call the
API directly.

## systemd

`/etc/systemd/system/nwm-panel.service`:

```ini
[Unit]
Description=NWM-HRI Modbus panel
After=network-online.target
Wants=network-online.target

[Service]
User=nwm-panel
Group=nwm-panel
SupplementaryGroups=dialout
EnvironmentFile=/etc/nwm-panel/env
ExecStart=/usr/local/bin/nwm-panel serve --config /etc/nwm-panel/config.yaml
WorkingDirectory=/var/lib/nwm-panel
StateDirectory=nwm-panel
RuntimeDirectory=nwm-panel
Restart=on-failure
RestartSec=5

NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/etc/nwm-panel /var/lib/nwm-panel
DevicePolicy=closed
DeviceAllow=char-ttyUSB rw
DeviceAllow=char-ttyACM rw
DeviceAllow=char-ttyS rw
CapabilityBoundingSet=
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
LockPersonality=yes
MemoryDenyWriteExecute=yes

[Install]
WantedBy=multi-user.target
```

`/etc/nwm-panel/env` (mode 0600, owned by root):

```sh
NWM_PANEL_API_KEY=<long random string>
# NWM_PANEL_STORAGE_TIMESCALEDB_DSN=postgres://nwm_panel:secret@127.0.0.1:5432/nwm_panel?sslmode=disable
```

In the config file, use paths under the working directory or absolute paths:

```yaml
storage:
  sqlite:
    path: /var/lib/nwm-panel/nwm-panel.db
export:
  directory: /var/lib/nwm-panel/exports
```

`/etc/nwm-panel` is writable so that inventory changes made in the dashboard
are saved to `config.yaml`; drop it from `ReadWritePaths` (and expect
`"persisted": false`) for a read-only configuration.

```sh
sudo useradd --system --home /var/lib/nwm-panel --shell /usr/sbin/nologin nwm-panel
sudo install -d -o nwm-panel -g nwm-panel -m 0750 /etc/nwm-panel
sudo -u nwm-panel nwm-panel config example | sudo -u nwm-panel tee /etc/nwm-panel/config.yaml
sudo systemctl daemon-reload && sudo systemctl enable --now nwm-panel
journalctl -u nwm-panel -f
```

Set `log.format: json` if logs are shipped to a collector. The service stops
gracefully on `SIGTERM`.

## Serial ports

- Give the service user access to the port (`dialout` group on most
  distributions).
- Prefer stable names: `/dev/serial/by-id/usb-FTDI_…-port0` survives reboots
  and replugging, `/dev/ttyUSB0` may not.
- Use one bus per physical line. Two buses must never point to the same port.
- Terminate long RS-485 lines and keep the default 9600 baud unless all
  encoders were reconfigured.

## Health checks

`GET /api/v1/health` is unauthenticated and returns `200 {"status":"ok"}`
while the server runs. For deeper monitoring, poll `GET /api/v1/system`
(authenticated): recorder results, storage statistics and errors.

## Backups

- **Configuration**: the YAML file (buses and devices included).
- **SQLite**: copy safely while running with
  `sqlite3 nwm-panel.db ".backup backup.db"`.
- **TimescaleDB**: regular `pg_dump` of the database.
