package export

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/schedule"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// ErrNotFound is returned for unknown export files.
var ErrNotFound = errors.New("export file not found")

// FileInfo describes a generated CSV file.
type FileInfo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// Result summarises one export pass.
type Result struct {
	Tick       time.Time  `json:"tick"`
	From       time.Time  `json:"from"`
	To         time.Time  `json:"to"`
	Files      []FileInfo `json:"files"`
	Rows       int        `json:"rows"`
	DurationMS int64      `json:"duration_ms"`
	Error      string     `json:"error,omitempty"`
}

// Status is the exporter state exposed by the API.
type Status struct {
	Enabled   bool       `json:"enabled"`
	Schedule  string     `json:"schedule"`
	Window    string     `json:"window"`
	Directory string     `json:"directory"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	LastRun   *Result    `json:"last_run,omitempty"`
}

// Exporter generates CSV files on a schedule.
type Exporter struct {
	cfg      config.ExportConfig
	maxRows  int
	loc      *time.Location
	store    *store.Store
	inv      *inventory.Manager
	log      *slog.Logger
	schedule schedule.Schedule

	runMu sync.Mutex
	mu    sync.Mutex
	next  time.Time
	last  *Result
}

// New creates an exporter and ensures the output directory exists when
// exports are enabled.
func New(cfg config.ExportConfig, maxRows int, st *store.Store, inv *inventory.Manager, log *slog.Logger) (*Exporter, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, err
	}
	s, err := schedule.Parse(cfg.Interval.D(), cfg.Cron, loc)
	if err != nil {
		return nil, err
	}
	if cfg.Enabled {
		if err := os.MkdirAll(cfg.Directory, 0o750); err != nil {
			return nil, fmt.Errorf("create export directory: %w", err)
		}
	}
	return &Exporter{cfg: cfg, maxRows: maxRows, loc: loc, store: st, inv: inv, log: log, schedule: s}, nil
}

// Location returns the export time zone.
func (e *Exporter) Location() *time.Location { return e.loc }

// Start runs the schedule until ctx is cancelled.
func (e *Exporter) Start(ctx context.Context) {
	if !e.cfg.Enabled {
		return
	}
	e.log.Info("CSV exporter started", "schedule", e.schedule.String(), "window", e.cfg.Window.String(), "dir", e.cfg.Directory)
	schedule.Run(ctx, e.schedule, e.log, func(t time.Time) {
		e.mu.Lock()
		e.next = t
		e.mu.Unlock()
	}, func(ctx context.Context, tick time.Time) {
		if _, err := e.RunOnce(ctx, tick); err != nil {
			e.log.Error("CSV export failed", "err", err)
		}
	})
}

// Status returns the exporter state.
func (e *Exporter) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := Status{Enabled: e.cfg.Enabled, Schedule: e.schedule.String(), Window: schedule.ShortDuration(e.cfg.Window.D()), Directory: e.cfg.Directory, LastRun: e.last}
	if e.cfg.Enabled && !e.next.IsZero() {
		n := e.next
		s.NextRun = &n
	}
	return s
}

// Format returns the configured CSV format with current device names.
func (e *Exporter) Format(columns []string) Format {
	if len(columns) == 0 {
		columns = e.cfg.Columns
	}
	names := map[string]string{}
	for _, d := range e.inv.Devices() {
		names[d.Config.ID] = d.Config.Name
	}
	return Format{
		Columns: columns, Delimiter: []rune(e.cfg.Delimiter)[0], Header: e.cfg.Header,
		Location: e.loc, TimeFormat: e.cfg.TimeFormat, Names: names,
	}
}

// RunOnce exports the window ending at tick: [tick-window, tick).
func (e *Exporter) RunOnce(ctx context.Context, tick time.Time) (*Result, error) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	started := time.Now()
	res := &Result{Tick: tick, From: tick.Add(-e.cfg.Window.D()), To: tick, Files: []FileInfo{}}
	err := e.run(ctx, res)
	if err != nil {
		res.Error = err.Error()
	}
	res.DurationMS = time.Since(started).Milliseconds()
	e.mu.Lock()
	e.last = res
	e.mu.Unlock()
	e.cleanup()
	return res, err
}

func (e *Exporter) run(ctx context.Context, res *Result) error {
	if err := os.MkdirAll(e.cfg.Directory, 0o750); err != nil {
		return err
	}
	devices := e.cfg.Devices
	if len(devices) == 0 {
		devices = e.inv.DeviceIDs()
	}
	groups := [][]string{devices}
	if e.cfg.PerDevice {
		groups = nil
		for _, d := range devices {
			groups = append(groups, []string{d})
		}
	}
	var errs []error
	for _, ids := range groups {
		device := "all"
		if e.cfg.PerDevice {
			device = ids[0]
		}
		name := e.filename(res.From, res.To, device)
		info, rows, err := e.writeFile(ctx, name, store.Filter{Devices: ids, From: res.From, To: res.To})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		res.Rows += rows
		if info != nil {
			res.Files = append(res.Files, *info)
			e.log.Info("CSV export written", "file", info.Name, "rows", rows)
		}
	}
	return errors.Join(errs...)
}

// filename expands {from}, {to}, {date} and {device} in the configured
// pattern.
func (e *Exporter) filename(from, to time.Time, device string) string {
	const stamp = "20060102T150405"
	r := strings.NewReplacer(
		"{from}", from.In(e.loc).Format(stamp),
		"{to}", to.In(e.loc).Format(stamp),
		"{date}", from.In(e.loc).Format("2006-01-02"),
		"{device}", device,
	)
	name := r.Replace(e.cfg.Filename)
	if e.cfg.PerDevice && !strings.Contains(e.cfg.Filename, "{device}") {
		name = strings.TrimSuffix(name, ".csv") + "_" + device + ".csv"
	}
	return name
}

func (e *Exporter) writeFile(ctx context.Context, name string, f store.Filter) (*FileInfo, int, error) {
	final := filepath.Join(e.cfg.Directory, name)
	tmp, err := os.CreateTemp(e.cfg.Directory, "."+name+".*.tmp")
	if err != nil {
		return nil, 0, err
	}
	defer os.Remove(tmp.Name())
	w, err := NewWriter(tmp, e.Format(nil))
	if err != nil {
		tmp.Close()
		return nil, 0, err
	}
	_, err = e.store.Stream(ctx, f, store.Asc, e.maxRows, w.Write)
	if err == nil {
		err = w.Flush()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, 0, err
	}
	if w.Rows() == 0 && e.cfg.SkipEmpty {
		return nil, 0, nil
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return nil, 0, err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		return nil, 0, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return nil, 0, err
	}
	return &FileInfo{Name: name, Size: st.Size(), Modified: st.ModTime()}, w.Rows(), nil
}

// List returns the CSV files in the export directory, newest first.
func (e *Exporter) List() ([]FileInfo, error) {
	entries, err := os.ReadDir(e.cfg.Directory)
	if errors.Is(err, fs.ErrNotExist) {
		return []FileInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []FileInfo{}
	for _, en := range entries {
		if en.IsDir() || !validName(en.Name()) {
			continue
		}
		info, err := en.Info()
		if err != nil {
			continue
		}
		out = append(out, FileInfo{Name: en.Name(), Size: info.Size(), Modified: info.ModTime()})
	}
	slices.SortFunc(out, func(a, b FileInfo) int { return b.Modified.Compare(a.Modified) })
	return out, nil
}

// Path resolves an export file name to its path, rejecting anything that is
// not a plain CSV file name inside the export directory.
func (e *Exporter) Path(name string) (string, error) {
	if !validName(name) {
		return "", ErrNotFound
	}
	p := filepath.Join(e.cfg.Directory, name)
	st, err := os.Lstat(p)
	if err != nil || !st.Mode().IsRegular() {
		return "", ErrNotFound
	}
	return p, nil
}

// Delete removes an export file.
func (e *Exporter) Delete(name string) error {
	p, err := e.Path(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func validName(name string) bool {
	return name != "" && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".csv") &&
		!strings.ContainsAny(name, `/\`) && filepath.Base(name) == name && len(name) <= 255
}

// cleanup deletes files older than export.keep.
func (e *Exporter) cleanup() {
	keep := e.cfg.Keep.D()
	if keep <= 0 {
		return
	}
	files, err := e.List()
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-keep)
	for _, f := range files {
		if f.Modified.Before(cutoff) {
			if err := os.Remove(filepath.Join(e.cfg.Directory, f.Name)); err == nil {
				e.log.Info("deleted expired CSV export", "file", f.Name)
			}
		}
	}
}
