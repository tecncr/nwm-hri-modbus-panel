package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

// Duration is a time.Duration that encodes as a Go duration string ("1m30s")
// in YAML and JSON, and accepts either a string or nanoseconds when decoding.
type Duration time.Duration

// D returns the value as a time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

func (d Duration) String() string { return time.Duration(d).String() }

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		*d = Duration(v)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("duration must be a string like \"1s\" or nanoseconds")
	}
	*d = Duration(n)
	return nil
}

// durationHook decodes strings into Duration for mapstructure.
func durationHook() mapstructure.DecodeHookFuncType {
	target := reflect.TypeFor[Duration]()
	return func(from, to reflect.Type, data any) (any, error) {
		if to != target {
			return data, nil
		}
		switch v := data.(type) {
		case string:
			d, err := time.ParseDuration(v)
			return Duration(d), err
		case time.Duration:
			return Duration(v), nil
		}
		return data, nil
	}
}
