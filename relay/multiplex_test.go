package relay

import (
	"testing"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var chanId1 = "MyCustomChanId1"
var chanId2 = "MyCustomChanId2"
var msg = []byte("Hello World")

func TestMultiplex(t *testing.T) {
	relay := newRelay(t)
	go relay.Start()
	defer relay.Stop()

	cl1 := newClient(t, cAddr1)
	cl2 := newClient(t, cAddr2)
	defer cl1.Close()
	defer cl2.Close()

	m1 := NewMultiplexer(cl1)
	m2 := NewMultiplexer(cl2)

	c11, err := m1.Channel(chanId1)
	assert.Nil(t, err)
	assert.NotNil(t, getChannel(m1, chanId1))
	c21, err := m2.Channel(chanId1)
	assert.Nil(t, err)
	assert.NotNil(t, getChannel(m2, chanId1))

	assert.Nil(t, c11.Send(msg))

	eg, err := c21.Receive()
	require.Nil(t, err)
	assert.Equal(t, msg, eg.GetBlob())
}

func getChannel(m *Multiplexer, id string) *clientChannel {
	m.chanMut.Lock()
	defer m.chanMut.Unlock()
	return m.channels[id]
}

func newRelay(t *testing.T) *Relay {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	return NewRelay(router)
}

func newClient(t *testing.T, addr net.Address) net.Conn {
	cl1, err := net.NewLocalConn(addr, rAddr)
	require.Nil(t, err)
	return cl1
}
