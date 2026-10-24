package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/hri"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
)

type deviceView struct {
	Config   config.DeviceConfig   `json:"config"`
	Enabled  bool                  `json:"enabled"`
	Recorded bool                  `json:"recorded"`
	Timezone string                `json:"timezone"`
	State    inventory.DeviceState `json:"state"`
}

func viewDevice(d *inventory.Device) deviceView {
	return deviceView{
		Config: d.Config, Enabled: d.Config.IsEnabled(), Recorded: d.Config.IsRecorded(),
		Timezone: d.Loc.String(), State: d.State(),
	}
}

func (s *Server) listDevices(c fiber.Ctx) error {
	out := []deviceView{}
	for _, d := range s.Inventory.Devices() {
		out = append(out, viewDevice(d))
	}
	return c.JSON(fiber.Map{"devices": out})
}

func (s *Server) getDevice(c fiber.Ctx) error {
	d, err := s.Inventory.Device(c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(viewDevice(d))
}

func (s *Server) createDevice(c fiber.Ctx) error {
	var cfg config.DeviceConfig
	if err := decode(c, &cfg); err != nil {
		return err
	}
	d, err := s.Inventory.PutDevice(cfg, true)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"device": viewDevice(d), "inventory": s.persisted()})
}

func (s *Server) updateDevice(c fiber.Ctx) error {
	var cfg config.DeviceConfig
	if err := decode(c, &cfg); err != nil {
		return err
	}
	if cfg.ID == "" {
		cfg.ID = c.Params("id")
	}
	if cfg.ID != c.Params("id") {
		return badRequest("device id in body (%q) does not match the URL; ids cannot be renamed", cfg.ID)
	}
	d, err := s.Inventory.PutDevice(cfg, false)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"device": viewDevice(d), "inventory": s.persisted()})
}

func (s *Server) deleteDevice(c fiber.Ctx) error {
	if err := s.Inventory.DeleteDevice(c.Params("id")); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"deleted": c.Params("id"), "inventory": s.persisted()})
}

// withEncoder resolves the device, bounds the request and records the
// outcome in the device's health state.
func (s *Server) withEncoder(c fiber.Ctx, fn func(ctx context.Context, dev *inventory.Device, enc hri.Device) error) error {
	dev, enc, err := s.Inventory.Encoder(c.Params("id"))
	if err != nil {
		return err
	}
	ctx, cancel := s.deviceCtx(c)
	defer cancel()
	err = fn(ctx, dev, enc)
	if err == nil || isDeviceError(err) {
		dev.Track(err)
	}
	return err
}

// isDeviceError reports whether err came from talking to the encoder (as
// opposed to request validation).
func isDeviceError(err error) bool {
	if _, ok := err.(*fiber.Error); ok {
		return false
	}
	return !errors.Is(err, modbus.ErrInvalidRequest)
}

func (s *Server) deviceSnapshot(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		snap, err := enc.ReadSnapshot(ctx)
		if err != nil {
			return err
		}
		resp := fiber.Map{"device_id": dev.Config.ID, "slave": dev.Config.Slave, "snapshot": snap}
		if t, ok := snap.Clock.Time(dev.Loc); ok {
			resp["clock_time"] = t
			resp["clock_drift_seconds"] = t.Sub(snap.ReadAt).Seconds()
		}
		return c.JSON(resp)
	})
}

func (s *Server) readRegisters(c fiber.Ctx) error {
	addr, err := intQuery(c, "address", 0, 0, 0xFFFF)
	if err != nil {
		return err
	}
	count, err := intQuery(c, "count", 1, 1, modbus.MaxReadQuantity)
	if err != nil {
		return err
	}
	if addr+count > 0x10000 {
		return badRequest("address + count exceeds 0xFFFF")
	}
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		words, err := enc.ReadRegisters(ctx, uint16(addr), uint16(count))
		if err != nil {
			return err
		}
		out := make([]hri.RegisterValue, len(words))
		for i, w := range words {
			a := uint16(addr + i)
			out[i] = hri.RegisterValue{Address: a, Value: w, Hex: fmt.Sprintf("0x%04X", w)}
			if r, ok := hri.RegisterAt(a); ok {
				out[i].Key = r.Key
			}
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "address": addr, "count": count, "registers": out})
	})
}

func (s *Server) requireRawWrites() error {
	if !s.Config.Modbus.AllowRawWrites {
		return fiber.NewError(fiber.StatusForbidden, "raw register writes are disabled (modbus.allow_raw_writes: false)")
	}
	return nil
}

func (s *Server) writeRegister(c fiber.Ctx) error {
	if err := s.requireRawWrites(); err != nil {
		return err
	}
	addr, err := strconv.ParseUint(c.Params("address"), 0, 16)
	if err != nil {
		return badRequest("address must be an integer 0-65535 (decimal or 0x-prefixed hex)")
	}
	var req struct {
		Value *int `json:"value"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Value == nil || *req.Value < 0 || *req.Value > 0xFFFF {
		return badRequest("value must be an integer between 0 and 65535")
	}
	return s.rawWrite(c, uint16(addr), []uint16{uint16(*req.Value)}, false)
}

func (s *Server) writeRegisters(c fiber.Ctx) error {
	if err := s.requireRawWrites(); err != nil {
		return err
	}
	var req struct {
		Address *int  `json:"address"`
		Values  []int `json:"values"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.Address == nil || *req.Address < 0 || *req.Address > 0xFFFF {
		return badRequest("address must be an integer between 0 and 65535")
	}
	if len(req.Values) < 1 || len(req.Values) > modbus.MaxWriteQuantity {
		return badRequest("values must contain between 1 and %d registers", modbus.MaxWriteQuantity)
	}
	if *req.Address+len(req.Values) > 0x10000 {
		return badRequest("address range exceeds 0xFFFF")
	}
	vals := make([]uint16, len(req.Values))
	for i, v := range req.Values {
		if v < 0 || v > 0xFFFF {
			return badRequest("values[%d] must be between 0 and 65535", i)
		}
		vals[i] = uint16(v)
	}
	return s.rawWrite(c, uint16(*req.Address), vals, true)
}

// rawWrite performs FC06/FC10 writes, keeping the inventory in sync when the
// slave address register is overwritten.
func (s *Server) rawWrite(c fiber.Ctx, addr uint16, vals []uint16, multiple bool) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		newSlave := -1
		if addr == hri.RegSlaveAddress {
			newSlave = int(vals[0])
			if err := s.checkSlaveAvailable(dev, newSlave); err != nil {
				return err
			}
		}
		var err error
		if multiple {
			err = enc.WriteRegisters(ctx, addr, vals)
		} else {
			err = enc.WriteRegister(ctx, addr, vals[0])
		}
		if err != nil {
			return err
		}
		s.Inventory.Invalidate(dev.Config.ID)
		resp := fiber.Map{"device_id": dev.Config.ID, "address": addr, "values": vals, "written": true}
		if newSlave >= 0 && newSlave != int(dev.Config.Slave) {
			if err := s.Inventory.SetSlave(dev.Config.ID, uint8(newSlave)); err != nil {
				return err
			}
			resp["slave"] = newSlave
			resp["inventory"] = s.persisted()
		}
		return c.JSON(resp)
	})
}

func (s *Server) checkSlaveAvailable(dev *inventory.Device, slave int) error {
	if slave < 1 || slave > 247 {
		return badRequest("slave address must be between 1 and 247")
	}
	if other, ok := s.Inventory.DeviceOnBus(dev.Config.Bus, uint8(slave)); ok && other != dev.Config.ID {
		return fiber.NewError(fiber.StatusConflict, "slave "+strconv.Itoa(slave)+" is already configured for device "+other)
	}
	return nil
}

func (s *Server) getCounters(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadCounters(ctx)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "counters": v})
	})
}

func (s *Server) getSlaveAddress(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadSlaveAddress(ctx)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "configured_slave": dev.Config.Slave, "slave_address": v})
	})
}

func (s *Server) setSlaveAddress(c fiber.Ctx) error {
	var req struct {
		Slave int `json:"slave"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		if err := s.checkSlaveAvailable(dev, req.Slave); err != nil {
			return err
		}
		if err := enc.SetSlaveAddress(ctx, uint8(req.Slave)); err != nil {
			return err
		}
		if err := s.Inventory.SetSlave(dev.Config.ID, uint8(req.Slave)); err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "slave": req.Slave, "inventory": s.persisted()})
	})
}

func (s *Server) getClock(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		clk, err := enc.ReadClock(ctx)
		if err != nil {
			return err
		}
		resp := fiber.Map{"device_id": dev.Config.ID, "clock": clk, "timezone": dev.Loc.String()}
		if t, ok := clk.Time(dev.Loc); ok {
			resp["time"] = t
			resp["drift_seconds"] = t.Sub(time.Now()).Seconds()
		}
		return c.JSON(resp)
	})
}

func (s *Server) setClock(c fiber.Ctx) error {
	var req struct {
		Time string `json:"time"`
	}
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	t := time.Now()
	if req.Time != "" {
		var err error
		if t, err = time.Parse(time.RFC3339, req.Time); err != nil {
			return badRequest("time must be RFC 3339 (omit it to synchronise with the server clock)")
		}
	}
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		local := t.In(dev.Loc)
		if err := enc.SetClock(ctx, local); err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "time": local.Format(time.RFC3339), "timezone": dev.Loc.String()})
	})
}

// valueRequest is the body of single-value register writes.
type valueRequest struct {
	Value *int `json:"value"`
}

func (r valueRequest) check() error {
	if r.Value == nil || *r.Value < 0 || *r.Value > 0xFFFF {
		return badRequest("value must be an integer between 0 and 65535")
	}
	return nil
}

func (s *Server) getDeviceType(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadDeviceType(ctx)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "value": v})
	})
}

func (s *Server) setDeviceType(c fiber.Ctx) error {
	var req valueRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if err := req.check(); err != nil {
		return err
	}
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		if err := enc.SetDeviceType(ctx, uint16(*req.Value)); err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "value": *req.Value})
	})
}

func (s *Server) getFirmware(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadFirmware(ctx)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "firmware": v})
	})
}

func (s *Server) getReverseThreshold(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadReverseFlowThreshold(ctx)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "value": v})
	})
}

func (s *Server) setReverseThreshold(c fiber.Ctx) error {
	var req valueRequest
	if err := decode(c, &req); err != nil {
		return err
	}
	if err := req.check(); err != nil {
		return err
	}
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		if err := enc.SetReverseFlowThreshold(ctx, uint16(*req.Value)); err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "value": *req.Value})
	})
}

func (s *Server) getDiagnostics(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadDiagnostics(ctx)
		resp := fiber.Map{"device_id": dev.Config.ID, "diagnostics": v}
		if err != nil {
			if v.DebugNumbers == nil && v.BatteryStatus == 0 && v.Failure == 0 {
				return err
			}
			resp["debug_error"] = err.Error()
		}
		return c.JSON(resp)
	})
}

func (s *Server) getPulseEquivalents(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadPulseEquivalents(ctx)
		if err != nil {
			return err
		}
		m := dev.Config.Meter
		return c.JSON(fiber.Map{
			"device_id": dev.Config.ID, "pulse_equivalents": v,
			"expected": fiber.Map{"base_liters_per_pulse": m.BaseLitersPerPulse, "output_liters_per_unit": m.OutputLitersPerUnit},
			"matches_expected": m.BaseLitersPerPulse == 0 ||
				(int(v.BaseLiters) == m.BaseLitersPerPulse && int(v.OutputLiters) == m.OutputLitersPerUnit),
		})
	})
}

func (s *Server) setPulseEquivalents(c fiber.Ctx) error {
	var req struct {
		Base   int `json:"base_liters_per_pulse"`
		Output int `json:"output_liters_per_unit"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	if err := hri.ValidatePulseEquivalents(req.Base, req.Output); err != nil {
		return badRequest("%v", err)
	}
	return s.writePulse(c, hri.PulseEquivalents{BaseLiters: uint16(req.Base), OutputLiters: uint16(req.Output)})
}

func (s *Server) applyPulseEquivalents(c fiber.Ctx) error {
	d, err := s.Inventory.Device(c.Params("id"))
	if err != nil {
		return err
	}
	m := d.Config.Meter
	if m.BaseLitersPerPulse == 0 {
		return badRequest("device has no meter.base_liters_per_pulse / meter.output_liters_per_unit configured")
	}
	return s.writePulse(c, hri.PulseEquivalents{BaseLiters: uint16(m.BaseLitersPerPulse), OutputLiters: uint16(m.OutputLitersPerUnit)})
}

func (s *Server) writePulse(c fiber.Ctx, p hri.PulseEquivalents) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		if err := enc.SetPulseEquivalents(ctx, p); err != nil {
			return err
		}
		s.Inventory.Invalidate(dev.Config.ID)
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "pulse_equivalents": p})
	})
}

func (s *Server) getBaseReadings(c fiber.Ctx) error {
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		v, err := enc.ReadBaseReadings(ctx)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "base_readings": v})
	})
}

// setBaseReadings presets the counters. Values can be given as raw counter
// units (forward/reverse) or as liters (forward_liters/reverse_liters), which
// are converted with the device's liters-per-unit and offset. Omitted values
// keep their current value.
func (s *Server) setBaseReadings(c fiber.Ctx) error {
	var req struct {
		Forward       *int64   `json:"forward"`
		Reverse       *int64   `json:"reverse"`
		ForwardLiters *float64 `json:"forward_liters"`
		ReverseLiters *float64 `json:"reverse_liters"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	if (req.Forward != nil && req.ForwardLiters != nil) || (req.Reverse != nil && req.ReverseLiters != nil) {
		return badRequest("give each counter either in raw units or in liters, not both")
	}
	if req.Forward == nil && req.Reverse == nil && req.ForwardLiters == nil && req.ReverseLiters == nil {
		return badRequest("nothing to write: set forward/reverse or forward_liters/reverse_liters")
	}
	return s.withEncoder(c, func(ctx context.Context, dev *inventory.Device, enc hri.Device) error {
		cur, err := enc.ReadBaseReadings(ctx)
		if err != nil {
			return err
		}
		lpu := dev.Config.Meter.LitersPerUnit
		if (req.ForwardLiters != nil || req.ReverseLiters != nil) && lpu <= 0 {
			p, err := enc.ReadPulseEquivalents(ctx)
			if err != nil {
				return err
			}
			lpu = float64(max(p.OutputLiters, 1))
		}
		toRaw := func(name string, liters float64, offset float64) (int64, error) {
			v := math.Round((liters - offset) / lpu)
			if math.IsNaN(v) || v < 0 || v > math.MaxUint32 {
				return 0, badRequest("%s converts to %v counter units, outside 0-%d", name, v, uint32(math.MaxUint32))
			}
			return int64(v), nil
		}
		next := cur
		if req.ForwardLiters != nil {
			v, err := toRaw("forward_liters", *req.ForwardLiters, dev.Config.Meter.OffsetLiters)
			if err != nil {
				return err
			}
			req.Forward = &v
		}
		if req.ReverseLiters != nil {
			v, err := toRaw("reverse_liters", *req.ReverseLiters, 0)
			if err != nil {
				return err
			}
			req.Reverse = &v
		}
		for name, v := range map[string]*int64{"forward": req.Forward, "reverse": req.Reverse} {
			if v != nil && (*v < 0 || *v > math.MaxUint32) {
				return badRequest("%s must be between 0 and %d", name, uint32(math.MaxUint32))
			}
		}
		if req.Forward != nil {
			next.Forward = uint32(*req.Forward)
		}
		if req.Reverse != nil {
			next.Reverse = uint32(*req.Reverse)
		}
		if err := enc.SetBaseReadings(ctx, next); err != nil {
			return err
		}
		s.Inventory.Invalidate(dev.Config.ID)
		return c.JSON(fiber.Map{"device_id": dev.Config.ID, "previous": cur, "base_readings": next, "liters_per_unit": lpu})
	})
}
