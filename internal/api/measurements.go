package api

import (
	"bufio"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/export"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// maxOffset bounds OFFSET-based pagination so deep pages cannot force huge
// scans; narrow the time range instead.
const maxOffset = 1_000_000

func orderParam(c fiber.Ctx, def store.Order) (store.Order, error) {
	switch o := store.Order(strings.ToLower(c.Query("order", string(def)))); o {
	case store.Asc, store.Desc:
		return o, nil
	}
	return "", badRequest("order must be asc or desc")
}

func (s *Server) listMeasurements(c fiber.Ctx) error {
	from, to, err := s.timeRange(c, 24*time.Hour)
	if err != nil {
		return err
	}
	ids, err := deviceList(c)
	if err != nil {
		return err
	}
	q := s.Config.Query
	page, err := intQuery(c, "page", 1, 1, maxOffset)
	if err != nil {
		return err
	}
	size, err := intQuery(c, "page_size", q.DefaultPageSize, 1, q.MaxPageSize)
	if err != nil {
		return err
	}
	offset := (page - 1) * size
	if offset > maxOffset {
		return badRequest("page too deep (offset limit %d); narrow the time range instead", maxOffset)
	}
	order, err := orderParam(c, store.Desc)
	if err != nil {
		return err
	}
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	rows, total, err := s.Store.Query(ctx, store.Filter{Devices: ids, From: from, To: to}, offset, size, order)
	if err != nil {
		return err
	}
	pages := (total + int64(size) - 1) / int64(size)
	return c.JSON(fiber.Map{
		"from": from, "to": to, "devices": ids, "order": order,
		"page": page, "page_size": size, "total": total, "total_pages": pages,
		"measurements": rows,
	})
}

func (s *Server) latestMeasurements(c fiber.Ctx) error {
	ids, err := deviceList(c)
	if err != nil {
		return err
	}
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	rows, err := s.Store.Latest(ctx, ids)
	if err != nil {
		return err
	}
	if rows == nil {
		rows = []store.Measurement{}
	}
	return c.JSON(fiber.Map{"measurements": rows})
}

func (s *Server) aggregateMeasurements(c fiber.Ctx) error {
	from, to, bucket, loc, err := s.bucketParams(c)
	if err != nil {
		return err
	}
	ids, err := deviceList(c)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		ids = s.Inventory.DeviceIDs()
	}
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	res, err := s.series(ctx, ids, from, to, bucket, loc)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"from": from, "to": to, "bucket": bucket.String(), "timezone": loc.String(), "series": res})
}

func (s *Server) measurementStats(c fiber.Ctx) error {
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	st, err := s.Store.Stats(ctx)
	if err != nil {
		return err
	}
	return c.JSON(st)
}

// measurementsCSV streams matching measurements as a CSV download, capped at
// query.max_export_rows.
func (s *Server) measurementsCSV(c fiber.Ctx) error {
	if !s.Store.Enabled() {
		return store.ErrDisabled
	}
	from, to, err := s.timeRange(c, 24*time.Hour)
	if err != nil {
		return err
	}
	ids, err := deviceList(c)
	if err != nil {
		return err
	}
	order, err := orderParam(c, store.Asc)
	if err != nil {
		return err
	}
	var cols []string
	if v := c.Query("columns"); v != "" {
		for _, col := range strings.Split(v, ",") {
			col = strings.TrimSpace(col)
			if !slices.Contains(config.ExportColumns, col) {
				return badRequest("unknown column %q (valid: %s)", col, strings.Join(config.ExportColumns, ", "))
			}
			cols = append(cols, col)
		}
	}
	f := s.Exporter.Format(cols)
	if loc, err := location(c, f.Location); err != nil {
		return err
	} else {
		f.Location = loc
	}
	limit := s.Config.Query.MaxExportRows
	name := fmt.Sprintf("measurements_%s_%s.csv", from.UTC().Format("20060102T150405Z"), to.UTC().Format("20060102T150405Z"))
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	c.Set("X-Max-Rows", fmt.Sprint(limit))
	filter := store.Filter{Devices: ids, From: from, To: to}
	timeout := s.Config.Query.Timeout.D() * 4
	return c.SendStreamWriter(func(w *bufio.Writer) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cw, err := export.NewWriter(w, f)
		if err != nil {
			return
		}
		if _, err := s.Store.Stream(ctx, filter, order, limit, cw.Write); err != nil {
			s.Log.Warn("CSV download interrupted", "err", err)
		}
		_ = cw.Flush()
	})
}

func (s *Server) recorderStatus(c fiber.Ctx) error {
	return c.JSON(s.Recorder.Status())
}

// recorderRun records all devices immediately.
func (s *Server) recorderRun(c fiber.Ctx) error {
	run := s.Recorder.RunOnce(context.WithoutCancel(c.Context()), time.Now().Truncate(time.Second))
	return c.JSON(run)
}
