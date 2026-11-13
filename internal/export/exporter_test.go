package export

import (
	"testing"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/schedule"
)

func TestTicks(t *testing.T) {
	loc := time.FixedZone("PET", -5*3600)
	e := &Exporter{
		cfg:      config.ExportConfig{Window: config.Duration(24 * time.Hour)},
		schedule: schedule.Aligned{Interval: 24 * time.Hour, Loc: loc},
	}
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, loc) }

	// Daily files at midnight cover the previous day: Nov 1-3 needs the files
	// ticking on Nov 2, 3 and 4.
	ticks, err := e.Ticks(day(1), day(4), day(10))
	if err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 3 || !ticks[0].Equal(day(2)) || !ticks[2].Equal(day(4)) {
		t.Fatalf("ticks = %v", ticks)
	}
	// A partial day still regenerates the file containing it.
	if ticks, _ := e.Ticks(day(1).Add(time.Hour), day(1).Add(2*time.Hour), day(10)); len(ticks) != 1 || !ticks[0].Equal(day(2)) {
		t.Fatalf("partial ticks = %v", ticks)
	}
	// Windows that have not finished yet are skipped.
	if ticks, _ := e.Ticks(day(1), day(9), day(3).Add(time.Hour)); len(ticks) != 2 {
		t.Fatalf("future ticks = %v", ticks)
	}
	if _, err := e.Ticks(day(2), day(1), day(10)); err == nil {
		t.Fatal("reversed range should fail")
	}
	e.schedule = schedule.Aligned{Interval: time.Minute, Loc: loc}
	if _, err := e.Ticks(day(1), day(9), day(10)); err == nil {
		t.Fatal("too many windows should fail")
	}
}
