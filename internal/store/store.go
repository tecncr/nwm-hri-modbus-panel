// Package store persists recorded measurements in SQLite or a TimescaleDB
// hypertable. Only measurements live in the database; all configuration lives
// in the config file.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
)

// Measurement is one recorded reading of an encoder.
type Measurement struct {
	Time              time.Time  `gorm:"primaryKey;not null" json:"time"`
	DeviceID          string     `gorm:"primaryKey;size:64;not null" json:"device_id"`
	Bus               string     `gorm:"size:64;not null" json:"bus"`
	Slave             int        `gorm:"not null" json:"slave"`
	TotalRaw          int64      `gorm:"not null" json:"total_raw"`
	ForwardRaw        int64      `gorm:"not null" json:"forward_raw"`
	ReverseRaw        int64      `gorm:"not null" json:"reverse_raw"`
	LitersPerUnit     float64    `gorm:"not null" json:"liters_per_unit"`
	TotalLiters       float64    `gorm:"not null" json:"total_liters"`
	ForwardLiters     float64    `gorm:"not null" json:"forward_liters"`
	ReverseLiters     float64    `gorm:"not null" json:"reverse_liters"`
	BatteryStatus     int        `gorm:"not null" json:"battery_status"`
	Failure           int        `gorm:"not null" json:"failure"`
	BasePulseLiters   int        `gorm:"not null" json:"base_pulse_liters"`
	OutputPulseLiters int        `gorm:"not null" json:"output_pulse_liters"`
	DeviceClock       *time.Time `json:"device_clock,omitempty"`
}

// TableName pins the table name.
func (Measurement) TableName() string { return "measurements" }

// Normalize stores times as UTC with second precision so that SQLite text
// comparisons stay ordered.
func (m *Measurement) Normalize() {
	m.Time = m.Time.UTC().Truncate(time.Second)
	if m.DeviceClock != nil {
		t := m.DeviceClock.UTC().Truncate(time.Second)
		m.DeviceClock = &t
	}
}

// Filter restricts a query.
type Filter struct {
	Devices []string
	From    time.Time // inclusive
	To      time.Time // exclusive
}

// Order is the sort direction by time.
type Order string

const (
	Asc  Order = "asc"
	Desc Order = "desc"
)

// Bucket is one aggregated time interval for one device.
type Bucket struct {
	DeviceID   string    `json:"device_id"`
	Start      time.Time `json:"start"`
	Samples    int64     `json:"samples"`
	FirstTime  time.Time `json:"first_time"`
	LastTime   time.Time `json:"last_time"`
	MinTotal   float64   `json:"min_total_liters"`
	MaxTotal   float64   `json:"max_total_liters"`
	MinForward float64   `json:"min_forward_liters"`
	MaxForward float64   `json:"max_forward_liters"`
	MinReverse float64   `json:"min_reverse_liters"`
	MaxReverse float64   `json:"max_reverse_liters"`
}

// Stats summarises the stored data.
type Stats struct {
	Driver    string     `json:"driver"`
	Rows      int64      `json:"rows"`
	Estimated bool       `json:"estimated"`
	Oldest    *time.Time `json:"oldest,omitempty"`
	Newest    *time.Time `json:"newest,omitempty"`
}

// ErrDisabled is returned by every method when storage is disabled.
var ErrDisabled = errors.New("measurement storage is disabled (storage.driver: none)")

// Store is the measurement repository.
type Store struct {
	db     *gorm.DB
	driver string
}

// Open connects to the configured database and migrates the schema. With
// driver "none" it returns a Store whose methods fail with ErrDisabled.
func Open(ctx context.Context, cfg config.StorageConfig, log *slog.Logger) (*Store, error) {
	gcfg := &gorm.Config{Logger: logger.Discard, PrepareStmt: true}
	s := &Store{driver: cfg.Driver}
	var err error
	switch cfg.Driver {
	case config.DriverNone:
		return s, nil
	case config.DriverSQLite:
		dsn := cfg.SQLite.Path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
		s.db, err = gorm.Open(sqlite.Open(dsn), gcfg)
		if err != nil {
			return nil, fmt.Errorf("open sqlite %s: %w", cfg.SQLite.Path, err)
		}
		sqlDB, _ := s.db.DB()
		// SQLite allows a single writer; one connection avoids SQLITE_BUSY.
		sqlDB.SetMaxOpenConns(1)
	case config.DriverTimescaleDB:
		s.db, err = gorm.Open(postgres.Open(cfg.TimescaleDB.DSN), gcfg)
		if err != nil {
			return nil, fmt.Errorf("open timescaledb: %w", err)
		}
		sqlDB, _ := s.db.DB()
		sqlDB.SetMaxOpenConns(cfg.TimescaleDB.MaxOpenConns)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	default:
		return nil, fmt.Errorf("unknown storage driver %q", cfg.Driver)
	}
	if err := s.migrate(ctx, cfg, log); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context, cfg config.StorageConfig, log *slog.Logger) error {
	db := s.db.WithContext(ctx)
	if s.driver == config.DriverTimescaleDB {
		if err := db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error; err != nil {
			log.Warn("could not ensure the timescaledb extension; continuing", "err", err)
		}
	}
	if err := db.AutoMigrate(&Measurement{}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_measurements_time ON measurements ("time")`).Error; err != nil {
		return fmt.Errorf("migrate index: %w", err)
	}
	if s.driver != config.DriverTimescaleDB {
		return nil
	}
	chunk := fmt.Sprintf("%d seconds", int64(cfg.TimescaleDB.ChunkInterval.D().Seconds()))
	if err := db.Exec("SELECT create_hypertable('measurements', by_range('time', ?::interval), if_not_exists => TRUE, migrate_data => TRUE)", chunk).Error; err != nil {
		return fmt.Errorf("create hypertable: %w", err)
	}
	// Apply the configured chunk interval to future chunks even when the
	// hypertable already existed.
	if err := db.Exec("SELECT set_chunk_time_interval('measurements', ?::interval)", chunk).Error; err != nil {
		return fmt.Errorf("set chunk interval: %w", err)
	}
	if after := cfg.TimescaleDB.CompressionAfter.D(); after > 0 {
		stmts := []string{
			"ALTER TABLE measurements SET (timescaledb.compress, timescaledb.compress_segmentby = 'device_id', timescaledb.compress_orderby = 'time DESC')",
			fmt.Sprintf("SELECT add_compression_policy('measurements', INTERVAL '%d seconds', if_not_exists => TRUE)", int64(after.Seconds())),
		}
		for _, q := range stmts {
			if err := db.Exec(q).Error; err != nil {
				log.Warn("could not enable compression", "err", err)
				break
			}
		}
	}
	if ret := cfg.Retention.D(); ret > 0 {
		q := fmt.Sprintf("SELECT add_retention_policy('measurements', INTERVAL '%d seconds', if_not_exists => TRUE)", int64(ret.Seconds()))
		if err := db.Exec(q).Error; err != nil {
			log.Warn("could not add retention policy; falling back to periodic pruning", "err", err)
		}
	}
	return nil
}

// Enabled reports whether a database is configured.
func (s *Store) Enabled() bool { return s.db != nil }

// Driver returns the configured driver name.
func (s *Store) Driver() string { return s.driver }

// Close closes the database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping checks connectivity.
func (s *Store) Ping(ctx context.Context) error {
	if s.db == nil {
		return ErrDisabled
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Insert stores measurements, ignoring duplicates (same device and second).
func (s *Store) Insert(ctx context.Context, ms []Measurement) error {
	if s.db == nil {
		return ErrDisabled
	}
	if len(ms) == 0 {
		return nil
	}
	for i := range ms {
		ms[i].Normalize()
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(ms, 500).Error
}

// bound converts a query bound to the stored second precision. Stored times
// are whole seconds, so for any stored t: t >= x ⇔ t >= ceil(x) and
// t < x ⇔ t < ceil(x).
func bound(t time.Time) time.Time {
	t = t.UTC()
	if tr := t.Truncate(time.Second); !tr.Equal(t) {
		return tr.Add(time.Second)
	}
	return t
}

func (s *Store) filtered(ctx context.Context, f Filter) *gorm.DB {
	q := s.db.WithContext(ctx).Model(&Measurement{})
	if len(f.Devices) > 0 {
		q = q.Where("device_id IN ?", f.Devices)
	}
	if !f.From.IsZero() {
		q = q.Where(`"time" >= ?`, bound(f.From))
	}
	if !f.To.IsZero() {
		q = q.Where(`"time" < ?`, bound(f.To))
	}
	return q
}

func orderClause(o Order) string {
	if o == Asc {
		return `"time" ASC, device_id ASC`
	}
	return `"time" DESC, device_id ASC`
}

// Query returns one page of measurements and the total number of matching rows.
func (s *Store) Query(ctx context.Context, f Filter, offset, limit int, order Order) ([]Measurement, int64, error) {
	if s.db == nil {
		return nil, 0, ErrDisabled
	}
	var total int64
	if err := s.filtered(ctx, f).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	out := make([]Measurement, 0, limit)
	if int64(offset) >= total {
		return out, total, nil
	}
	err := s.filtered(ctx, f).Order(orderClause(order)).Offset(offset).Limit(limit).Find(&out).Error
	return out, total, err
}

// Stream calls fn for each matching measurement in time order, stopping after
// limit rows. It returns the number of rows visited.
func (s *Store) Stream(ctx context.Context, f Filter, order Order, limit int, fn func(*Measurement) error) (int, error) {
	if s.db == nil {
		return 0, ErrDisabled
	}
	rows, err := s.filtered(ctx, f).Order(orderClause(order)).Limit(limit).Rows()
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var m Measurement
		if err := s.db.ScanRows(rows, &m); err != nil {
			return n, err
		}
		if err := fn(&m); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

// Latest returns the most recent measurement of each device (all devices when
// ids is empty).
func (s *Store) Latest(ctx context.Context, ids []string) ([]Measurement, error) {
	if s.db == nil {
		return nil, ErrDisabled
	}
	var out []Measurement
	sub := s.db.Model(&Measurement{}).Select(`device_id, MAX("time") AS "time"`).Group("device_id")
	if len(ids) > 0 {
		sub = sub.Where("device_id IN ?", ids)
	}
	err := s.db.WithContext(ctx).Model(&Measurement{}).
		Joins(`JOIN (?) AS latest ON latest.device_id = measurements.device_id AND latest."time" = measurements."time"`, sub).
		Order("measurements.device_id").Find(&out).Error
	return out, err
}

// Before returns the latest measurement of a device strictly before t, or nil.
func (s *Store) Before(ctx context.Context, device string, t time.Time) (*Measurement, error) {
	if s.db == nil {
		return nil, ErrDisabled
	}
	var m Measurement
	err := s.db.WithContext(ctx).Where(`device_id = ? AND "time" < ?`, device, bound(t)).
		Order(`"time" DESC`).Limit(1).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}

// Aggregate groups measurements into fixed buckets per device. Buckets are
// aligned to multiples of bucket counted from the Unix epoch, shifted by the
// UTC offset of loc so that daily buckets start at local midnight.
func (s *Store) Aggregate(ctx context.Context, f Filter, bucket time.Duration, loc *time.Location) ([]Bucket, error) {
	if s.db == nil {
		return nil, ErrDisabled
	}
	secs := int64(bucket.Seconds())
	if secs < 1 {
		return nil, errors.New("bucket must be at least 1s")
	}
	_, offset := f.From.In(loc).Zone()
	var bucketExpr string
	switch s.driver {
	case config.DriverSQLite:
		bucketExpr = fmt.Sprintf("((CAST(strftime('%%s', \"time\") AS INTEGER) + %d) / %d) * %d - %d", offset, secs, secs, offset)
	default:
		bucketExpr = fmt.Sprintf("CAST(floor((extract(epoch FROM \"time\") + %d) / %d) AS BIGINT) * %d - %d", offset, secs, secs, offset)
	}
	type row struct {
		DeviceID   string
		BucketUnix int64
		Samples    int64
		FirstTime  string
		LastTime   string
		MinTotal   float64
		MaxTotal   float64
		MinForward float64
		MaxForward float64
		MinReverse float64
		MaxReverse float64
	}
	var rows []row
	err := s.filtered(ctx, f).Select(strings.Join([]string{
		"device_id",
		bucketExpr + " AS bucket_unix",
		"COUNT(*) AS samples",
		timeText(s.driver, `MIN("time")`) + " AS first_time",
		timeText(s.driver, `MAX("time")`) + " AS last_time",
		"MIN(total_liters) AS min_total", "MAX(total_liters) AS max_total",
		"MIN(forward_liters) AS min_forward", "MAX(forward_liters) AS max_forward",
		"MIN(reverse_liters) AS min_reverse", "MAX(reverse_liters) AS max_reverse",
	}, ", ")).Group("device_id, bucket_unix").Order("device_id, bucket_unix").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Bucket, 0, len(rows))
	for _, r := range rows {
		out = append(out, Bucket{
			DeviceID: r.DeviceID, Start: time.Unix(r.BucketUnix, 0).In(loc), Samples: r.Samples,
			FirstTime: parseTimeText(r.FirstTime), LastTime: parseTimeText(r.LastTime),
			MinTotal: r.MinTotal, MaxTotal: r.MaxTotal, MinForward: r.MinForward, MaxForward: r.MaxForward,
			MinReverse: r.MinReverse, MaxReverse: r.MaxReverse,
		})
	}
	return out, nil
}

// timeText renders an aggregate time expression as RFC 3339 text so that both
// drivers scan identically.
func timeText(driver, expr string) string {
	if driver == config.DriverSQLite {
		return "strftime('%Y-%m-%dT%H:%M:%SZ', " + expr + ")"
	}
	return "to_char(" + expr + " AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')"
}

func parseTimeText(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// Prune deletes measurements older than before.
func (s *Store) Prune(ctx context.Context, before time.Time) (int64, error) {
	if s.db == nil {
		return 0, ErrDisabled
	}
	res := s.db.WithContext(ctx).Where(`"time" < ?`, bound(before)).Delete(&Measurement{})
	return res.RowsAffected, res.Error
}

// Stats returns the row count and the time span of stored data. On
// TimescaleDB the row count is an estimate.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	st := Stats{Driver: s.driver}
	if s.db == nil {
		return st, nil
	}
	var r struct {
		RowCount int64
		Oldest   *string
		Newest   *string
	}
	count := "COUNT(*)"
	if s.driver == config.DriverTimescaleDB {
		count = "(SELECT approximate_row_count('measurements'))"
		st.Estimated = true
	}
	err := s.db.WithContext(ctx).Model(&Measurement{}).Select(
		count + " AS row_count, " +
			timeText(s.driver, `MIN("time")`) + " AS oldest, " + timeText(s.driver, `MAX("time")`) + " AS newest",
	).Scan(&r).Error
	if err != nil {
		return st, err
	}
	st.Rows = r.RowCount
	if r.Oldest != nil {
		t := parseTimeText(*r.Oldest)
		st.Oldest = &t
	}
	if r.Newest != nil {
		t := parseTimeText(*r.Newest)
		st.Newest = &t
	}
	return st, nil
}
