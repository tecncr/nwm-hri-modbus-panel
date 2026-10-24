package api

import (
	"net/url"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/hri"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/version"
)

func (s *Server) systemInfo(c fiber.Ctx) error {
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	stats, err := s.Store.Stats(ctx)
	storage := fiber.Map{"driver": s.Store.Driver(), "enabled": s.Store.Enabled(), "stats": stats}
	if err != nil {
		storage["error"] = err.Error()
	}
	buses, devices := s.Inventory.Inventory()
	return c.JSON(fiber.Map{
		"name":    "nwm-panel",
		"version": version.String(),
		"build": fiber.Map{
			"version": version.Version, "commit": version.Commit, "date": version.Date,
			"os": version.Os, "arch": version.Arch, "go": runtime.Version(),
		},
		"started_at":     s.startedAt,
		"uptime_seconds": int64(time.Since(s.startedAt).Seconds()),
		"config_file":    s.ConfigPath,
		"persistent":     s.Inventory.Persistent(),
		"buses":          len(buses),
		"devices":        len(devices),
		"storage":        storage,
		"recorder":       s.Recorder.Status(),
		"export":         s.Exporter.Status(),
		"modbus": fiber.Map{
			"allow_raw_writes": s.Config.Modbus.AllowRawWrites,
			"allow_broadcast":  s.Config.Modbus.AllowBroadcast,
			"timeout":          s.Config.Modbus.Timeout,
			"retries":          s.Config.Modbus.Retries,
			"live_cache_ttl":   s.Config.Modbus.LiveCacheTTL,
		},
		"query": s.Config.Query,
	})
}

// systemConfig returns the effective configuration with secrets redacted.
func (s *Server) systemConfig(c fiber.Ctx) error {
	cfg := *s.Config
	cfg.Storage.TimescaleDB.DSN = redactDSN(cfg.Storage.TimescaleDB.DSN)
	cfg.Buses, cfg.Devices = s.Inventory.Inventory()
	return c.JSON(cfg)
}

func redactDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		if _, has := u.User.Password(); has {
			u.User = url.UserPassword(u.User.Username(), "REDACTED")
		}
		q := u.Query()
		if q.Has("password") {
			q.Set("password", "REDACTED")
			u.RawQuery = q.Encode()
		}
		return u.String()
	}
	return "REDACTED"
}

func (s *Server) registerMap(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"registers": hri.Registers,
		"limits": fiber.Map{
			"slave_address":           fiber.Map{"min": 1, "max": 247},
			"base_pulse_equivalent":   fiber.Map{"min": hri.MinBasePulseLiters, "max": hri.MaxBasePulseLiters},
			"output_pulse_equivalent": fiber.Map{"min": hri.MinOutputPulseLiters, "max": hri.MaxOutputPulseLiters},
			"read_quantity":           fiber.Map{"min": 1, "max": 125},
			"write_quantity":          fiber.Map{"min": 1, "max": 123},
		},
		"export_columns":         config.ExportColumns,
		"default_export_columns": config.DefaultExportColumns,
	})
}
