package modbus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"go.bug.st/serial"
)

// Conn is a byte stream carrying RTU frames: a serial port, or a TCP socket to
// a transparent serial server (RTU over TCP).
type Conn interface {
	Write(p []byte) (int, error)
	// ReadFull fills p completely or returns ErrTimeout once deadline passes.
	ReadFull(p []byte, deadline time.Time) error
	// Discard drops any pending, unread input.
	Discard() error
	Close() error
}

// Dialer opens a new Conn.
type Dialer func(ctx context.Context) (Conn, error)

// SerialConfig describes an RS-485 serial line.
type SerialConfig struct {
	Device   string
	BaudRate int
	DataBits int
	Parity   string // "N", "E" or "O"
	StopBits int    // 1 or 2
}

// SerialDialer returns a Dialer that opens the configured serial port.
func SerialDialer(cfg SerialConfig) Dialer {
	return func(context.Context) (Conn, error) {
		mode := &serial.Mode{BaudRate: cfg.BaudRate, DataBits: cfg.DataBits}
		switch strings.ToUpper(cfg.Parity) {
		case "", "N", "NONE":
			mode.Parity = serial.NoParity
		case "E", "EVEN":
			mode.Parity = serial.EvenParity
		case "O", "ODD":
			mode.Parity = serial.OddParity
		default:
			return nil, fmt.Errorf("modbus: unsupported parity %q", cfg.Parity)
		}
		switch cfg.StopBits {
		case 0, 1:
			mode.StopBits = serial.OneStopBit
		case 2:
			mode.StopBits = serial.TwoStopBits
		default:
			return nil, fmt.Errorf("modbus: unsupported stop bits %d", cfg.StopBits)
		}
		p, err := serial.Open(cfg.Device, mode)
		if err != nil {
			return nil, fmt.Errorf("modbus: open %s: %w", cfg.Device, err)
		}
		return &serialConn{port: p}, nil
	}
}

type serialConn struct {
	port serial.Port
}

func (s *serialConn) Write(p []byte) (int, error) { return s.port.Write(p) }

func (s *serialConn) ReadFull(p []byte, deadline time.Time) error {
	off := 0
	for off < len(p) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrTimeout
		}
		if err := s.port.SetReadTimeout(remaining); err != nil {
			return err
		}
		n, err := s.port.Read(p[off:])
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrTimeout
		}
		off += n
	}
	return nil
}

func (s *serialConn) Discard() error { return s.port.ResetInputBuffer() }

func (s *serialConn) Close() error { return s.port.Close() }

// TCPDialer returns a Dialer that connects to a transparent serial-to-Ethernet
// gateway which forwards raw RTU frames (often called "RTU over TCP").
func TCPDialer(address string, connectTimeout time.Duration) Dialer {
	return func(ctx context.Context) (Conn, error) {
		d := net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}
		c, err := d.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("modbus: dial %s: %w", address, err)
		}
		return &tcpConn{conn: c}, nil
	}
}

type tcpConn struct {
	conn net.Conn
}

func (t *tcpConn) Write(p []byte) (int, error) {
	_ = t.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return t.conn.Write(p)
}

func (t *tcpConn) ReadFull(p []byte, deadline time.Time) error {
	if err := t.conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	if _, err := io.ReadFull(t.conn, p); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return ErrTimeout
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return ErrTimeout
		}
		return err
	}
	return nil
}

func (t *tcpConn) Discard() error {
	buf := make([]byte, 256)
	for {
		_ = t.conn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
		n, err := t.conn.Read(buf)
		if n == 0 || err != nil {
			return nil
		}
	}
}

func (t *tcpConn) Close() error { return t.conn.Close() }
