package modbus

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestCRC16KnownFrames(t *testing.T) {
	frames := []string{
		"18 03 00 00 00 01 86 03",
		"18 03 02 00 18 A5 8C",
		"18 06 00 00 00 17 CB CD",
		"17 06 00 0D 00 05 DA FC",
		"17 10 00 10 00 02 04 00 01 00 01 28 EB",
		"17 10 00 10 00 02 42 FB",
		"17 10 00 12 00 04 08 87 76 65 54 43 32 21 10 D3 E3",
		"17 03 00 12 00 04 E6 FA",
		"17 03 08 87 76 65 54 43 32 21 10 14 30",
		"17 03 00 01 00 02 97 3D",
		"17 03 04 44 44 44 44 EA 24",
		"17 03 00 03 00 04 B6 FF",
		"17 03 00 07 00 03 B6 FC",
		"17 03 06 11 05 0B 08 11 2A 06 3D",
		"17 03 00 0A 00 01 A6 FE",
		"17 03 02 00 01 30 47",
		"17 03 00 0B 00 02 B7 3F",
		"17 03 04 14 12 08 01 EE 07",
		"17 03 00 19 00 03 D6 FA",
		"17 03 06 00 00 00 00 00 00 C7 15",
	}
	for _, f := range frames {
		b, err := hex.DecodeString(strings.ReplaceAll(f, " ", ""))
		if err != nil {
			t.Fatal(err)
		}
		if !checkCRC(b) {
			crc := CRC16(b[:len(b)-2])
			t.Errorf("%s: CRC mismatch, computed %02X %02X", f, byte(crc), byte(crc>>8))
		}
	}
}

func TestFrameDelayFor(t *testing.T) {
	if d := FrameDelayFor(9600); d < 4*1e6 || d > 4.1*1e6 {
		t.Errorf("9600 baud: got %v", d)
	}
	if d := FrameDelayFor(115200); d != 1750*1e3 {
		t.Errorf("115200 baud: got %v", d)
	}
}
