package relay

import (
	"fmt"
	"testing"
	"time"

	"github.com/dedis/onet/log"
	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/slog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var rAddr = net.NewLocalAddress("relay")
var cAddr1 = net.NewLocalAddress("client-1")
var cAddr2 = net.NewLocalAddress("client-2")

var enc = net.NewSingleProtoEncoder(RelayMessage{})

type wrongMessage struct {
	Val int
}

// To avoid setting up testing-verbosity in all tests
func TestMain(m *testing.M) {
	slog.Level = slog.LevelDebug
	log.MainTest(m)
}

func TestRelayProcess(t *testing.T) {
	log.TestOutput(true, 5)
	router, err := net.NewLocalRouter(rAddr, enc)
	require.Nil(t, err)

	relay := NewRelay(router)
	go relay.Start()
	defer relay.Stop()

	log.TestOutput(true, 2)
	_, err = net.NewLocalConn(cAddr1, rAddr)
	require.Nil(t, err)

	log.OutputToBuf()
	relay.Process(cAddr1, &wrongMessage{10})
	relay.Process(cAddr1, &RelayMessage{})

	require.Nil(t, relay.channels[chanId1])
	relay.Process(cAddr1, &RelayMessage{
		Type:    RelayMessage_JOIN,
		Channel: chanId1,
	})
	relay.channelsMut.Lock()
	assert.NotNil(t, relay.channels[chanId1])
	relay.channelsMut.Unlock()

	relay.Process(cAddr1, &RelayMessage{
		Type:    RelayMessage_LEAVE,
		Channel: chanId1})
	time.Sleep(10 * time.Millisecond)
	var found bool
	for i := 0; i < 10; i++ {
		relay.channelsMut.Lock()
		c := relay.channels[chanId1]
		relay.channelsMut.Unlock()
		if c == nil {
			found = true
			break
		}
		time.Sleep(time.Duration(i*10) * time.Second)
	}
	if !found {
		t.Fatal("leave message did not work")
	}

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
