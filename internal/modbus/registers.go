package modbus

type Register struct {
	Address uint16 `json:"address"`
	Name    string `json:"name"`
	Access  string `json:"access"`
	Group   string `json:"group"`
	Note    string `json:"note,omitempty"`
}

var RegisterMap = []Register{
	{0x0000, "Communication address", "read/write", "Identity", ""},
	{0x0001, "Total flow · high 16 bits", "read only", "Flow", ""},
	{0x0002, "Total flow · low 16 bits", "read only", "Flow", ""},
	{0x0003, "Forward flow · high 16 bits", "read only", "Flow", ""},
	{0x0004, "Forward flow · low 16 bits", "read only", "Flow", ""},
	{0x0005, "Reverse flow · high 16 bits", "read only", "Flow", ""},
	{0x0006, "Reverse flow · low 16 bits", "read only", "Flow", ""},
	{0x0007, "Clock · year / month", "read/write", "Clock", ""},
	{0x0008, "Clock · day / hour", "read/write", "Clock", ""},
	{0x0009, "Clock · minute / second", "read/write", "Clock", ""},
	{0x000A, "Meter type ID", "read/write", "Identity", ""},
	{0x000B, "Firmware year", "read only", "Diagnostics", ""},
	{0x000C, "Firmware month / day", "read only", "Diagnostics", ""},
	{0x000D, "Reverse-flow threshold", "read/write", "Configuration", ""},
	{0x000E, "Battery status", "read only", "Diagnostics", ""},
	{0x000F, "Failure code", "read only", "Diagnostics", ""},
	{0x0010, "Base meter pulse equivalent", "read/write", "Configuration", "1..10000 L/pulse"},
	{0x0011, "Output pulse equivalent", "read/write", "Configuration", "1..1000 L/pulse; must exceed base equivalent"},
	{0x0012, "Forward baseline · high 16 bits", "read/write", "Baselines", ""},
	{0x0013, "Forward baseline · low 16 bits", "read/write", "Baselines", ""},
	{0x0014, "Reverse baseline · high 16 bits", "read/write", "Baselines", ""},
	{0x0015, "Reverse baseline · low 16 bits", "read/write", "Baselines", ""},
	{0x0019, "Debug numbers 1–2", "read only", "Diagnostics", ""},
	{0x001A, "Debug numbers 3–4", "read only", "Diagnostics", ""},
	{0x001B, "Debug numbers 5–6", "read only", "Diagnostics", ""},
}
