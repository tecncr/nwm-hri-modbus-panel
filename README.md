# nwm-panel

A Modbus RTU master, recorder and web dashboard for **NWM-HRI** pulse encoders
on water meters — in a single, dependency-free Go binary.

- **Multi-device management** — any number of RS-485 lines (local serial
  ports or transparent RTU-over-TCP gateways) and encoders, editable from the
  API or dashboard and saved back to the YAML config. Bus scanning,
  broadcast addressing and clock synchronisation for commissioning.
- **Every register** — decoded snapshots, a validated endpoint for each
  writable register (slave address, clock, device type, reverse-flow
  threshold, pulse equivalents, base readings), and raw 0x03/0x06/0x10 access.
- **Real units** — per-meter liters-per-unit (from the encoder or overridden),
  offsets for replaced meters, expected pulse equivalents with mismatch
  warnings. High-level endpoints return liters and m³, consumption over any
  period and bucketed series, no registers involved.
- **Recording** — periodic measurements (default every minute at `:00`) to
  SQLite or a TimescaleDB hypertable partitioned by week, with retention and
  compression.
- **CSV exports** — periodic files (default hourly, previous hour) with
  configurable columns, window, delimiter, file names and per-device split;
  plus on-demand downloads.
- **Safe API** — master API key, failed-auth lockout, rate limiting,
  validated and bounded queries, strict JSON, reverse-proxy aware.
- **Dashboard** — React 19 + shadcn/ui, black-and-white, embedded in the
  binary, with a tooltip on every option.

## Quick start

```sh
make build
export NWM_PANEL_API_KEY="$(openssl rand -hex 24)"
./bin/nwm-panel config example > nwm-panel.yaml   # edit buses and devices
./bin/nwm-panel
```

Open <http://127.0.0.1:8080/> and sign in with the key. No hardware at hand?
Use a `simulator` bus — see [getting started](docs/getting-started.md).

```text
nwm-panel [serve]        run the server (default command)
nwm-panel config example print an annotated config file
nwm-panel config validate
nwm-panel config show    effective configuration, secrets redacted
nwm-panel version        build details   (also: --version)
```

## Documentation

- [Getting started](docs/getting-started.md)
- [Configuration](docs/configuration.md)
- [API reference](docs/api.md)
- [Devices and registers](docs/devices-and-registers.md)
- [Recording and exports](docs/recording-and-exports.md)
- [Deployment](docs/deployment.md) (Caddy, systemd)
- [Dashboard](docs/dashboard.md)

## Development

```sh
make test                  # Go tests with the race detector
make lint                  # go vet, tsc, oxlint
NWM_PANEL_TEST_TIMESCALEDB_DSN=postgres://… make test-timescale
```

Project layout:

| Path | Content |
|---|---|
| `cmd/nwm-panel` | Entry point |
| `internal/modbus` | Modbus RTU master: framing, CRC, serial and TCP transports |
| `internal/hri` | Encoder register map, decoding, typed operations, simulator |
| `internal/config` | Configuration model, loading, validation, persistence |
| `internal/inventory` | Runtime buses and devices, unit conversion, scans |
| `internal/store` | Measurement storage (SQLite / TimescaleDB) |
| `internal/recorder`, `internal/export`, `internal/schedule` | Periodic jobs |
| `internal/api` | REST API and dashboard serving |
| `internal/cli`, `internal/version` | Command line and build metadata |
| `web/` | Dashboard source; `web/dist` is embedded at build time |
| `docs/` | Documentation |

## Releases

Pushing a `vX.Y.Z` tag that points to a commit on `master` runs the release
workflow (`.github/workflows/release.yml`): it builds the dashboard, runs
`go vet` and the tests, and publishes a GitHub release with
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) — static binaries for
Linux (amd64, arm64, armv7), macOS and Windows, plus checksums.

```sh
git tag -a v1.0.0 -m "v1.0.0" && git push origin v1.0.0
goreleaser release --snapshot --clean   # local dry run into dist/
```
