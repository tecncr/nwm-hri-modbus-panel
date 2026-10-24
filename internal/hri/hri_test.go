package hri

import (
	"context"
	"testing"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

func TestDecodeClock(t *testing.T) {
	c := DecodeClock(0x1105, 0x0B08, 0x0E32)
	want := Clock{Year: 2017, Month: 5, Day: 11, Hour: 8, Minute: 14, Second: 50, Valid: true}
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
	regs, err := EncodeClock(time.Date(2017, 5, 11, 8, 14, 50, 0, time.UTC))
	if err != nil || regs != [3]uint16{0x1105, 0x0B08, 0x0E32} {
		t.Fatalf("EncodeClock = %04X, %v", regs, err)
	}
	if DecodeClock(0, 0, 0).Valid {
		t.Error("zero clock should be invalid")
	}
	if _, ok := DecodeClock(0x1102, 0x1F00, 0).Time(time.UTC); ok {
		t.Error("Feb 31st should be rejected")
	}
}

func TestDecodeFirmware(t *testing.T) {
	if f := DecodeFirmware(0x1412, 0x0801); f.Text != "2018-08-01" {
		t.Fatalf("got %+v", f)
	}
}

func TestValidatePulseEquivalents(t *testing.T) {
	cases := []struct {
		base, out int
		ok        bool
	}{{1, 1, true}, {10, 100, true}, {0, 1, false}, {10, 5, false}, {10001, 1000, false}, {1, 1001, false}}
	for _, c := range cases {
		if err := ValidatePulseEquivalents(c.base, c.out); (err == nil) != c.ok {
			t.Errorf("(%d,%d): err=%v", c.base, c.out, err)
		}
	}
}

func newSimDevice(t *testing.T, grouped bool) (Device, *Simulator) {
	t.Helper()
	sim := NewSimulator([]uint8{23}, 10)
	c := modbus.NewClient(sim.Dialer(), modbus.Options{Timeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = c.Close() })
	return Device{Client: c, Slave: 23, Grouped: grouped}, sim
}

func TestSimulatorRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, grouped := range []bool{false, true} {
		d, _ := newSimDevice(t, grouped)
		s, err := d.ReadSnapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if s.SlaveAddress != 23 || s.Firmware.Text != "2018-08-01" || !s.Clock.Valid {
			t.Fatalf("unexpected snapshot %+v", s)
		}
		if s.Counters.Total != s.Counters.Forward-s.Counters.Reverse {
			t.Fatalf("total %d != forward %d - reverse %d", s.Counters.Total, s.Counters.Forward, s.Counters.Reverse)
		}
		if len(s.DebugNumbers) != 6 {
			t.Fatalf("debug numbers: %v (%s)", s.DebugNumbers, s.DebugError)
		}
		st, err := d.ReadStatus(ctx)
		if err != nil || st.OutputPulseLiters != 10 {
			t.Fatalf("status %+v, %v", st, err)
		}
	}

	d, _ := newSimDevice(t, false)
	if err := d.SetPulseEquivalents(ctx, PulseEquivalents{BaseLiters: 1, OutputLiters: 1}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetBaseReadings(ctx, BaseReadings{Forward: 0x87766554, Reverse: 0x43322110}); err != nil {
		t.Fatal(err)
	}
	c, err := d.ReadCounters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.Forward != 0x87766554 || c.Reverse != 0x43322110 || c.Total != 0x44444444 {
		t.Fatalf("counters %+v", c)
	}
	when := time.Date(2030, 1, 2, 3, 4, 5, 0, time.Local)
	if err := d.SetClock(ctx, when); err != nil {
		t.Fatal(err)
	}
	clk, err := d.ReadClock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := clk.Time(time.Local); got.Sub(when).Abs() > 2*time.Second {
		t.Fatalf("clock %v, want %v", got, when)
	}
	// Invalid equivalents are rejected client-side; raw writes get an exception.
	if err := d.WriteRegisters(ctx, RegBasePulseEquivalent, []uint16{100, 10}); err == nil {
		t.Fatal("expected exception")
	} else if _, ok := err.(*modbus.ExceptionError); !ok {
		t.Fatalf("expected ExceptionError, got %T %v", err, err)
	}
	if err := d.SetSlaveAddress(ctx, 42); err != nil {
		t.Fatal(err)
	}
	d.Slave = 42
	if a, err := d.ReadSlaveAddress(ctx); err != nil || a != 42 {
		t.Fatalf("address %d, %v", a, err)
	}
	if err := BroadcastSlaveAddress(ctx, d.Client, 7); err != nil {
		t.Fatal(err)
	}
	d.Slave = 7
	if _, err := d.ReadCounters(ctx); err != nil {
		t.Fatal(err)
	}
	d.Slave = 42
	if _, err := d.ReadCounters(ctx); err != modbus.ErrTimeout {
		t.Fatalf("expected timeout on old address, got %v", err)
	}
}
