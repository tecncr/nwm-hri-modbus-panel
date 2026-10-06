package modbus

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"go.bug.st/serial"
)

type Transaction struct {
	At       time.Time `json:"at"`
	Request  string    `json:"request"`
	Response string    `json:"response,omitempty"`
	Duration int64     `json:"durationMs"`
	Error    string    `json:"error,omitempty"`
}

type SerialLink struct {
	PortName string
	BaudRate int
	Timeout  time.Duration
	mu       sync.Mutex
	logMu    sync.RWMutex
	log      []Transaction
}

func (s *SerialLink) Exchange(ctx context.Context, request []byte, expected int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	started := time.Now()
	entry := Transaction{At: started, Request: hex.EncodeToString(request)}
	defer func() {
		entry.Duration = time.Since(started).Milliseconds()
		s.logMu.Lock()
		s.log = append(s.log, entry)
		if len(s.log) > 100 {
			s.log = append([]Transaction(nil), s.log[len(s.log)-100:]...)
		}
		s.logMu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		entry.Error = err.Error()
		return nil, err
	}
	port, err := serial.Open(s.PortName, &serial.Mode{
		BaudRate: s.BaudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		entry.Error = err.Error()
		return nil, fmt.Errorf("open serial port %q: %w", s.PortName, err)
	}
	defer port.Close()
	// Drop bytes left behind by late replies before sending the next RTU frame.
	if err := port.ResetInputBuffer(); err != nil {
		entry.Error = err.Error()
		return nil, fmt.Errorf("clear stale serial input: %w", err)
	}
	readTimeout := s.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			entry.Error = context.DeadlineExceeded.Error()
			return nil, context.DeadlineExceeded
		}
		if remaining < readTimeout {
			readTimeout = remaining
		}
	}
	if err := port.SetReadTimeout(readTimeout); err != nil {
		entry.Error = err.Error()
		return nil, fmt.Errorf("set serial read timeout: %w", err)
	}
	n, err := port.Write(request)
	if err != nil {
		entry.Error = err.Error()
		return nil, fmt.Errorf("write serial frame: %w", err)
	}
	if n != len(request) {
		entry.Error = io.ErrShortWrite.Error()
		return nil, io.ErrShortWrite
	}
	if expected == 0 {
		return nil, nil
	}
	response := make([]byte, expected)
	read := 0
	for read < 2 {
		if err := ctx.Err(); err != nil {
			entry.Error = err.Error()
			return nil, err
		}
		n, err := port.Read(response[read:2])
		read += n
		if err != nil {
			entry.Error = err.Error()
			return nil, fmt.Errorf("read serial response (%d/%d bytes): %w", read, expected, err)
		}
		if n == 0 {
			entry.Error = fmt.Sprintf("serial response timeout (%d/%d bytes)", read, expected)
			return nil, fmt.Errorf("serial response timeout (%d/%d bytes)", read, expected)
		}
	}
	if response[1]&0x80 != 0 {
		expected = 5 // Exception response: address, function, exception code, CRC.
	}
	for read < expected {
		if err := ctx.Err(); err != nil {
			entry.Error = err.Error()
			return nil, err
		}
		n, err := port.Read(response[read:expected])
		read += n
		if err != nil {
			entry.Error = err.Error()
			return nil, fmt.Errorf("read serial response (%d/%d bytes): %w", read, expected, err)
		}
		if n == 0 {
			entry.Error = fmt.Sprintf("serial response timeout (%d/%d bytes)", read, expected)
			return nil, fmt.Errorf("serial response timeout (%d/%d bytes)", read, expected)
		}
	}
	response = response[:expected]
	entry.Response = hex.EncodeToString(response)
	return response, nil
}

func (s *SerialLink) Transactions() []Transaction {
	s.logMu.RLock()
	defer s.logMu.RUnlock()
	result := make([]Transaction, len(s.log))
	copy(result, s.log)
	return result
}
