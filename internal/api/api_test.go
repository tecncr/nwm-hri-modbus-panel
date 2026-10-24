package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/export"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/recorder"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

const testKey = "test-key"

func newTestServer(t *testing.T, mutate func(*config.Config)) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg, _, err := config.Load(config.NewViper(), "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Storage.SQLite.Path = filepath.Join(dir, "t.db")
	cfg.Export.Directory = filepath.Join(dir, "exports")
	cfg.Server.AccessLog = false
	cfg.Buses = []config.BusConfig{{ID: "sim", Transport: config.TransportSimulator, Simulator: config.SimulatorConfig{Slaves: []int{1, 2}}}}
	cfg.Devices = []config.DeviceConfig{{ID: "a", Name: "A", Bus: "sim", Slave: 1}, {ID: "b", Bus: "sim", Slave: 2}}
	if mutate != nil {
		mutate(cfg)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(context.Background(), cfg.Storage, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	inv, err := inventory.New(cfg, "", log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inv.Close)
	rec, err := recorder.New(cfg.Recorder, cfg.Storage, inv, st, log)
	if err != nil {
		t.Fatal(err)
	}
	exp, err := export.New(cfg.Export, 1000, st, inv, log)
	if err != nil {
		t.Fatal(err)
	}
	return New(Deps{Config: cfg, APIKey: testKey, Inventory: inv, Store: st, Recorder: rec, Exporter: exp, Log: log})
}

func do(t *testing.T, s *Server, method, path, key, body string, hdr ...string) (*http.Response, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := s.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp, out
}

func TestAuth(t *testing.T) {
	s := newTestServer(t, nil)
	if r, _ := do(t, s, "GET", "/api/v1/health", "", ""); r.StatusCode != 200 {
		t.Fatalf("health: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "GET", "/api/v1/meters", "", ""); r.StatusCode != 401 {
		t.Fatalf("no key: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "GET", "/api/v1/meters", "", "", "X-API-Key", testKey); r.StatusCode != 200 {
		t.Fatalf("x-api-key: %d", r.StatusCode)
	}
	if r, body := do(t, s, "GET", "/api/v1/nope", testKey, ""); r.StatusCode != 404 || body["error"] == nil {
		t.Fatalf("unknown endpoint: %d %v", r.StatusCode, body)
	}
}

func TestAuthFailureLimit(t *testing.T) {
	s := newTestServer(t, func(c *config.Config) { c.Server.RateLimit.AuthFailuresPerMinute = 3 })
	for range 3 {
		do(t, s, "GET", "/api/v1/meters", "wrong", "")
	}
	if r, _ := do(t, s, "GET", "/api/v1/meters", testKey, ""); r.StatusCode != 429 {
		t.Fatalf("expected 429 after repeated failures, got %d", r.StatusCode)
	}
}

func TestSpoofedForwardedFor(t *testing.T) {
	// Without trusted proxies, a spoofed X-Forwarded-For must not dodge the
	// failure limiter.
	s := newTestServer(t, func(c *config.Config) { c.Server.RateLimit.AuthFailuresPerMinute = 2 })
	do(t, s, "GET", "/api/v1/meters", "wrong", "", "X-Forwarded-For", "10.0.0.1")
	do(t, s, "GET", "/api/v1/meters", "wrong", "", "X-Forwarded-For", "10.0.0.2")
	if r, _ := do(t, s, "GET", "/api/v1/meters", testKey, "", "X-Forwarded-For", "10.0.0.3"); r.StatusCode != 429 {
		t.Fatalf("spoofed header bypassed limiter: %d", r.StatusCode)
	}
}

func TestBasePathAndUI(t *testing.T) {
	s := newTestServer(t, func(c *config.Config) { c.Server.BasePath = "/nwm/" })
	if r, _ := do(t, s, "GET", "/nwm/api/v1/meters", testKey, ""); r.StatusCode != 200 {
		t.Fatalf("base path api: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "GET", "/api/v1/meters", testKey, ""); r.StatusCode != 404 {
		t.Fatalf("api outside base path: %d", r.StatusCode)
	}
	r, _ := do(t, s, "GET", "/nwm", "", "")
	if r.StatusCode != 301 || r.Header.Get("Location") != "nwm/" {
		t.Fatalf("redirect: %d %s", r.StatusCode, r.Header.Get("Location"))
	}
	r, _ = do(t, s, "GET", "/nwm/", "", "")
	if r.StatusCode != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("ui: %d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}
	if r.Header.Get("Content-Security-Policy") == "" || r.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers missing")
	}
}

func TestDeviceFlow(t *testing.T) {
	s := newTestServer(t, nil)
	r, body := do(t, s, "GET", "/api/v1/meters/a?max_age=0s", testKey, "")
	if r.StatusCode != 200 || body["reading"] == nil {
		t.Fatalf("meter: %d %v", r.StatusCode, body)
	}
	// Byte slices must serialise as JSON arrays, not base64 strings.
	r, body = do(t, s, "GET", "/api/v1/devices/a/snapshot", testKey, "")
	if _, ok := body["snapshot"].(map[string]any)["debug_numbers"].([]any); r.StatusCode != 200 || !ok {
		t.Fatalf("snapshot debug_numbers: %d %v", r.StatusCode, body["snapshot"])
	}
	_, body = do(t, s, "GET", "/api/v1/buses/sim", testKey, "")
	if _, ok := body["config"].(map[string]any)["simulator"].(map[string]any)["slaves"].([]any); !ok {
		t.Fatalf("bus simulator slaves: %v", body["config"])
	}
	if r, _ := do(t, s, "PUT", "/api/v1/devices/a/slave-address", testKey, `{"slave":2}`); r.StatusCode != 409 {
		t.Fatalf("slave conflict: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "PUT", "/api/v1/devices/a/slave-address", testKey, `{"slave":5}`); r.StatusCode != 200 {
		t.Fatalf("slave change: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "GET", "/api/v1/devices/a/counters", testKey, ""); r.StatusCode != 200 {
		t.Fatalf("read after readdress: %d", r.StatusCode)
	}
	run := s.Recorder.RunOnce(context.Background(), time.Now().Truncate(time.Second))
	if run.Recorded != 2 {
		t.Fatalf("recorder: %+v", run)
	}
	r, body = do(t, s, "GET", "/api/v1/measurements?page_size=1", testKey, "")
	if r.StatusCode != 200 || body["total"].(float64) != 2 || body["total_pages"].(float64) != 2 {
		t.Fatalf("measurements: %d %v", r.StatusCode, body)
	}
	if r, _ := do(t, s, "GET", "/api/v1/measurements/aggregate?from=-1h&bucket=10s", testKey, ""); r.StatusCode != 400 {
		t.Fatalf("tiny bucket accepted: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "GET", "/api/v1/measurements/aggregate?from=-8000h&bucket=1m", testKey, ""); r.StatusCode != 400 {
		t.Fatalf("too many buckets accepted: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "PUT", "/api/v1/devices/a/registers/16", testKey, `{"value":70000}`); r.StatusCode != 400 {
		t.Fatalf("out-of-range raw value accepted: %d", r.StatusCode)
	}
}

func TestRawWritesDisabled(t *testing.T) {
	s := newTestServer(t, func(c *config.Config) { c.Modbus.AllowRawWrites = false; c.Modbus.AllowBroadcast = false })
	if r, _ := do(t, s, "PUT", "/api/v1/devices/a/registers/13", testKey, `{"value":1}`); r.StatusCode != 403 {
		t.Fatalf("raw write: %d", r.StatusCode)
	}
	if r, _ := do(t, s, "POST", "/api/v1/buses/sim/broadcast/clock", testKey, ""); r.StatusCode != 403 {
		t.Fatalf("broadcast: %d", r.StatusCode)
	}
}

func TestWithLanguage(t *testing.T) {
	in := []byte(`<html lang="en"><head><meta name="nwm-panel-default-language" content="en" /></head></html>`)
	want := `<html lang="es"><head><meta name="nwm-panel-default-language" content="es" /></head></html>`
	if got := string(withLanguage(in, "es")); got != want {
		t.Fatalf("got %s", got)
	}
}
