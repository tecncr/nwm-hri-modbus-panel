// Types mirroring the nwm-panel REST API (see docs/api.md).

export interface Counters {
  total: number
  forward: number
  reverse: number
}

export interface Reading {
  device_id: string
  name: string
  bus: string
  slave: number
  time: string
  raw: Counters
  liters_per_unit: number
  liters_per_unit_source: "config" | "encoder" | "default"
  total_liters: number
  forward_liters: number
  reverse_liters: number
  total_m3: number
  forward_m3: number
  reverse_m3: number
  battery_status: number
  failure: number
  base_pulse_liters: number
  output_pulse_liters: number
  device_clock?: string
  clock_drift_seconds?: number
  warnings?: string[]
}

export interface Measurement {
  time: string
  device_id: string
  bus: string
  slave: number
  total_raw: number
  forward_raw: number
  reverse_raw: number
  liters_per_unit: number
  total_liters: number
  forward_liters: number
  reverse_liters: number
  battery_status: number
  failure: number
  base_pulse_liters: number
  output_pulse_liters: number
  device_clock?: string
}

export interface Meter {
  id: string
  name: string
  description?: string
  bus: string
  slave: number
  enabled: boolean
  online: boolean
  last_error?: string
  model?: string
  serial_number?: string
  tags?: Record<string, string>
  reading?: Reading
  last_recorded?: Measurement
}

export interface MeterConfig {
  liters_per_unit: number
  offset_liters: number
  base_liters_per_pulse: number
  output_liters_per_unit: number
  model: string
  serial_number: string
}

export interface DeviceConfig {
  id: string
  name: string
  description: string
  bus: string
  slave: number
  enabled: boolean | null
  record: boolean | null
  timezone: string
  grouped_reads: boolean
  meter: MeterConfig
  tags: Record<string, string> | null
}

export interface DeviceState {
  online: boolean
  last_ok_at?: string
  last_error?: string
  last_error_at?: string
  last_reading?: Reading
}

export interface DeviceView {
  config: DeviceConfig
  enabled: boolean
  recorded: boolean
  timezone: string
  state: DeviceState
}

export type Transport = "serial" | "tcp" | "simulator"

export interface BusConfig {
  id: string
  name: string
  transport: Transport
  serial: { device: string; baud_rate: number; data_bits: number; parity: string; stop_bits: number }
  tcp: { address: string; connect_timeout: string }
  simulator: { slaves: number[] | null; flow_lpm: number }
  timeout: string
  retries: number | null
  frame_delay: string
}

export interface BusStats {
  requests: number
  failures: number
  timeouts: number
  crc_errors: number
  exceptions: number
  last_error?: string
  last_error_at?: string
  last_ok_at?: string
  connected: boolean
}

export interface BusView {
  config: BusConfig
  stats: BusStats
  devices: string[]
}

export interface ScanResult {
  slave: number
  slave_address_register: number
  device_type?: number
  firmware?: Firmware
  configured_as?: string
}

export interface ScanJob {
  bus: string
  from: number
  to: number
  timeout_ms: number
  current: number
  progress: number
  done: boolean
  cancelled: boolean
  started_at: string
  finished_at?: string
  found: ScanResult[]
}

export interface Clock {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  second: number
  valid: boolean
}

export interface Firmware {
  year: number
  month: number
  day: number
  text: string
}

export interface RegisterValue {
  address: number
  key: string
  value: number
  hex: string
}

export interface Snapshot {
  slave_address: number
  counters: Counters
  clock: Clock
  device_type: number
  firmware: Firmware
  reverse_flow_threshold: number
  battery_status: number
  failure: number
  base_pulse_liters: number
  output_pulse_liters: number
  forward_base_reading: number
  reverse_base_reading: number
  debug_numbers: number[] | null
  debug_error?: string
  raw: RegisterValue[]
  read_at: string
}

export interface SnapshotResponse {
  device_id: string
  slave: number
  snapshot: Snapshot
  clock_time?: string
  clock_drift_seconds?: number
}

export interface RegisterDef {
  key: string
  address: number
  words: number
  access: "ro" | "rw"
  title: string
  description: string
}

export interface RegisterMap {
  registers: RegisterDef[]
  limits: Record<string, { min: number; max: number }>
  export_columns: string[]
  default_export_columns: string[]
}

export interface SeriesPoint {
  start: string
  samples: number
  total_liters: number
  forward_liters: number
  reverse_liters: number
  net_liters: number
  avg_flow_lpm: number
  counter_reset?: boolean
  min_total_liters: number
  max_total_liters: number
}

export interface Consumption {
  device_id: string
  name?: string
  from: string
  to: string
  start_time?: string
  end_time?: string
  start_total_liters?: number
  end_total_liters?: number
  forward_liters: number
  reverse_liters: number
  net_liters: number
  net_m3: number
  has_data: boolean
  counter_reset: boolean
}

export interface ConsumptionSummary {
  from: string
  to: string
  total_net_liters: number
  total_net_m3: number
  meters: Consumption[]
}

export interface MeasurementPage {
  from: string
  to: string
  page: number
  page_size: number
  total: number
  total_pages: number
  order: "asc" | "desc"
  measurements: Measurement[]
}

export interface RecorderRun {
  tick: string
  started_at: string
  duration_ms: number
  recorded: number
  failed: number
  store_error?: string
  devices: { device_id: string; ok: boolean; error?: string }[] | null
}

export interface RecorderStatus {
  enabled: boolean
  schedule: string
  next_run?: string
  last_run?: RecorderRun
  running: boolean
}

export interface ExportFile {
  name: string
  size: number
  modified: string
}

export interface ExportResult {
  tick: string
  from: string
  to: string
  files: ExportFile[]
  rows: number
  duration_ms: number
  error?: string
}

export interface ExportRegeneration {
  from: string
  to: string
  windows: number
  files: ExportFile[]
  rows: number
  errors: string[]
  duration_ms: number
}

export interface ExportStatus {
  enabled: boolean
  schedule: string
  window: string
  directory: string
  next_run?: string
  last_run?: ExportResult
}

export interface StorageStats {
  driver: string
  rows: number
  estimated: boolean
  oldest?: string
  newest?: string
}

export interface SystemInfo {
  name: string
  version: string
  build: { version: string; commit: string; date: string; os: string; arch: string; go: string }
  started_at: string
  uptime_seconds: number
  config_file: string
  persistent: boolean
  buses: number
  devices: number
  storage: { driver: string; enabled: boolean; stats: StorageStats; error?: string }
  recorder: RecorderStatus
  export: ExportStatus
  modbus: {
    allow_raw_writes: boolean
    allow_broadcast: boolean
    timeout: string
    retries: number
    live_cache_ttl: string
  }
  query: {
    default_page_size: number
    max_page_size: number
    max_range: string
    max_export_rows: number
    max_buckets: number
    timeout: string
  }
}

export interface InventoryResult {
  persisted: boolean
  warning?: string
}
