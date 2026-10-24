package api

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/hri"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

type busView struct {
	Config  config.BusConfig `json:"config"`
	Stats   modbus.Stats     `json:"stats"`
	Devices []string         `json:"devices"`
}

func (s *Server) busView(b *inventory.Bus) busView {
	v := busView{Config: b.Config, Stats: b.Client.Stats(), Devices: []string{}}
	for _, d := range s.Inventory.Devices() {
		if d.Config.Bus == b.Config.ID {
			v.Devices = append(v.Devices, d.Config.ID)
		}
	}
	return v
}

func (s *Server) listBuses(c fiber.Ctx) error {
	out := []busView{}
	for _, b := range s.Inventory.Buses() {
		out = append(out, s.busView(b))
	}
	return c.JSON(fiber.Map{"buses": out})
}

func (s *Server) getBus(c fiber.Ctx) error {
	b, err := s.Inventory.Bus(c.Params("bus"))
	if err != nil {
		return err
	}
	return c.JSON(s.busView(b))
}

func (s *Server) createBus(c fiber.Ctx) error {
	var cfg config.BusConfig
	if err := decode(c, &cfg); err != nil {
		return err
	}
	b, err := s.Inventory.PutBus(cfg, true)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"bus": s.busView(b), "inventory": s.persisted()})
}

func (s *Server) updateBus(c fiber.Ctx) error {
	var cfg config.BusConfig
	if err := decode(c, &cfg); err != nil {
		return err
	}
	if cfg.ID == "" {
		cfg.ID = c.Params("bus")
	}
	if cfg.ID != c.Params("bus") {
		return badRequest("bus id in body (%q) does not match the URL; ids cannot be renamed", cfg.ID)
	}
	b, err := s.Inventory.PutBus(cfg, false)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"bus": s.busView(b), "inventory": s.persisted()})
}

func (s *Server) deleteBus(c fiber.Ctx) error {
	if err := s.Inventory.DeleteBus(c.Params("bus")); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"deleted": c.Params("bus"), "inventory": s.persisted()})
}

func (s *Server) startScan(c fiber.Ctx) error {
	req := struct {
		From      int `json:"from"`
		To        int `json:"to"`
		TimeoutMS int `json:"timeout_ms"`
	}{From: 1, To: 247, TimeoutMS: 300}
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	job, err := s.Inventory.StartScan(c.Params("bus"), req.From, req.To, time.Duration(req.TimeoutMS)*time.Millisecond)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(job)
}

func (s *Server) getScan(c fiber.Ctx) error {
	job, err := s.Inventory.Scan(c.Params("bus"))
	if err != nil {
		return err
	}
	return c.JSON(job)
}

func (s *Server) cancelScan(c fiber.Ctx) error {
	if err := s.Inventory.CancelScan(c.Params("bus")); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"cancelled": true})
}

func (s *Server) requireBroadcast() error {
	if !s.Config.Modbus.AllowBroadcast {
		return fiber.NewError(fiber.StatusForbidden, "broadcast writes are disabled (modbus.allow_broadcast: false)")
	}
	return nil
}

// broadcastSlaveAddress assigns an address to every encoder on the bus; it
// is intended for commissioning a single, unknown encoder.
func (s *Server) broadcastSlaveAddress(c fiber.Ctx) error {
	if err := s.requireBroadcast(); err != nil {
		return err
	}
	var req struct {
		Slave   int  `json:"slave"`
		Confirm bool `json:"confirm"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	if !req.Confirm {
		return badRequest("set \"confirm\": true; every encoder on the bus will take this address")
	}
	if req.Slave < 1 || req.Slave > 247 {
		return badRequest("slave must be between 1 and 247")
	}
	b, err := s.Inventory.Bus(c.Params("bus"))
	if err != nil {
		return err
	}
	ctx, cancel := s.deviceCtx(c)
	defer cancel()
	if err := hri.BroadcastSlaveAddress(ctx, b.Client, uint8(req.Slave)); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"broadcast": true, "slave": req.Slave})
}

// broadcastClock sets the clock of every encoder on the bus.
func (s *Server) broadcastClock(c fiber.Ctx) error {
	if err := s.requireBroadcast(); err != nil {
		return err
	}
	var req struct {
		Time     string `json:"time"`
		Timezone string `json:"timezone"`
	}
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	loc := time.Local
	if req.Timezone != "" {
		l, err := time.LoadLocation(req.Timezone)
		if err != nil {
			return badRequest("timezone: %v", err)
		}
		loc = l
	}
	t := time.Now()
	if req.Time != "" {
		var err error
		if t, err = time.Parse(time.RFC3339, req.Time); err != nil {
			return badRequest("time must be RFC 3339")
		}
	}
	b, err := s.Inventory.Bus(c.Params("bus"))
	if err != nil {
		return err
	}
	ctx, cancel := s.deviceCtx(c)
	defer cancel()
	t = t.In(loc)
	if err := hri.BroadcastClock(ctx, b.Client, t); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"broadcast": true, "time": t.Format(time.RFC3339)})
}
