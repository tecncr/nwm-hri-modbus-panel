// Package modbus implements a minimal Modbus RTU master supporting the
// function codes used by NWM-HRI encoders: 0x03 (read holding registers),
// 0x06 (write single register) and 0x10 (write multiple registers).
package modbus

// crcTable is the precomputed table for the Modbus CRC-16 (poly 0xA001,
// reflected 0x8005, init 0xFFFF).
var crcTable = func() [256]uint16 {
	var t [256]uint16
	for i := range t {
		crc := uint16(i)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
	return t
}()

// CRC16 computes the Modbus RTU CRC of b. The result is transmitted low byte
// first.
func CRC16(b []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, v := range b {
		crc = (crc >> 8) ^ crcTable[byte(crc)^v]
	}
	return crc
}

// appendCRC appends the CRC of b to b, low byte first.
func appendCRC(b []byte) []byte {
	crc := CRC16(b)
	return append(b, byte(crc), byte(crc>>8))
}

// checkCRC reports whether the last two bytes of frame are a valid CRC of the
// rest of the frame.
func checkCRC(frame []byte) bool {
	if len(frame) < 4 {
		return false
	}
	n := len(frame) - 2
	crc := CRC16(frame[:n])
	return frame[n] == byte(crc) && frame[n+1] == byte(crc>>8)
}
