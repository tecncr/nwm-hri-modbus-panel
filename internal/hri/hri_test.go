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
	if err := d.SetBaseReadings(ctx, BaseReadings{Forward: 183540, Reverse: 3323}); err != nil {
		t.Fatal(err)
	}
	c, err := d.ReadCounters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetBaseReadings(ctx, BaseReadings{Forward: MaxCounter + 1}); err == nil {
		t.Fatal("base reading above MaxCounter should be rejected")
	}
	if c.Forward != 183540 || c.Reverse != 3323 || c.Total != 180217 {
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

// TestCounterEncoding checks words read from an encoder around a carry: the
// low word rolls over at 10000, not 65536.
func TestCounterEncoding(t *testing.T) {
	before := JoinCounter(0x000B, 0x270F) // 11 × 10000 + 9999
	after := JoinCounter(0x000C, 0x0002)
	if before != 119999 || after != 120002 {
		t.Fatalf("JoinCounter: before %d, after %d", before, after)
	}
	for _, v := range []uint32{0, 9999, 10000, 119999, MaxCounter} {
		hi, lo, ok := SplitCounter(v)
		if !ok || lo >= CounterLowModulus || JoinCounter(hi, lo) != v {
			t.Errorf("SplitCounter(%d) = %d, %d, %v", v, hi, lo, ok)
		}
	}
	if _, _, ok := SplitCounter(MaxCounter + 1); ok {
		t.Error("SplitCounter should reject values above MaxCounter")
	}
	// Net counter read from the same encoder: forward 0x000C0002 minus
	// reverse 0x00000CFB reads 0x000B1A17.
	if hi, lo := simNet(0x000C, 0x0002, 0x0000, 0x0CFB); hi != 0x000B || lo != 0x1A17 {
		t.Fatalf("simNet = %04X %04X", hi, lo)
	}
	if JoinCounter(0x000B, 0x1A17) != after-3323 {
		t.Fatal("net counter should equal forward minus reverse")
	}
}
