package client

import (
	"fmt"
	"testing"
	"time"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/slog"

	"github.com/stretchr/testify/require"
)

var name1 = "name1"
var name2 = "name2"
var cAddr1 = net.NewLocalAddress("client-1")
var cAddr2 = net.NewLocalAddress("client-2")
var chanid = "chanid"

func TestRouterLots(t *testing.T) {
	n := 5
	//thr := n/2 + 1
	privs, ids := BatchPrivateIdentity(n)
	slog.Level = slog.LevelDebug
	defer func() { slog.Level = slog.LevelPrint }()

	//routers := BatchRouters(ids)
	relay, routers := BatchRelayRouters(privs, ids)
	defer relay.Stop()

	var incoming = make(chan *ClientMessage)
	var proc processor
	proc = func(i *Identity, c *ClientMessage) {
		go func() { incoming <- c }()
	}

	for i, r := range routers {
		fmt.Printf("router %d: %p\n", i, r)
	}

	routers[0].RegisterProcessor(&proc)
	for i, id := range ids[1:] {
		routers[i+1].RegisterProcessor(&proc)
		go routers[0].Send(id, &ClientMessage{Type: 10})
		go routers[i+1].Send(ids[0], &ClientMessage{Type: 10})
		<-incoming
		<-incoming
	}
}

func TestRouterBasic(t *testing.T) {
	_, ids := BatchPrivateIdentity(2)
	glob := NewGlobalStreamFactory()
	sf1 := glob.Sub(ids[0])
	sf2 := glob.Sub(ids[1])

	r1 := NewRouter(sf1)
	r2 := NewRouter(sf2)

	var incoming = make(chan *ClientMessage)
	var proc processor
	proc = func(i *Identity, c *ClientMessage) {
		go func() { incoming <- c }()
	}
	r1.RegisterProcessor(&proc)
	r2.RegisterProcessor(&proc)

	// both parties send a message to each other
	send := func(r *Router, dest *Identity) {
		if err := r.Send(dest, &ClientMessage{Type: 15}); err != nil {
			t.Fatal(err)
		}
	}
	go send(r1, ids[1])
	go send(r2, ids[0])

	// receiving should work
	cm := <-incoming
	require.Equal(t, cm.Type, uint32(15))
	cm = <-incoming
	require.Equal(t, cm.Type, uint32(15))

}

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
