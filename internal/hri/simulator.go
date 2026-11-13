package hri

import (
	"context"
	"encoding/binary"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

// Simulator emulates a bus of NWM-HRI encoders at the RTU frame level. It is
// meant for development, demos and tests without hardware.
type Simulator struct {
	mu      sync.Mutex
	devices map[uint8]*simDevice
}

type simDevice struct {
	forwardLiters float64
	reverseLiters float64
	clockOffset   time.Duration
	updated       time.Time
	deviceType    uint16
	threshold     uint16
	base, output  uint16
	fwdBase       uint32
	revBase       uint32
	rate          float64
}

// NewSimulator creates a simulator with encoders at the given addresses,
// each drawing around flowLPM liters per minute.
func NewSimulator(slaves []uint8, flowLPM float64) *Simulator {
	s := &Simulator{devices: map[uint8]*simDevice{}}
	for _, a := range slaves {
		s.devices[a] = &simDevice{
			forwardLiters: 1_000_000 + rand.Float64()*500_000,
			reverseLiters: rand.Float64() * 200,
			updated:       time.Now(),
			deviceType:    1,
			threshold:     5,
			base:          10,
			output:        10,
			rate:          flowLPM * (0.5 + rand.Float64()),
		}
	}
	return s
}

// Dialer returns a modbus.Dialer connected to this simulator.
func (s *Simulator) Dialer() modbus.Dialer {
	return func(context.Context) (modbus.Conn, error) { return &simConn{sim: s}, nil }
}

func (d *simDevice) advance(now time.Time) {
	dt := now.Sub(d.updated).Minutes()
	if dt <= 0 {
		return
	}
	// Daily usage pattern with noise; occasional small backflow.
	h := float64(now.Hour()) + float64(now.Minute())/60
	pattern := 0.35 + 0.65*max(0, 1-((h-8)*(h-8))/40) + 0.4*max(0, 1-((h-19)*(h-19))/12)
	d.forwardLiters += d.rate * pattern * dt * (0.8 + 0.4*rand.Float64())
	if rand.Float64() < 0.02 {
		d.reverseLiters += rand.Float64() * float64(d.output)
	}
	d.updated = now
}

func (d *simDevice) words(now time.Time) []uint16 {
	d.advance(now)
	out := float64(max(d.output, 1))
	fwd := d.fwdBase + uint32(d.forwardLiters/out)
	rev := d.revBase + uint32(d.reverseLiters/out)
	clock, _ := EncodeClock(now.Add(d.clockOffset))
	r := make([]uint16, mainBlockLen)
	r[RegForwardFlow], r[RegForwardFlow+1] = simCounter(fwd)
	r[RegReverseFlow], r[RegReverseFlow+1] = simCounter(rev)
	r[RegTotalFlow], r[RegTotalFlow+1] = simNet(r[RegForwardFlow], r[RegForwardFlow+1], r[RegReverseFlow], r[RegReverseFlow+1])
	copy(r[RegClockYearMonth:], clock[:])
	r[RegDeviceType] = d.deviceType
	r[RegFirmwareYear] = 0x1412
	r[RegFirmwareMonthDay] = 0x0801
	r[RegReverseFlowThreshold] = d.threshold
	r[RegBasePulseEquivalent] = d.base
	r[RegOutputPulseEquivalent] = d.output
	r[RegForwardBaseReading], r[RegForwardBaseReading+1] = simCounter(fwd)
	r[RegReverseBaseReading], r[RegReverseBaseReading+1] = simCounter(rev)
	return r
}

// simCounter encodes v like the firmware, rolling over past MaxCounter.
func simCounter(v uint32) (hi, lo uint16) {
	hi, lo, _ = SplitCounter(v % (MaxCounter + 1))
	return hi, lo
}

// simNet reproduces the firmware's net counter: forward minus reverse word by
// word, borrowing CounterLowModulus from the high word. A negative net wraps
// the high word.
func simNet(fh, fl, rh, rl uint16) (hi, lo uint16) {
	l := int(fl) - int(rl)
	hi = fh - rh
	if l < 0 {
		l += CounterLowModulus
		hi--
	}
	return hi, uint16(l)
}

// handle processes one request frame and returns the response frame, or nil
// when no response is due.
func (s *Simulator) handle(req []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(req) < 8 || modbus.CRC16(req[:len(req)-2]) != binary.LittleEndian.Uint16(req[len(req)-2:]) {
		return nil
	}
	slave, fn := req[0], req[1]
	targets := map[uint8]*simDevice{}
	if slave == modbus.BroadcastAddress {
		targets = s.devices
	} else if d, ok := s.devices[slave]; ok {
		targets[slave] = d
	} else {
		return nil
	}
	addr := binary.BigEndian.Uint16(req[2:])
	now := time.Now()
	exc := func(code byte) []byte { return crc([]byte{slave, fn | 0x80, code}) }

	switch fn {
	case modbus.FuncReadHoldingRegisters:
		if slave == modbus.BroadcastAddress {
			return nil
		}
		qty := binary.BigEndian.Uint16(req[4:])
		if qty < 1 || qty > modbus.MaxReadQuantity {
			return exc(0x03)
		}
		words := targets[slave].words(now)
		words[RegSlaveAddress] = uint16(slave)
		resp := []byte{slave, fn, byte(qty * 2)}
		for a := addr; a < addr+qty; a++ {
			switch {
			case a < mainBlockLen:
				resp = binary.BigEndian.AppendUint16(resp, words[a])
			case a >= RegDebugNumbers && a < RegDebugNumbers+debugBlockLen:
				resp = binary.BigEndian.AppendUint16(resp, 0)
			default:
				return exc(0x02)
			}
		}
		return crc(resp)

	case modbus.FuncWriteSingleRegister, modbus.FuncWriteMultipleRegisters:
		var values []uint16
		if fn == modbus.FuncWriteSingleRegister {
			values = []uint16{binary.BigEndian.Uint16(req[4:])}
		} else {
			qty := int(binary.BigEndian.Uint16(req[4:]))
			if len(req) < 9+qty*2 || int(req[6]) != qty*2 {
				return exc(0x03)
			}
			for i := range qty {
				values = append(values, binary.BigEndian.Uint16(req[7+i*2:]))
			}
		}
		for a, d := range targets {
			if code := s.write(a, d, addr, values, now); code != 0 {
				if slave == modbus.BroadcastAddress {
					return nil
				}
				return exc(code)
			}
		}
		if slave == modbus.BroadcastAddress {
			return nil
		}
		// Both write functions echo address and value/quantity.
		return crc(append([]byte(nil), req[:6]...))
	default:
		return exc(0x01)
	}
}

func (s *Simulator) write(slave uint8, d *simDevice, addr uint16, v []uint16, now time.Time) byte {
	d.advance(now)
	end := addr + uint16(len(v))
	at := func(a uint16) uint16 { return v[a-addr] }
	covers := func(a uint16) bool { return a >= addr && a < end }
	for a := addr; a < end; a++ {
		switch a {
		case RegSlaveAddress, RegClockYearMonth, RegClockDayHour, RegClockMinuteSecond, RegDeviceType,
			RegReverseFlowThreshold, RegBasePulseEquivalent, RegOutputPulseEquivalent,
			RegForwardBaseReading, RegForwardBaseReading + 1, RegReverseBaseReading, RegReverseBaseReading + 1:
		default:
			return 0x02
		}
	}
	if covers(RegBasePulseEquivalent) || covers(RegOutputPulseEquivalent) {
		base, out := d.base, d.output
		if covers(RegBasePulseEquivalent) {
			base = at(RegBasePulseEquivalent)
		}
		if covers(RegOutputPulseEquivalent) {
			out = at(RegOutputPulseEquivalent)
		}
		if ValidatePulseEquivalents(int(base), int(out)) != nil {
			return 0x03
		}
		d.base, d.output = base, out
	}
	if covers(RegClockYearMonth) && covers(RegClockMinuteSecond) {
		c := DecodeClock(at(RegClockYearMonth), at(RegClockDayHour), at(RegClockMinuteSecond))
		t, ok := c.Time(now.Location())
		if !ok {
			return 0x03
		}
		d.clockOffset = t.Sub(now)
	}
	if covers(RegDeviceType) {
		d.deviceType = at(RegDeviceType)
	}
	if covers(RegReverseFlowThreshold) {
		d.threshold = at(RegReverseFlowThreshold)
	}
	if covers(RegForwardBaseReading) && covers(RegForwardBaseReading+1) {
		d.fwdBase = JoinCounter(at(RegForwardBaseReading), at(RegForwardBaseReading+1))
		d.forwardLiters = 0
	}
	if covers(RegReverseBaseReading) && covers(RegReverseBaseReading+1) {
		d.revBase = JoinCounter(at(RegReverseBaseReading), at(RegReverseBaseReading+1))
		d.reverseLiters = 0
	}
	if covers(RegSlaveAddress) {
		n := at(RegSlaveAddress)
		if n < modbus.MinSlaveAddress || n > modbus.MaxSlaveAddress {
			return 0x03
		}
		if uint8(n) != slave {
			if _, taken := s.devices[uint8(n)]; taken {
				return 0x04
			}
			delete(s.devices, slave)
			s.devices[uint8(n)] = d
		}
	}
	return 0
}

func crc(b []byte) []byte {
	c := modbus.CRC16(b)
	return append(b, byte(c), byte(c>>8))
}

type simConn struct {
	sim *Simulator
	mu  sync.Mutex
	buf []byte
}

func (c *simConn) Write(p []byte) (int, error) {
	resp := c.sim.handle(p)
	c.mu.Lock()
	c.buf = append(c.buf, resp...)
	c.mu.Unlock()
	return len(p), nil
}

func (c *simConn) ReadFull(p []byte, deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.buf) < len(p) {
		c.mu.Unlock()
		time.Sleep(time.Until(deadline))
		c.mu.Lock()
		return modbus.ErrTimeout
	}
	copy(p, c.buf)
	c.buf = c.buf[len(p):]
	return nil
}

func (c *simConn) Discard() error {
	c.mu.Lock()
	c.buf = nil
	c.mu.Unlock()
	return nil
}

func (c *simConn) Close() error { return nil }
