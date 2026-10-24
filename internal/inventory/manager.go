// Package inventory manages the configured buses and encoders at runtime:
// Modbus clients, unit conversion, a short-lived live-reading cache, bus
// scans and persistence of inventory changes back to the config file.
package inventory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/hri"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

// Errors returned by the manager.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid")
	ErrDisabled = errors.New("device is disabled")
)

// Bus is a running bus.
type Bus struct {
	Config config.BusConfig
	Client *modbus.Client

	scanMu sync.Mutex
	scan   *ScanJob
}

// Device is a configured encoder with its runtime state.
type Device struct {
	Config config.DeviceConfig
	Loc    *time.Location

	mu        sync.Mutex
	last      *Reading
	lastOKAt  time.Time
	lastErr   string
	lastErrAt time.Time
}

// DeviceState is the runtime state exposed by the API.
type DeviceState struct {
	Online    bool       `json:"online"`
	LastOKAt  *time.Time `json:"last_ok_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
	LastErrAt *time.Time `json:"last_error_at,omitempty"`
	Last      *Reading   `json:"last_reading,omitempty"`
}

// Manager owns buses and devices.
type Manager struct {
	mu         sync.RWMutex
	buses      map[string]*Bus
	devices    map[string]*Device
	busOrder   []string
	devOrder   []string
	modbusCfg  config.ModbusConfig
	configPath string
	log        *slog.Logger
}

// New builds a manager from the configuration. configPath is where inventory
// changes are persisted ("" keeps them in memory only).
func New(cfg *config.Config, configPath string, log *slog.Logger) (*Manager, error) {
	m := &Manager{
		buses: map[string]*Bus{}, devices: map[string]*Device{},
		modbusCfg: cfg.Modbus, configPath: configPath, log: log,
	}
	for _, b := range cfg.Buses {
		bus, err := m.newBus(b)
		if err != nil {
			m.Close()
			return nil, err
		}
		m.buses[b.ID] = bus
		m.busOrder = append(m.busOrder, b.ID)
	}
	for _, d := range cfg.Devices {
		dev, err := newDevice(d)
		if err != nil {
			m.Close()
			return nil, err
		}
		m.devices[d.ID] = dev
		m.devOrder = append(m.devOrder, d.ID)
	}
	return m, nil
}

// Persistent reports whether inventory changes are written to a file.
func (m *Manager) Persistent() bool { return m.configPath != "" }

// ConfigPath returns the file inventory changes are written to.
func (m *Manager) ConfigPath() string { return m.configPath }

// ModbusConfig returns the global Modbus settings.
func (m *Manager) ModbusConfig() config.ModbusConfig { return m.modbusCfg }

func (m *Manager) newBus(b config.BusConfig) (*Bus, error) {
	var dial modbus.Dialer
	baud := 9600
	switch b.Transport {
	case config.TransportSerial:
		baud = b.Serial.BaudRate
		dial = modbus.SerialDialer(modbus.SerialConfig{
			Device: b.Serial.Device, BaudRate: b.Serial.BaudRate, DataBits: b.Serial.DataBits,
			Parity: b.Serial.Parity, StopBits: b.Serial.StopBits,
		})
	case config.TransportTCP:
		dial = modbus.TCPDialer(b.TCP.Address, b.TCP.ConnectTimeout.D())
	case config.TransportSimulator:
		slaves := make([]uint8, len(b.Simulator.Slaves))
		for i, a := range b.Simulator.Slaves {
			slaves[i] = uint8(a)
		}
		dial = hri.NewSimulator(slaves, b.Simulator.FlowLPM).Dialer()
	default:
		return nil, fmt.Errorf("%w: bus %s: unknown transport %q", ErrInvalid, b.ID, b.Transport)
	}
	opts := modbus.Options{
		Timeout:    m.modbusCfg.Timeout.D(),
		Retries:    m.modbusCfg.Retries,
		FrameDelay: modbus.FrameDelayFor(baud),
	}
	if b.Timeout > 0 {
		opts.Timeout = b.Timeout.D()
	}
	if b.Retries != nil {
		opts.Retries = *b.Retries
	}
	if b.FrameDelay > 0 {
		opts.FrameDelay = b.FrameDelay.D()
	}
	return &Bus{Config: b, Client: modbus.NewClient(dial, opts)}, nil
}

func newDevice(d config.DeviceConfig) (*Device, error) {
	loc := time.Local
	if d.Timezone != "" {
		l, err := time.LoadLocation(d.Timezone)
		if err != nil {
			return nil, fmt.Errorf("%w: device %s: %v", ErrInvalid, d.ID, err)
		}
		loc = l
	}
	return &Device{Config: d, Loc: loc}, nil
}

// Close closes every bus client.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.buses {
		_ = b.Client.Close()
	}
}

// Buses returns all buses in configuration order.
func (m *Manager) Buses() []*Bus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Bus, 0, len(m.busOrder))
	for _, id := range m.busOrder {
		out = append(out, m.buses[id])
	}
	return out
}

// Bus returns one bus.
func (m *Manager) Bus(id string) (*Bus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.buses[id]
	if !ok {
		return nil, fmt.Errorf("%w: bus %q", ErrNotFound, id)
	}
	return b, nil
}

// Devices returns all devices in configuration order.
func (m *Manager) Devices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Device, 0, len(m.devOrder))
	for _, id := range m.devOrder {
		out = append(out, m.devices[id])
	}
	return out
}

// Device returns one device.
func (m *Manager) Device(id string) (*Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("%w: device %q", ErrNotFound, id)
	}
	return d, nil
}

// DeviceIDs returns the IDs of all devices.
func (m *Manager) DeviceIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Clone(m.devOrder)
}

// Encoder returns the hri.Device used to talk to a device.
func (m *Manager) Encoder(id string) (*Device, hri.Device, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[id]
	if !ok {
		return nil, hri.Device{}, fmt.Errorf("%w: device %q", ErrNotFound, id)
	}
	b, ok := m.buses[d.Config.Bus]
	if !ok {
		return nil, hri.Device{}, fmt.Errorf("%w: bus %q", ErrNotFound, d.Config.Bus)
	}
	return d, hri.Device{Client: b.Client, Slave: d.Config.Slave, Grouped: d.Config.GroupedReads}, nil
}

// State returns the runtime state of a device.
func (d *Device) State() DeviceState {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := DeviceState{LastError: d.lastErr, Last: d.last}
	if !d.lastOKAt.IsZero() {
		t := d.lastOKAt
		s.LastOKAt = &t
	}
	if !d.lastErrAt.IsZero() {
		t := d.lastErrAt
		s.LastErrAt = &t
	}
	s.Online = !d.lastOKAt.IsZero() && d.lastOKAt.After(d.lastErrAt)
	return s
}

// Track records the outcome of a request to the device.
func (d *Device) Track(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err == nil {
		d.lastOKAt = time.Now()
		return
	}
	d.lastErr = err.Error()
	d.lastErrAt = time.Now()
}

// Read returns a converted reading, served from cache when it is younger than
// maxAge (maxAge <= 0 always reads the encoder). at is the timestamp assigned
// to fresh readings (zero means now).
func (m *Manager) Read(ctx context.Context, id string, maxAge time.Duration, at time.Time) (*Reading, error) {
	dev, enc, err := m.Encoder(id)
	if err != nil {
		return nil, err
	}
	if !dev.Config.IsEnabled() {
		return nil, fmt.Errorf("%w: %s", ErrDisabled, id)
	}
	if maxAge > 0 {
		dev.mu.Lock()
		last := dev.last
		dev.mu.Unlock()
		if last != nil && time.Since(last.Time) < maxAge {
			return last, nil
		}
	}
	st, err := enc.ReadStatus(ctx)
	dev.Track(err)
	if err != nil {
		return nil, err
	}
	if at.IsZero() {
		at = time.Now()
	}
	r := Convert(dev.Config, dev.Loc, st, at)
	dev.mu.Lock()
	dev.last = &r
	dev.mu.Unlock()
	return &r, nil
}

// LiveRead reads a device honouring the configured live cache TTL.
func (m *Manager) LiveRead(ctx context.Context, id string) (*Reading, error) {
	return m.Read(ctx, id, m.modbusCfg.LiveCacheTTL.D(), time.Time{})
}

// invalidate drops the cached reading of a device after a write.
func (d *Device) invalidate() {
	d.mu.Lock()
	d.last = nil
	d.mu.Unlock()
}

// Invalidate drops the cached reading of a device.
func (m *Manager) Invalidate(id string) {
	if d, err := m.Device(id); err == nil {
		d.invalidate()
	}
}

// snapshot copies the inventory as config slices; callers hold m.mu.
func (m *Manager) snapshot() ([]config.BusConfig, []config.DeviceConfig) {
	buses := make([]config.BusConfig, 0, len(m.busOrder))
	for _, id := range m.busOrder {
		buses = append(buses, m.buses[id].Config)
	}
	devs := make([]config.DeviceConfig, 0, len(m.devOrder))
	for _, id := range m.devOrder {
		devs = append(devs, m.devices[id].Config)
	}
	return buses, devs
}

// Inventory returns copies of the bus and device configurations.
func (m *Manager) Inventory() ([]config.BusConfig, []config.DeviceConfig) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot()
}

// persist writes the inventory to the config file; callers hold m.mu.
func (m *Manager) persist() error {
	if m.configPath == "" {
		return nil
	}
	buses, devs := m.snapshot()
	if err := config.SaveInventory(m.configPath, buses, devs); err != nil {
		return fmt.Errorf("persist inventory to %s: %w", m.configPath, err)
	}
	return nil
}

// validateInventory runs config validation over a candidate inventory.
func validateInventory(buses []config.BusConfig, devs []config.DeviceConfig) error {
	c := config.Config{Buses: buses, Devices: devs}
	var errs []error
	busIDs := map[string]bool{}
	for _, b := range buses {
		if busIDs[b.ID] {
			errs = append(errs, fmt.Errorf("duplicate bus id %q", b.ID))
		}
		busIDs[b.ID] = true
	}
	devIDs := map[string]bool{}
	slavesOnBus := map[string]string{}
	for _, d := range c.Devices {
		if err := d.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("device %s: %w", d.ID, err))
		}
		if devIDs[d.ID] {
			errs = append(errs, fmt.Errorf("duplicate device id %q", d.ID))
		}
		devIDs[d.ID] = true
		if !busIDs[d.Bus] {
			errs = append(errs, fmt.Errorf("device %s: unknown bus %q", d.ID, d.Bus))
		}
		key := fmt.Sprintf("%s/%d", d.Bus, d.Slave)
		if other, ok := slavesOnBus[key]; ok {
			errs = append(errs, fmt.Errorf("device %s: slave %d on bus %s already used by %s", d.ID, d.Slave, d.Bus, other))
		}
		slavesOnBus[key] = d.ID
	}
	return errors.Join(errs...)
}

// PutBus creates or replaces a bus. Replacing reconnects the bus.
func (m *Manager) PutBus(b config.BusConfig, create bool) (*Bus, error) {
	b.Normalize()
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	old, exists := m.buses[b.ID]
	if create && exists {
		return nil, fmt.Errorf("%w: bus %q already exists", ErrConflict, b.ID)
	}
	if !create && !exists {
		return nil, fmt.Errorf("%w: bus %q", ErrNotFound, b.ID)
	}
	bus, err := m.newBus(b)
	if err != nil {
		return nil, err
	}
	m.buses[b.ID] = bus
	if exists {
		_ = old.Client.Close()
	} else {
		m.busOrder = append(m.busOrder, b.ID)
	}
	return bus, m.persist()
}

// DeleteBus removes a bus that no device references.
func (m *Manager) DeleteBus(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.buses[id]
	if !ok {
		return fmt.Errorf("%w: bus %q", ErrNotFound, id)
	}
	for _, d := range m.devices {
		if d.Config.Bus == id {
			return fmt.Errorf("%w: bus %q is used by device %q", ErrConflict, id, d.Config.ID)
		}
	}
	_ = b.Client.Close()
	delete(m.buses, id)
	m.busOrder = slices.DeleteFunc(m.busOrder, func(s string) bool { return s == id })
	return m.persist()
}

// PutDevice creates or replaces a device configuration.
func (m *Manager) PutDevice(d config.DeviceConfig, create bool) (*Device, error) {
	d.Normalize()
	m.mu.Lock()
	defer m.mu.Unlock()
	old, exists := m.devices[d.ID]
	if create && exists {
		return nil, fmt.Errorf("%w: device %q already exists", ErrConflict, d.ID)
	}
	if !create && !exists {
		return nil, fmt.Errorf("%w: device %q", ErrNotFound, d.ID)
	}
	buses, devs := m.snapshot()
	if exists {
		for i := range devs {
			if devs[i].ID == d.ID {
				devs[i] = d
			}
		}
	} else {
		devs = append(devs, d)
	}
	if err := validateInventory(buses, devs); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	dev, err := newDevice(d)
	if err != nil {
		return nil, err
	}
	if exists {
		// Keep runtime health but drop the cached reading: conversion may change.
		old.mu.Lock()
		dev.lastOKAt, dev.lastErr, dev.lastErrAt = old.lastOKAt, old.lastErr, old.lastErrAt
		old.mu.Unlock()
	} else {
		m.devOrder = append(m.devOrder, d.ID)
	}
	m.devices[d.ID] = dev
	return dev, m.persist()
}

// DeleteDevice removes a device. Recorded measurements are kept.
func (m *Manager) DeleteDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.devices[id]; !ok {
		return fmt.Errorf("%w: device %q", ErrNotFound, id)
	}
	delete(m.devices, id)
	m.devOrder = slices.DeleteFunc(m.devOrder, func(s string) bool { return s == id })
	return m.persist()
}

// SetSlave updates a device's slave address after the encoder itself was
// re-addressed.
func (m *Manager) SetSlave(id string, slave uint8) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[id]
	if !ok {
		return fmt.Errorf("%w: device %q", ErrNotFound, id)
	}
	for _, o := range m.devices {
		if o != d && o.Config.Bus == d.Config.Bus && o.Config.Slave == slave {
			return fmt.Errorf("%w: slave %d already used by %q", ErrConflict, slave, o.Config.ID)
		}
	}
	// Device configs are treated as immutable; swap in an updated copy.
	nd := &Device{Config: d.Config, Loc: d.Loc}
	nd.Config.Slave = slave
	d.mu.Lock()
	nd.lastOKAt, nd.lastErr, nd.lastErrAt = d.lastOKAt, d.lastErr, d.lastErrAt
	d.mu.Unlock()
	m.devices[id] = nd
	return m.persist()
}

// DeviceOnBus returns the device configured at slave on bus, if any.
func (m *Manager) DeviceOnBus(bus string, slave uint8) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, d := range m.devices {
		if d.Config.Bus == bus && d.Config.Slave == slave {
			return d.Config.ID, true
		}
	}
	return "", false
}
