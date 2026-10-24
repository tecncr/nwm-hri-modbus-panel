package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	src := "# keep me\nserver:\n  listen: 0.0.0.0:9000\nbuses:\n  - id: sim\n    transport: simulator\n    simulator: {slaves: [1, 2]}\n    timeout: 500ms\ndevices:\n  - id: a\n    bus: sim\n    slave: 1\n"
	if err := os.WriteFile(path, []byte(src), 0o640); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NWM_PANEL_RECORDER_INTERVAL", "5m")
	cfg, used, err := Load(NewViper(), path)
	if err != nil {
		t.Fatal(err)
	}
	if used != path || cfg.Server.Listen != "0.0.0.0:9000" || cfg.Recorder.Interval.D() != 5*time.Minute {
		t.Fatalf("unexpected config: %+v", cfg.Server)
	}
	if cfg.Buses[0].Timeout.D() != 500*time.Millisecond || cfg.Export.Window.D() != time.Hour {
		t.Fatalf("durations: %v %v", cfg.Buses[0].Timeout, cfg.Export.Window)
	}
	cfg.Devices = append(cfg.Devices, DeviceConfig{ID: "b", Bus: "sim", Slave: 2, Meter: MeterConfig{LitersPerUnit: 2.5}})
	if err := SaveInventory(path, cfg.Buses, cfg.Devices); err != nil {
		t.Fatal(err)
	}
	cfg2, _, err := Load(NewViper(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg2.Devices) != 2 || cfg2.Devices[1].Meter.LitersPerUnit != 2.5 || cfg2.Buses[0].Timeout.D() != 500*time.Millisecond {
		t.Fatalf("round trip failed: %+v", cfg2.Devices)
	}
	raw, _ := os.ReadFile(path)
	if string(raw[:9]) != "# keep me" {
		t.Fatalf("comment lost:\n%s", raw)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o640 {
		t.Fatalf("mode changed to %v", st.Mode())
	}
}

func TestValidateRejects(t *testing.T) {
	cfg, _, err := Load(NewViper(), "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Buses = []BusConfig{{ID: "Bad ID", Transport: "serial"}}
	cfg.Devices = []DeviceConfig{{ID: "x", Bus: "nope", Slave: 0}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation errors")
	}
}
