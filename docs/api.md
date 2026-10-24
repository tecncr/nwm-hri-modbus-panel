# REST API

All endpoints live under `/api/v1` (prefixed by `server.base_path` when set)
and exchange JSON. The dashboard uses exactly this API.

- [Conventions](#conventions)
- [System](#system) · [Buses](#buses) · [Devices](#devices)
- [Registers](#registers) — low level, for integrators and technicians
- [Meters](#meters) — high level, for consumers of measurements
- [Measurements](#measurements) · [Recorder](#recorder) · [Exports](#exports)

## Conventions

### Authentication

Every endpoint except `GET /api/v1/health` requires the master API key, as
either header:

```http
Authorization: Bearer <key>
X-API-Key: <key>
```

Missing or wrong keys return `401`. After
`server.rate_limit.auth_failures_per_minute` failures from one client IP, that
IP receives `429` (with `Retry-After`) until the minute is over — even with the
correct key. Keys are never accepted in query strings, so they do not leak
into logs.

### Errors

Errors share one envelope:

```json
{ "error": { "code": "invalid_request", "message": "page_size must be an integer between 1 and 1000" } }
```

| Status | `code` | Meaning |
|---|---|---|
| 400 | `invalid_request` | Validation failed (body, parameters, value ranges). |
| 401 | `unauthorized` | Missing or invalid API key. |
| 403 | `forbidden` | Disabled by configuration (raw writes, broadcast). |
| 404 | `not_found` | Unknown bus, device, file or endpoint. |
| 409 | `conflict` | Duplicate ID, slave address in use, bus still referenced, device disabled. |
| 413 | `payload_too_large` | Body exceeds `server.body_limit`. |
| 429 | `rate_limited` | Rate limit or authentication-failure limit reached. |
| 502 | `modbus_exception` | The encoder answered with a Modbus exception. `details` holds `function`, `exception_code` and `exception` (e.g. `illegal data value`). |
| 502 | `bad_response` | CRC error or malformed response from the encoder. |
| 503 | `storage_disabled` | Endpoint needs a database but `storage.driver` is `none`. |
| 504 | `timeout` | The encoder did not answer in time. |
| 500 | `internal_error` | Unexpected server error (details are logged, not returned). |

### Bodies

Request bodies are strict JSON objects: unknown fields are rejected, which
catches typos.

### Time parameters

`from` and `to` accept:

- RFC 3339: `2026-10-23T15:00:00Z`, `2026-10-23T10:00:00-05:00`
- Unix seconds: `1792760400`
- `now`, or a signed duration relative to now: `-24h`, `-90m`
- Local date or date-time (server zone): `2026-10-23`, `2026-10-23T10:00`

`from` is inclusive, `to` exclusive. Defaults: `to=now`, `from=to−24h`. The
range may not exceed `query.max_range`.

`device` (alias `devices`) is a comma-separated list of up to 100 device IDs.

### Inventory responses

Mutations of buses and devices include whether the change was saved to the
config file:

```json
{ "inventory": { "persisted": false, "warning": "no config file in use: inventory changes are kept in memory only and lost on restart" } }
```

---

## System

### `GET /health`

Unauthenticated liveness probe: `{"status":"ok"}`.

### `GET /auth`

Validates the API key: `{"ok":true}`.

### `GET /system`

Version and build, uptime, config file, inventory counts, storage driver and
statistics, recorder and exporter status, Modbus options and query limits.

### `GET /system/config`

The effective configuration (defaults + file + environment + flags), with the
TimescaleDB password redacted.

### `GET /registers`

The documented register map (`key`, `address`, `words`, `access`, `title`,
`description`), value limits, and the available CSV export columns.

---

## Buses

### `GET /buses`

```json
{ "buses": [ {
  "config": { "id": "rs485-1", "name": "Main line", "transport": "serial",
              "serial": { "device": "/dev/ttyUSB0", "baud_rate": 9600, "data_bits": 8, "parity": "N", "stop_bits": 1 },
              "tcp": { "address": "", "connect_timeout": "0s" },
              "simulator": { "slaves": null, "flow_lpm": 0 },
              "timeout": "0s", "retries": null, "frame_delay": "0s" },
  "stats": { "requests": 1520, "failures": 3, "timeouts": 3, "crc_errors": 0, "exceptions": 0,
             "last_error": "modbus: response timeout", "last_error_at": "…", "last_ok_at": "…", "connected": true },
  "devices": ["building-a", "building-b"]
} ] }
```

### `GET /buses/{bus}`

One bus, same shape.

### `POST /buses` · `PUT /buses/{bus}`

Create or replace a bus. The body is the `config` object (see
[configuration](configuration.md#buses)); durations are strings. On `PUT` the
`id` may be omitted but cannot change. Replacing a bus reconnects it.
Returns `201`/`200` with `{ "bus": …, "inventory": … }`.

```sh
curl -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"id":"gw1","transport":"tcp","tcp":{"address":"192.168.1.50:4001"}}' \
  http://127.0.0.1:8080/api/v1/buses
```

### `DELETE /buses/{bus}`

Deletes a bus. `409` while devices still reference it.

### `POST /buses/{bus}/scan`

Starts an asynchronous scan reading the slave-address register of each
address. Body (all optional):

| Field | Default | Range |
|---|---|---|
| `from` | `1` | 1-247 |
| `to` | `247` | 1-247, ≥ `from` |
| `timeout_ms` | `300` | 20-5000 per address |

Returns `202` with the job. Only one scan per bus runs at a time (`409`).
Regular traffic continues between probes.

### `GET /buses/{bus}/scan`

Progress and results of the current or last scan:

```json
{ "bus": "rs485-1", "from": 1, "to": 247, "timeout_ms": 300, "current": 120, "progress": 0.48,
  "done": false, "cancelled": false, "started_at": "…",
  "found": [ { "slave": 23, "slave_address_register": 23, "device_type": 1,
               "firmware": { "year": 2018, "month": 8, "day": 1, "text": "2018-08-01" },
               "configured_as": "building-a" } ] }
```

### `DELETE /buses/{bus}/scan`

Cancels a running scan.

### `POST /buses/{bus}/broadcast/slave-address`

Writes a slave address to **every** encoder on the bus via broadcast address
0 — for commissioning a single encoder whose address is unknown. Requires
`modbus.allow_broadcast`.

```json
{ "slave": 23, "confirm": true }
```

`confirm` must be `true`.

### `POST /buses/{bus}/broadcast/clock`

Sets every encoder's clock. Body (optional): `{"time": "RFC 3339", "timezone": "America/Lima"}`;
defaults to the current server time in the server zone. Broadcasts are not
acknowledged by the encoders.

---

## Devices

### `GET /devices`

```json
{ "devices": [ {
  "config": { "id": "building-a", "name": "Building A", "description": "", "bus": "rs485-1", "slave": 1,
              "enabled": null, "record": null, "timezone": "America/Lima", "grouped_reads": false,
              "meter": { "liters_per_unit": 0, "offset_liters": 0, "base_liters_per_pulse": 10,
                         "output_liters_per_unit": 10, "model": "DN50", "serial_number": "" },
              "tags": { "site": "hq" } },
  "enabled": true, "recorded": true, "timezone": "America/Lima",
  "state": { "online": true, "last_ok_at": "…", "last_error": "", "last_reading": { … } }
} ] }
```

`enabled`/`record` are `null` when unset (default `true`); the top-level
`enabled` and `recorded` are the resolved values.

### `GET /devices/{id}` · `POST /devices` · `PUT /devices/{id}` · `DELETE /devices/{id}`

Same semantics as buses. Device changes only affect the panel configuration —
they never write to the encoder. Deleting a device keeps its recorded
measurements.

---

## Registers

Low-level access to an encoder. All of these talk to the device over the bus;
failures surface as `502`/`504`. Writes invalidate the live-reading cache.

### `GET /devices/{id}/snapshot`

Reads and decodes every documented register (`0x0000-0x0015` plus the debug
block `0x0019-0x001B`):

```json
{ "device_id": "building-a", "slave": 1,
  "snapshot": {
    "slave_address": 1,
    "counters": { "total": 116458, "forward": 116467, "reverse": 9 },
    "clock": { "year": 2026, "month": 10, "day": 23, "hour": 19, "minute": 55, "second": 26, "valid": true },
    "device_type": 1,
    "firmware": { "year": 2018, "month": 8, "day": 1, "text": "2018-08-01" },
    "reverse_flow_threshold": 5, "battery_status": 0, "failure": 0,
    "base_pulse_liters": 10, "output_pulse_liters": 10,
    "forward_base_reading": 116467, "reverse_base_reading": 9,
    "debug_numbers": [0, 0, 0, 0, 0, 0],
    "raw": [ { "address": 0, "key": "slave_address", "value": 1, "hex": "0x0001" }, … ],
    "read_at": "…" },
  "clock_time": "2026-10-23T19:55:26-05:00", "clock_drift_seconds": -0.4 }
```

### `GET /devices/{id}/registers?address=&count=`

Raw read (function 0x03) of `count` (1-125, default 1) registers from
`address` (0-65535, default 0). Works for undocumented addresses too.

### `PUT /devices/{id}/registers/{address}`

Raw single write (function 0x06). `{address}` is decimal or `0x` hex. Body:
`{"value": 0-65535}`. Requires `modbus.allow_raw_writes`.

### `POST /devices/{id}/registers`

Raw multiple write (function 0x10): `{"address": 18, "values": [34678, 25940]}`
(1-123 values). Requires `modbus.allow_raw_writes`.

Raw writes to the slave-address register (0x0000) also update the device's
configured `slave`, after checking that the address is free on the bus.

### Typed endpoints

| Method & path | Body | Register(s) | Notes |
|---|---|---|---|
| `GET /devices/{id}/counters` | | 0x0001-0x0006 | `{counters: {total, forward, reverse}}` raw units |
| `GET /devices/{id}/slave-address` | | 0x0000 | Returns the register and the configured address |
| `PUT /devices/{id}/slave-address` | `{"slave": 1-247}` | 0x0000 | Re-addresses the encoder **and** updates the config. `409` if the address is used on the bus |
| `GET /devices/{id}/clock` | | 0x0007-0x0009 | Decoded clock, time in the device zone, `drift_seconds` |
| `PUT /devices/{id}/clock` | `{"time": "RFC 3339"}` or `{}` | 0x0007-0x0009 | Empty body synchronises with the server clock. The time is converted to the device zone |
| `GET`/`PUT /devices/{id}/device-type` | `{"value": 0-65535}` | 0x000A | |
| `GET /devices/{id}/firmware` | | 0x000B-0x000C | Software version date |
| `GET`/`PUT /devices/{id}/reverse-flow-threshold` | `{"value": 0-65535}` | 0x000D | |
| `GET /devices/{id}/diagnostics` | | 0x000E-0x000F, 0x0019-0x001B | Battery, failure, six debug bytes |
| `GET /devices/{id}/pulse-equivalents` | | 0x0010-0x0011 | Also the expected values from config and `matches_expected` |
| `PUT /devices/{id}/pulse-equivalents` | `{"base_liters_per_pulse": 1-10000, "output_liters_per_unit": 1-1000}` | 0x0010-0x0011 | Output must be ≥ base. Written in one request |
| `POST /devices/{id}/pulse-equivalents/apply` | | 0x0010-0x0011 | Writes `meter.base_liters_per_pulse` / `meter.output_liters_per_unit` from config |
| `GET /devices/{id}/base-readings` | | 0x0012-0x0015 | Forward and reverse base readings, raw units |
| `PUT /devices/{id}/base-readings` | see below | 0x0012-0x0015 | Presets the counters |

`PUT /base-readings` accepts each counter either raw or in liters; omitted
counters keep their current value:

```json
{ "forward_liters": 123400 }
{ "forward": 12340, "reverse": 0 }
```

Liters are converted with the device's liters-per-unit (config, or the
encoder's output pulse equivalent) after subtracting `meter.offset_liters`
from the forward value. The response includes the previous and new values.

---

## Meters

High-level, register-free views for applications that just need volumes.

### `GET /meters`

Every meter with its last live reading (if any since startup) and its last
recorded measurement. Does **not** touch the bus.

```json
{ "meters": [ {
  "id": "building-a", "name": "Building A", "bus": "rs485-1", "slave": 1,
  "enabled": true, "online": true, "model": "DN50",
  "reading": { … },
  "last_recorded": { "time": "…", "total_liters": 1200640, … }
} ] }
```

### `GET /meters/{id}?max_age=2s`

A live reading. `max_age` (duration) accepts a cached reading up to that age;
default `modbus.live_cache_ttl`, `0s` forces a bus read.

```json
{ "id": "building-a", "name": "Building A", "online": true, …,
  "reading": {
    "device_id": "building-a", "time": "2026-10-23T19:27:55-05:00",
    "raw": { "total": 120064, "forward": 120067, "reverse": 3 },
    "liters_per_unit": 10, "liters_per_unit_source": "encoder",
    "total_liters": 1200640, "forward_liters": 1200670, "reverse_liters": 30,
    "total_m3": 1200.64, "forward_m3": 1200.67, "reverse_m3": 0.03,
    "battery_status": 0, "failure": 0, "base_pulse_liters": 10, "output_pulse_liters": 10,
    "device_clock": "2026-10-23T19:27:55-05:00", "clock_drift_seconds": -0.4,
    "warnings": [] } }
```

`liters_per_unit_source` is `config`, `encoder`, or `default` (the encoder
reported 0; 1 L/unit is assumed and a warning added). `warnings` also reports
pulse-equivalent mismatches against the configuration and non-zero failure
codes.

### `GET /meters/{id}/consumption?from=&to=`

Volume used in a period, from recorded measurements: the difference between
the last measurement at or before `from` (or the first one after it) and the
last one before `to`.

```json
{ "device_id": "building-a", "from": "…", "to": "…", "start_time": "…", "end_time": "…",
  "start_total_liters": 8023490, "end_total_liters": 8047120,
  "forward_liters": 23720, "reverse_liters": 150, "net_liters": 23570, "net_m3": 23.57,
  "has_data": true, "counter_reset": false }
```

`counter_reset` is `true` when a counter went backwards (e.g. base readings
were rewritten); negative differences are then clamped to 0.

### `GET /meters/{id}/series?from=&to=&bucket=1h&tz=`

Consumption per time bucket:

```json
{ "device_id": "building-a", "bucket": "1h0m0s", "timezone": "Local",
  "points": [ { "start": "…", "samples": 60, "total_liters": 8024750,
                "forward_liters": 1260, "reverse_liters": 0, "net_liters": 1260,
                "avg_flow_lpm": 21, "min_total_liters": 8023500, "max_total_liters": 8024750 } ] }
```

- `bucket`: duration ≥ `1m`, whole seconds; `range / bucket ≤ query.max_buckets`.
- Buckets align to multiples of the bucket size counted from midnight in `tz`
  (default: server zone), so `24h` buckets start at local midnight.
- Each bucket's consumption is measured from the end of the previous bucket
  (or the last measurement before `from`), so no usage is lost between buckets.
- `avg_flow_lpm` = forward liters / bucket minutes.
- Only buckets with data are returned.

### `GET /consumption?from=&to=&device=`

Consumption of several meters (default: all) plus the total:

```json
{ "from": "…", "to": "…", "total_net_liters": 68500, "total_net_m3": 68.5, "meters": [ { … } ] }
```

---

## Measurements

### `GET /measurements`

Paginated recorded measurements.

| Parameter | Default | Validation |
|---|---|---|
| `device` | all | ≤ 100 valid IDs |
| `from`, `to` | last 24 h | ≤ `query.max_range` |
| `page` | `1` | ≥ 1; offset ≤ 1 000 000 |
| `page_size` | `query.default_page_size` | 1 - `query.max_page_size` |
| `order` | `desc` | `asc` or `desc` (by time) |

```json
{ "from": "…", "to": "…", "devices": ["building-a"], "order": "desc",
  "page": 1, "page_size": 100, "total": 4299, "total_pages": 43,
  "measurements": [ { "time": "2026-10-24T00:45:00Z", "device_id": "building-a", "bus": "rs485-1", "slave": 1,
                      "total_raw": 804712, "forward_raw": 804736, "reverse_raw": 24, "liters_per_unit": 10,
                      "total_liters": 8047120, "forward_liters": 8047360, "reverse_liters": 240,
                      "battery_status": 0, "failure": 0, "base_pulse_liters": 10, "output_pulse_liters": 10,
                      "device_clock": "2026-10-24T00:45:00Z" } ] }
```

For deep history, narrow the time range instead of paging far.

### `GET /measurements/latest?device=`

The most recent measurement of each device.

### `GET /measurements/aggregate?from=&to=&bucket=&tz=&device=`

Like `/meters/{id}/series` for several devices (default: all):
`{"series": {"building-a": [ … ], "building-b": [ … ]}}`.

### `GET /measurements/stats`

`{"driver": "sqlite", "rows": 8634, "estimated": false, "oldest": "…", "newest": "…"}`.
On TimescaleDB `rows` is a fast estimate.

### `GET /measurements/csv?from=&to=&device=&columns=&order=asc&tz=`

Streams matching rows as a CSV download (`Content-Disposition: attachment`),
capped at `query.max_export_rows` (also sent as `X-Max-Rows`). `columns` is a
comma-separated subset of the [export columns](configuration.md#export)
(default `export.columns`); delimiter, header and time format follow the
`export` settings; `tz` overrides `export.timezone`.

---

## Recorder

### `GET /recorder`

```json
{ "enabled": true, "schedule": "every 1m (aligned)", "next_run": "…", "running": false,
  "last_run": { "tick": "…", "started_at": "…", "duration_ms": 41, "recorded": 3, "failed": 1,
                "devices": [ { "device_id": "spare", "ok": false, "error": "modbus: response timeout" } ] } }
```

### `POST /recorder/run`

Records all devices now (stamped with the current second) and returns the run.
Runs never overlap with scheduled ones.

---

## Exports

### `GET /exports`

Exporter status and the CSV files in the export directory, newest first:

```json
{ "status": { "enabled": true, "schedule": "every 1h (aligned)", "window": "1h", "directory": "exports",
              "next_run": "…", "last_run": { "tick": "…", "from": "…", "to": "…", "rows": 180,
                                              "files": [ { "name": "…", "size": 3372, "modified": "…" } ] } },
  "files": [ { "name": "measurements_20261023T180000_20261023T190000.csv", "size": 3372, "modified": "…" } ] }
```

### `POST /exports/run`

Generates the export now for the window ending at `to` (body
`{"to": "<time>"}`, default now; same formats as [time parameters](#time-parameters)).

### `GET /exports/files/{name}` · `DELETE /exports/files/{name}`

Download or delete one file. Only plain `.csv` names inside the export
directory are accepted; anything else is `404`.
