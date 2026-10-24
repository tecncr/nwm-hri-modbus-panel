# Devices and registers

## Communication

NWM-HRI encoders are Modbus RTU slaves on a half-duplex RS-485 line. The
factory serial format is **9600 baud, 8 data bits, no parity, 1 stop bit**.
nwm-panel acts as the single master and implements the three function codes
the encoders support:

| Code | Function | Used for |
|---|---|---|
| `0x03` | Read holding registers (1-125) | All reads |
| `0x06` | Write single register | Single-value writes |
| `0x10` | Write multiple registers (1-123) | Multi-register values (clock, pulse equivalents, base readings) |

Valid slave addresses are 1-247; address 0 broadcasts to every encoder on the
bus. Encoders silently ignore frames addressed to someone else.

All requests to the same bus are serialised and separated by the RTU
inter-frame silence (3.5 character times). Failed requests (timeout, CRC
error, malformed response) are retried `modbus.retries` times; Modbus
exception responses are reported immediately. A transport error closes the
connection, which is reopened on the next request.

### Transports

- **serial** — a local RS-485 adapter (`/dev/ttyUSB0`, …). The user running
  nwm-panel needs access to the device (on most distributions, membership of
  the `dialout` group).
- **tcp** — a serial-to-Ethernet converter in *transparent* (raw/TCP server)
  mode that passes RTU frames through unchanged. Gateways that translate to
  Modbus TCP are not supported by this transport.
- **simulator** — in-memory encoders that implement the full register map at
  the frame level, with realistic daily consumption patterns. Use it to try
  the dashboard and API without hardware.

## Register map

All values are 16-bit holding registers. 32-bit values span two registers,
**high word first**.

| Address | Register | Access | Encoding |
|---|---|---|---|
| `0x0000` | Slave address | R/W | 1-247 |
| `0x0001-0x0002` | Accumulated (net) flow | R | uint32, output units |
| `0x0003-0x0004` | Forward flow | R | uint32, output units |
| `0x0005-0x0006` | Reverse flow | R | uint32, output units |
| `0x0007` | Clock: year / month | R/W | high byte: year − 2000; low byte: month |
| `0x0008` | Clock: day / hour | R/W | high byte: day; low byte: hour |
| `0x0009` | Clock: minute / second | R/W | high byte: minute; low byte: second |
| `0x000A` | Device type ID | R/W | slave type code |
| `0x000B` | Firmware year | R | high byte: century digits, low byte: year digits (`0x1412` = 2018) |
| `0x000C` | Firmware month / day | R | high byte: month; low byte: day |
| `0x000D` | Reverse flow threshold | R/W | encoder units |
| `0x000E` | Battery status | R | status code |
| `0x000F` | Failure | R | failure code, 0 = none |
| `0x0010` | Base pulse equivalent | R/W | 1-10000 L/pulse |
| `0x0011` | Output pulse equivalent | R/W | 1-1000 L/unit, ≥ base |
| `0x0012-0x0013` | Forward base reading | R/W | uint32, output units |
| `0x0014-0x0015` | Reverse base reading | R/W | uint32, output units |
| `0x0019-0x001B` | Debug numbers | R | six bytes, two per register |

The accumulated flow equals forward minus reverse (32-bit arithmetic). Writing
the base readings presets the forward and reverse counters.

Every register is reachable through the API: decoded via
`GET /devices/{id}/snapshot`, through a typed endpoint for each writable
register, and raw through `GET/PUT/POST /devices/{id}/registers`. See the
[API reference](api.md#registers).

### Reading strategy

Snapshots read `0x0000-0x0015` in one request and the debug block separately.
Recordings and live readings read `0x0001-0x0011` (counters, clock, health and
pulse equivalents) in one request. If the encoder rejects a block read with an
exception, the panel falls back to smaller groups automatically;
`grouped_reads: true` forces groups from the start.

## From pulses to liters

Two encoder registers describe the physical installation:

- **Base pulse equivalent** — liters per pulse emitted by the meter's sender
  (the meter's nameplate value, e.g. 10 L/pulse).
- **Output pulse equivalent** — the resolution of the counters reported over
  Modbus, in liters per unit. It must not be lower than the base value.

nwm-panel converts counters to volume per device:

```
liters_per_unit = meter.liters_per_unit        if set (> 0)
                  else output pulse equivalent  (read from the encoder)

net_liters      = net_counter     × liters_per_unit + meter.offset_liters
forward_liters  = forward_counter × liters_per_unit + meter.offset_liters
reverse_liters  = reverse_counter × liters_per_unit
m³              = liters / 1000
```

This lets one panel serve meters with different resolutions:

| Situation | Configuration |
|---|---|
| Encoder correctly configured (normal case) | Leave `liters_per_unit: 0`; the encoder register is used. |
| Fractional or externally scaled resolution | Set `liters_per_unit`, e.g. `2.5`. |
| Meter replaced; continue the old reading | Set `offset_liters` to the old meter's final reading. |
| Enforce a known installation | Set `base_liters_per_pulse` and `output_liters_per_unit`. Readings warn on mismatch, and `POST /devices/{id}/pulse-equivalents/apply` (or *Apply expected* in the dashboard) writes them. |

Each measurement stores the factor used, so history stays correct if the
configuration changes later. When the net counter wraps below zero (more
reverse than forward flow) the sign is recovered.

## Clocks and time zones

The encoder clock stores local date and time without a zone. Each device has a
`timezone` (default: the server's zone) used to interpret and set it.
Readings include `device_clock` and `clock_drift_seconds` (device minus
server). Synchronise a single encoder with `PUT /devices/{id}/clock` (empty
body) or all encoders on a bus with `POST /buses/{id}/broadcast/clock`.

## Workflows

### Commissioning a new encoder

1. Wire the encoder alone on a bus (or make sure no other encoder shares its
   address).
2. If its address is unknown, use **Broadcast slave address** (Devices → bus
   actions) to assign one; otherwise **Scan** the bus to find it.
3. **Add** the device from the scan results (or manually), with the meter's
   model, time zone and expected pulse equivalents.
4. On the device's **Registers** page: *Apply expected* pulse equivalents,
   *Synchronise* the clock, and preset the **base readings** to the meter's
   mechanical register (in liters).

### Re-addressing

`PUT /devices/{id}/slave-address` writes the new address and updates the
configuration in one step; the panel refuses addresses already used on the
bus. Raw writes to register `0x0000` are kept in sync the same way.

### Safety switches

- `modbus.allow_raw_writes: false` disables arbitrary register writes; typed,
  validated endpoints remain available.
- `modbus.allow_broadcast: false` disables broadcast writes.
- The dashboard asks for confirmation before re-addressing, presetting
  counters, raw writes and deletions.
