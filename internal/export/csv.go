// Package export writes recorded measurements as CSV, both on demand and as
// periodic files in a directory.
package export

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// Format controls CSV rendering.
type Format struct {
	Columns    []string
	Delimiter  rune
	Header     bool
	Location   *time.Location
	TimeFormat string
	// Names maps device IDs to display names.
	Names map[string]string
}

// Writer streams measurements as CSV.
type Writer struct {
	f   Format
	csv *csv.Writer
	n   int
}

// NewWriter creates a CSV writer and emits the header if requested.
func NewWriter(w io.Writer, f Format) (*Writer, error) {
	cw := csv.NewWriter(w)
	if f.Delimiter != 0 {
		cw.Comma = f.Delimiter
	}
	if f.Location == nil {
		f.Location = time.UTC
	}
	if f.TimeFormat == "" {
		f.TimeFormat = time.RFC3339
	}
	wr := &Writer{f: f, csv: cw}
	if f.Header {
		if err := cw.Write(f.Columns); err != nil {
			return nil, err
		}
	}
	return wr, nil
}

// Write appends one measurement.
func (w *Writer) Write(m *store.Measurement) error {
	rec := make([]string, len(w.f.Columns))
	for i, c := range w.f.Columns {
		rec[i] = w.value(c, m)
	}
	w.n++
	return w.csv.Write(rec)
}

// Rows returns the number of data rows written.
func (w *Writer) Rows() int { return w.n }

// Flush flushes buffered output and reports any write error.
func (w *Writer) Flush() error {
	w.csv.Flush()
	return w.csv.Error()
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func (w *Writer) value(col string, m *store.Measurement) string {
	switch col {
	case "time":
		return m.Time.In(w.f.Location).Format(w.f.TimeFormat)
	case "unix":
		return strconv.FormatInt(m.Time.Unix(), 10)
	case "device_id":
		return m.DeviceID
	case "device_name":
		return safeText(w.f.Names[m.DeviceID])
	case "bus":
		return m.Bus
	case "slave":
		return strconv.Itoa(m.Slave)
	case "total_raw":
		return strconv.FormatInt(m.TotalRaw, 10)
	case "forward_raw":
		return strconv.FormatInt(m.ForwardRaw, 10)
	case "reverse_raw":
		return strconv.FormatInt(m.ReverseRaw, 10)
	case "liters_per_unit":
		return num(m.LitersPerUnit)
	case "total_liters":
		return num(m.TotalLiters)
	case "forward_liters":
		return num(m.ForwardLiters)
	case "reverse_liters":
		return num(m.ReverseLiters)
	case "total_m3":
		return num(m.TotalLiters / 1000)
	case "forward_m3":
		return num(m.ForwardLiters / 1000)
	case "reverse_m3":
		return num(m.ReverseLiters / 1000)
	case "battery_status":
		return strconv.Itoa(m.BatteryStatus)
	case "failure":
		return strconv.Itoa(m.Failure)
	case "base_pulse_liters":
		return strconv.Itoa(m.BasePulseLiters)
	case "output_pulse_liters":
		return strconv.Itoa(m.OutputPulseLiters)
	case "device_clock":
		if m.DeviceClock == nil {
			return ""
		}
		return m.DeviceClock.In(w.f.Location).Format(w.f.TimeFormat)
	}
	return ""
}

// safeText neutralises values that spreadsheet applications would evaluate
// as formulas (CSV injection).
func safeText(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
