package config

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Defaults lists every scalar setting with its default value. Keys are also
// the names used for environment variables (NWM_PANEL_ + upper-cased key with
// dots replaced by underscores).
var Defaults = map[string]any{
	"server.listen":                              "127.0.0.1:8080",
	"server.base_path":                           "",
	"server.trusted_proxies":                     []string{},
	"server.proxy_header":                        "X-Forwarded-For",
	"server.read_timeout":                        30 * time.Second,
	"server.write_timeout":                       2 * time.Minute,
	"server.idle_timeout":                        2 * time.Minute,
	"server.body_limit":                          1 << 20,
	"server.cors_origins":                        []string{},
	"server.ui":                                  true,
	"server.ui_language":                         "en",
	"server.access_log":                          true,
	"server.rate_limit.enabled":                  true,
	"server.rate_limit.requests_per_minute":      600,
	"server.rate_limit.auth_failures_per_minute": 10,

	"log.level":  "info",
	"log.format": "text",

	"modbus.timeout":          time.Second,
	"modbus.retries":          1,
	"modbus.allow_raw_writes": true,
	"modbus.allow_broadcast":  true,
	"modbus.live_cache_ttl":   2 * time.Second,

	"storage.driver":                        DriverSQLite,
	"storage.retention":                     time.Duration(0),
	"storage.sqlite.path":                   "nwm-panel.db",
	"storage.timescaledb.dsn":               "",
	"storage.timescaledb.chunk_interval":    7 * 24 * time.Hour,
	"storage.timescaledb.compression_after": time.Duration(0),
	"storage.timescaledb.max_open_conns":    10,

	"recorder.enabled":  true,
	"recorder.interval": time.Minute,
	"recorder.cron":     "",
	"recorder.timeout":  30 * time.Second,

	"export.enabled":     true,
	"export.directory":   "exports",
	"export.interval":    time.Hour,
	"export.cron":        "",
	"export.window":      time.Hour,
	"export.devices":     []string{},
	"export.columns":     DefaultExportColumns,
	"export.delimiter":   ",",
	"export.header":      true,
	"export.timezone":    "Local",
	"export.time_format": time.RFC3339,
	"export.filename":    "measurements_{from}_{to}.csv",
	"export.per_device":  false,
	"export.skip_empty":  false,
	"export.keep":        time.Duration(0),

	"query.default_page_size": 100,
	"query.max_page_size":     1000,
	"query.max_range":         366 * 24 * time.Hour,
	"query.max_export_rows":   500000,
	"query.max_buckets":       5000,
	"query.timeout":           30 * time.Second,
}

// DefaultExportColumns are the CSV columns written when none are configured.
var DefaultExportColumns = []string{
	"time", "device_id", "device_name", "slave", "total_liters", "forward_liters", "reverse_liters",
	"total_m3", "total_raw", "forward_raw", "reverse_raw", "liters_per_unit", "battery_status", "failure",
}

// ExportColumns lists every column the CSV exporter can produce.
var ExportColumns = []string{
	"time", "unix", "device_id", "device_name", "bus", "slave",
	"total_raw", "forward_raw", "reverse_raw", "liters_per_unit",
	"total_liters", "forward_liters", "reverse_liters",
	"total_m3", "forward_m3", "reverse_m3",
	"battery_status", "failure", "base_pulse_liters", "output_pulse_liters", "device_clock",
}

// SearchPaths are tried in order when no config file is given explicitly.
func SearchPaths() []string {
	paths := []string{"nwm-panel.yaml", "nwm-panel.yml"}
	if dir, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(dir, "nwm-panel", "config.yaml"))
	}
	return append(paths, "/etc/nwm-panel/config.yaml")
}

// NewViper returns a viper instance with defaults and environment binding.
func NewViper() *viper.Viper {
	v := viper.New()
	for k, val := range Defaults {
		v.SetDefault(k, val)
	}
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	return v
}

// Load reads the config file (explicit path or first match in SearchPaths)
// into v, unmarshals and validates it. It returns the config and the file
// actually used ("" when running without a file).
func Load(v *viper.Viper, path string) (*Config, string, error) {
	if path == "" {
		for _, p := range SearchPaths() {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	if path != "" {
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, "", fmt.Errorf("read config %s: %w", path, err)
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	var cfg Config
	err := v.Unmarshal(&cfg, func(dc *mapstructure.DecoderConfig) {
		dc.ErrorUnused = true
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			durationHook(),
			mapstructure.StringToSliceHookFunc(","),
		)
	})
	if err != nil {
		return nil, "", fmt.Errorf("parse config: %w", err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, "", err
	}
	return &cfg, path, nil
}

// Normalize fills derived defaults for buses and devices.
func (c *Config) Normalize() {
	c.Server.BasePath = NormalizeBasePath(c.Server.BasePath)
	c.Server.UILanguage = strings.ToLower(strings.TrimSpace(c.Server.UILanguage))
	for i := range c.Buses {
		c.Buses[i].Normalize()
	}
	for i := range c.Devices {
		c.Devices[i].Normalize()
	}
}

// NormalizeBasePath returns "" or a path with a leading and no trailing slash.
func NormalizeBasePath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// Normalize fills serial defaults (9600 8N1).
func (b *BusConfig) Normalize() {
	b.ID = strings.TrimSpace(b.ID)
	b.Transport = strings.ToLower(strings.TrimSpace(b.Transport))
	if b.Transport == "" {
		b.Transport = TransportSerial
	}
	if b.Transport == TransportSerial {
		if b.Serial.BaudRate == 0 {
			b.Serial.BaudRate = 9600
		}
		if b.Serial.DataBits == 0 {
			b.Serial.DataBits = 8
		}
		if b.Serial.Parity == "" {
			b.Serial.Parity = "N"
		}
		b.Serial.Parity = strings.ToUpper(b.Serial.Parity[:1])
		if b.Serial.StopBits == 0 {
			b.Serial.StopBits = 1
		}
	}
	if b.Transport == TransportTCP && b.TCP.ConnectTimeout == 0 {
		b.TCP.ConnectTimeout = Duration(5 * time.Second)
	}
	if b.Transport == TransportSimulator && b.Simulator.FlowLPM == 0 {
		b.Simulator.FlowLPM = 10
	}
}

// Normalize trims identifiers.
func (d *DeviceConfig) Normalize() {
	d.ID = strings.TrimSpace(d.ID)
	d.Bus = strings.TrimSpace(d.Bus)
}

// Validate checks the whole configuration and reports every problem found.
func (c *Config) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if c.Server.Listen == "" {
		add("server.listen must not be empty")
	}
	if c.Server.BodyLimit <= 0 {
		add("server.body_limit must be positive")
	}
	if !slices.Contains(UILanguages, c.Server.UILanguage) {
		add("server.ui_language must be one of %s", strings.Join(UILanguages, ", "))
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		add("log.level must be one of debug, info, warn, error")
	}
	switch c.Log.Format {
	case "text", "json":
	default:
		add("log.format must be text or json")
	}
	if c.Modbus.Timeout <= 0 {
		add("modbus.timeout must be positive")
	}
	if c.Modbus.Retries < 0 || c.Modbus.Retries > 10 {
		add("modbus.retries must be between 0 and 10")
	}

	switch c.Storage.Driver {
	case DriverSQLite:
		if c.Storage.SQLite.Path == "" {
			add("storage.sqlite.path must not be empty")
		}
	case DriverTimescaleDB:
		if c.Storage.TimescaleDB.DSN == "" {
			add("storage.timescaledb.dsn must be set (or NWM_PANEL_STORAGE_TIMESCALEDB_DSN)")
		}
		if c.Storage.TimescaleDB.ChunkInterval.D() < time.Hour {
			add("storage.timescaledb.chunk_interval must be at least 1h")
		}
	case DriverNone:
	default:
		add("storage.driver must be sqlite, timescaledb or none")
	}
	if c.Storage.Retention < 0 {
		add("storage.retention must not be negative")
	}

	if c.Recorder.Enabled {
		if c.Recorder.Cron == "" && c.Recorder.Interval.D() < time.Second {
			add("recorder.interval must be at least 1s")
		}
		if c.Recorder.Timeout <= 0 {
			add("recorder.timeout must be positive")
		}
	}
	if c.Export.Enabled {
		if c.Export.Directory == "" {
			add("export.directory must not be empty")
		}
		if c.Export.Cron == "" && c.Export.Interval.D() < time.Minute {
			add("export.interval must be at least 1m")
		}
		if c.Export.Window <= 0 {
			add("export.window must be positive")
		}
		if c.Storage.Driver == DriverNone {
			add("export requires a storage driver (storage.driver is none)")
		}
	}
	if len([]rune(c.Export.Delimiter)) != 1 || c.Export.Delimiter == "\"" || c.Export.Delimiter == "\n" {
		add("export.delimiter must be a single character other than quote or newline")
	}
	for _, col := range c.Export.Columns {
		if !slices.Contains(ExportColumns, col) {
			add("export.columns: unknown column %q (valid: %s)", col, strings.Join(ExportColumns, ", "))
		}
	}
	if len(c.Export.Columns) == 0 {
		add("export.columns must not be empty")
	}
	if _, err := time.LoadLocation(c.Export.Timezone); err != nil {
		add("export.timezone: %v", err)
	}
	if strings.ContainsAny(c.Export.Filename, `/\`) || !strings.HasSuffix(c.Export.Filename, ".csv") {
		add("export.filename must be a plain file name ending in .csv")
	}

	q := c.Query
	if q.MaxPageSize < 1 || q.MaxPageSize > 100000 {
		add("query.max_page_size must be between 1 and 100000")
	}
	if q.DefaultPageSize < 1 || q.DefaultPageSize > q.MaxPageSize {
		add("query.default_page_size must be between 1 and query.max_page_size")
	}
	if q.MaxRange.D() < time.Hour {
		add("query.max_range must be at least 1h")
	}
	if q.MaxExportRows < 1 || q.MaxBuckets < 1 {
		add("query.max_export_rows and query.max_buckets must be positive")
	}
	if q.Timeout <= 0 {
		add("query.timeout must be positive")
	}

	busIDs := map[string]bool{}
	for i, b := range c.Buses {
		if err := b.Validate(); err != nil {
			add("buses[%d]: %v", i, err)
		}
		if busIDs[b.ID] {
			add("buses[%d]: duplicate id %q", i, b.ID)
		}
		busIDs[b.ID] = true
	}
	devIDs := map[string]bool{}
	slaves := map[string]string{}
	for i, d := range c.Devices {
		if err := d.Validate(); err != nil {
			add("devices[%d]: %v", i, err)
		}
		if devIDs[d.ID] {
			add("devices[%d]: duplicate id %q", i, d.ID)
		}
		devIDs[d.ID] = true
		if !busIDs[d.Bus] {
			add("devices[%d]: unknown bus %q", i, d.Bus)
		}
		key := fmt.Sprintf("%s/%d", d.Bus, d.Slave)
		if other, ok := slaves[key]; ok {
			add("devices[%d]: slave %d on bus %q already used by %q", i, d.Slave, d.Bus, other)
		}
		slaves[key] = d.ID
	}
	return errors.Join(errs...)
}

// Validate checks a single bus definition.
func (b BusConfig) Validate() error {
	if !ValidID(b.ID) {
		return fmt.Errorf("id %q must match [a-z0-9][a-z0-9_-]{0,63}", b.ID)
	}
	switch b.Transport {
	case TransportSerial:
		if b.Serial.Device == "" {
			return errors.New("serial.device must be set")
		}
		if b.Serial.BaudRate < 300 || b.Serial.BaudRate > 921600 {
			return errors.New("serial.baud_rate must be between 300 and 921600")
		}
		if b.Serial.DataBits < 5 || b.Serial.DataBits > 8 {
			return errors.New("serial.data_bits must be between 5 and 8")
		}
		if !strings.Contains("NEO", b.Serial.Parity) || len(b.Serial.Parity) != 1 {
			return errors.New("serial.parity must be N, E or O")
		}
		if b.Serial.StopBits != 1 && b.Serial.StopBits != 2 {
			return errors.New("serial.stop_bits must be 1 or 2")
		}
	case TransportTCP:
		if b.TCP.Address == "" {
			return errors.New("tcp.address must be set (host:port)")
		}
	case TransportSimulator:
		for _, s := range b.Simulator.Slaves {
			if s < 1 || s > 247 {
				return fmt.Errorf("simulator.slaves: %d out of range 1-247", s)
			}
		}
	default:
		return fmt.Errorf("transport %q must be serial, tcp or simulator", b.Transport)
	}
	if b.Timeout < 0 || b.FrameDelay < 0 {
		return errors.New("timeout and frame_delay must not be negative")
	}
	if b.Retries != nil && (*b.Retries < 0 || *b.Retries > 10) {
		return errors.New("retries must be between 0 and 10")
	}
	return nil
}

// Validate checks a single device definition.
func (d DeviceConfig) Validate() error {
	if !ValidID(d.ID) {
		return fmt.Errorf("id %q must match [a-z0-9][a-z0-9_-]{0,63}", d.ID)
	}
	if d.Slave < 1 || d.Slave > 247 {
		return fmt.Errorf("slave %d out of range 1-247", d.Slave)
	}
	if d.Timezone != "" {
		if _, err := time.LoadLocation(d.Timezone); err != nil {
			return fmt.Errorf("timezone: %w", err)
		}
	}
	m := d.Meter
	if m.LitersPerUnit < 0 {
		return errors.New("meter.liters_per_unit must not be negative")
	}
	if (m.BaseLitersPerPulse != 0) != (m.OutputLitersPerUnit != 0) {
		return errors.New("meter.base_liters_per_pulse and meter.output_liters_per_unit must be set together")
	}
	if m.BaseLitersPerPulse != 0 {
		if m.BaseLitersPerPulse < 1 || m.BaseLitersPerPulse > 10000 {
			return errors.New("meter.base_liters_per_pulse must be between 1 and 10000")
		}
		if m.OutputLitersPerUnit < 1 || m.OutputLitersPerUnit > 1000 {
			return errors.New("meter.output_liters_per_unit must be between 1 and 1000")
		}
		if m.OutputLitersPerUnit < m.BaseLitersPerPulse {
			return errors.New("meter.output_liters_per_unit must not be lower than meter.base_liters_per_pulse")
		}
	}
	if len(d.Name) > 128 || len(d.Description) > 1024 || len(d.Tags) > 32 {
		return errors.New("name (128), description (1024) or tags (32) too long")
	}
	return nil
}

// ValidID reports whether s is a valid bus/device identifier.
func ValidID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case (r == '-' || r == '_') && i > 0:
		default:
			return false
		}
	}
	return true
}

// Example is an annotated example configuration file.
//
//go:embed example.yaml
var Example string
