// Package recorder periodically reads every enabled encoder and stores the
// measurements in the database.
package recorder

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/schedule"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// DeviceResult is the outcome of one device in a run.
type DeviceResult struct {
	DeviceID string `json:"device_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// Run summarises one recording pass.
type Run struct {
	Tick       time.Time      `json:"tick"`
	StartedAt  time.Time      `json:"started_at"`
	DurationMS int64          `json:"duration_ms"`
	Recorded   int            `json:"recorded"`
	Failed     int            `json:"failed"`
	StoreError string         `json:"store_error,omitempty"`
	Devices    []DeviceResult `json:"devices"`
}

// Status is the recorder state exposed by the API.
type Status struct {
	Enabled  bool       `json:"enabled"`
	Schedule string     `json:"schedule"`
	NextRun  *time.Time `json:"next_run,omitempty"`
	LastRun  *Run       `json:"last_run,omitempty"`
	Running  bool       `json:"running"`
}

// Recorder records measurements on a schedule.
type Recorder struct {
	cfg      config.RecorderConfig
	storage  config.StorageConfig
	inv      *inventory.Manager
	store    *store.Store
	log      *slog.Logger
	schedule schedule.Schedule

	runMu sync.Mutex // serialises runs (scheduled and manual)

	mu        sync.Mutex
	next      time.Time
	last      *Run
	running   bool
	lastPrune time.Time
}

// New creates a recorder.
func New(cfg config.RecorderConfig, storage config.StorageConfig, inv *inventory.Manager, st *store.Store, log *slog.Logger) (*Recorder, error) {
	s, err := schedule.Parse(cfg.Interval.D(), cfg.Cron, time.Local)
	if err != nil {
		return nil, err
	}
	return &Recorder{cfg: cfg, storage: storage, inv: inv, store: st, log: log, schedule: s}, nil
}

// Start runs the schedule until ctx is cancelled.
func (r *Recorder) Start(ctx context.Context) {
	if !r.cfg.Enabled {
		return
	}
	r.log.Info("recorder started", "schedule", r.schedule.String())
	schedule.Run(ctx, r.schedule, r.log, func(t time.Time) {
		r.mu.Lock()
		r.next = t
		r.mu.Unlock()
	}, func(ctx context.Context, tick time.Time) {
		r.RunOnce(ctx, tick)
	})
}

// Status returns the recorder state.
func (r *Recorder) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Status{Enabled: r.cfg.Enabled, Schedule: r.schedule.String(), LastRun: r.last, Running: r.running}
	if r.cfg.Enabled && !r.next.IsZero() {
		n := r.next
		s.NextRun = &n
	}
	return s
}

// RunOnce reads every recordable device and stores the measurements stamped
// with tick. Buses are read in parallel; devices on a bus sequentially.
func (r *Recorder) RunOnce(ctx context.Context, tick time.Time) Run {
	r.runMu.Lock()
	defer r.runMu.Unlock()
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	run := Run{Tick: tick, StartedAt: time.Now()}
	ctx, cancel := context.WithTimeout(ctx, r.cfg.Timeout.D())
	defer cancel()

	byBus := map[string][]string{}
	for _, d := range r.inv.Devices() {
		if d.Config.IsRecorded() {
			byBus[d.Config.Bus] = append(byBus[d.Config.Bus], d.Config.ID)
		}
	}
	var (
		mu           sync.Mutex
		wg           sync.WaitGroup
		measurements []store.Measurement
	)
	for _, ids := range byBus {
		wg.Go(func() {
			for _, id := range ids {
				reading, err := r.inv.Read(ctx, id, 0, tick)
				mu.Lock()
				res := DeviceResult{DeviceID: id, OK: err == nil}
				if err != nil {
					res.Error = err.Error()
					run.Failed++
					r.log.Warn("recording failed", "device", id, "err", err)
				} else {
					measurements = append(measurements, reading.Measurement())
				}
				run.Devices = append(run.Devices, res)
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if r.store.Enabled() && len(measurements) > 0 {
		sctx, scancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		if err := r.store.Insert(sctx, measurements); err != nil {
			run.StoreError = err.Error()
			r.log.Error("storing measurements failed", "err", err, "count", len(measurements))
		} else {
			run.Recorded = len(measurements)
		}
		scancel()
		r.prune(context.WithoutCancel(ctx))
	}
	run.DurationMS = time.Since(run.StartedAt).Milliseconds()
	r.log.Debug("recording pass finished", "tick", tick, "recorded", run.Recorded, "failed", run.Failed, "ms", run.DurationMS)

	r.mu.Lock()
	r.last = &run
	r.mu.Unlock()
	return run
}

// prune applies storage.retention at most hourly. TimescaleDB uses a native
// retention policy instead.
func (r *Recorder) prune(ctx context.Context) {
	ret := r.storage.Retention.D()
	if ret <= 0 || r.store.Driver() != config.DriverSQLite || time.Since(r.lastPrune) < time.Hour {
		return
	}
	r.lastPrune = time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	n, err := r.store.Prune(ctx, time.Now().Add(-ret))
	if err != nil {
		r.log.Warn("pruning old measurements failed", "err", err)
		return
	}
	if n > 0 {
		r.log.Info("pruned old measurements", "rows", n)
	}
}
