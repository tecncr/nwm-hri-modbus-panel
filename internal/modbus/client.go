package modbus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Options tunes the behaviour of a Client.
type Options struct {
	// Timeout is how long to wait for a complete response.
	Timeout time.Duration
	// FrameDelay is the minimum silent interval enforced between frames.
	FrameDelay time.Duration
	// Retries is how many times a request is repeated after a timeout or CRC
	// error. Exception responses are never retried.
	Retries int
	// BroadcastWait is how long to linger after a broadcast write, swallowing
	// any (non-standard) reply before the bus is released.
	BroadcastWait time.Duration
}

// FrameDelayFor returns the RTU inter-frame silent interval (3.5 character
// times, 11 bits per character) for a baud rate, floored at 1.75ms as the
// specification recommends for rates above 19200 baud.
func FrameDelayFor(baud int) time.Duration {
	if baud <= 0 {
		baud = 9600
	}
	d := time.Duration(float64(time.Second) * 3.5 * 11 / float64(baud))
	return max(d, 1750*time.Microsecond)
}

// Stats are cumulative counters for a Client.
type Stats struct {
	Requests    uint64    `json:"requests"`
	Failures    uint64    `json:"failures"`
	Timeouts    uint64    `json:"timeouts"`
	CRCErrors   uint64    `json:"crc_errors"`
	Exceptions  uint64    `json:"exceptions"`
	LastError   string    `json:"last_error,omitempty"`
	LastErrorAt time.Time `json:"last_error_at,omitzero"`
	LastOKAt    time.Time `json:"last_ok_at,omitzero"`
	Connected   bool      `json:"connected"`
}

// Client is a Modbus RTU master bound to a single bus. It serialises access to
// the half-duplex line, so it is safe for concurrent use.
type Client struct {
	dial Dialer
	opts Options

	sem    chan struct{}
	conn   Conn
	last   time.Time
	closed bool

	statsMu sync.Mutex
	stats   Stats
}

// NewClient creates a client. The connection is opened lazily on first use and
// reopened automatically after I/O errors.
func NewClient(dial Dialer, opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = time.Second
	}
	if opts.BroadcastWait <= 0 {
		opts.BroadcastWait = 200 * time.Millisecond
	}
	return &Client{dial: dial, opts: opts, sem: make(chan struct{}, 1)}
}

// Stats returns a snapshot of the client counters.
func (c *Client) Stats() Stats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return c.stats
}

// Close releases the underlying connection. Further calls fail with ErrClosed.
func (c *Client) Close() error {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	c.closed = true
	return c.dropConn()
}

// ReadHoldingRegisters reads qty consecutive registers (function 0x03).
func (c *Client) ReadHoldingRegisters(ctx context.Context, slave uint8, addr, qty uint16) ([]uint16, error) {
	if slave < MinSlaveAddress || slave > MaxSlaveAddress {
		return nil, fmt.Errorf("%w: slave address %d out of range 1-247", ErrInvalidRequest, slave)
	}
	if qty < 1 || qty > MaxReadQuantity {
		return nil, fmt.Errorf("%w: quantity %d out of range 1-%d", ErrInvalidRequest, qty, MaxReadQuantity)
	}
	if uint32(addr)+uint32(qty) > 0x10000 {
		return nil, fmt.Errorf("%w: address range overflows 0xFFFF", ErrInvalidRequest)
	}
	req := []byte{slave, FuncReadHoldingRegisters, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(req[2:], addr)
	binary.BigEndian.PutUint16(req[4:], qty)
	resp, err := c.transact(ctx, req)
	if err != nil {
		return nil, err
	}
	count := int(resp[2])
	if count != int(qty)*2 {
		return nil, fmt.Errorf("%w: expected %d data bytes, got %d", ErrInvalidResponse, qty*2, count)
	}
	out := make([]uint16, qty)
	for i := range out {
		out[i] = binary.BigEndian.Uint16(resp[3+i*2:])
	}
	return out, nil
}

// WriteSingleRegister writes one register (function 0x06). A slave of 0
// broadcasts the write to every device on the bus.
func (c *Client) WriteSingleRegister(ctx context.Context, slave uint8, addr, value uint16) error {
	if slave > MaxSlaveAddress {
		return fmt.Errorf("%w: slave address %d out of range 0-247", ErrInvalidRequest, slave)
	}
	req := []byte{slave, FuncWriteSingleRegister, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(req[2:], addr)
	binary.BigEndian.PutUint16(req[4:], value)
	if slave == BroadcastAddress {
		return c.broadcast(ctx, req)
	}
	resp, err := c.transact(ctx, req)
	if err != nil {
		return err
	}
	if binary.BigEndian.Uint16(resp[2:]) != addr || binary.BigEndian.Uint16(resp[4:]) != value {
		return fmt.Errorf("%w: echo mismatch", ErrInvalidResponse)
	}
	return nil
}

// WriteMultipleRegisters writes consecutive registers (function 0x10). A
// slave of 0 broadcasts the write to every device on the bus.
func (c *Client) WriteMultipleRegisters(ctx context.Context, slave uint8, addr uint16, values []uint16) error {
	if slave > MaxSlaveAddress {
		return fmt.Errorf("%w: slave address %d out of range 0-247", ErrInvalidRequest, slave)
	}
	qty := len(values)
	if qty < 1 || qty > MaxWriteQuantity {
		return fmt.Errorf("%w: quantity %d out of range 1-%d", ErrInvalidRequest, qty, MaxWriteQuantity)
	}
	if int(addr)+qty > 0x10000 {
		return fmt.Errorf("%w: address range overflows 0xFFFF", ErrInvalidRequest)
	}
	req := make([]byte, 7, 7+qty*2)
	req[0], req[1] = slave, FuncWriteMultipleRegisters
	binary.BigEndian.PutUint16(req[2:], addr)
	binary.BigEndian.PutUint16(req[4:], uint16(qty))
	req[6] = byte(qty * 2)
	for _, v := range values {
		req = binary.BigEndian.AppendUint16(req, v)
	}
	if slave == BroadcastAddress {
		return c.broadcast(ctx, req)
	}
	resp, err := c.transact(ctx, req)
	if err != nil {
		return err
	}
	if binary.BigEndian.Uint16(resp[2:]) != addr || binary.BigEndian.Uint16(resp[4:]) != uint16(qty) {
		return fmt.Errorf("%w: echo mismatch", ErrInvalidResponse)
	}
	return nil
}

func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	if c.closed {
		<-c.sem
		return ErrClosed
	}
	return nil
}

func (c *Client) release() { <-c.sem }

// transact sends req (without CRC) and returns the validated response frame
// (including CRC), retrying on timeouts and CRC errors.
func (c *Client) transact(ctx context.Context, req []byte) ([]byte, error) {
	if err := c.acquire(ctx); err != nil {
		return nil, err
	}
	defer c.release()

	frame := appendCRC(req)
	var err error
	for attempt := 0; attempt <= c.opts.Retries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var resp []byte
		resp, err = c.exchange(ctx, frame)
		c.record(err)
		if err == nil {
			return resp, nil
		}
		var exc *ExceptionError
		if errors.As(err, &exc) {
			return nil, err
		}
	}
	return nil, err
}

// broadcast sends a frame to address 0. Standard slaves never answer
// broadcasts; any reply that does arrive is drained and ignored.
func (c *Client) broadcast(ctx context.Context, req []byte) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	conn, err := c.connection(ctx)
	if err != nil {
		c.record(err)
		return err
	}
	c.waitSilence()
	_ = conn.Discard()
	_, err = conn.Write(appendCRC(req))
	if err == nil {
		time.Sleep(c.opts.BroadcastWait)
		_ = conn.Discard()
	} else {
		_ = c.dropConn()
	}
	c.last = time.Now()
	c.record(err)
	return err
}

func (c *Client) exchange(ctx context.Context, frame []byte) ([]byte, error) {
	conn, err := c.connection(ctx)
	if err != nil {
		return nil, err
	}
	c.waitSilence()
	_ = conn.Discard()
	if _, err := conn.Write(frame); err != nil {
		_ = c.dropConn()
		return nil, err
	}
	deadline := time.Now().Add(c.opts.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	resp, err := readResponse(conn, frame[0], frame[1], deadline)
	c.last = time.Now()
	if err != nil {
		var exc *ExceptionError
		if !errors.As(err, &exc) && !errors.Is(err, ErrTimeout) && !errors.Is(err, ErrCRC) && !errors.Is(err, ErrInvalidResponse) {
			// Transport-level failure: reopen on next use.
			_ = c.dropConn()
		}
		return nil, err
	}
	return resp, nil
}

func (c *Client) connection(ctx context.Context) (Conn, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.setConnected(true)
	return conn, nil
}

func (c *Client) dropConn() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.setConnected(false)
	return err
}

func (c *Client) waitSilence() {
	if wait := time.Until(c.last.Add(c.opts.FrameDelay)); wait > 0 {
		time.Sleep(wait)
	}
}

func (c *Client) setConnected(v bool) {
	c.statsMu.Lock()
	c.stats.Connected = v
	c.statsMu.Unlock()
}

func (c *Client) record(err error) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.stats.Requests++
	if err == nil {
		c.stats.LastOKAt = time.Now()
		return
	}
	c.stats.Failures++
	c.stats.LastError = err.Error()
	c.stats.LastErrorAt = time.Now()
	var exc *ExceptionError
	switch {
	case errors.Is(err, ErrTimeout):
		c.stats.Timeouts++
	case errors.Is(err, ErrCRC):
		c.stats.CRCErrors++
	case errors.As(err, &exc):
		c.stats.Exceptions++
	}
}

// readResponse reads one RTU response frame for the given slave/function.
func readResponse(conn Conn, slave, fn byte, deadline time.Time) ([]byte, error) {
	buf := make([]byte, maxRTUFrameLength)
	if err := conn.ReadFull(buf[:2], deadline); err != nil {
		return nil, err
	}
	if buf[0] != slave {
		return nil, fmt.Errorf("%w: unexpected slave address %d", ErrInvalidResponse, buf[0])
	}
	var n int
	switch buf[1] {
	case fn | exceptionFlag:
		n = exceptionFrameSize
	case fn:
		switch fn {
		case FuncReadHoldingRegisters:
			if err := conn.ReadFull(buf[2:3], deadline); err != nil {
				return nil, err
			}
			n = 3 + int(buf[2]) + 2
		case FuncWriteSingleRegister, FuncWriteMultipleRegisters:
			n = 8
		default:
			return nil, fmt.Errorf("%w: unsupported function 0x%02X", ErrInvalidRequest, fn)
		}
	default:
		return nil, fmt.Errorf("%w: unexpected function 0x%02X", ErrInvalidResponse, buf[1])
	}
	have := 2
	if fn == FuncReadHoldingRegisters && buf[1] == fn {
		have = 3
	}
	if err := conn.ReadFull(buf[have:n], deadline); err != nil {
		return nil, err
	}
	frame := buf[:n]
	if !checkCRC(frame) {
		return nil, ErrCRC
	}
	if frame[1] == fn|exceptionFlag {
		return nil, &ExceptionError{Function: fn, Code: frame[2]}
	}
	return frame, nil
}
