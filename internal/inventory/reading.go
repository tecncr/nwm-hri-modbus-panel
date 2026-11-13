package inventory

import (
	"fmt"
	"time"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/config"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/hri"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// Conversion sources for LitersPerUnit.
const (
	SourceConfig  = "config"
	SourceEncoder = "encoder"
	SourceDefault = "default"
)

// Reading is a measurement expressed in physical units.
type Reading struct {
	DeviceID            string       `json:"device_id"`
	Name                string       `json:"name"`
	Bus                 string       `json:"bus"`
	Slave               int          `json:"slave"`
	Time                time.Time    `json:"time"`
	Raw                 hri.Counters `json:"raw"`
	LitersPerUnit       float64      `json:"liters_per_unit"`
	LitersPerUnitSource string       `json:"liters_per_unit_source"`
	TotalLiters         float64      `json:"total_liters"`
	ForwardLiters       float64      `json:"forward_liters"`
	ReverseLiters       float64      `json:"reverse_liters"`
	TotalM3             float64      `json:"total_m3"`
	ForwardM3           float64      `json:"forward_m3"`
	ReverseM3           float64      `json:"reverse_m3"`
	BatteryStatus       int          `json:"battery_status"`
	Failure             int          `json:"failure"`
	BasePulseLiters     int          `json:"base_pulse_liters"`
	OutputPulseLiters   int          `json:"output_pulse_liters"`
	DeviceClock         *time.Time   `json:"device_clock,omitempty"`
	ClockDriftSeconds   *float64     `json:"clock_drift_seconds,omitempty"`
	Warnings            []string     `json:"warnings,omitempty"`
}

// Convert turns raw encoder status into a Reading using the meter settings.
//
// Liters per counter unit come from meter.liters_per_unit when set, otherwise
// from the encoder's output pulse equivalent register. offset_liters is added
// to the net and forward volumes.
func Convert(dev config.DeviceConfig, loc *time.Location, st *hri.Status, at time.Time) Reading {
	r := Reading{
		DeviceID: dev.ID, Name: dev.Name, Bus: dev.Bus, Slave: int(dev.Slave), Time: at,
		Raw: st.Counters, BatteryStatus: int(st.BatteryStatus), Failure: int(st.Failure),
		BasePulseLiters: int(st.BasePulseLiters), OutputPulseLiters: int(st.OutputPulseLiters),
	}
	m := dev.Meter
	switch {
	case m.LitersPerUnit > 0:
		r.LitersPerUnit, r.LitersPerUnitSource = m.LitersPerUnit, SourceConfig
	case st.OutputPulseLiters > 0:
		r.LitersPerUnit, r.LitersPerUnitSource = float64(st.OutputPulseLiters), SourceEncoder
	default:
		r.LitersPerUnit, r.LitersPerUnitSource = 1, SourceDefault
		r.Warnings = append(r.Warnings, "encoder reports an output pulse equivalent of 0; assuming 1 L/unit")
	}
	// The encoder computes the net counter as forward minus reverse, and a
	// negative result wraps the high word; recover the sign in that case.
	net := int64(st.Total)
	if st.Total > st.Forward {
		net = int64(st.Forward) - int64(st.Reverse)
	}
	r.TotalLiters = float64(net)*r.LitersPerUnit + m.OffsetLiters
	r.ForwardLiters = float64(st.Forward)*r.LitersPerUnit + m.OffsetLiters
	r.ReverseLiters = float64(st.Reverse) * r.LitersPerUnit
	r.TotalM3, r.ForwardM3, r.ReverseM3 = r.TotalLiters/1000, r.ForwardLiters/1000, r.ReverseLiters/1000

	if m.BaseLitersPerPulse > 0 && int(st.BasePulseLiters) != m.BaseLitersPerPulse {
		r.Warnings = append(r.Warnings, fmt.Sprintf("encoder base pulse equivalent is %d L/pulse, expected %d", st.BasePulseLiters, m.BaseLitersPerPulse))
	}
	if m.OutputLitersPerUnit > 0 && int(st.OutputPulseLiters) != m.OutputLitersPerUnit {
		r.Warnings = append(r.Warnings, fmt.Sprintf("encoder output pulse equivalent is %d L/unit, expected %d", st.OutputPulseLiters, m.OutputLitersPerUnit))
	}
	if t, ok := st.Clock.Time(loc); ok {
		r.DeviceClock = &t
		drift := t.Sub(time.Now()).Seconds()
		r.ClockDriftSeconds = &drift
	}
	if st.Failure != 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("encoder reports failure code %d", st.Failure))
	}
	return r
}

// Measurement converts a Reading into a database row.
func (r Reading) Measurement() store.Measurement {
	return store.Measurement{
		Time: r.Time, DeviceID: r.DeviceID, Bus: r.Bus, Slave: r.Slave,
		TotalRaw: int64(r.Raw.Total), ForwardRaw: int64(r.Raw.Forward), ReverseRaw: int64(r.Raw.Reverse),
		LitersPerUnit: r.LitersPerUnit, TotalLiters: r.TotalLiters, ForwardLiters: r.ForwardLiters,
		ReverseLiters: r.ReverseLiters, BatteryStatus: r.BatteryStatus, Failure: r.Failure,
		BasePulseLiters: r.BasePulseLiters, OutputPulseLiters: r.OutputPulseLiters,
		DeviceClock: r.DeviceClock,
	}
}
