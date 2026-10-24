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
