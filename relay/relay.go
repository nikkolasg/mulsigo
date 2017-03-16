package relay

import (
	"sync"

	"github.com/dedis/onet/log"
	net "github.com/nikkolasg/mulsigo/network"
)

// MaxChannel represents the maximum number of channels allowed for this relay.
// Let's start slowly :)
const MaxChannel = 50

// ChannelQueueSize represents how large the queue of message for one
// channel is.
const ChannelQueueSize = 100

// ChannelSize represents the maximum number of participants in a channel.
const ChannelSize = 30

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
	return relay
}

// Start starts the listening process to allow incoming connections.
func (r *Relay) Start() {
	r.router.Start()
}

func (r *Relay) Process(from net.Address, msg net.Message) {
	switch m := msg.(type) {
	case *RelayMessage:
		switch {
		case m.Join != nil:
			r.joinChannel(from, m.Join)
		case m.Leave != nil:
			r.leaveChannel(from, m.Leave)
		case m.Incoming != nil:
			r.dispatchToChannel(from, m.Incoming)
		default:
			log.Warn("received nil msg from ", from)
		}
	default:
		log.Warn("received unknown msg from ", from)
	}
}

// dispatchToChannel finds the channel the message is destined for and dispatch
// it accordingly. It is a no-op if the destination channel is non existant.
func (r *Relay) dispatchToChannel(from net.Address, msg *ChannelIncomingMessage) {
	id := msg.Channel
	// check if channel exists
	r.channelsMut.Lock()
	ch, ok := r.channels[id]
	r.channelsMut.Unlock()
	if !ok {
		log.Error("relay: channel %s does not exist", id)
		return
	}

	ch.broadcast(from, msg.Blob)
}

// joinChannel registers a client to a channel. If the channel does not exists,
// then the channel is first created.
func (r *Relay) joinChannel(client net.Address, msg *JoinMessage) {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()
	// check if channel exists
	id := string(msg.Channel)
	ch, ok := r.channels[id]
	if !ok {
		// create a new one
		ch = newChannel(r.router, id)
		r.channels[id] = ch
	}
	ch.addClient(client)
}

// leaveChannel is called when a client is a leaving a channel. If the channel
// is empty after, it is deleted from the list of channels. Leaving a channel
// where c is not registered is a no-op.
func (r *Relay) leaveChannel(client net.Address, msg *LeaveMessage) {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()
	// check if channel exists
	id := msg.Channel
	ch, ok := r.channels[id]
	if !ok {
		return
	}

	if ch.removeClient(client) {
		// channel is empty,delete it
		ch.stop()
		delete(r.channels, id)
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
