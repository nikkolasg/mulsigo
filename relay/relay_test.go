package relay

import (
	"testing"

	net "github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

var listenerAddr = net.NewLocalAddress("relay")

var localAddr1 = net.NewLocalAddress("client-1")
var localAddr2 = net.NewLocalAddress("client-2")

var chanId1 = "myChannel1"
var chanId2 = "myChannel2"

func TestRelayJoinLeave(t *testing.T) {
	listener, err := net.NewLocalListener(listenerAddr)
	require.Nil(t, err)
	relay := NewRelay(listener)
	defer relay.Stop()

	go relay.Start()

	c, err := net.NewLocalConn(localAddr1, listenerAddr)
	require.Nil(t, err)
	client := newClient(c, relay)

}
