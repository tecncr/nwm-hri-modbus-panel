# Recording and exports

## Recorder

The recorder reads every device with `enabled` and `record` set, converts the
counters to volume (see [devices and registers](devices-and-registers.md#from-pulses-to-liters))
and stores one row per device.

- **Schedule.** `recorder.interval` (default `1m`) is aligned to the wall
  clock: every minute at `:00`, every `15m` at quarter hours, `24h` at local
  midnight. `recorder.cron` replaces it with a cron expression
  (e.g. `*/30 * * * * *` for every 30 seconds).
- **Timestamps.** Rows are stamped with the scheduled tick, not the moment the
  bus answered, so series line up across devices.
- **Concurrency.** Buses are read in parallel; devices on the same bus
  sequentially. A pass is bounded by `recorder.timeout`; devices that fail are
  logged, counted and skipped — they never block the others. Passes never
  overlap; missed ticks are skipped.
- **Duplicates.** A device is stored at most once per second; repeats are
  ignored.
- **On demand.** `POST /api/v1/recorder/run` (or *Record now* on the System
  page) runs a pass immediately.

## Storage

### Schema

Both drivers use the same table:

| Column | Type | Notes |
|---|---|---|
| `time` | timestamp | UTC, whole seconds. Primary key with `device_id`. |
| `device_id`, `bus`, `slave` | | Identification at recording time |
| `total_raw`, `forward_raw`, `reverse_raw` | bigint | Raw counters |
| `liters_per_unit` | double | Conversion factor used |
| `total_liters`, `forward_liters`, `reverse_liters` | double | Volumes |
| `battery_status`, `failure` | int | Health registers |
| `base_pulse_liters`, `output_pulse_liters` | int | Encoder pulse equivalents |
| `device_clock` | timestamp, nullable | Encoder clock |

An index on `time` complements the primary key.

### SQLite

The default. Uses a pure-Go driver (no CGO), WAL journaling and a single
writer connection. Suitable for many devices at one-minute resolution; a year
of minute data for 10 meters is about 5 million rows. `storage.retention`
prunes old rows hourly.

### TimescaleDB

Set `storage.driver: timescaledb` and provide a DSN. On startup nwm-panel:

1. runs `CREATE EXTENSION IF NOT EXISTS timescaledb` (skipped with a warning if
   the role lacks the privilege — install the extension beforehand),
2. creates the table and turns it into a **hypertable partitioned by `time`**
   with chunks of `storage.timescaledb.chunk_interval` (default one week),
3. applies the chunk interval to future chunks if it changed,
4. optionally enables native compression (`compression_after`, segmented by
   device) and a native retention policy (`storage.retention`).

Create a dedicated database and owner, for example:

```sql
CREATE DATABASE nwm_panel;
CREATE USER nwm_panel WITH PASSWORD 'change-me';
ALTER DATABASE nwm_panel OWNER TO nwm_panel;
\c nwm_panel
CREATE EXTENSION IF NOT EXISTS timescaledb;
ALTER SCHEMA public OWNER TO nwm_panel;
```

```sh
export NWM_PANEL_STORAGE_TIMESCALEDB_DSN='postgres://nwm_panel:change-me@127.0.0.1:5432/nwm_panel?sslmode=disable'
```

Since the data lives in a regular hypertable, tools such as Grafana or
Metabase can query it directly.

### Query protection

API queries are bounded by the `query` settings: maximum time range, page
size, CSV row cap, number of aggregation buckets, a query timeout, and a
maximum pagination offset. Invalid parameters are rejected with `400` before
reaching the database.

## Periodic CSV exports

The exporter writes the measurements of a window to files in
`export.directory`. Default: **every hour on the hour, a file with the
previous hour**, named `measurements_<from>_<to>.csv`.

```yaml
export:
  interval: 1h
  window: 1h
  filename: "measurements_{from}_{to}.csv"
```

Each run covers `[tick − window, tick)`. Interval and window are independent:
`interval: 1h` with `window: 24h` writes a rolling last-24-hours file every
hour (use a constant file name such as `last_24h.csv` to overwrite it).

Common setups:

| Goal | Settings |
|---|---|
| Hourly files (default) | `interval: 1h`, `window: 1h` |
| Daily file per meter at midnight | `interval: 24h`, `window: 24h`, `per_device: true`, `filename: "{device}_{date}.csv"` |
| Rolling file for another system to poll | `interval: 5m`, `window: 24h`, `filename: latest.csv` |
| Spreadsheet-friendly (comma decimals) | `delimiter: ";"` |
| Only some meters and columns | `devices: [a, b]`, `columns: [time, device_id, total_m3]` |
| Clean up | `keep: 720h` deletes files older than 30 days |

Files are written to a temporary name and renamed into place, so consumers
never see partial files. Each file is capped at `query.max_export_rows`.

### Regenerating files

Periodic files are a snapshot of the database when they were written. If
stored measurements change afterwards (corrected or backfilled), regenerate
the affected files from the database:

```sh
nwm-panel export regenerate --from 2026-10-27 --to 2026-11-14
```

or `POST /api/v1/exports/regenerate` with `{"from": "…", "to": "…"}`. Every
scheduled window that overlaps `[from, to)` and has already ended is
rewritten under its usual file name, replacing the existing file. Dates
without a zone are read in the export time zone. The command reads only the
database, so it can run next to a running server. Regenerated files get a
new modification time, so `keep` counts their age from the regeneration.

Files can be listed, downloaded, deleted and generated on demand through the
[API](api.md#exports) and the dashboard's **Exports** page. Ad-hoc CSV
downloads for any range are available via `GET /api/v1/measurements/csv` and
the **Measurements** page.
