package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/export"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/modbus"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// apiError is the JSON error envelope.
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	} `json:"error"`
}

func (s *Server) errorHandler(c fiber.Ctx, err error) error {
	status, code := fiber.StatusInternalServerError, "internal_error"
	msg := err.Error()
	var details any

	var fe *fiber.Error
	if errors.As(err, &fe) {
		status = fe.Code
		code = statusCode(status)
		msg = fe.Message
	} else if exc, ok := errors.AsType[*modbus.ExceptionError](err); ok {
		status, code = fiber.StatusBadGateway, "modbus_exception"
		details = fiber.Map{"function": exc.Function, "exception_code": exc.Code, "exception": modbus.ExceptionName(exc.Code)}
	} else {
		switch {
		case errors.Is(err, inventory.ErrNotFound), errors.Is(err, export.ErrNotFound):
			status, code = fiber.StatusNotFound, "not_found"
		case errors.Is(err, inventory.ErrConflict), errors.Is(err, inventory.ErrDisabled):
			status, code = fiber.StatusConflict, "conflict"
		case errors.Is(err, inventory.ErrInvalid), errors.Is(err, modbus.ErrInvalidRequest):
			status, code = fiber.StatusBadRequest, "invalid_request"
		case errors.Is(err, modbus.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
			status, code = fiber.StatusGatewayTimeout, "timeout"
		case errors.Is(err, modbus.ErrCRC), errors.Is(err, modbus.ErrInvalidResponse):
			status, code = fiber.StatusBadGateway, "bad_response"
		case errors.Is(err, store.ErrDisabled):
			status, code = fiber.StatusServiceUnavailable, "storage_disabled"
		}
	}
	if status >= 500 && status != fiber.StatusBadGateway && status != fiber.StatusGatewayTimeout && status != fiber.StatusServiceUnavailable {
		s.Log.Error("request failed", "method", c.Method(), "path", c.Path(), "err", err)
		msg = "internal server error"
	}
	var body apiError
	body.Error.Code, body.Error.Message, body.Error.Details = code, msg, details
	return c.Status(status).JSON(body)
}

func statusCode(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "invalid_request"
	case fiber.StatusUnauthorized:
		return "unauthorized"
	case fiber.StatusForbidden:
		return "forbidden"
	case fiber.StatusNotFound:
		return "not_found"
	case fiber.StatusMethodNotAllowed:
		return "method_not_allowed"
	case fiber.StatusConflict:
		return "conflict"
	case fiber.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case fiber.StatusTooManyRequests:
		return "rate_limited"
	case fiber.StatusServiceUnavailable:
		return "unavailable"
	}
	if status >= 500 {
		return "internal_error"
	}
	return "error"
}

func badRequest(format string, a ...any) error {
	return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf(format, a...))
}

// decode strictly parses a JSON body into v, rejecting unknown fields.
func decode(c fiber.Ctx, v any) error {
	body := c.Body()
	if len(bytes.TrimSpace(body)) == 0 {
		return badRequest("request body must be a JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return badRequest("invalid JSON body: %v", err)
	}
	if dec.More() {
		return badRequest("invalid JSON body: trailing data")
	}
	return nil
}

// deviceCtx bounds a request that talks to an encoder.
func (s *Server) deviceCtx(c fiber.Ctx) (context.Context, context.CancelFunc) {
	t := s.Config.Modbus.Timeout.D()*time.Duration(s.Config.Modbus.Retries+1)*4 + 2*time.Second
	return context.WithTimeout(c.Context(), t)
}

// queryCtx bounds a database query.
func (s *Server) queryCtx(c fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Context(), s.Config.Query.Timeout.D())
}

// parseTime accepts RFC 3339, Unix seconds, "now", or a signed duration
// relative to now ("-24h", "-15m").
func parseTime(v string, now time.Time) (time.Time, error) {
	v = strings.TrimSpace(v)
	switch {
	case v == "" || v == "now":
		return now, nil
	case strings.HasPrefix(v, "-") || strings.HasPrefix(v, "+"):
		d, err := time.ParseDuration(v)
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(n, 0), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", v, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised time %q (use RFC 3339, Unix seconds, \"now\" or a relative duration like -24h)", v)
}

// timeRange parses from/to query parameters, defaulting to the given window
// ending now, and enforces query.max_range.
func (s *Server) timeRange(c fiber.Ctx, defaultWindow time.Duration) (time.Time, time.Time, error) {
	now := time.Now()
	to, err := parseTime(c.Query("to", "now"), now)
	if err != nil {
		return time.Time{}, time.Time{}, badRequest("to: %v", err)
	}
	fromStr := c.Query("from")
	from := to.Add(-defaultWindow)
	if fromStr != "" {
		if from, err = parseTime(fromStr, now); err != nil {
			return time.Time{}, time.Time{}, badRequest("from: %v", err)
		}
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, badRequest("from must be before to")
	}
	if max := s.Config.Query.MaxRange.D(); to.Sub(from) > max {
		return time.Time{}, time.Time{}, badRequest("time range exceeds query.max_range (%s)", max)
	}
	return from, to, nil
}

// deviceList parses a comma-separated "device" parameter.
func deviceList(c fiber.Ctx) ([]string, error) {
	raw := c.Query("device")
	if raw == "" {
		raw = c.Query("devices")
	}
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 100 {
		return nil, badRequest("at most 100 devices per query")
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !config.ValidID(p) {
			return nil, badRequest("invalid device id %q", p)
		}
		out = append(out, p)
	}
	return out, nil
}

// location parses a "tz" parameter, defaulting to def.
func location(c fiber.Ctx, def *time.Location) (*time.Location, error) {
	tz := c.Query("tz")
	if tz == "" {
		return def, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, badRequest("tz: %v", err)
	}
	return loc, nil
}

func intQuery(c fiber.Ctx, key string, def, lo, hi int) (int, error) {
	v := c.Query(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, badRequest("%s must be an integer between %d and %d", key, lo, hi)
	}
	return n, nil
}

// persisted reports whether inventory changes were written to disk, for
// inclusion in mutation responses.
func (s *Server) persisted() fiber.Map {
	m := fiber.Map{"persisted": s.Inventory.Persistent()}
	if !s.Inventory.Persistent() {
		m["warning"] = "no config file in use: inventory changes are kept in memory only and lost on restart"
	}
	return m
}
