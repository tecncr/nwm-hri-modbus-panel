package modbus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	FunctionReadHoldingRegisters byte = 0x03
	FunctionWriteSingleRegister  byte = 0x06
	FunctionWriteMultiple        byte = 0x10
	BroadcastAddress             byte = 0
	MaxRegisters                      = 125
	MaxWriteRegisters                 = 123
)

var (
	ErrInvalidCRC      = errors.New("invalid Modbus CRC")
	ErrInvalidResponse = errors.New("invalid Modbus response")
)

// ExchangeLink performs one half-duplex transaction. expected is zero for a
// broadcast, otherwise it is the exact RTU response length.
type ExchangeLink interface {
	Exchange(context.Context, []byte, int) ([]byte, error)
}

type Client struct {
	link ExchangeLink
}

func NewClient(link ExchangeLink) *Client { return &Client{link: link} }

func (c *Client) ReadRegisters(ctx context.Context, slave byte, address, count uint16) ([]uint16, error) {
	if slave == BroadcastAddress || slave > 247 {
		return nil, fmt.Errorf("read requires a slave address from 1 to 247")
	}
	if count == 0 || count > MaxRegisters || uint32(address)+uint32(count) > 0x10000 {
		return nil, fmt.Errorf("read count must be 1..%d and stay within the register address range", MaxRegisters)
	}
	request := []byte{slave, FunctionReadHoldingRegisters, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(request[2:4], address)
	binary.BigEndian.PutUint16(request[4:6], count)
	request = AppendCRC(request)
	response, err := c.link.Exchange(ctx, request, 5+int(count)*2)
	if err != nil {
		return nil, fmt.Errorf("read slave 0x%02X at register 0x%04X: %w", slave, address, err)
	}
	if err := validateResponse(response, slave, FunctionReadHoldingRegisters); err != nil {
		return nil, err
	}
	if len(response) != 5+int(count)*2 || response[2] != byte(count*2) {
		return nil, fmt.Errorf("%w: unexpected read payload length", ErrInvalidResponse)
	}
	values := make([]uint16, count)
	for i := range values {
		values[i] = binary.BigEndian.Uint16(response[3+i*2 : 5+i*2])
	}
	return values, nil
}

func (c *Client) WriteSingle(ctx context.Context, slave byte, address, value uint16) error {
	if slave > 247 {
		return fmt.Errorf("slave address must be 0..247")
	}
	request := []byte{slave, FunctionWriteSingleRegister, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(request[2:4], address)
	binary.BigEndian.PutUint16(request[4:6], value)
	request = AppendCRC(request)
	return c.write(ctx, request, slave, 6)
}

func (c *Client) WriteMultiple(ctx context.Context, slave byte, address uint16, values []uint16) error {
	if slave > 247 {
		return fmt.Errorf("slave address must be 0..247")
	}
	if len(values) == 0 || len(values) > MaxWriteRegisters || uint32(address)+uint32(len(values)) > 0x10000 {
		return fmt.Errorf("write count must be 1..%d and stay within the register address range", MaxWriteRegisters)
	}
	request := make([]byte, 7+len(values)*2)
	request[0], request[1] = slave, FunctionWriteMultiple
	binary.BigEndian.PutUint16(request[2:4], address)
	binary.BigEndian.PutUint16(request[4:6], uint16(len(values)))
	request[6] = byte(len(values) * 2)
	for i, value := range values {
		binary.BigEndian.PutUint16(request[7+i*2:9+i*2], value)
	}
	request = AppendCRC(request)
	return c.write(ctx, request, slave, 0x10)
}

func (c *Client) write(ctx context.Context, request []byte, slave byte, function byte) error {
	expected := 8
	if slave == BroadcastAddress {
		expected = 0
	}
	response, err := c.link.Exchange(ctx, request, expected)
	if err != nil || expected == 0 {
		return err
	}
	if err := validateResponse(response, slave, function); err != nil {
		return err
	}
	if len(response) != 8 || string(response[:6]) != string(request[:6]) {
		return fmt.Errorf("%w: write acknowledgement does not match request", ErrInvalidResponse)
	}
	return nil
}

func validateResponse(frame []byte, slave, function byte) error {
	if len(frame) < 5 {
		return fmt.Errorf("%w: response too short", ErrInvalidResponse)
	}
	if !ValidCRC(frame) {
		return ErrInvalidCRC
	}
	if frame[0] != slave {
		return fmt.Errorf("%w: got slave 0x%02X, expected 0x%02X", ErrInvalidResponse, frame[0], slave)
	}
	if frame[1] == function|0x80 {
		return fmt.Errorf("Modbus exception from slave: code 0x%02X", frame[2])
	}
	if frame[1] != function {
		return fmt.Errorf("%w: got function 0x%02X, expected 0x%02X", ErrInvalidResponse, frame[1], function)
	}
	return nil
}

// CRC16 calculates the Modbus RTU CRC-16 (poly 0xA001, initial 0xFFFF).
func CRC16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, value := range data {
		crc ^= uint16(value)
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func AppendCRC(frame []byte) []byte {
	crc := CRC16(frame)
	return append(frame, byte(crc), byte(crc>>8))
}

func ValidCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	crc := CRC16(frame[:len(frame)-2])
	return frame[len(frame)-2] == byte(crc) && frame[len(frame)-1] == byte(crc>>8)
}
