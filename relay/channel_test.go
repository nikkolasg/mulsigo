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
	go r.Start()
	defer r.Stop()

	cl1, err := net.NewLocalConn(cAddr1, rAddr)
	require.Nil(t, err)

	addClient := func(c net.Conn) bool {
		require.Nil(t, c.Send(&RelayMessage{
			Channel: chanId1,
			Type:    RelayMessage_JOIN,
		}))
		m, err := c.Receive()
		require.Nil(t, err)
		rm, ok := m.(*RelayMessage)
		require.True(t, ok)
		require.Equal(t, RelayMessage_JOIN_RESPONSE, rm.GetType())
		return JoinResponse_OK == rm.JoinResponse.Status
	}
	getList := func() map[net.Address]bool {
		r.channelsMut.Lock()
		defer r.channelsMut.Unlock()
		ch, ok := r.channels[chanId1]
		require.True(t, ok)
		return ch.clients
	}

	addClient(cl1)
	assert.Contains(t, getList(), cAddr1)
	assert.Len(t, getList(), 1)
	// registering twice
	addClient(cl1)
	assert.Len(t, getList(), 1)

	// too many clients
	conns := make([]net.Conn, ChannelSize+1)
	for i := 0; i < ChannelSize+1; i++ {
		addr := net.NewLocalAddress(fmt.Sprintf("channel-%d", i))
		conns[i], err = net.NewLocalConn(addr, rAddr)
		require.Nil(t, err)
		defer conns[i].Close()
		addClient(conns[i])
		//if i == ChannelSize {
		//	require.False(t, addClient(conns[i]))
		//}
	}
	time.Sleep(10 * time.Millisecond)
	assert.Len(t, getList(), ChannelSize)

	// remove our client
	require.Nil(t, cl1.Send(&RelayMessage{
		Channel: chanId1,
		Type:    RelayMessage_LEAVE,
	}))
	time.Sleep(10 * time.Millisecond)
	_, ok := getList()[cAddr1]
	assert.False(t, ok)
}

func TestChannelProcess(t *testing.T) {
	//log.TestOutput(true, 3)
	router, err := net.NewLocalRouter(rAddr, enc)
	//rAddr = net.NewTCPAddress("127.0.0.1:2000")
	//router, err := net.NewTCPRouter(rAddr, enc)
	require.Nil(t, err)
	go router.Start()
	defer router.Stop()
	r := NewRelay(router)
	defer r.Stop()
	//ch := newChannel(r, chanId1)
	cl1, err := net.NewLocalConn(cAddr1, rAddr)
	//cl1, err := net.NewTCPConn(rAddr, enc)
	require.Nil(t, err)
	cl2, err := net.NewLocalConn(cAddr2, rAddr)
	//cl2, err := net.NewTCPConn(rAddr, enc)
	require.Nil(t, err)
	defer cl1.Close()
	defer cl2.Close()
	fmt.Println("cl1.Local()", cl1.Local().String())
	fmt.Println("cl2.Local()", cl2.Local().String())

	// should be no op
	fmt.Println("\n -- MSG FROM ", cl1.Local(), " --\n")
	require.Nil(t, cl1.Send(relayMessage([]byte("whoup whoup"))))

	// add cl1
	fmt.Println("\n -- JOIN FROM ", cl1.Local(), " --\n")
	require.Nil(t, cl1.Send(joinMessage(cl1.Local())))
	//require.Nil(t, waitNClients(ch, 1))
	nm, err := cl1.Receive()
	require.Nil(t, err)
	rm, ok := nm.(*RelayMessage)
	require.True(t, ok)
	require.NotNil(t, rm.GetJoinResponse())
	require.Equal(t, JoinResponse_OK, rm.GetJoinResponse().Status)
	// should just continue as there's no other destination
	fmt.Println("\n -- MSG FROM ", cl1.Local(), " --\n")
	require.Nil(t, cl1.Send(relayMessage([]byte("still wrong"))))
	// let the time to the goroutine
	time.Sleep(5 * time.Millisecond)
	// add cl2
	fmt.Println("\n -- JOIN FROM ", cl2.Local(), " --\n")
	require.Nil(t, cl2.Send(joinMessage(cl2.Local())))
	// join response ack
	_, err = cl2.Receive()
	require.Nil(t, err)

	//require.Nil(t, waitNClients(ch, 2))
	fmt.Println("\n -- MSG FROM ", cl1.Local(), " --\n")
	require.Nil(t, cl1.Send(relayMessage([]byte("good one"))))

	good := make(chan net.Message)
	bad := make(chan error)
	go func() {
		// message broadcasted
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
			//fmt.Printf("%+v\n", rm)
			t.Fatal("no outgoing message")
		}
		if !bytes.Contains(egress.GetBlob(), []byte("good")) {
			t.Fatal("wrong message" + string(egress.Blob))
		}
		if egress.GetAddress() != cl1.Local().String() {
			t.Fatal("wrong sender")
		}
		if rm.GetChannel() != chanId1 {
			t.Fatal("wrong channel")
		}
	case e := <-bad:
		t.Error(e)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no message broadcasted")
	}

}

func TestChannelConnectionDown(t *testing.T) {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	r := NewRelay(router)
	go r.Start()
	defer r.Stop()

	cl1, err := net.NewLocalConn(cAddr1, rAddr)
	require.Nil(t, err)
	cl2, err := net.NewLocalConn(cAddr2, rAddr)
	require.Nil(t, err)

	addClient := func(c net.Conn) bool {
		require.Nil(t, c.Send(&RelayMessage{
			Channel: chanId1,
			Type:    RelayMessage_JOIN,
		}))
		m, err := c.Receive()
		require.Nil(t, err)
		rm, ok := m.(*RelayMessage)
		require.True(t, ok)
		require.Equal(t, RelayMessage_JOIN_RESPONSE, rm.GetType())
		return JoinResponse_OK == rm.JoinResponse.Status
	}

	send := func(c1 net.Conn) {
		require.Nil(t, c1.Send(&RelayMessage{
			Channel: chanId1,
			Type:    RelayMessage_INGRESS,
			Ingress: &Ingress{
				Blob: []byte("hello world"),
			},
		}))
	}

	require.True(t, addClient(cl1))
	require.True(t, addClient(cl2))
	send(cl1)
	_, err = cl2.Receive()
	require.Nil(t, err)
	require.Nil(t, cl2.Close())
	// make sure the router drops the connection
	require.Error(t, router.Send(cl2.Local(), &RelayMessage{}))
	send(cl1)
	r.channelsMut.Lock()
	require.Len(t, r.channels[chanId1].clients, 1)
	r.channelsMut.Unlock()
}

func waitNClients(ch *channel, n int) error {
	for i := 0; i < 10; i++ {

		if len(ch.clients) == n {
			return nil
		}
		time.Sleep(time.Duration(5*i) * time.Millisecond)
	}
	return fmt.Errorf("no %d clients found in channel", n)
}

func relayMessage(msg []byte) *RelayMessage {
	return &RelayMessage{
		Channel: chanId1,
		Type:    RelayMessage_INGRESS,
		Ingress: &Ingress{
			Blob: msg,
		},
	}
}

func joinMessage(addr net.Address) *RelayMessage {
	return &RelayMessage{
		Channel: chanId1,
		Type:    RelayMessage_JOIN,
	}
}
