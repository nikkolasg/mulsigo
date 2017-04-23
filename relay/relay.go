package relay

import (
	"sync"

	"github.com/dedis/onet/log"
	"github.com/nikkolasg/mulsigo/event"
	net "github.com/nikkolasg/mulsigo/network"
)

// ChannelQueueSize represents how large the queue of message for one
// channel is.
const ChannelQueueSize = 100

// ChannelSize represents the maximum number of participants in a channel.
const ChannelSize = 30

// MaxChannel represents the maximum number of channels allowed for this relay.
// Let's start slowly :)
const MaxChannel = 50

// Relay is the main struct that keep tracks of all clients and channels
// active.
type Relay struct {
	router *net.Router

	channels    map[string]*channel
	channelsMut sync.Mutex
}

// NewRelay returns a relay that can be `Start()`ed with `Start`.
func NewRelay(r *net.Router) *Relay {
	relay := &Relay{
		router:   r,
		channels: make(map[string]*channel),
	}
	r.RegisterProcessor(relay, RelayMessage{})
	r.Register(net.EventConnDown, relay)
	return relay
}

// Start starts the listening process to allow incoming connections.
func (r *Relay) Start() {
	r.router.Start()
}

func (r *Relay) Process(from net.Address, msg net.Message) {
	switch m := msg.(type) {
	case *RelayMessage:
		r.channelsMut.Lock()
		id := m.GetChannel()
		ch, ok := r.channels[id]
		if !ok {
			if len(r.channels) > MaxChannel {
				// too many channels
				r.router.Send(from, &RelayMessage{
					Type: RelayMessage_JOIN_RESPONSE,
					JoinResponse: &JoinResponse{
						Status: JoinResponse_FAILURE,
						Reason: "too many channels",
					},
				})
				return
			}
			// create a new one
			ch = newChannel(r, id)
			r.channels[id] = ch
		}
		ch.newMessage(from, m)
		r.channelsMut.Unlock()
	default:
		log.Warn("received unknown msg from ", from)
	}
}

// Stop closes all channel, and closes all connections to the Relay.
func (r *Relay) Stop() {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()

	r.router.Stop()
	for _, ch := range r.channels {
		ch.stop()
	}
}

func (r *Relay) deleteChannel(id string) {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()
	delete(r.channels, id)
}

func (r *Relay) Receive(e event.Event) {
	if e.Name() != net.EventConnDown {
		return
	}
	down := e.(*net.EventDown)
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()
	for id, ch := range r.channels {
		ch.newMessage(down.Address, &RelayMessage{Channel: id, Type: RelayMessage_LEAVE})
	}
}
