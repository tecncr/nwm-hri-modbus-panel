package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
)

// backends returns the stores under test. TimescaleDB is included when
// NWM_PANEL_TEST_TIMESCALEDB_DSN points at a disposable database.
func backends(t *testing.T) map[string]*Store {
	t.Helper()
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	out := map[string]*Store{}
	sq, err := Open(ctx, config.StorageConfig{Driver: config.DriverSQLite, SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "t.db")}}, log)
	if err != nil {
		t.Fatal(err)
	}
	out["sqlite"] = sq
	if dsn := os.Getenv("NWM_PANEL_TEST_TIMESCALEDB_DSN"); dsn != "" {
		ts, err := Open(ctx, config.StorageConfig{Driver: config.DriverTimescaleDB, TimescaleDB: config.TimescaleDBConfig{
			DSN: dsn, ChunkInterval: config.Duration(7 * 24 * time.Hour), MaxOpenConns: 4,
		}}, log)
		if err != nil {
			t.Fatal(err)
		}
		if err := ts.db.Exec("TRUNCATE measurements").Error; err != nil {
			t.Fatal(err)
		}
		out["timescaledb"] = ts
	}
	for _, s := range out {
		t.Cleanup(func() { _ = s.Close() })
	}
	return out
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	var rows []Measurement
	for i := range 120 { // two hours, one per minute, two devices
		for _, dev := range []string{"a", "b"} {
			clock := base.Add(time.Duration(i) * time.Minute).Add(3 * time.Second)
			rows = append(rows, Measurement{
				Time: base.Add(time.Duration(i)*time.Minute + 400*time.Millisecond), DeviceID: dev, Bus: "bus", Slave: 1,
				ForwardRaw: int64(i * 10), ForwardLiters: float64(i * 100), TotalLiters: float64(i * 100),
				LitersPerUnit: 10, DeviceClock: &clock,
			})
		}
	}
	for name, s := range backends(t) {
		t.Run(name, func(t *testing.T) {
			if err := s.Insert(ctx, rows); err != nil {
				t.Fatal(err)
			}
			// Duplicates are ignored.
			if err := s.Insert(ctx, rows[:4]); err != nil {
				t.Fatal(err)
			}
			f := Filter{Devices: []string{"a"}, From: base, To: base.Add(time.Hour)}
			page, total, err := s.Query(ctx, f, 10, 5, Asc)
			if err != nil {
				t.Fatal(err)
			}
			if total != 60 || len(page) != 5 || !page[0].Time.Equal(base.Add(10*time.Minute)) {
				t.Fatalf("query: total=%d len=%d first=%v", total, len(page), page[0].Time)
			}
			if page[0].DeviceClock == nil || !page[0].DeviceClock.Equal(base.Add(10*time.Minute+3*time.Second)) {
				t.Fatalf("device clock round trip: %v", page[0].DeviceClock)
			}
			n, err := s.Stream(ctx, Filter{From: base, To: base.Add(2 * time.Hour)}, Desc, 7, func(*Measurement) error { return nil })
			if err != nil || n != 7 {
				t.Fatalf("stream: %d %v", n, err)
			}
			latest, err := s.Latest(ctx, nil)
			if err != nil || len(latest) != 2 || latest[0].ForwardLiters != 11900 {
				t.Fatalf("latest: %+v %v", latest, err)
			}
			before, err := s.Before(ctx, "b", base.Add(30*time.Minute))
			if err != nil || before == nil || !before.Time.Equal(base.Add(29*time.Minute)) {
				t.Fatalf("before: %+v %v", before, err)
			}
			lima, _ := time.LoadLocation("America/Lima")
			buckets, err := s.Aggregate(ctx, Filter{Devices: []string{"a"}, From: base, To: base.Add(2 * time.Hour)}, time.Hour, lima)
			if err != nil {
				t.Fatal(err)
			}
			if len(buckets) != 2 || buckets[0].Samples != 60 || buckets[1].MaxForward != 11900 || buckets[0].Start.Hour() != 5 {
				t.Fatalf("aggregate: %+v", buckets)
			}
			if !buckets[0].LastTime.Equal(base.Add(59 * time.Minute)) {
				t.Fatalf("aggregate last time: %v", buckets[0].LastTime)
			}
			st, err := s.Stats(ctx)
			if err != nil || st.Oldest == nil || !st.Oldest.Equal(base) {
				t.Fatalf("stats: %+v %v", st, err)
			}
			if name == "sqlite" && st.Rows != 240 {
				t.Fatalf("stats rows: %d", st.Rows)
			}
			pruned, err := s.Prune(ctx, base.Add(time.Hour))
			if err != nil || pruned != 120 {
				t.Fatalf("prune: %d %v", pruned, err)
			}
		})
	}
}
