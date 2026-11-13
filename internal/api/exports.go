package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

func (s *Server) listExports(c fiber.Ctx) error {
	files, err := s.Exporter.List()
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"status": s.Exporter.Status(), "files": files})
}

// runExport generates the periodic export immediately. The window ends at
// "to" (default now).
func (s *Server) runExport(c fiber.Ctx) error {
	if !s.Store.Enabled() {
		return store.ErrDisabled
	}
	var req struct {
		To string `json:"to"`
	}
	if len(c.Body()) > 0 {
		if err := decode(c, &req); err != nil {
			return err
		}
	}
	tick, err := parseTime(req.To, time.Now())
	if err != nil {
		return badRequest("to: %v", err)
	}
	res, err := s.Exporter.RunOnce(context.WithoutCancel(c.Context()), tick.Truncate(time.Second))
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// regenerateExports rewrites the periodic export files whose windows overlap
// [from, to) from the current database contents. Both bounds are required.
func (s *Server) regenerateExports(c fiber.Ctx) error {
	if !s.Store.Enabled() {
		return store.ErrDisabled
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := decode(c, &req); err != nil {
		return err
	}
	if req.From == "" || req.To == "" {
		return badRequest("from and to are required")
	}
	now := time.Now()
	from, err := parseTime(req.From, now)
	if err != nil {
		return badRequest("from: %v", err)
	}
	to, err := parseTime(req.To, now)
	if err != nil {
		return badRequest("to: %v", err)
	}
	if _, err := s.Exporter.Ticks(from, to, now); err != nil {
		return badRequest("%v", err)
	}
	res, err := s.Exporter.Regenerate(context.WithoutCancel(c.Context()), from, to)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

func (s *Server) downloadExport(c fiber.Ctx) error {
	p, err := s.Exporter.Path(c.Params("name"))
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	return c.Download(p, c.Params("name"))
}

func (s *Server) deleteExport(c fiber.Ctx) error {
	if err := s.Exporter.Delete(c.Params("name")); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"deleted": c.Params("name")})
}
