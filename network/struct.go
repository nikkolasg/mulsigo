package network

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// MaxRetryConnect defines how many times we should try to connect.
const MaxRetryConnect = 5

// MaxIdentityExchange is the timeout for an identityExchange.
const MaxIdentityExchange = 5 * time.Second

// WaitRetry is the timeout on connection-setups.
const WaitRetry = 20 * time.Millisecond

// The various errors you can have
// XXX not working as expected, often falls on errunknown

// ErrClosed is when a connection has been closed.
var ErrClosed = errors.New("Connection Closed")

// ErrEOF is when the connection sends an EOF signal (mostly because it has
// been shut down).
var ErrEOF = errors.New("EOF")

// ErrCanceled means something went wrong in the sending or receiving part.
var ErrCanceled = errors.New("Operation Canceled")

// ErrTimeout is raised if the timeout has been reached.
var ErrTimeout = errors.New("Timeout Error")

// ErrUnknown is an unknown error.
var ErrUnknown = errors.New("Unknown Error")

// Size is a type to reprensent the size that is sent before every packet to
// correctly decode it.
type Size uint32

// GlobalBind returns the global-binding address. Given any IP:PORT combination,
// it will return 0.0.0.0:PORT.
func GlobalBind(address string) (string, error) {
	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		return "", errors.New("Not a host:port address")
	}
	return "0.0.0.0:" + addr[1], nil
}

// counterSafe is a struct that enables to update two counters Rx & Tx
// atomically that can be have increasing values.
// It's main use is for Conn to update how many bytes they've
// written / read. This struct implements the monitor.CounterIO interface.
type counterSafe struct {
	tx uint64
	rx uint64
	sync.Mutex
}

// Rx returns the rx counter
func (c *counterSafe) Rx() uint64 {
	c.Lock()
	defer c.Unlock()
	return c.rx
}

// Tx returns the tx counter
func (c *counterSafe) Tx() uint64 {
	c.Lock()
	defer c.Unlock()
	return c.tx
}

// updateRx adds delta to the rx counter
func (c *counterSafe) updateRx(delta uint64) {
	c.Lock()
	defer c.Unlock()
	c.rx += delta
}

// updateTx adds delta to the tx counter
func (c *counterSafe) updateTx(delta uint64) {
	c.Lock()
	defer c.Unlock()
	c.tx += delta
}
