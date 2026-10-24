package inventory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/hri"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

// ScanResult describes an encoder found during a bus scan.
type ScanResult struct {
	Slave        int           `json:"slave"`
	SlaveAddress int           `json:"slave_address_register"`
	DeviceType   *int          `json:"device_type,omitempty"`
	Firmware     *hri.Firmware `json:"firmware,omitempty"`
	ConfiguredAs string        `json:"configured_as,omitempty"`
}

// ScanJob is an asynchronous bus scan.
type ScanJob struct {
	mu         sync.Mutex
	Bus        string        `json:"bus"`
	From       int           `json:"from"`
	To         int           `json:"to"`
	Timeout    time.Duration `json:"-"`
	TimeoutMS  int64         `json:"timeout_ms"`
	Current    int           `json:"current"`
	Done       bool          `json:"done"`
	Cancelled  bool          `json:"cancelled"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	Found      []ScanResult  `json:"found"`
	cancel     context.CancelFunc
}

// ScanSnapshot is a copy of a ScanJob safe to serialise.
type ScanSnapshot struct {
	Bus        string       `json:"bus"`
	From       int          `json:"from"`
	To         int          `json:"to"`
	TimeoutMS  int64        `json:"timeout_ms"`
	Current    int          `json:"current"`
	Progress   float64      `json:"progress"`
	Done       bool         `json:"done"`
	Cancelled  bool         `json:"cancelled"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	Found      []ScanResult `json:"found"`
}

func (j *ScanJob) snapshot() ScanSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	total := j.To - j.From + 1
	done := j.Current - j.From
	if j.Done {
		done = total
	}
	return ScanSnapshot{
		Bus: j.Bus, From: j.From, To: j.To, TimeoutMS: j.TimeoutMS, Current: j.Current,
		Progress: float64(max(done, 0)) / float64(total), Done: j.Done, Cancelled: j.Cancelled,
		StartedAt: j.StartedAt, FinishedAt: j.FinishedAt, Found: append([]ScanResult{}, j.Found...),
	}
}

// StartScan probes slave addresses from..to on a bus in the background,
// reading register 0x0000 with the given per-address timeout.
func (m *Manager) StartScan(busID string, from, to int, timeout time.Duration) (ScanSnapshot, error) {
	bus, err := m.Bus(busID)
	if err != nil {
		return ScanSnapshot{}, err
	}
	if from < modbus.MinSlaveAddress || to > modbus.MaxSlaveAddress || from > to {
		return ScanSnapshot{}, fmt.Errorf("%w: scan range must be within 1-247 and from <= to", ErrInvalid)
	}
	if timeout < 20*time.Millisecond || timeout > 5*time.Second {
		return ScanSnapshot{}, fmt.Errorf("%w: timeout must be between 20ms and 5s", ErrInvalid)
	}
	bus.scanMu.Lock()
	defer bus.scanMu.Unlock()
	if bus.scan != nil && !bus.scan.snapshot().Done {
		return ScanSnapshot{}, fmt.Errorf("%w: a scan is already running on bus %q", ErrConflict, busID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &ScanJob{
		Bus: busID, From: from, To: to, Timeout: timeout, TimeoutMS: timeout.Milliseconds(),
		Current: from, StartedAt: time.Now(), Found: []ScanResult{}, cancel: cancel,
	}
	bus.scan = job
	go m.runScan(ctx, bus, job)
	return job.snapshot(), nil
}

func (m *Manager) runScan(ctx context.Context, bus *Bus, job *ScanJob) {
	defer func() {
		job.mu.Lock()
		job.Done = true
		now := time.Now()
		job.FinishedAt = &now
		job.mu.Unlock()
		job.cancel()
	}()
	for addr := job.From; addr <= job.To; addr++ {
		job.mu.Lock()
		job.Current = addr
		job.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		dev := hri.Device{Client: bus.Client, Slave: uint8(addr)}
		probe, cancel := context.WithTimeout(ctx, job.Timeout)
		reg, err := dev.ReadSlaveAddress(probe)
		cancel()
		if err != nil {
			// A non-timeout error (exception, CRC) still means something answered.
			if errors.Is(err, modbus.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				continue
			}
			if _, ok := errors.AsType[*modbus.ExceptionError](err); !ok {
				continue
			}
		}
		res := ScanResult{Slave: addr, SlaveAddress: int(reg)}
		probe, cancel = context.WithTimeout(ctx, job.Timeout*2)
		if t, err := dev.ReadDeviceType(probe); err == nil {
			v := int(t)
			res.DeviceType = &v
		}
		if f, err := dev.ReadFirmware(probe); err == nil {
			res.Firmware = &f
		}
		cancel()
		if id, ok := m.DeviceOnBus(bus.Config.ID, uint8(addr)); ok {
			res.ConfiguredAs = id
		}
		job.mu.Lock()
		job.Found = append(job.Found, res)
		job.mu.Unlock()
	}
}

// Scan returns the current or last scan of a bus.
func (m *Manager) Scan(busID string) (*ScanSnapshot, error) {
	bus, err := m.Bus(busID)
	if err != nil {
		return nil, err
	}
	bus.scanMu.Lock()
	defer bus.scanMu.Unlock()
	if bus.scan == nil {
		return nil, fmt.Errorf("%w: no scan has been run on bus %q", ErrNotFound, busID)
	}
	s := bus.scan.snapshot()
	return &s, nil
}

// CancelScan stops a running scan.
func (m *Manager) CancelScan(busID string) error {
	bus, err := m.Bus(busID)
	if err != nil {
		return err
	}
	bus.scanMu.Lock()
	defer bus.scanMu.Unlock()
	if bus.scan == nil {
		return fmt.Errorf("%w: no scan on bus %q", ErrNotFound, busID)
	}
	bus.scan.mu.Lock()
	bus.scan.Cancelled = !bus.scan.Done
	bus.scan.mu.Unlock()
	bus.scan.cancel()
	return nil
}
