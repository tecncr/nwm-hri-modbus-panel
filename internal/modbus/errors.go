package modbus

import (
	"errors"
	"fmt"
)

// Function codes supported by this master.
const (
	FuncReadHoldingRegisters   byte = 0x03
	FuncWriteSingleRegister    byte = 0x06
	FuncWriteMultipleRegisters byte = 0x10
)

// Protocol limits.
const (
	BroadcastAddress   = 0
	MinSlaveAddress    = 1
	MaxSlaveAddress    = 247
	MaxReadQuantity    = 125
	MaxWriteQuantity   = 123
	maxRTUFrameLength  = 256
	exceptionFlag      = 0x80
	exceptionFrameSize = 5
)

var (
	// ErrTimeout is returned when the slave does not answer in time.
	ErrTimeout = errors.New("modbus: response timeout")
	// ErrCRC is returned when a response fails CRC validation.
	ErrCRC = errors.New("modbus: CRC mismatch")
	// ErrInvalidResponse is returned for malformed or mismatched responses.
	ErrInvalidResponse = errors.New("modbus: invalid response")
	// ErrInvalidRequest is returned when request parameters are out of range.
	ErrInvalidRequest = errors.New("modbus: invalid request")
	// ErrClosed is returned when the client has been closed.
	ErrClosed = errors.New("modbus: client closed")
)

// ExceptionError is returned when a slave answers with an exception response.
type ExceptionError struct {
	Function byte
	Code     byte
}

func (e *ExceptionError) Error() string {
	return fmt.Sprintf("modbus: exception 0x%02X (%s) for function 0x%02X", e.Code, ExceptionName(e.Code), e.Function)
}

// ExceptionName returns the standard name of a Modbus exception code.
func ExceptionName(code byte) string {
	switch code {
	case 0x01:
		return "illegal function"
	case 0x02:
		return "illegal data address"
	case 0x03:
		return "illegal data value"
	case 0x04:
		return "slave device failure"
	case 0x05:
		return "acknowledge"
	case 0x06:
		return "slave device busy"
	case 0x08:
		return "memory parity error"
	case 0x0A:
		return "gateway path unavailable"
	case 0x0B:
		return "gateway target device failed to respond"
	default:
		return "unknown"
	}
}
