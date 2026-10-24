// Package api exposes the REST API and serves the embedded dashboard.
package api

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/export"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/recorder"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// Deps are the services the API depends on.
type Deps struct {
	Config     *config.Config
	ConfigPath string
	APIKey     string
	Inventory  *inventory.Manager
	Store      *store.Store
	Recorder   *recorder.Recorder
	Exporter   *export.Exporter
	Log        *slog.Logger
}

// Server holds the HTTP application.
type Server struct {
	Deps
	App       *fiber.App
	startedAt time.Time
	authFail  *failureLimiter
}

// contentSecurityPolicy allows only same-origin resources. Inline styles are
// required by the UI component library for positioning.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; " +
	"base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// New builds the Fiber application with every route mounted under
// server.base_path.
func New(d Deps) *Server {
	sc := d.Config.Server
	s := &Server{Deps: d, startedAt: time.Now()}
	s.authFail = newFailureLimiter(sc.RateLimit.AuthFailuresPerMinute, time.Minute)

	fcfg := fiber.Config{
		AppName:      "nwm-panel",
		BodyLimit:    sc.BodyLimit,
		ReadTimeout:  sc.ReadTimeout.D(),
		WriteTimeout: sc.WriteTimeout.D(),
		IdleTimeout:  sc.IdleTimeout.D(),
		ErrorHandler: s.errorHandler,
	}
	if len(sc.TrustedProxies) > 0 {
		fcfg.TrustProxy = true
		fcfg.ProxyHeader = sc.ProxyHeader
		fcfg.TrustProxyConfig = trustConfig(sc.TrustedProxies)
	}
	app := fiber.New(fcfg)
	s.App = app

	app.Use(recover.New())
	app.Use(requestid.New())
	if sc.AccessLog {
		app.Use(s.accessLog)
	}
	app.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy:     contentSecurityPolicy,
		ReferrerPolicy:            "no-referrer",
		CrossOriginEmbedderPolicy: "unsafe-none",
		PermissionPolicy:          "camera=(), microphone=(), geolocation=(), payment=()",
	}))
	app.Use(compress.New())
	if len(sc.CORSOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins: sc.CORSOrigins,
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Authorization", "X-API-Key", "Content-Type"},
			MaxAge:       600,
		}))
	}

	root := app.Group(sc.BasePath)
	root.Get("/api/v1/health", s.health)

	apiGroup := root.Group("/api/v1")
	if sc.RateLimit.Enabled && sc.RateLimit.RequestsPerMinute > 0 {
		apiGroup.Use(limiter.New(limiter.Config{
			Max:        sc.RateLimit.RequestsPerMinute,
			Expiration: time.Minute,
			KeyGenerator: func(c fiber.Ctx) string {
				return c.IP()
			},
			LimitReached: func(c fiber.Ctx) error {
				return fiber.NewError(fiber.StatusTooManyRequests, "rate limit exceeded")
			},
		}))
	}
	apiGroup.Use(s.auth)
	s.routes(apiGroup)
	apiGroup.Use(func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "no such endpoint: "+c.Method()+" "+c.Path())
	})

	if sc.UI {
		s.mountUI(app, sc.BasePath, sc.UILanguage)
	}
	return s
}

func (s *Server) routes(r fiber.Router) {
	r.Get("/auth", s.authCheck)
	r.Get("/system", s.systemInfo)
	r.Get("/system/config", s.systemConfig)
	r.Get("/registers", s.registerMap)

	r.Get("/buses", s.listBuses)
	r.Post("/buses", s.createBus)
	r.Get("/buses/:bus", s.getBus)
	r.Put("/buses/:bus", s.updateBus)
	r.Delete("/buses/:bus", s.deleteBus)
	r.Post("/buses/:bus/scan", s.startScan)
	r.Get("/buses/:bus/scan", s.getScan)
	r.Delete("/buses/:bus/scan", s.cancelScan)
	r.Post("/buses/:bus/broadcast/slave-address", s.broadcastSlaveAddress)
	r.Post("/buses/:bus/broadcast/clock", s.broadcastClock)

	r.Get("/devices", s.listDevices)
	r.Post("/devices", s.createDevice)
	r.Get("/devices/:id", s.getDevice)
	r.Put("/devices/:id", s.updateDevice)
	r.Delete("/devices/:id", s.deleteDevice)

	r.Get("/devices/:id/snapshot", s.deviceSnapshot)
	r.Get("/devices/:id/registers", s.readRegisters)
	r.Post("/devices/:id/registers", s.writeRegisters)
	r.Put("/devices/:id/registers/:address", s.writeRegister)
	r.Get("/devices/:id/counters", s.getCounters)
	r.Get("/devices/:id/slave-address", s.getSlaveAddress)
	r.Put("/devices/:id/slave-address", s.setSlaveAddress)
	r.Get("/devices/:id/clock", s.getClock)
	r.Put("/devices/:id/clock", s.setClock)
	r.Get("/devices/:id/device-type", s.getDeviceType)
	r.Put("/devices/:id/device-type", s.setDeviceType)
	r.Get("/devices/:id/firmware", s.getFirmware)
	r.Get("/devices/:id/reverse-flow-threshold", s.getReverseThreshold)
	r.Put("/devices/:id/reverse-flow-threshold", s.setReverseThreshold)
	r.Get("/devices/:id/diagnostics", s.getDiagnostics)
	r.Get("/devices/:id/pulse-equivalents", s.getPulseEquivalents)
	r.Put("/devices/:id/pulse-equivalents", s.setPulseEquivalents)
	r.Post("/devices/:id/pulse-equivalents/apply", s.applyPulseEquivalents)
	r.Get("/devices/:id/base-readings", s.getBaseReadings)
	r.Put("/devices/:id/base-readings", s.setBaseReadings)

	r.Get("/meters", s.listMeters)
	r.Get("/meters/:id", s.getMeter)
	r.Get("/meters/:id/consumption", s.meterConsumption)
	r.Get("/meters/:id/series", s.meterSeries)
	r.Get("/consumption", s.consumptionSummary)

	r.Get("/measurements", s.listMeasurements)
	r.Get("/measurements/latest", s.latestMeasurements)
	r.Get("/measurements/aggregate", s.aggregateMeasurements)
	r.Get("/measurements/stats", s.measurementStats)
	r.Get("/measurements/csv", s.measurementsCSV)

	r.Get("/recorder", s.recorderStatus)
	r.Post("/recorder/run", s.recorderRun)

	r.Get("/exports", s.listExports)
	r.Post("/exports/run", s.runExport)
	r.Get("/exports/files/:name", s.downloadExport)
	r.Delete("/exports/files/:name", s.deleteExport)
}

func trustConfig(entries []string) fiber.TrustProxyConfig {
	tc := fiber.TrustProxyConfig{}
	for _, e := range entries {
		switch strings.ToLower(e) {
		case "loopback":
			tc.Loopback = true
		case "private":
			tc.Private = true
		case "linklocal", "link-local":
			tc.LinkLocal = true
		case "unix":
			tc.UnixSocket = true
		default:
			tc.Proxies = append(tc.Proxies, e)
		}
	}
	return tc
}

func (s *Server) accessLog(c fiber.Ctx) error {
	start := time.Now()
	err := c.Next()
	status := c.Response().StatusCode()
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			status = fe.Code
		} else {
			status = fiber.StatusInternalServerError
		}
	}
	level := slog.LevelInfo
	if status >= 500 {
		level = slog.LevelWarn
	}
	s.Log.Log(c.Context(), level, "http",
		"method", c.Method(), "path", c.Path(), "status", status,
		"ms", time.Since(start).Milliseconds(), "ip", c.IP(),
		"request_id", requestid.FromContext(c))
	return err
}

func (s *Server) health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
