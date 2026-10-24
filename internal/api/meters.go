package api

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/tecncr/nwm-hri-modbus-panel/internal/inventory"
	"github.com/tecncr/nwm-hri-modbus-panel/internal/store"
)

// meterView is the high-level, register-free view of a meter.
type meterView struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Bus         string             `json:"bus"`
	Slave       int                `json:"slave"`
	Enabled     bool               `json:"enabled"`
	Online      bool               `json:"online"`
	LastError   string             `json:"last_error,omitempty"`
	Model       string             `json:"model,omitempty"`
	Serial      string             `json:"serial_number,omitempty"`
	Tags        map[string]string  `json:"tags,omitempty"`
	Reading     *inventory.Reading `json:"reading,omitempty"`
	Recorded    *store.Measurement `json:"last_recorded,omitempty"`
}

func newMeterView(d *inventory.Device) meterView {
	st := d.State()
	return meterView{
		ID: d.Config.ID, Name: d.Config.Name, Description: d.Config.Description, Bus: d.Config.Bus,
		Slave: int(d.Config.Slave), Enabled: d.Config.IsEnabled(), Online: st.Online, LastError: st.LastError,
		Model: d.Config.Meter.Model, Serial: d.Config.Meter.SerialNumber, Tags: d.Config.Tags, Reading: st.Last,
	}
}

// listMeters returns every meter with its last known reading without
// touching the bus.
func (s *Server) listMeters(c fiber.Ctx) error {
	devs := s.Inventory.Devices()
	latest := map[string]store.Measurement{}
	if s.Store.Enabled() {
		ctx, cancel := s.queryCtx(c)
		defer cancel()
		if rows, err := s.Store.Latest(ctx, nil); err == nil {
			for _, r := range rows {
				latest[r.DeviceID] = r
			}
		}
	}
	out := make([]meterView, 0, len(devs))
	for _, d := range devs {
		v := newMeterView(d)
		if m, ok := latest[d.Config.ID]; ok {
			v.Recorded = &m
		}
		out = append(out, v)
	}
	return c.JSON(fiber.Map{"meters": out})
}

// getMeter reads a meter live. max_age (duration) accepts a cached reading up
// to that age; it defaults to modbus.live_cache_ttl and 0 forces a bus read.
func (s *Server) getMeter(c fiber.Ctx) error {
	d, err := s.Inventory.Device(c.Params("id"))
	if err != nil {
		return err
	}
	maxAge := s.Config.Modbus.LiveCacheTTL.D()
	if v := c.Query("max_age"); v != "" {
		if maxAge, err = time.ParseDuration(v); err != nil || maxAge < 0 {
			return badRequest("max_age must be a non-negative duration such as 0s, 5s or 1m")
		}
	}
	ctx, cancel := s.deviceCtx(c)
	defer cancel()
	r, err := s.Inventory.Read(ctx, d.Config.ID, maxAge, time.Time{})
	if err != nil {
		return err
	}
	v := newMeterView(d)
	v.Reading = r
	v.Online, v.LastError = true, ""
	return c.JSON(v)
}

// Consumption is the volume used by a meter over a period.
type Consumption struct {
	DeviceID      string     `json:"device_id"`
	Name          string     `json:"name,omitempty"`
	From          time.Time  `json:"from"`
	To            time.Time  `json:"to"`
	StartTime     *time.Time `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	StartLiters   *float64   `json:"start_total_liters,omitempty"`
	EndLiters     *float64   `json:"end_total_liters,omitempty"`
	ForwardLiters float64    `json:"forward_liters"`
	ReverseLiters float64    `json:"reverse_liters"`
	NetLiters     float64    `json:"net_liters"`
	NetM3         float64    `json:"net_m3"`
	Samples       bool       `json:"has_data"`
	CounterReset  bool       `json:"counter_reset"`
}

// consumption computes usage between the last measurement at or before from
// (or the first one after it) and the last one before to.
func (s *Server) consumption(ctx context.Context, id, name string, from, to time.Time) (Consumption, error) {
	out := Consumption{DeviceID: id, Name: name, From: from, To: to}
	start, err := s.Store.Before(ctx, id, from.Add(time.Second))
	if err != nil {
		return out, err
	}
	if start == nil {
		rows, _, err := s.Store.Query(ctx, store.Filter{Devices: []string{id}, From: from, To: to}, 0, 1, store.Asc)
		if err != nil {
			return out, err
		}
		if len(rows) == 0 {
			return out, nil
		}
		start = &rows[0]
	}
	end, err := s.Store.Before(ctx, id, to)
	if err != nil || end == nil || end.Time.Before(start.Time) {
		return out, err
	}
	out.Samples = true
	out.StartTime, out.EndTime = &start.Time, &end.Time
	out.StartLiters, out.EndLiters = &start.TotalLiters, &end.TotalLiters
	out.ForwardLiters = end.ForwardLiters - start.ForwardLiters
	out.ReverseLiters = end.ReverseLiters - start.ReverseLiters
	if out.ForwardLiters < 0 || out.ReverseLiters < 0 {
		out.CounterReset = true
		out.ForwardLiters, out.ReverseLiters = max(out.ForwardLiters, 0), max(out.ReverseLiters, 0)
	}
	out.NetLiters = out.ForwardLiters - out.ReverseLiters
	out.NetM3 = out.NetLiters / 1000
	return out, nil
}

func (s *Server) meterConsumption(c fiber.Ctx) error {
	d, err := s.Inventory.Device(c.Params("id"))
	if err != nil {
		return err
	}
	from, to, err := s.timeRange(c, 24*time.Hour)
	if err != nil {
		return err
	}
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	res, err := s.consumption(ctx, d.Config.ID, d.Config.Name, from, to)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// consumptionSummary returns consumption per device over a period.
func (s *Server) consumptionSummary(c fiber.Ctx) error {
	from, to, err := s.timeRange(c, 24*time.Hour)
	if err != nil {
		return err
	}
	ids, err := deviceList(c)
	if err != nil {
		return err
	}
	names := map[string]string{}
	for _, d := range s.Inventory.Devices() {
		names[d.Config.ID] = d.Config.Name
	}
	if len(ids) == 0 {
		ids = s.Inventory.DeviceIDs()
	}
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	out := make([]Consumption, 0, len(ids))
	var total float64
	for _, id := range ids {
		res, err := s.consumption(ctx, id, names[id], from, to)
		if err != nil {
			return err
		}
		total += res.NetLiters
		out = append(out, res)
	}
	return c.JSON(fiber.Map{"from": from, "to": to, "total_net_liters": total, "total_net_m3": total / 1000, "meters": out})
}

// SeriesPoint is the consumption of one device in one bucket.
type SeriesPoint struct {
	Start          time.Time `json:"start"`
	Samples        int64     `json:"samples"`
	TotalLiters    float64   `json:"total_liters"`
	ForwardLiters  float64   `json:"forward_liters"`
	ReverseLiters  float64   `json:"reverse_liters"`
	NetLiters      float64   `json:"net_liters"`
	AvgFlowLPM     float64   `json:"avg_flow_lpm"`
	CounterReset   bool      `json:"counter_reset,omitempty"`
	MinTotalLiters float64   `json:"min_total_liters"`
	MaxTotalLiters float64   `json:"max_total_liters"`
}

// series aggregates measurements and turns cumulative counters into
// per-bucket consumption. The first bucket is measured from the last reading
// before the range when one exists.
func (s *Server) series(ctx context.Context, ids []string, from, to time.Time, bucket time.Duration, loc *time.Location) (map[string][]SeriesPoint, error) {
	buckets, err := s.Store.Aggregate(ctx, store.Filter{Devices: ids, From: from, To: to}, bucket, loc)
	if err != nil {
		return nil, err
	}
	out := map[string][]SeriesPoint{}
	for _, id := range ids {
		out[id] = []SeriesPoint{}
	}
	prevF, prevR := map[string]float64{}, map[string]float64{}
	for _, b := range buckets {
		pf, okF := prevF[b.DeviceID]
		pr := prevR[b.DeviceID]
		if !okF {
			pf, pr = b.MinForward, b.MinReverse
			if prev, err := s.Store.Before(ctx, b.DeviceID, from); err == nil && prev != nil && prev.ForwardLiters <= b.MinForward {
				pf, pr = prev.ForwardLiters, prev.ReverseLiters
			}
		}
		p := SeriesPoint{
			Start: b.Start, Samples: b.Samples, TotalLiters: b.MaxTotal,
			ForwardLiters: b.MaxForward - pf, ReverseLiters: b.MaxReverse - pr,
			MinTotalLiters: b.MinTotal, MaxTotalLiters: b.MaxTotal,
		}
		if p.ForwardLiters < 0 || p.ReverseLiters < 0 {
			p.CounterReset = true
			p.ForwardLiters, p.ReverseLiters = b.MaxForward-b.MinForward, b.MaxReverse-b.MinReverse
		}
		p.NetLiters = p.ForwardLiters - p.ReverseLiters
		p.AvgFlowLPM = p.ForwardLiters / bucket.Minutes()
		prevF[b.DeviceID], prevR[b.DeviceID] = b.MaxForward, b.MaxReverse
		out[b.DeviceID] = append(out[b.DeviceID], p)
	}
	return out, nil
}

// bucketParams parses range and bucket size, enforcing query.max_buckets.
func (s *Server) bucketParams(c fiber.Ctx) (time.Time, time.Time, time.Duration, *time.Location, error) {
	from, to, err := s.timeRange(c, 24*time.Hour)
	if err != nil {
		return from, to, 0, nil, err
	}
	bucket := time.Hour
	if v := c.Query("bucket"); v != "" {
		if bucket, err = time.ParseDuration(v); err != nil {
			return from, to, 0, nil, badRequest("bucket must be a duration such as 15m, 1h or 24h")
		}
	}
	if bucket < time.Minute || bucket%time.Second != 0 {
		return from, to, 0, nil, badRequest("bucket must be at least 1m and a whole number of seconds")
	}
	if n := int(to.Sub(from) / bucket); n > s.Config.Query.MaxBuckets {
		return from, to, 0, nil, badRequest("range/bucket yields %d buckets; the limit is %d (query.max_buckets)", n, s.Config.Query.MaxBuckets)
	}
	loc, err := location(c, time.Local)
	return from, to, bucket, loc, err
}

func (s *Server) meterSeries(c fiber.Ctx) error {
	d, err := s.Inventory.Device(c.Params("id"))
	if err != nil {
		return err
	}
	from, to, bucket, loc, err := s.bucketParams(c)
	if err != nil {
		return err
	}
	ctx, cancel := s.queryCtx(c)
	defer cancel()
	res, err := s.series(ctx, []string{d.Config.ID}, from, to, bucket, loc)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"device_id": d.Config.ID, "from": from, "to": to, "bucket": bucket.String(), "timezone": loc.String(), "points": res[d.Config.ID]})
}
