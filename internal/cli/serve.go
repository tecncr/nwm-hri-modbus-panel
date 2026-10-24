package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/spf13/cobra"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/api"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/export"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/recorder"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/version"
)

// flagBindings maps serve flags to config keys.
var flagBindings = map[string]string{
	"listen":            "server.listen",
	"base-path":         "server.base_path",
	"trusted-proxies":   "server.trusted_proxies",
	"ui":                "server.ui",
	"ui-language":       "server.ui_language",
	"log-level":         "log.level",
	"log-format":        "log.format",
	"storage-driver":    "storage.driver",
	"sqlite-path":       "storage.sqlite.path",
	"timescaledb-dsn":   "storage.timescaledb.dsn",
	"retention":         "storage.retention",
	"recorder":          "recorder.enabled",
	"recorder-interval": "recorder.interval",
	"recorder-cron":     "recorder.cron",
	"export":            "export.enabled",
	"export-dir":        "export.directory",
	"export-interval":   "export.interval",
	"export-cron":       "export.cron",
	"export-window":     "export.window",
	"modbus-timeout":    "modbus.timeout",
	"modbus-retries":    "modbus.retries",
	"allow-raw-writes":  "modbus.allow_raw_writes",
	"allow-broadcast":   "modbus.allow_broadcast",
}

func newServeCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the API server, recorder, CSV exporter and dashboard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := bindFlags(cmd, opts); err != nil {
				return err
			}
			return serve(cmd.Context(), opts)
		},
	}
	f := cmd.Flags()
	f.String("listen", "", "listen address (host:port or unix:/path/to.sock)")
	f.String("base-path", "", "URL path prefix when served under a sub-path by a reverse proxy, e.g. /nwm")
	f.StringSlice("trusted-proxies", nil, "reverse proxy IPs/CIDRs (or loopback, private) allowed to set client IP headers")
	f.Bool("ui", true, "serve the embedded web dashboard")
	f.String("ui-language", "", "dashboard language when the browser prefers none of the supported ones: "+strings.Join(config.UILanguages, ", "))
	f.String("log-level", "", "log level: debug, info, warn, error")
	f.String("log-format", "", "log format: text or json")
	f.String("storage-driver", "", "measurement database: sqlite, timescaledb or none")
	f.String("sqlite-path", "", "SQLite database file")
	f.String("timescaledb-dsn", "", "TimescaleDB connection string (prefer NWM_PANEL_STORAGE_TIMESCALEDB_DSN)")
	f.Duration("retention", 0, "delete measurements older than this (0 keeps everything)")
	f.Bool("recorder", true, "enable periodic recording")
	f.Duration("recorder-interval", 0, "recording interval, aligned to the wall clock (e.g. 1m fires at every :00)")
	f.String("recorder-cron", "", "recording cron expression (overrides --recorder-interval)")
	f.Bool("export", true, "enable periodic CSV exports")
	f.String("export-dir", "", "directory for periodic CSV files")
	f.Duration("export-interval", 0, "CSV export interval, aligned to the wall clock")
	f.String("export-cron", "", "CSV export cron expression (overrides --export-interval)")
	f.Duration("export-window", 0, "time span covered by each CSV file")
	f.Duration("modbus-timeout", 0, "Modbus response timeout")
	f.Int("modbus-retries", 0, "Modbus retries after a timeout or CRC error")
	f.Bool("allow-raw-writes", true, "allow raw register writes through the API")
	f.Bool("allow-broadcast", true, "allow broadcast writes through the API")
	return cmd
}

// bindFlags binds only flags that were explicitly set, so unset flags do not
// shadow config file values with their zero defaults.
func bindFlags(cmd *cobra.Command, opts *options) error {
	for name, key := range flagBindings {
		fl := cmd.Flags().Lookup(name)
		if fl == nil || !fl.Changed {
			continue
		}
		if err := opts.v.BindPFlag(key, fl); err != nil {
			return err
		}
	}
	return nil
}

func serve(ctx context.Context, opts *options) error {
	apiKey := os.Getenv(config.APIKeyEnv)
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("the %s environment variable must be set to a non-empty master API key", config.APIKeyEnv)
	}
	cfg, path, err := loadConfig(opts)
	if err != nil {
		return err
	}
	log := newLogger(cfg.Log)
	log.Info("starting "+appName, "version", version.String(), "commit", version.Commit, "config", orNone(path))
	if path == "" {
		log.Warn("no config file found: running with defaults, inventory changes will not be persisted")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	st, err := store.Open(startCtx, cfg.Storage, log)
	cancel()
	if err != nil {
		return err
	}
	defer st.Close()
	log.Info("measurement storage ready", "driver", cfg.Storage.Driver)

	inv, err := inventory.New(cfg, path, log)
	if err != nil {
		return err
	}
	defer inv.Close()
	log.Info("inventory loaded", "buses", len(cfg.Buses), "devices", len(cfg.Devices))

	rec, err := recorder.New(cfg.Recorder, cfg.Storage, inv, st, log)
	if err != nil {
		return fmt.Errorf("recorder: %w", err)
	}
	exp, err := export.New(cfg.Export, cfg.Query.MaxExportRows, st, inv, log)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	srv := api.New(api.Deps{
		Config: cfg, ConfigPath: path, APIKey: apiKey, Inventory: inv,
		Store: st, Recorder: rec, Exporter: exp, Log: log,
	})

	if cfg.Recorder.Enabled && st.Enabled() {
		go rec.Start(ctx)
	} else if cfg.Recorder.Enabled {
		log.Warn("recorder disabled because storage.driver is none")
	}
	if cfg.Export.Enabled {
		go exp.Start(ctx)
	}

	ln, err := listen(cfg.Server.Listen)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() {
		errc <- srv.App.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	log.Info("HTTP server listening", "addr", cfg.Server.Listen, "base_path", orNone(cfg.Server.BasePath), "ui", cfg.Server.UI)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	if err := srv.App.ShutdownWithContext(sctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		log.Warn("HTTP shutdown", "err", err)
	}
	return nil
}

func loadConfig(opts *options) (*config.Config, string, error) {
	return config.Load(opts.v, opts.configFile)
}

// listen supports "host:port" and "unix:/path/to.sock".
func listen(addr string) (net.Listener, error) {
	if p, ok := strings.CutPrefix(addr, "unix:"); ok {
		_ = os.Remove(p)
		ln, err := net.Listen("unix", p)
		if err != nil {
			return nil, err
		}
		_ = os.Chmod(p, 0o660)
		return ln, nil
	}
	return net.Listen("tcp", addr)
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
