// Package schedule runs jobs on wall-clock aligned intervals or cron
// expressions.
package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Schedule yields activation times.
type Schedule interface {
	// Next returns the first activation strictly after t.
	Next(t time.Time) time.Time
	String() string
}

// Aligned fires at every multiple of an interval, counted from midnight
// 1970-01-01 in loc: a 1m interval fires at every :00 second, 1h at every
// hh:00:00, 24h at local midnight.
type Aligned struct {
	Interval time.Duration
	Loc      *time.Location
}

// Next implements Schedule.
func (a Aligned) Next(t time.Time) time.Time {
	secs := int64(a.Interval / time.Second)
	if secs < 1 {
		secs = 1
	}
	_, off := t.In(a.Loc).Zone()
	local := t.Unix() + int64(off)
	next := (local/secs + 1) * secs
	return time.Unix(next-int64(off), 0).In(a.Loc)
}

func (a Aligned) String() string { return "every " + ShortDuration(a.Interval) + " (aligned)" }

// ShortDuration formats d without zero-valued trailing units ("1h", "1m30s").
func ShortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

type cronSchedule struct {
	expr string
	s    cron.Schedule
	loc  *time.Location
}

func (c cronSchedule) Next(t time.Time) time.Time { return c.s.Next(t.In(c.loc)) }
func (c cronSchedule) String() string             { return "cron " + c.expr }

var parser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Parse returns a cron schedule when expr is non-empty, otherwise an aligned
// interval schedule.
func Parse(interval time.Duration, expr string, loc *time.Location) (Schedule, error) {
	if loc == nil {
		loc = time.Local
	}
	if expr != "" {
		s, err := parser.Parse(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid cron expression %q: %w", expr, err)
		}
		return cronSchedule{expr: expr, s: s, loc: loc}, nil
	}
	if interval < time.Second {
		return nil, fmt.Errorf("interval must be at least 1s")
	}
	return Aligned{Interval: interval, Loc: loc}, nil
}

// Run calls fn at every activation until ctx is cancelled. Activations are
// never run concurrently; ticks missed while fn is running are skipped. The
// tick passed to fn is the scheduled time, not the wake-up time.
func Run(ctx context.Context, s Schedule, log *slog.Logger, onNext func(time.Time), fn func(ctx context.Context, tick time.Time)) {
	for {
		next := s.Next(time.Now())
		if onNext != nil {
			onNext(next)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error("scheduled job panicked", "panic", r)
				}
			}()
			fn(ctx, next)
		}()
	}
}
