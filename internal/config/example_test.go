package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleIsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.yaml")
	if err := os.WriteFile(path, []byte(Example), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(NewViper(), path)
	if err != nil {
		t.Fatalf("example config does not load: %v", err)
	}
	if len(cfg.Buses) != 1 || len(cfg.Devices) != 1 {
		t.Fatalf("unexpected inventory: %d buses, %d devices", len(cfg.Buses), len(cfg.Devices))
	}
}
