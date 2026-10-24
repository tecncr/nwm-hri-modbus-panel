# Getting started

## Requirements

- To run: Linux, macOS or Windows; an RS-485 adapter or a transparent
  serial-to-Ethernet gateway for real encoders.
- To build: Go 1.27+, Node.js 24+ and pnpm, `make`.

## Build

```sh
make build            # dashboard + optimized static binary in bin/nwm-panel
make install-local    # copy it to ~/.local/bin
```

Other targets: `make build-debug`, `make install-global` (to
`/usr/local/bin`, override with `PREFIX=`), `make uninstall-local`,
`make uninstall-global`, `make test`, `make lint`, `make clean`,
`make distclean`, `make help`. Cross-compile with
`make build GOOS=linux GOARCH=arm64` — the binary is built with
`CGO_ENABLED=0`.

Builds from a `vX.Y.Z` tag embed that version and commit
(`nwm-panel --version` → `nwm-panel v1.2.3`); other builds report
`nwm-panel development build`. `nwm-panel version` prints all build details.

## Try it without hardware

The simulator transport emulates encoders at the Modbus frame level.

```yaml
# nwm-panel.yaml
buses:
  - id: demo
    transport: simulator
    simulator:
      slaves: [1, 2, 3]
      flow_lpm: 20
devices:
  - { id: building-a, name: Building A, bus: demo, slave: 1 }
  - { id: building-b, name: Building B, bus: demo, slave: 2 }
  - { id: garden, name: Garden, bus: demo, slave: 3 }
```

```sh
export NWM_PANEL_API_KEY="$(openssl rand -hex 24)"
echo "$NWM_PANEL_API_KEY"
nwm-panel                     # same as: nwm-panel serve
```

Open <http://127.0.0.1:8080/> and sign in with the key. Measurements are
recorded every minute into `nwm-panel.db`; a CSV with the previous hour lands
in `exports/` on the hour.

## Connect real encoders

1. Create a config file: `nwm-panel config example > nwm-panel.yaml`.
2. Define a bus for your adapter (`/dev/ttyUSB0`, 9600 8N1 by default).
3. Start the server, open **Devices**, and **Scan** the bus — or add devices
   at known addresses.
4. For each encoder, open **Registers** to check the pulse equivalents,
   synchronise the clock, and preset the counters to the meter's mechanical
   reading.

Devices added in the dashboard are saved back to the config file.

## Using the API

```sh
KEY=$NWM_PANEL_API_KEY
curl -s -H "Authorization: Bearer $KEY" http://127.0.0.1:8080/api/v1/meters
curl -s -H "Authorization: Bearer $KEY" "http://127.0.0.1:8080/api/v1/meters/building-a/consumption?from=-24h"
curl -s -H "Authorization: Bearer $KEY" "http://127.0.0.1:8080/api/v1/measurements/csv?from=-7d&device=building-a" -o week.csv
```

See the [API reference](api.md).

## Next steps

- [Configuration](configuration.md) — every option.
- [Devices and registers](devices-and-registers.md) — unit conversion and workflows.
- [Recording and exports](recording-and-exports.md) — SQLite, TimescaleDB, CSV files.
- [Deployment](deployment.md) — Caddy, systemd, serial ports.
- [Dashboard](dashboard.md) — the web UI and how to develop it.
