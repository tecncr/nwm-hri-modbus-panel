package hri

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

// Counters are the raw 32-bit flow counters, in output pulse units.
type Counters struct {
	Total   uint32 `json:"total"`
	Forward uint32 `json:"forward"`
	Reverse uint32 `json:"reverse"`
}

// Status is the subset of registers needed to produce a measurement.
type Status struct {
	Counters
	Clock             Clock  `json:"clock"`
	BatteryStatus     uint16 `json:"battery_status"`
	Failure           uint16 `json:"failure"`
	BasePulseLiters   uint16 `json:"base_pulse_liters"`
	OutputPulseLiters uint16 `json:"output_pulse_liters"`
}

// RegisterValue is one raw word read from the encoder.
type RegisterValue struct {
	Address uint16 `json:"address"`
	Key     string `json:"key"`
	Value   uint16 `json:"value"`
	Hex     string `json:"hex"`
}

// Snapshot is a decoded read of every documented register.
type Snapshot struct {
	SlaveAddress         uint16          `json:"slave_address"`
	Counters             Counters        `json:"counters"`
	Clock                Clock           `json:"clock"`
	DeviceType           uint16          `json:"device_type"`
	Firmware             Firmware        `json:"firmware"`
	ReverseFlowThreshold uint16          `json:"reverse_flow_threshold"`
	BatteryStatus        uint16          `json:"battery_status"`
	Failure              uint16          `json:"failure"`
	BasePulseLiters      uint16          `json:"base_pulse_liters"`
	OutputPulseLiters    uint16          `json:"output_pulse_liters"`
	ForwardBaseReading   uint32          `json:"forward_base_reading"`
	ReverseBaseReading   uint32          `json:"reverse_base_reading"`
	DebugNumbers         []int           `json:"debug_numbers"`
	DebugError           string          `json:"debug_error,omitempty"`
	Raw                  []RegisterValue `json:"raw"`
	ReadAt               time.Time       `json:"read_at"`
}

// Device addresses a single encoder on a bus.
type Device struct {
	Client *modbus.Client
	Slave  uint8
	// Grouped forces reading registers in small documented groups instead of a
	// single block request, for firmware that rejects large block reads.
	Grouped bool
}

func (d Device) read(ctx context.Context, addr, qty uint16) ([]uint16, error) {
	return d.Client.ReadHoldingRegisters(ctx, d.Slave, addr, qty)
}

// ReadRegisters reads qty raw words starting at addr.
func (d Device) ReadRegisters(ctx context.Context, addr, qty uint16) ([]uint16, error) {
	return d.read(ctx, addr, qty)
}

// WriteRegister writes a single raw word.
func (d Device) WriteRegister(ctx context.Context, addr, value uint16) error {
	return d.Client.WriteSingleRegister(ctx, d.Slave, addr, value)
}

// WriteRegisters writes consecutive raw words.
func (d Device) WriteRegisters(ctx context.Context, addr uint16, values []uint16) error {
	return d.Client.WriteMultipleRegisters(ctx, d.Slave, addr, values)
}

// readMain returns registers 0x0000..0x0015, either in one request or in
// documented groups.
func (d Device) readMain(ctx context.Context) ([]uint16, error) {
	if !d.Grouped {
		regs, err := d.read(ctx, mainBlockStart, mainBlockLen)
		if err == nil {
			return regs, nil
		}
		if _, ok := errors.AsType[*modbus.ExceptionError](err); !ok {
			return nil, err
		}
		// The firmware rejected the block: fall back to groups.
	}
	groups := [][2]uint16{
		{RegSlaveAddress, 1},
		{RegTotalFlow, 6},
		{RegClockYearMonth, 3},
		{RegDeviceType, 1},
		{RegFirmwareYear, 2},
		{RegReverseFlowThreshold, 3},
		{RegBasePulseEquivalent, 2},
		{RegForwardBaseReading, 4},
	}
	out := make([]uint16, mainBlockLen)
	for _, g := range groups {
		regs, err := d.read(ctx, g[0], g[1])
		if err != nil {
			return nil, fmt.Errorf("read 0x%04X+%d: %w", g[0], g[1], err)
		}
		copy(out[g[0]:], regs)
	}
	return out, nil
}

// ReadSnapshot reads and decodes every documented register.
func (d Device) ReadSnapshot(ctx context.Context) (*Snapshot, error) {
	r, err := d.readMain(ctx)
	if err != nil {
		return nil, err
	}
	s := &Snapshot{
		SlaveAddress: r[RegSlaveAddress],
		Counters: Counters{
			Total:   U32(r[RegTotalFlow], r[RegTotalFlow+1]),
			Forward: U32(r[RegForwardFlow], r[RegForwardFlow+1]),
			Reverse: U32(r[RegReverseFlow], r[RegReverseFlow+1]),
		},
		Clock:                DecodeClock(r[RegClockYearMonth], r[RegClockDayHour], r[RegClockMinuteSecond]),
		DeviceType:           r[RegDeviceType],
		Firmware:             DecodeFirmware(r[RegFirmwareYear], r[RegFirmwareMonthDay]),
		ReverseFlowThreshold: r[RegReverseFlowThreshold],
		BatteryStatus:        r[RegBatteryStatus],
		Failure:              r[RegFailure],
		BasePulseLiters:      r[RegBasePulseEquivalent],
		OutputPulseLiters:    r[RegOutputPulseEquivalent],
		ForwardBaseReading:   U32(r[RegForwardBaseReading], r[RegForwardBaseReading+1]),
		ReverseBaseReading:   U32(r[RegReverseBaseReading], r[RegReverseBaseReading+1]),
		ReadAt:               time.Now(),
	}
	for i, v := range r {
		s.Raw = append(s.Raw, rawValue(uint16(i), v))
	}
	if dbg, err := d.read(ctx, RegDebugNumbers, debugBlockLen); err == nil {
		for i, v := range dbg {
			s.DebugNumbers = append(s.DebugNumbers, int(v>>8), int(v&0xFF))
			s.Raw = append(s.Raw, rawValue(RegDebugNumbers+uint16(i), v))
		}
	} else {
		s.DebugError = err.Error()
	}
	return s, nil
}

func rawValue(addr, v uint16) RegisterValue {
	rv := RegisterValue{Address: addr, Value: v, Hex: fmt.Sprintf("0x%04X", v)}
	if reg, ok := RegisterAt(addr); ok {
		rv.Key = reg.Key
	}
	return rv
}

// ReadStatus reads counters, clock, battery, failure and pulse equivalents
// (0x0001..0x0011) in a single request.
func (d Device) ReadStatus(ctx context.Context) (*Status, error) {
	var r []uint16
	var err error
	if d.Grouped {
		r = make([]uint16, 17)
		for _, g := range [][2]uint16{{RegTotalFlow, 6}, {RegClockYearMonth, 3}, {RegReverseFlowThreshold, 3}, {RegBasePulseEquivalent, 2}} {
			part, err := d.read(ctx, g[0], g[1])
			if err != nil {
				return nil, err
			}
			copy(r[g[0]-RegTotalFlow:], part)
		}
	} else if r, err = d.read(ctx, RegTotalFlow, 17); err != nil {
		return nil, err
	}
	at := func(addr uint16) uint16 { return r[addr-RegTotalFlow] }
	return &Status{
		Counters: Counters{
			Total:   U32(at(RegTotalFlow), at(RegTotalFlow+1)),
			Forward: U32(at(RegForwardFlow), at(RegForwardFlow+1)),
			Reverse: U32(at(RegReverseFlow), at(RegReverseFlow+1)),
		},
		Clock:             DecodeClock(at(RegClockYearMonth), at(RegClockDayHour), at(RegClockMinuteSecond)),
		BatteryStatus:     at(RegBatteryStatus),
		Failure:           at(RegFailure),
		BasePulseLiters:   at(RegBasePulseEquivalent),
		OutputPulseLiters: at(RegOutputPulseEquivalent),
	}, nil
}

// ReadCounters reads the three flow counters.
func (d Device) ReadCounters(ctx context.Context) (Counters, error) {
	r, err := d.read(ctx, RegTotalFlow, 6)
	if err != nil {
		return Counters{}, err
	}
	return Counters{Total: U32(r[0], r[1]), Forward: U32(r[2], r[3]), Reverse: U32(r[4], r[5])}, nil
}

// ReadSlaveAddress reads the configured slave address.
func (d Device) ReadSlaveAddress(ctx context.Context) (uint16, error) {
	r, err := d.read(ctx, RegSlaveAddress, 1)
	if err != nil {
		return 0, err
	}
	return r[0], nil
}

// SetSlaveAddress changes the encoder's slave address. Subsequent requests
// must use the new address.
func (d Device) SetSlaveAddress(ctx context.Context, addr uint8) error {
	if addr < modbus.MinSlaveAddress || addr > modbus.MaxSlaveAddress {
		return fmt.Errorf("%w: slave address must be 1-247", modbus.ErrInvalidRequest)
	}
	return d.WriteRegister(ctx, RegSlaveAddress, uint16(addr))
}

// ReadClock reads the real-time clock.
func (d Device) ReadClock(ctx context.Context) (Clock, error) {
	r, err := d.read(ctx, RegClockYearMonth, 3)
	if err != nil {
		return Clock{}, err
	}
	return DecodeClock(r[0], r[1], r[2]), nil
}

// SetClock writes the real-time clock. t must already be in the device's
// location.
func (d Device) SetClock(ctx context.Context, t time.Time) error {
	regs, err := EncodeClock(t)
	if err != nil {
		return fmt.Errorf("%w: %v", modbus.ErrInvalidRequest, err)
	}
	return d.WriteRegisters(ctx, RegClockYearMonth, regs[:])
}

// ReadDeviceType reads the device type ID.
func (d Device) ReadDeviceType(ctx context.Context) (uint16, error) {
	r, err := d.read(ctx, RegDeviceType, 1)
	if err != nil {
		return 0, err
	}
	return r[0], nil
}

// SetDeviceType writes the device type ID.
func (d Device) SetDeviceType(ctx context.Context, v uint16) error {
	return d.WriteRegister(ctx, RegDeviceType, v)
}

// ReadFirmware reads the software version date.
func (d Device) ReadFirmware(ctx context.Context) (Firmware, error) {
	r, err := d.read(ctx, RegFirmwareYear, 2)
	if err != nil {
		return Firmware{}, err
	}
	return DecodeFirmware(r[0], r[1]), nil
}

// ReadReverseFlowThreshold reads the reverse flow threshold.
func (d Device) ReadReverseFlowThreshold(ctx context.Context) (uint16, error) {
	r, err := d.read(ctx, RegReverseFlowThreshold, 1)
	if err != nil {
		return 0, err
	}
	return r[0], nil
}

// SetReverseFlowThreshold writes the reverse flow threshold.
func (d Device) SetReverseFlowThreshold(ctx context.Context, v uint16) error {
	return d.WriteRegister(ctx, RegReverseFlowThreshold, v)
}

// Diagnostics groups the encoder's health registers.
type Diagnostics struct {
	BatteryStatus uint16 `json:"battery_status"`
	Failure       uint16 `json:"failure"`
	DebugNumbers  []int  `json:"debug_numbers"`
}

// ReadDiagnostics reads battery status, failure and debug numbers.
func (d Device) ReadDiagnostics(ctx context.Context) (Diagnostics, error) {
	r, err := d.read(ctx, RegBatteryStatus, 2)
	if err != nil {
		return Diagnostics{}, err
	}
	dg := Diagnostics{BatteryStatus: r[0], Failure: r[1]}
	dbg, err := d.read(ctx, RegDebugNumbers, debugBlockLen)
	if err != nil {
		return dg, err
	}
	for _, v := range dbg {
		dg.DebugNumbers = append(dg.DebugNumbers, int(v>>8), int(v&0xFF))
	}
	return dg, nil
}

// PulseEquivalents are the meter and output resolutions in liters.
type PulseEquivalents struct {
	BaseLiters   uint16 `json:"base_liters_per_pulse"`
	OutputLiters uint16 `json:"output_liters_per_unit"`
}

// ReadPulseEquivalents reads the base and output pulse equivalents.
func (d Device) ReadPulseEquivalents(ctx context.Context) (PulseEquivalents, error) {
	r, err := d.read(ctx, RegBasePulseEquivalent, 2)
	if err != nil {
		return PulseEquivalents{}, err
	}
	return PulseEquivalents{BaseLiters: r[0], OutputLiters: r[1]}, nil
}

// SetPulseEquivalents writes both pulse equivalents in one request.
func (d Device) SetPulseEquivalents(ctx context.Context, p PulseEquivalents) error {
	if err := ValidatePulseEquivalents(int(p.BaseLiters), int(p.OutputLiters)); err != nil {
		return fmt.Errorf("%w: %v", modbus.ErrInvalidRequest, err)
	}
	return d.WriteRegisters(ctx, RegBasePulseEquivalent, []uint16{p.BaseLiters, p.OutputLiters})
}

// BaseReadings are the preset values of the forward and reverse counters.
type BaseReadings struct {
	Forward uint32 `json:"forward"`
	Reverse uint32 `json:"reverse"`
}

// ReadBaseReadings reads the forward and reverse base readings.
func (d Device) ReadBaseReadings(ctx context.Context) (BaseReadings, error) {
	r, err := d.read(ctx, RegForwardBaseReading, 4)
	if err != nil {
		return BaseReadings{}, err
	}
	return BaseReadings{Forward: U32(r[0], r[1]), Reverse: U32(r[2], r[3])}, nil
}

// SetBaseReadings presets the forward and reverse counters in one request.
func (d Device) SetBaseReadings(ctx context.Context, b BaseReadings) error {
	fh, fl := SplitU32(b.Forward)
	rh, rl := SplitU32(b.Reverse)
	return d.WriteRegisters(ctx, RegForwardBaseReading, []uint16{fh, fl, rh, rl})
}

// BroadcastSlaveAddress assigns addr to every encoder on the bus. Only use
// it with a single encoder connected.
func BroadcastSlaveAddress(ctx context.Context, c *modbus.Client, addr uint8) error {
	if addr < modbus.MinSlaveAddress || addr > modbus.MaxSlaveAddress {
		return fmt.Errorf("%w: slave address must be 1-247", modbus.ErrInvalidRequest)
	}
	return c.WriteSingleRegister(ctx, modbus.BroadcastAddress, RegSlaveAddress, uint16(addr))
}

// BroadcastClock sets the clock of every encoder on the bus.
func BroadcastClock(ctx context.Context, c *modbus.Client, t time.Time) error {
	regs, err := EncodeClock(t)
	if err != nil {
		return fmt.Errorf("%w: %v", modbus.ErrInvalidRequest, err)
	}
	return c.WriteMultipleRegisters(ctx, modbus.BroadcastAddress, RegClockYearMonth, regs[:])
}
