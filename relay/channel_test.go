package relay

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelAddRemove(t *testing.T) {
	//log.TestOutput(true, 3)
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	r := NewRelay(router)

	ch := newChannel(r, chanId1)
	defer ch.stop()

	addClient := func(addr net.Address) {
		ch.join <- addr
		time.Sleep(10 * time.Millisecond)
	}
	getList := func() map[net.Address]bool {
		return ch.clients
	}

	addClient(cAddr1)
	assert.Contains(t, getList(), cAddr1)
	// registering twice
	addClient(cAddr1)
	assert.Len(t, getList(), 1)

	// too many clients
	routers := make([]*net.Router, ChannelSize+1)
	for i := 0; i < ChannelSize+1; i++ {
		addr := net.NewLocalAddress(fmt.Sprintf("channel-%d", i))
		routers[i], err = net.NewLocalRouter(addr, enc)
		go routers[i].Start()
		defer routers[i].Stop()
		require.Nil(t, err)
		addClient(addr)
	}
	assert.Len(t, getList(), ChannelSize)

	// remove our client
	ch.leave <- cAddr1
	time.Sleep(10 * time.Millisecond)
	_, ok := getList()[cAddr1]
	assert.False(t, ok)
}

func TestChannelProcess(t *testing.T) {
	//log.TestOutput(true, 3)
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)
	go router.Start()
	defer router.Stop()
	r := NewRelay(router)
	ch := newChannel(r, chanId1)
	defer ch.stop()
	cl1, err := net.NewLocalConn(cAddr1, rAddr)
	require.Nil(t, err)
	cl2, err := net.NewLocalConn(cAddr2, rAddr)
	require.Nil(t, err)
	defer cl1.Close()
	defer cl2.Close()

	// should be no op
	ch.newMessage(cAddr1, relayMessage(cAddr1, []byte("whoup whoup")))

	// add cl1
	ch.join <- cAddr1
	// should just continue as there's no other destination
	ch.newMessage(cAddr1, relayMessage(cAddr1, []byte("still wrong")))
	// let the time to the goroutine
	time.Sleep(5 * time.Millisecond)
	// add cl2
	ch.join <- cAddr2
	ch.newMessage(cAddr1, relayMessage(cAddr1, []byte("good one")))

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
		egress := rm.GetEgress()
		if egress == nil {
			t.Fatal("no outgoing message")
		}
		if !bytes.Contains(egress.GetBlob(), []byte("good")) {
			t.Fatal("wrong message" + string(egress.Blob))
		}
		if egress.GetAddress() != cAddr1.String() {
			t.Fatal("wrong sender")
		}
		if rm.GetChannel() != chanId1 {
			t.Fatal("wrong channel")
		}
	case e := <-bad:
		t.Error(e)
	case <-time.After(10 * time.Millisecond):
		t.Fatal("no message broadcasted")
	}

}

func relayMessage(sender net.Address, msg []byte) *RelayMessage {
	return &RelayMessage{
		Type: RelayMessage_INGRESS,
		Ingress: &Ingress{
			Blob: msg,
		},
	}
}
