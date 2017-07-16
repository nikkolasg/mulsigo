package client

import (
	"testing"
	"time"

	net "github.com/nikkolasg/mulsigo/network"

	"github.com/stretchr/testify/require"
)

var name1 = "name1"
var name2 = "name2"
var cAddr1 = net.NewLocalAddress("client-1")
var cAddr2 = net.NewLocalAddress("client-2")
var chanid = "chanid"

// a stupid typed function
type processor func(*Identity, *ClientMessage)

func (p *processor) Process(i *Identity, c *ClientMessage) {
	(*p)(i, c)
}

func TestSequentialDispatcher(t *testing.T) {
	var incoming = make(chan *ClientMessage)
	var proc processor
	proc = func(i *Identity, c *ClientMessage) {
		incoming <- c
	}

	d := newSeqDispatcher()
	d.RegisterProcessor(&proc)

	go d.Dispatch(nil, &ClientMessage{})
	select {
	case c := <-incoming:
		require.NotNil(t, c)
	case <-time.After(10 * time.Millisecond):
		t.FailNow()
	}
}
