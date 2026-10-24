package schedule

import (
	"testing"
	"time"
)

func TestAligned(t *testing.T) {
	lima, _ := time.LoadLocation("America/Lima")
	cases := []struct {
		interval time.Duration
		at, want time.Time
	}{
		{time.Minute, time.Date(2026, 1, 1, 10, 0, 30, 0, lima), time.Date(2026, 1, 1, 10, 1, 0, 0, lima)},
		{time.Minute, time.Date(2026, 1, 1, 10, 1, 0, 0, lima), time.Date(2026, 1, 1, 10, 2, 0, 0, lima)},
		{time.Hour, time.Date(2026, 1, 1, 10, 59, 59, 0, lima), time.Date(2026, 1, 1, 11, 0, 0, 0, lima)},
		{24 * time.Hour, time.Date(2026, 1, 1, 23, 0, 0, 0, lima), time.Date(2026, 1, 2, 0, 0, 0, 0, lima)},
		{15 * time.Minute, time.Date(2026, 1, 1, 10, 7, 0, 0, lima), time.Date(2026, 1, 1, 10, 15, 0, 0, lima)},
	}
	for _, c := range cases {
		got := Aligned{Interval: c.interval, Loc: lima}.Next(c.at)
		if !got.Equal(c.want) {
			t.Errorf("%v after %v: got %v want %v", c.interval, c.at, got, c.want)
		}
	}
}

func TestCron(t *testing.T) {
	s, err := Parse(0, "30 */5 * * * *", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Next(time.Date(2026, 1, 1, 10, 2, 0, 0, time.UTC))
	if want := time.Date(2026, 1, 1, 10, 5, 30, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if _, err := Parse(0, "*/15 * * * *", time.UTC); err != nil {
		t.Fatalf("5-field cron should parse: %v", err)
	}
	if _, err := Parse(0, "nope", time.UTC); err == nil {
		t.Fatal("expected error")
	}
}

func TestShortDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{time.Minute: "1m", time.Hour: "1h", 90 * time.Second: "1m30s", 5 * time.Second: "5s", 25 * time.Hour: "25h"} {
		if got := ShortDuration(d); got != want {
			t.Errorf("%v: got %q want %q", d, got, want)
		}
	}
}
