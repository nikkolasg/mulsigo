package client

import (
	"crypto/rand"
	"testing"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/relay"

	"github.com/stretchr/testify/require"
)

var name1 = "name1"
var name2 = "name2"
var cAddr1 = net.NewLocalAddress("client-1")
var cAddr2 = net.NewLocalAddress("client-2")
var chanid = "chanid"

type fakeChannel struct {
	addr   string
	own    chan relay.Egress
	remote *fakeChannel
}

func createFakeChannelPair() (relay.Channel, relay.Channel) {
	c1 := &fakeChannel{addr: cAddr1.String(), own: make(chan relay.Egress, 50)}
	c2 := &fakeChannel{addr: cAddr2.String(), own: make(chan relay.Egress, 50)}
	c1.remote = c2
	c2.remote = c1
	return c1, c2
}

func (f *fakeChannel) Send(b []byte) error {
	f.remote.own <- relay.Egress{
		Address: f.addr,
		Blob:    b,
	}
	return nil
}

func (f *fakeChannel) Receive() (*relay.Egress, error) {
	e := <-f.own
	return &e, nil
}

func (f *fakeChannel) Id() string {
	return "fake"
}

func (f *fakeChannel) Close() {
	close(f.own)
}

func TestNoiseStreamHandshake(t *testing.T) {

	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	c1, c2 := createFakeChannelPair()

	n1 := newNoiseStream(p1, i1, i2, c1)
	n2 := newNoiseStream(p2, i2, i1, c2)

	var ch = make(chan error)
	go func() {
		err := n1.doHandshake()
		ch <- err
	}()

	err = n2.doHandshake()
	require.Nil(t, err)
	require.Nil(t, <-ch)

	require.Error(t, n2.doHandshake())
}
