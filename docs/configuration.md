# Configuration

nwm-panel is configured from four sources, in increasing order of precedence:

1. Built-in defaults
2. A YAML config file
3. Environment variables (`NWM_PANEL_*`)
4. Command-line flags

Only **measurements** are stored in the database. Everything else — server
settings, buses and devices — lives in the config file, environment and flags.

Print an annotated example to start from:

```sh
nwm-panel config example > nwm-panel.yaml
nwm-panel config validate          # load and validate, print a summary
nwm-panel config show              # print the effective config (secrets redacted)
```

## The master API key

The API key is **only** read from the `NWM_PANEL_API_KEY` environment
variable. It is never read from the config file or flags, and it never appears
in `config show` or `/api/v1/system/config`. Any non-empty string is accepted;
use a long random value:

```sh
export NWM_PANEL_API_KEY="$(openssl rand -base64 32)"
```

`nwm-panel serve` exits with an error when the variable is missing or empty.

## Config file location

`--config / -c <path>` selects a file explicitly. Otherwise the first existing
file of this list is used:

1. `./nwm-panel.yaml`, `./nwm-panel.yml`
2. `$XDG_CONFIG_HOME/nwm-panel/config.yaml` (usually `~/.config/nwm-panel/config.yaml`)
3. `/etc/nwm-panel/config.yaml`

Without any file the server runs on defaults. Buses and devices added through
the API or dashboard are then kept **in memory only** and lost on restart; the
API flags this with `"persisted": false`.

When a file is in use, inventory changes made through the API rewrite only its
`buses:` and `devices:` sections (atomically, preserving file permissions and
the comments of other sections). Unknown keys are rejected at startup to catch
typos.

## Environment variables

Every scalar setting maps to `NWM_PANEL_` + the upper-cased key path with dots
replaced by underscores:

| Setting | Variable |
|---|---|
| `server.listen` | `NWM_PANEL_SERVER_LISTEN` |
| `storage.timescaledb.dsn` | `NWM_PANEL_STORAGE_TIMESCALEDB_DSN` |
| `recorder.interval` | `NWM_PANEL_RECORDER_INTERVAL` |
| `server.trusted_proxies` | `NWM_PANEL_SERVER_TRUSTED_PROXIES` (comma-separated) |

Lists are comma-separated. `buses` and `devices` can only be defined in the
config file (or through the API).

## Value formats

- **Durations** use Go syntax: `500ms`, `30s`, `1m`, `1h30m`, `168h`. `0s` disables where noted.
- **Cron expressions** have five fields (`min hour dom month dow`) or six with
  leading seconds, and support descriptors such as `@hourly` and `@daily`.
  To pin a time zone prefix the expression: `CRON_TZ=America/Lima 0 0 * * *`.
- **Time zones** are IANA names (`America/Lima`, `Europe/Madrid`) or `Local`.

## Reference

### `server`

| Key | Default | Flag | Description |
|---|---|---|---|
| `listen` | `127.0.0.1:8080` | `--listen` | Listen address. `host:port`, or `unix:/path/to.sock` for a Unix socket (mode 0660). Keep it on loopback when behind a reverse proxy. |
| `base_path` | `""` | `--base-path` | Path prefix when the reverse proxy forwards a sub-path **without stripping it**, e.g. `/nwm`. Leave empty when served at the root or when the proxy strips the prefix. See [deployment](deployment.md). |
| `trusted_proxies` | `[]` | `--trusted-proxies` | IPs/CIDRs allowed to set the client IP header, plus the keywords `loopback`, `private`, `linklocal`, `unix`. Empty means the header is ignored and the TCP peer address is used (safe default). |
| `proxy_header` | `X-Forwarded-For` | | Header carrying the client IP from trusted proxies. |
| `read_timeout` | `30s` | | Maximum time to read a request. |
| `write_timeout` | `2m` | | Maximum time to write a response (bounds large CSV downloads). |
| `idle_timeout` | `2m` | | Keep-alive idle timeout. |
| `body_limit` | `1048576` | | Maximum request body size in bytes. |
| `cors_origins` | `[]` | | Origins allowed to call the API cross-origin. Empty = same-origin only (the embedded dashboard needs nothing). |
| `ui` | `true` | `--ui` | Serve the embedded dashboard. Set `false` for an API-only server. |
| `ui_language` | `en` | `--ui-language` | Dashboard language (`en` or `es`) for browsers whose preferred languages are all unsupported. See [dashboard languages](dashboard.md#languages). |
| `access_log` | `true` | | Log one line per HTTP request. |
| `rate_limit.enabled` | `true` | | Enable per-IP rate limiting of `/api/v1`. |
| `rate_limit.requests_per_minute` | `600` | | Requests per client IP per minute. |
| `rate_limit.auth_failures_per_minute` | `10` | | Failed API-key attempts per client IP per minute before that IP is blocked for the rest of the minute. `0` disables. |

### `log`

| Key | Default | Flag | Description |
|---|---|---|---|
| `level` | `info` | `--log-level` | `debug`, `info`, `warn` or `error`. |
| `format` | `text` | `--log-format` | `text` or `json` (structured, for log collectors). |

### `modbus`

| Key | Default | Flag | Description |
|---|---|---|---|
| `timeout` | `1s` | `--modbus-timeout` | Response timeout. Buses may override it. |
| `retries` | `1` | `--modbus-retries` | Retries after a timeout, CRC error or malformed response (0-10). Exception responses are never retried. Buses may override it. |
| `allow_raw_writes` | `true` | `--allow-raw-writes` | Allow `PUT/POST /devices/{id}/registers` (arbitrary register writes). Typed endpoints are always available. |
| `allow_broadcast` | `true` | `--allow-broadcast` | Allow broadcast writes to address 0 (`/buses/{id}/broadcast/*`). |
| `live_cache_ttl` | `2s` | | Live readings younger than this are served from memory, protecting the bus from many dashboard viewers. |

### `storage`

| Key | Default | Flag | Description |
|---|---|---|---|
| `driver` | `sqlite` | `--storage-driver` | `sqlite`, `timescaledb` or `none` (no recording, no history). |
| `retention` | `0s` | `--retention` | Delete measurements older than this. `0s` keeps everything. SQLite prunes hourly; TimescaleDB installs a native retention policy. |
| `sqlite.path` | `nwm-panel.db` | `--sqlite-path` | Database file. WAL mode is enabled, so `-wal`/`-shm` files appear beside it. |
| `timescaledb.dsn` | `""` | `--timescaledb-dsn` | PostgreSQL connection string, e.g. `postgres://user:pass@host:5432/db?sslmode=disable`. Prefer the environment variable to keep the password out of files and process lists. |
| `timescaledb.chunk_interval` | `168h` | | Hypertable partition size (one week). Applies to new chunks; changing it later is safe. |
| `timescaledb.compression_after` | `0s` | | Compress chunks older than this (segmented by device). `0s` disables. |
| `timescaledb.max_open_conns` | `10` | | Connection pool size. |

### `recorder`

| Key | Default | Flag | Description |
|---|---|---|---|
| `enabled` | `true` | `--recorder` | Record measurements periodically. Requires a storage driver. |
| `interval` | `1m` | `--recorder-interval` | Recording interval, **aligned to the wall clock**: `1m` records at every `:00` second, `15m` at `:00/:15/:30/:45`, `24h` at local midnight. |
| `cron` | `""` | `--recorder-cron` | Cron expression; overrides `interval` when set. |
| `timeout` | `30s` | | Upper bound for one pass over all devices. |

### `export`

| Key | Default | Flag | Description |
|---|---|---|---|
| `enabled` | `true` | `--export` | Write CSV files periodically. Requires a storage driver. |
| `directory` | `exports` | `--export-dir` | Output directory (created with mode 0750). Files are written atomically: a file appears only once complete. |
| `interval` | `1h` | `--export-interval` | Export interval, aligned to the wall clock (`1h` runs on the hour). |
| `cron` | `""` | `--export-cron` | Cron expression; overrides `interval`. |
| `window` | `1h` | `--export-window` | Time span of each file, ending at the scheduled time: `[tick − window, tick)`. |
| `devices` | `[]` | | Device IDs to include. Empty = all configured devices. |
| `columns` | see below | | Columns, in order. |
| `delimiter` | `,` | | Single field separator character, e.g. `;` for spreadsheets in comma-decimal locales. |
| `header` | `true` | | Write a header row. |
| `timezone` | `Local` | | Zone for timestamps and file names. |
| `time_format` | `2006-01-02T15:04:05Z07:00` | | Go time layout for the `time` and `device_clock` columns (RFC 3339 by default). |
| `filename` | `measurements_{from}_{to}.csv` | | File name pattern. Tokens: `{from}`, `{to}` (`20261023T150000`), `{date}` (`2026-10-23`), `{device}` (`all` unless `per_device`). Must end in `.csv`. |
| `per_device` | `false` | | One file per device. Without `{device}` in the pattern, `_<device>` is appended. |
| `skip_empty` | `false` | | Do not write files that would contain no rows. |
| `keep` | `0s` | | Delete export files older than this after each run. `0s` keeps everything. |

Available columns:

| Column | Content |
|---|---|
| `time` | Recording timestamp (scheduled tick) |
| `unix` | Same, as Unix seconds |
| `device_id`, `device_name`, `bus`, `slave` | Identification |
| `total_raw`, `forward_raw`, `reverse_raw` | Raw encoder counters |
| `liters_per_unit` | Conversion factor used |
| `total_liters`, `forward_liters`, `reverse_liters` | Volumes in liters |
| `total_m3`, `forward_m3`, `reverse_m3` | Volumes in cubic meters |
| `battery_status`, `failure` | Encoder health registers |
| `base_pulse_liters`, `output_pulse_liters` | Encoder pulse equivalents at recording time |
| `device_clock` | The encoder's clock at recording time |

Default: `time, device_id, device_name, slave, total_liters, forward_liters,
reverse_liters, total_m3, total_raw, forward_raw, reverse_raw, liters_per_unit,
battery_status, failure`.

Device names beginning with `=`, `+`, `-` or `@` are prefixed with `'` so
spreadsheets do not evaluate them as formulas.

### `query`

Bounds that protect the server and database from expensive API requests.

| Key | Default | Description |
|---|---|---|
| `default_page_size` | `100` | Page size when `page_size` is omitted. |
| `max_page_size` | `1000` | Largest allowed `page_size`. |
| `max_range` | `8784h` | Longest time range (366 days) a single query may cover. |
| `max_export_rows` | `500000` | Row cap for CSV downloads and each periodic export file. |
| `max_buckets` | `5000` | Maximum `range / bucket` for aggregations. |
| `timeout` | `30s` | Database query timeout (CSV downloads get four times this). |

### `buses`

A bus is one half-duplex RS-485 line (or a gateway in front of one). Requests
to all encoders on a bus are serialised.

| Key | Default | Description |
|---|---|---|
| `id` | *required* | Identifier: lowercase letters, digits, `-`, `_`; up to 64 characters; starts with a letter or digit. |
| `name` | | Display name. |
| `transport` | `serial` | `serial`, `tcp` or `simulator`. |
| `serial.device` | *required for serial* | Port, e.g. `/dev/ttyUSB0`. Prefer stable `/dev/serial/by-id/...` paths. |
| `serial.baud_rate` | `9600` | 300-921600. Encoders default to 9600. |
| `serial.data_bits` | `8` | 5-8. |
| `serial.parity` | `N` | `N`, `E` or `O`. |
| `serial.stop_bits` | `1` | `1` or `2`. |
| `tcp.address` | *required for tcp* | `host:port` of a **transparent** serial server that forwards raw RTU frames (not a Modbus TCP gateway). |
| `tcp.connect_timeout` | `5s` | TCP connect timeout. |
| `simulator.slaves` | `[]` | Addresses of simulated encoders. |
| `simulator.flow_lpm` | `10` | Mean simulated flow per encoder, liters/minute. |
| `timeout` | `modbus.timeout` | Per-bus response timeout. |
| `retries` | `modbus.retries` | Per-bus retries. |
| `frame_delay` | 3.5 characters | Minimum silence between frames (≈ 4 ms at 9600 baud; 1.75 ms floor above 19200). |

### `devices`

| Key | Default | Description |
|---|---|---|
| `id` | *required* | Identifier (same rules as bus IDs). Used in URLs, the database and CSV files. |
| `name`, `description` | | Display name (≤128) and free text (≤1024). |
| `bus` | *required* | ID of the bus. |
| `slave` | *required* | Modbus address 1-247, unique per bus. |
| `enabled` | `true` | When `false`, the device is never polled. |
| `record` | `true` | Include in periodic recordings. |
| `timezone` | server local | Zone of the encoder's real-time clock (the encoder stores local time without a zone). |
| `grouped_reads` | `false` | Read registers in small groups instead of one block. The panel also falls back automatically when a block read is rejected. |
| `meter.liters_per_unit` | `0` | Liters per counter unit. `0` = use the encoder's output pulse equivalent register (recommended). Set it to override, e.g. fractional resolutions. |
| `meter.offset_liters` | `0` | Added to net and forward volumes — e.g. the final reading of a replaced meter. |
| `meter.base_liters_per_pulse` | `0` | Expected sender resolution of the physical meter (1-10000 L/pulse). |
| `meter.output_liters_per_unit` | `0` | Expected encoder output resolution (1-1000 L/unit, ≥ base). With the previous key it can be pushed to the encoder (`POST /devices/{id}/pulse-equivalents/apply`) and mismatches are reported as warnings. Set both or neither. |
| `meter.model`, `meter.serial_number` | | Free-form identifiers of the physical meter. |
| `tags` | | Up to 32 `key: value` labels. |

See [devices and registers](devices-and-registers.md) for how counters are
converted to volume.

## Full example

```yaml
server:
  listen: 127.0.0.1:8080
  trusted_proxies: [loopback]
storage:
  driver: timescaledb
  retention: 17520h            # two years
  timescaledb:
    chunk_interval: 168h
    compression_after: 720h
recorder:
  interval: 1m
export:
  directory: /var/lib/nwm-panel/exports
  interval: 24h                # daily at local midnight…
  window: 24h                  # …with the previous day
  per_device: true
  filename: "{device}_{date}.csv"
  delimiter: ";"
  keep: 2160h
buses:
  - id: rs485-1
    transport: serial
    serial: { device: /dev/serial/by-id/usb-FTDI_FT232R-if00-port0 }
devices:
  - id: building-a
    name: Building A main meter
    bus: rs485-1
    slave: 1
    timezone: America/Lima
    meter: { base_liters_per_pulse: 10, output_liters_per_unit: 10, model: DN50 }
```

```sh
export NWM_PANEL_API_KEY=...
export NWM_PANEL_STORAGE_TIMESCALEDB_DSN='postgres://nwm_panel:secret@127.0.0.1:5432/nwm_panel?sslmode=disable'
nwm-panel --config /etc/nwm-panel/config.yaml
```
