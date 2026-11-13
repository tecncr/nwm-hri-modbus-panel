package hri

import (
	"fmt"
	"time"
)

// Clock is the encoder's real-time clock. The encoder has no notion of time
// zone; values are interpreted in the location configured for the device.
type Clock struct {
	Year   int  `json:"year"`
	Month  int  `json:"month"`
	Day    int  `json:"day"`
	Hour   int  `json:"hour"`
	Minute int  `json:"minute"`
	Second int  `json:"second"`
	Valid  bool `json:"valid"`
}

// DecodeClock decodes the three clock registers.
func DecodeClock(yearMonth, dayHour, minuteSecond uint16) Clock {
	c := Clock{
		Year:   ClockYearBase + int(yearMonth>>8),
		Month:  int(yearMonth & 0xFF),
		Day:    int(dayHour >> 8),
		Hour:   int(dayHour & 0xFF),
		Minute: int(minuteSecond >> 8),
		Second: int(minuteSecond & 0xFF),
	}
	c.Valid = c.Month >= 1 && c.Month <= 12 && c.Day >= 1 && c.Day <= 31 &&
		c.Hour <= 23 && c.Minute <= 59 && c.Second <= 59
	return c
}

// EncodeClock encodes t (already converted to the device location) into the
// three clock registers.
func EncodeClock(t time.Time) ([3]uint16, error) {
	y := t.Year() - ClockYearBase
	if y < 0 || y > 255 {
		return [3]uint16{}, fmt.Errorf("year %d cannot be represented (2000-2255)", t.Year())
	}
	return [3]uint16{
		uint16(y)<<8 | uint16(t.Month()),
		uint16(t.Day())<<8 | uint16(t.Hour()),
		uint16(t.Minute())<<8 | uint16(t.Second()),
	}, nil
}

// Time converts the clock to a time.Time in loc. ok is false when the clock
// holds an invalid date.
func (c Clock) Time(loc *time.Location) (time.Time, bool) {
	if !c.Valid {
		return time.Time{}, false
	}
	t := time.Date(c.Year, time.Month(c.Month), c.Day, c.Hour, c.Minute, c.Second, 0, loc)
	// Reject dates normalised by time.Date (e.g. Feb 31st).
	if t.Day() != c.Day || int(t.Month()) != c.Month {
		return time.Time{}, false
	}
	return t, true
}

// Firmware is the decoded software version date.
type Firmware struct {
	Year  int    `json:"year"`
	Month int    `json:"month"`
	Day   int    `json:"day"`
	Text  string `json:"text"`
}

// DecodeFirmware decodes the two software version registers.
func DecodeFirmware(year, monthDay uint16) Firmware {
	f := Firmware{
		Year:  int(year>>8)*100 + int(year&0xFF),
		Month: int(monthDay >> 8),
		Day:   int(monthDay & 0xFF),
	}
	f.Text = fmt.Sprintf("%04d-%02d-%02d", f.Year, f.Month, f.Day)
	return f
}

// CounterLowModulus is the rollover of the low word of the encoder's two-word
// counters. Contrary to the protocol document, which describes plain 32-bit
// big-endian values, the firmware keeps the low word in 0-9999 and carries
// into the high word every 10000 units: value = high × 10000 + low. Joining
// the words as a binary uint32 makes the counter jump by 55536 (65536 − 10000)
// at every carry.
const CounterLowModulus = 10000

// MaxCounter is the largest value a two-word counter can hold.
const MaxCounter = 0xFFFF*CounterLowModulus + CounterLowModulus - 1

// JoinCounter decodes a two-word counter, high word first.
func JoinCounter(hi, lo uint16) uint32 {
	return uint32(hi)*CounterLowModulus + uint32(lo)
}

// SplitCounter encodes v as a two-word counter, high word first. ok is false
// when v exceeds MaxCounter.
func SplitCounter(v uint32) (hi, lo uint16, ok bool) {
	if v > MaxCounter {
		return 0, 0, false
	}
	return uint16(v / CounterLowModulus), uint16(v % CounterLowModulus), true
}

// ValidatePulseEquivalents checks base/output pulse equivalents against the
// encoder's documented limits.
func ValidatePulseEquivalents(base, output int) error {
	if base < MinBasePulseLiters || base > MaxBasePulseLiters {
		return fmt.Errorf("base pulse equivalent %d out of range %d-%d L/pulse", base, MinBasePulseLiters, MaxBasePulseLiters)
	}
	if output < MinOutputPulseLiters || output > MaxOutputPulseLiters {
		return fmt.Errorf("output pulse equivalent %d out of range %d-%d L/unit", output, MinOutputPulseLiters, MaxOutputPulseLiters)
	}
	if output < base {
		return fmt.Errorf("output pulse equivalent (%d) must not be lower than the base pulse equivalent (%d)", output, base)
	}
	return nil
}
