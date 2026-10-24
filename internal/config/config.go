// Package config defines the application configuration, its defaults and
// validation. Configuration is read from a YAML file, environment variables
// (prefixed NWM_PANEL_) and command-line flags, in increasing precedence.
package config

// EnvPrefix prefixes every environment variable.
const EnvPrefix = "NWM_PANEL"

// APIKeyEnv is the environment variable holding the master API key. It is
// intentionally never read from the config file or flags.
const APIKeyEnv = EnvPrefix + "_API_KEY"

// Config is the root configuration.
type Config struct {
	Server   ServerConfig   `mapstructure:"server" yaml:"server" json:"server"`
	Log      LogConfig      `mapstructure:"log" yaml:"log" json:"log"`
	Modbus   ModbusConfig   `mapstructure:"modbus" yaml:"modbus" json:"modbus"`
	Storage  StorageConfig  `mapstructure:"storage" yaml:"storage" json:"storage"`
	Recorder RecorderConfig `mapstructure:"recorder" yaml:"recorder" json:"recorder"`
	Export   ExportConfig   `mapstructure:"export" yaml:"export" json:"export"`
	Query    QueryConfig    `mapstructure:"query" yaml:"query" json:"query"`
	Buses    []BusConfig    `mapstructure:"buses" yaml:"buses" json:"buses"`
	Devices  []DeviceConfig `mapstructure:"devices" yaml:"devices" json:"devices"`
}

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	Listen         string          `mapstructure:"listen" yaml:"listen" json:"listen"`
	BasePath       string          `mapstructure:"base_path" yaml:"base_path" json:"base_path"`
	TrustedProxies []string        `mapstructure:"trusted_proxies" yaml:"trusted_proxies" json:"trusted_proxies"`
	ProxyHeader    string          `mapstructure:"proxy_header" yaml:"proxy_header" json:"proxy_header"`
	ReadTimeout    Duration        `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout   Duration        `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout    Duration        `mapstructure:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	BodyLimit      int             `mapstructure:"body_limit" yaml:"body_limit" json:"body_limit"`
	CORSOrigins    []string        `mapstructure:"cors_origins" yaml:"cors_origins" json:"cors_origins"`
	UI             bool            `mapstructure:"ui" yaml:"ui" json:"ui"`
	AccessLog      bool            `mapstructure:"access_log" yaml:"access_log" json:"access_log"`
	RateLimit      RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`
}

// RateLimitConfig limits requests per client IP.
type RateLimitConfig struct {
	Enabled               bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	RequestsPerMinute     int  `mapstructure:"requests_per_minute" yaml:"requests_per_minute" json:"requests_per_minute"`
	AuthFailuresPerMinute int  `mapstructure:"auth_failures_per_minute" yaml:"auth_failures_per_minute" json:"auth_failures_per_minute"`
}

// LogConfig controls logging.
type LogConfig struct {
	Level  string `mapstructure:"level" yaml:"level" json:"level"`
	Format string `mapstructure:"format" yaml:"format" json:"format"`
}

// ModbusConfig holds bus-independent Modbus settings.
type ModbusConfig struct {
	Timeout        Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Retries        int      `mapstructure:"retries" yaml:"retries" json:"retries"`
	AllowRawWrites bool     `mapstructure:"allow_raw_writes" yaml:"allow_raw_writes" json:"allow_raw_writes"`
	AllowBroadcast bool     `mapstructure:"allow_broadcast" yaml:"allow_broadcast" json:"allow_broadcast"`
	LiveCacheTTL   Duration `mapstructure:"live_cache_ttl" yaml:"live_cache_ttl" json:"live_cache_ttl"`
}

// Storage drivers.
const (
	DriverSQLite      = "sqlite"
	DriverTimescaleDB = "timescaledb"
	DriverNone        = "none"
)

// StorageConfig selects and configures the measurement database.
type StorageConfig struct {
	Driver      string            `mapstructure:"driver" yaml:"driver" json:"driver"`
	Retention   Duration          `mapstructure:"retention" yaml:"retention" json:"retention"`
	SQLite      SQLiteConfig      `mapstructure:"sqlite" yaml:"sqlite" json:"sqlite"`
	TimescaleDB TimescaleDBConfig `mapstructure:"timescaledb" yaml:"timescaledb" json:"timescaledb"`
}

// SQLiteConfig configures the embedded SQLite database.
type SQLiteConfig struct {
	Path string `mapstructure:"path" yaml:"path" json:"path"`
}

// TimescaleDBConfig configures the TimescaleDB hypertable.
type TimescaleDBConfig struct {
	DSN              string   `mapstructure:"dsn" yaml:"dsn" json:"dsn"`
	ChunkInterval    Duration `mapstructure:"chunk_interval" yaml:"chunk_interval" json:"chunk_interval"`
	CompressionAfter Duration `mapstructure:"compression_after" yaml:"compression_after" json:"compression_after"`
	MaxOpenConns     int      `mapstructure:"max_open_conns" yaml:"max_open_conns" json:"max_open_conns"`
}

// RecorderConfig schedules periodic measurement recording.
type RecorderConfig struct {
	Enabled  bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Interval Duration `mapstructure:"interval" yaml:"interval" json:"interval"`
	Cron     string   `mapstructure:"cron" yaml:"cron" json:"cron"`
	Timeout  Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
}

// ExportConfig schedules periodic CSV exports.
type ExportConfig struct {
	Enabled    bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Directory  string   `mapstructure:"directory" yaml:"directory" json:"directory"`
	Interval   Duration `mapstructure:"interval" yaml:"interval" json:"interval"`
	Cron       string   `mapstructure:"cron" yaml:"cron" json:"cron"`
	Window     Duration `mapstructure:"window" yaml:"window" json:"window"`
	Devices    []string `mapstructure:"devices" yaml:"devices" json:"devices"`
	Columns    []string `mapstructure:"columns" yaml:"columns" json:"columns"`
	Delimiter  string   `mapstructure:"delimiter" yaml:"delimiter" json:"delimiter"`
	Header     bool     `mapstructure:"header" yaml:"header" json:"header"`
	Timezone   string   `mapstructure:"timezone" yaml:"timezone" json:"timezone"`
	TimeFormat string   `mapstructure:"time_format" yaml:"time_format" json:"time_format"`
	Filename   string   `mapstructure:"filename" yaml:"filename" json:"filename"`
	PerDevice  bool     `mapstructure:"per_device" yaml:"per_device" json:"per_device"`
	SkipEmpty  bool     `mapstructure:"skip_empty" yaml:"skip_empty" json:"skip_empty"`
	Keep       Duration `mapstructure:"keep" yaml:"keep" json:"keep"`
}

// QueryConfig bounds API queries against the measurement database.
type QueryConfig struct {
	DefaultPageSize int      `mapstructure:"default_page_size" yaml:"default_page_size" json:"default_page_size"`
	MaxPageSize     int      `mapstructure:"max_page_size" yaml:"max_page_size" json:"max_page_size"`
	MaxRange        Duration `mapstructure:"max_range" yaml:"max_range" json:"max_range"`
	MaxExportRows   int      `mapstructure:"max_export_rows" yaml:"max_export_rows" json:"max_export_rows"`
	MaxBuckets      int      `mapstructure:"max_buckets" yaml:"max_buckets" json:"max_buckets"`
	Timeout         Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
}

// Bus transports.
const (
	TransportSerial    = "serial"
	TransportTCP       = "tcp"
	TransportSimulator = "simulator"
)

// BusConfig describes one RS-485 line (or gateway) with encoders on it.
type BusConfig struct {
	ID         string          `mapstructure:"id" yaml:"id" json:"id"`
	Name       string          `mapstructure:"name" yaml:"name,omitempty" json:"name"`
	Transport  string          `mapstructure:"transport" yaml:"transport" json:"transport"`
	Serial     SerialConfig    `mapstructure:"serial" yaml:"serial,omitempty" json:"serial"`
	TCP        TCPConfig       `mapstructure:"tcp" yaml:"tcp,omitempty" json:"tcp"`
	Simulator  SimulatorConfig `mapstructure:"simulator" yaml:"simulator,omitempty" json:"simulator"`
	Timeout    Duration        `mapstructure:"timeout" yaml:"timeout,omitempty" json:"timeout"`
	Retries    *int            `mapstructure:"retries" yaml:"retries,omitempty" json:"retries"`
	FrameDelay Duration        `mapstructure:"frame_delay" yaml:"frame_delay,omitempty" json:"frame_delay"`
}

// SerialConfig describes a local serial port.
type SerialConfig struct {
	Device   string `mapstructure:"device" yaml:"device,omitempty" json:"device"`
	BaudRate int    `mapstructure:"baud_rate" yaml:"baud_rate,omitempty" json:"baud_rate"`
	DataBits int    `mapstructure:"data_bits" yaml:"data_bits,omitempty" json:"data_bits"`
	Parity   string `mapstructure:"parity" yaml:"parity,omitempty" json:"parity"`
	StopBits int    `mapstructure:"stop_bits" yaml:"stop_bits,omitempty" json:"stop_bits"`
}

// TCPConfig describes a transparent serial-to-Ethernet gateway.
type TCPConfig struct {
	Address        string   `mapstructure:"address" yaml:"address,omitempty" json:"address"`
	ConnectTimeout Duration `mapstructure:"connect_timeout" yaml:"connect_timeout,omitempty" json:"connect_timeout"`
}

// SimulatorConfig configures a simulated bus.
type SimulatorConfig struct {
	Slaves  []int   `mapstructure:"slaves" yaml:"slaves,omitempty" json:"slaves"`
	FlowLPM float64 `mapstructure:"flow_lpm" yaml:"flow_lpm,omitempty" json:"flow_lpm"`
}

// DeviceConfig describes one encoder and the meter it is attached to.
type DeviceConfig struct {
	ID           string            `mapstructure:"id" yaml:"id" json:"id"`
	Name         string            `mapstructure:"name" yaml:"name,omitempty" json:"name"`
	Description  string            `mapstructure:"description" yaml:"description,omitempty" json:"description"`
	Bus          string            `mapstructure:"bus" yaml:"bus" json:"bus"`
	Slave        uint8             `mapstructure:"slave" yaml:"slave" json:"slave"`
	Enabled      *bool             `mapstructure:"enabled" yaml:"enabled,omitempty" json:"enabled"`
	Record       *bool             `mapstructure:"record" yaml:"record,omitempty" json:"record"`
	Timezone     string            `mapstructure:"timezone" yaml:"timezone,omitempty" json:"timezone"`
	GroupedReads bool              `mapstructure:"grouped_reads" yaml:"grouped_reads,omitempty" json:"grouped_reads"`
	Meter        MeterConfig       `mapstructure:"meter" yaml:"meter,omitempty" json:"meter"`
	Tags         map[string]string `mapstructure:"tags" yaml:"tags,omitempty" json:"tags"`
}

// IsEnabled reports whether the device is polled at all (default true).
func (d DeviceConfig) IsEnabled() bool { return d.Enabled == nil || *d.Enabled }

// IsRecorded reports whether the recorder stores its measurements (default true).
func (d DeviceConfig) IsRecorded() bool { return d.IsEnabled() && (d.Record == nil || *d.Record) }

// MeterConfig maps encoder counters to physical volume.
type MeterConfig struct {
	// LitersPerUnit converts raw counter units to liters. 0 means "use the
	// encoder's output pulse equivalent register".
	LitersPerUnit float64 `mapstructure:"liters_per_unit" yaml:"liters_per_unit,omitempty" json:"liters_per_unit"`
	// OffsetLiters is added to the net and forward volumes, e.g. to continue
	// the reading of a replaced meter.
	OffsetLiters float64 `mapstructure:"offset_liters" yaml:"offset_liters,omitempty" json:"offset_liters"`
	// BaseLitersPerPulse and OutputLitersPerUnit are the expected encoder
	// pulse equivalents. When set they can be pushed to the encoder and
	// mismatches are reported.
	BaseLitersPerPulse  int `mapstructure:"base_liters_per_pulse" yaml:"base_liters_per_pulse,omitempty" json:"base_liters_per_pulse"`
	OutputLitersPerUnit int `mapstructure:"output_liters_per_unit" yaml:"output_liters_per_unit,omitempty" json:"output_liters_per_unit"`
	// Model and SerialNumber are free-form identifiers of the physical meter.
	Model        string `mapstructure:"model" yaml:"model,omitempty" json:"model"`
	SerialNumber string `mapstructure:"serial_number" yaml:"serial_number,omitempty" json:"serial_number"`
}
