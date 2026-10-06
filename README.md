# NWM-HRI Modbus Panel

A local web console for configuring and diagnosing NWM-HRI water meter encoders over Modbus RTU. The React UI is compiled into the Go executable; no separate web server or runtime frontend install is needed.

## Build and test

Requirements: Go 1.23+, Node.js 20.19+ (or 22.12+), and npm.

```sh
make test
make build
```

The optimized, CGO-free standalone executable is written to `build/nwm-panel` for the current Go target platform. To build without Make:

```sh
cd cmd/nwm-panel/web && npm ci && npm test && npm run build
cd ../../.. && go test ./... && CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o build/nwm-panel ./cmd/nwm-panel
```

## Run

```sh
SERIAL_PORT=/dev/ttyUSB0 ./build/nwm-panel
```

Open the printed URL (by default `http://127.0.0.1:8080`). The meter uses 9600 baud, 8 data bits, no parity, and 1 stop bit as specified in the protocol guide.

| Environment variable | Default | Description |
| --- | --- | --- |
| `HTTP_ADDR` | `127.0.0.1:8080` | Web server bind address, e.g. `:8080` |
| `SERIAL_PORT` | `/dev/ttyUSB0` | Serial device path or platform port name |
| `SERIAL_BAUD` | `9600` | Serial baud rate (meter guide specifies 9600) |
| `SERIAL_TIMEOUT_MS` | `1000` | Serial response timeout in milliseconds |
| `DEFAULT_SLAVE_ID` | `15` | Initial Modbus slave address (1–247) |

The panel implements RTU function codes `0x03` (read holding registers), `0x06` (write single register), and `0x10` (write multiple registers), including CRC16 verification, broadcast writes, a mapped register browser, meter configuration forms, and a bounded in-memory transaction log. Diagnostics can scan slave IDs 1–247 when a meter address is unknown. Register writes take effect immediately; raw writes are intentionally not restricted by the read/write hints in the register map.

Flow totals are shown as raw 32-bit counts and converted liters/cubic meters using register `0x0011` (output pulse equivalent, L/pulse). The manual does not document a meter-memory clear/reset function; writable baseline registers are not treated as a reset command.

The HTTP server has no authentication. It binds to loopback by default; only bind it to a trusted network interface if remote access is required.

## Layout

- `internal/modbus`: RTU framing, CRC validation, register map, and serial transport.
- `internal/webapi`: HTTP API and embedded static asset serving.
- `cmd/nwm-panel/web`: React frontend and shadcn-style reusable UI primitives.
- `cmd/nwm-panel/web/dist`: production frontend assets embedded by `go:embed`.
