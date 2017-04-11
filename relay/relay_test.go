package relay

import (
	"testing"
	"time"

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

func TestRelayProcess(t *testing.T) {
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	relay := NewRelay(router)
	go relay.Start()
	defer relay.Stop()

	log.TestOutput(true, 2)
	log.OutputToBuf()
	relay.Process(cAddr1, &wrongMessage{10})
	relay.Process(cAddr1, &RelayMessage{})

	require.Nil(t, relay.channels[chanId1])
	relay.Process(cAddr1, &RelayMessage{
		Type:    RelayMessage_JOIN,
		Channel: chanId1,
	})
	assert.NotNil(t, relay.channels[chanId1])

	relay.Process(cAddr1, &RelayMessage{
		Type:    RelayMessage_LEAVE,
		Channel: chanId1})
	time.Sleep(10 * time.Millisecond)
	assert.Nil(t, relay.channels[chanId1])

	relay.Process(cAddr2, &RelayMessage{
		Channel: chanId2,
		Type:    RelayMessage_INGRESS,
		Ingress: &Ingress{
			Blob: []byte("hell"),
		}})
	time.Sleep(10 * time.Millisecond)
	assert.True(t, log.ContainsStdOut("unregistered"))
	log.OutputToOs()
}
