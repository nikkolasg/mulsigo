package relay

import (
	"testing"

	"github.com/dedis/onet/log"
	net "github.com/nikkolasg/mulsigo/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var rAddr = net.NewLocalAddress("relay")
var cAddr1 = net.NewLocalAddress("client-1")
var cAddr2 = net.NewLocalAddress("client-2")

var chanId1 = "myChannel1"
var chanId2 = "myChannel2"

var enc = net.NewSingleProtoEncoder(RelayMessage{})

type wrongMessage struct {
	Val int
}

// To avoid setting up testing-verbosity in all tests
func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestRelayJoinLeave(t *testing.T) {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	relay := NewRelay(router)

	jm := &JoinMessage{
		Channel: chanId1,
	}
	assert.Nil(t, relay.channels[chanId1])
	relay.joinChannel(cAddr1, jm)
	assert.NotNil(t, relay.channels[chanId1])

	relay.joinChannel(cAddr2, jm)
	assert.NotNil(t, relay.channels[chanId1])

	wrongLm := &LeaveMessage{
		Channel: "arthur",
	}

	relay.leaveChannel(cAddr1, wrongLm)
	assert.NotNil(t, relay.channels[chanId1])

	lm := &LeaveMessage{
		Channel: chanId1,
	}
	relay.leaveChannel(cAddr1, lm)
	assert.NotNil(t, relay.channels[chanId1])

	relay.leaveChannel(cAddr2, lm)
	assert.Nil(t, relay.channels[chanId1])
}

func TestRelayProcess(t *testing.T) {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	relay := NewRelay(router)

	log.OutputToBuf()
	relay.Process(cAddr1, &wrongMessage{10})
	relay.Process(cAddr1, &RelayMessage{})

	require.Nil(t, relay.channels[chanId1])
	relay.Process(cAddr1, &RelayMessage{Join: &JoinMessage{chanId1}})
	assert.NotNil(t, relay.channels[chanId1])

	relay.Process(cAddr1, &RelayMessage{Leave: &LeaveMessage{chanId1}})
	assert.Nil(t, relay.channels[chanId1])

	relay.Process(cAddr2, &RelayMessage{Incoming: &ChannelIncomingMessage{chanId2, nil}})
	assert.True(t, log.ContainsStdErr("does not exist"))
	log.OutputToOs()
}
