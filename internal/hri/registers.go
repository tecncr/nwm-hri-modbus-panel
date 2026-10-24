// Package hri implements the register map and typed operations of NWM-HRI
// pulse encoders on top of the Modbus RTU master.
package hri

// Register addresses.
const (
	RegSlaveAddress          uint16 = 0x0000
	RegTotalFlow             uint16 = 0x0001 // 2 words, high word first
	RegForwardFlow           uint16 = 0x0003 // 2 words, high word first
	RegReverseFlow           uint16 = 0x0005 // 2 words, high word first
	RegClockYearMonth        uint16 = 0x0007
	RegClockDayHour          uint16 = 0x0008
	RegClockMinuteSecond     uint16 = 0x0009
	RegDeviceType            uint16 = 0x000A
	RegFirmwareYear          uint16 = 0x000B
	RegFirmwareMonthDay      uint16 = 0x000C
	RegReverseFlowThreshold  uint16 = 0x000D
	RegBatteryStatus         uint16 = 0x000E
	RegFailure               uint16 = 0x000F
	RegBasePulseEquivalent   uint16 = 0x0010
	RegOutputPulseEquivalent uint16 = 0x0011
	RegForwardBaseReading    uint16 = 0x0012 // 2 words, high word first
	RegReverseBaseReading    uint16 = 0x0014 // 2 words, high word first
	RegDebugNumbers          uint16 = 0x0019 // 3 words, 6 bytes
)

// Value limits enforced by the encoder firmware.
const (
	MinBasePulseLiters   = 1
	MaxBasePulseLiters   = 10000
	MinOutputPulseLiters = 1
	MaxOutputPulseLiters = 1000
	ClockYearBase        = 2000
)

// Contiguous blocks used for efficient reads.
const (
	mainBlockStart uint16 = RegSlaveAddress
	mainBlockLen   uint16 = 0x0016 // 0x0000..0x0015
	debugBlockLen  uint16 = 3
)

// Access describes whether a register can be written.
type Access string

const (
	ReadOnly  Access = "ro"
	ReadWrite Access = "rw"
)

// Register describes one logical register (possibly spanning several words).
type Register struct {
	Key         string `json:"key"`
	Address     uint16 `json:"address"`
	Words       int    `json:"words"`
	Access      Access `json:"access"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Registers is the complete register map, in address order.
var Registers = []Register{
	{"slave_address", RegSlaveAddress, 1, ReadWrite, "Slave address",
		"Modbus address of the encoder (1-247). Address 0 is the broadcast address."},
	{"total_flow", RegTotalFlow, 2, ReadOnly, "Accumulated flow",
		"Net accumulated flow counter (forward minus reverse), in output pulse units. 32-bit, high word first."},
	{"forward_flow", RegForwardFlow, 2, ReadOnly, "Forward flow",
		"Forward (positive direction) flow counter, in output pulse units. 32-bit, high word first."},
	{"reverse_flow", RegReverseFlow, 2, ReadOnly, "Reverse flow",
		"Reverse (backflow) counter, in output pulse units. 32-bit, high word first."},
	{"clock_year_month", RegClockYearMonth, 1, ReadWrite, "Clock: year / month",
		"Real-time clock. High byte: year since 2000; low byte: month (1-12)."},
	{"clock_day_hour", RegClockDayHour, 1, ReadWrite, "Clock: day / hour",
		"Real-time clock. High byte: day of month (1-31); low byte: hour (0-23)."},
	{"clock_minute_second", RegClockMinuteSecond, 1, ReadWrite, "Clock: minute / second",
		"Real-time clock. High byte: minute (0-59); low byte: second (0-59)."},
	{"device_type", RegDeviceType, 1, ReadWrite, "Device type ID",
		"Slave type code / device type identifier."},
	{"firmware_year", RegFirmwareYear, 1, ReadOnly, "Firmware year",
		"Software version year. High byte: century digits; low byte: year digits (e.g. 0x1412 = 2018)."},
	{"firmware_month_day", RegFirmwareMonthDay, 1, ReadOnly, "Firmware month / day",
		"Software version date. High byte: month; low byte: day."},
	{"reverse_flow_threshold", RegReverseFlowThreshold, 1, ReadWrite, "Reverse flow threshold",
		"Threshold used by the encoder to detect reverse (backflow) conditions."},
	{"battery_status", RegBatteryStatus, 1, ReadOnly, "Battery status",
		"Battery status code reported by the encoder."},
	{"failure", RegFailure, 1, ReadOnly, "Failure",
		"Failure code reported by the encoder."},
	{"base_pulse_equivalent", RegBasePulseEquivalent, 1, ReadWrite, "Base pulse equivalent",
		"Liters per pulse emitted by the physical meter's sender (1-10000 L/pulse)."},
	{"output_pulse_equivalent", RegOutputPulseEquivalent, 1, ReadWrite, "Output pulse equivalent",
		"Resolution of the communicated counters in liters per unit (1-1000 L/unit). Must not be lower than the base pulse equivalent."},
	{"forward_base_reading", RegForwardBaseReading, 2, ReadWrite, "Forward base reading",
		"Initial value of the forward counter, in output pulse units. Writing it presets the forward counter. 32-bit, high word first."},
	{"reverse_base_reading", RegReverseBaseReading, 2, ReadWrite, "Reverse base reading",
		"Initial value of the reverse counter, in output pulse units. Writing it presets the reverse counter. 32-bit, high word first."},
	{"debug_numbers", RegDebugNumbers, 3, ReadOnly, "Debug numbers",
		"Six diagnostic bytes (debugNum1-debugNum6), two per register."},
}

// RegisterByKey looks up a register definition by key.
func RegisterByKey(key string) (Register, bool) {
	for _, r := range Registers {
		if r.Key == key {
			return r, true
		}
	}
	return Register{}, false
}

// RegisterAt returns the logical register covering word address addr.
func RegisterAt(addr uint16) (Register, bool) {
	for _, r := range Registers {
		if addr >= r.Address && addr < r.Address+uint16(r.Words) {
			return r, true
		}
	}
	return Register{}, false
}
