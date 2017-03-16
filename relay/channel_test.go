package relay

import (
	"bytes"
	"testing"
	"time"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelAddRemove(t *testing.T) {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	ch := newChannel(router, chanId1)
	defer ch.stop()

	assert.Nil(t, ch.addClient(cAddr1))
	assert.Len(t, ch.clients, 1)
	// registering twice
	assert.Error(t, ch.addClient(cAddr1))
	// too many clients
	goodClients := ch.clients
	ch.clients = make([]net.Address, ChannelSize+1)
	assert.Error(t, ch.addClient(cAddr2))
	ch.clients = goodClients

	// add the other
	assert.Nil(t, ch.addClient(cAddr2))
	// remove the first
	assert.False(t, ch.removeClient(cAddr1))
	// remove the second => empty
	assert.True(t, ch.removeClient(cAddr2))

}

func TestChannelProcess(t *testing.T) {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)
	go router.Start()
	defer router.Stop()

	ch := newChannel(router, chanId1)
	defer ch.stop()
	cl1, err := net.NewLocalConn(cAddr1, rAddr)
	require.Nil(t, err)
	cl2, err := net.NewLocalConn(cAddr2, rAddr)
	require.Nil(t, err)
	defer cl1.Close()
	defer cl2.Close()

	// should be no op
	ch.broadcast(cAddr1, []byte("whoup whoup"))

	// add cl1
	require.Nil(t, ch.addClient(cAddr1))
	// should just continue as there's no other destination
	ch.broadcast(cAddr1, []byte("still wrong"))
	// let the time to the goroutine
	time.Sleep(5 * time.Millisecond)
	// add cl2
	require.Nil(t, ch.addClient(cAddr2))
	ch.broadcast(cAddr1, []byte("good one"))

	good := make(chan net.Message)
	bad := make(chan error)
	go func() {
		m, err := cl2.Receive()
		if err != nil {
			bad <- err
		} else {
			good <- m
		}
	}()

	select {
	case m := <-good:
		rm, ok := m.(*RelayMessage)
		if !ok {
			t.Fatal("wrong message type")
		}
		if rm.Outgoing == nil {
			t.Fatal("no outgoing message")
		}
		if !bytes.Contains(rm.Outgoing.Blob, []byte("good")) {
			t.Fatal("wrong message" + string(rm.Outgoing.Blob))
		}
		if rm.Outgoing.Address != cAddr1.String() {
			t.Fatal("wrong sender")
		}
		if rm.Outgoing.Channel != chanId1 {
			t.Fatal("wrong channel")
		}
	case e := <-bad:
		t.Error(e)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("no message broadcasted")
	}

}
