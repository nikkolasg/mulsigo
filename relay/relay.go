package relay

import (
	"fmt"
	"sync"

	"github.com/dedis/onet/log"
	net "github.com/dedis/onet/network"
)

// MaxChannel represents the maximum number of channels allowed for this relay.
// Let's start slowly :)
const MaxChannel = 50

// ChannelQueueSize represents how large the queue of message for one
// channel is.
const ChannelQueueSize = 100

// ChannelSize represents the maximum number of participants in a channel.
const ChannelSize = 30

// RelayMessageType is the type sent by the network library.
var RelayMessageType net.MessageTypeID

func init() {
	log.SetDebugVisible(log.FormatPython)
	RelayMessageType = net.RegisterMessage(RelayMessage{})
}

// Relay is the main struct that keep tracks of all clients and channels
// active.
type Relay struct {
	listener net.Listener

	channels    map[string]*channel
	channelsMut sync.Mutex
}

// NewRelay returns a relay that can be `Start()`ed with `Start`.
func NewRelay(l net.Listener) *Relay {
	r := &Relay{
		listener: l,
		channels: make(map[string]*channel),
	}
	return r
}

// Start starts the listening process to allow incoming connections.
func (r *Relay) Start() {
	r.listener.Listen(r.handleNewClient)
}

// handleNewClient is called for each new connection to the relay.
func (r *Relay) handleNewClient(c net.Conn) {
	client := newClient(c, r)
	go client.writeMessages()
	client.readMessages()
}

// dispatchToChannel finds the channel the message is destined for and dispatch
// it accordingly. It is a no-op if the destination channel is non existant.
func (r *Relay) dispatchToChannel(c *client, msg *ChannelIncomingMessage) error {
	id := string(msg.Channel)

	// check if channel exists
	r.channelsMut.Lock()
	ch, ok := r.channels[id]
	r.channelsMut.Unlock()
	if !ok {
		return fmt.Errorf("relay: channel %s does not exist", id)
	}

	ch.broadcast(c, msg.Blob)
	return nil
}

// joinChannel registers a client to a channel. If the channel does not exists,
// then the channel is first created.
func (r *Relay) joinChannel(c *client, msg *JoinMessage) {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()
	// check if channel exists
	id := string(msg.Channel)
	ch, ok := r.channels[id]
	if !ok {
		// create a new one
		ch = newChannel(id)
		r.channels[id] = ch
	}
	ch.addClient(c)
}

// leaveChannel is called when a client is a leaving a channel. If the channel
// is empty after, it is deleted from the list of channels. Leaving a channel
// where c is not registered is a no-op.
func (r *Relay) leaveChannel(c *client, msg *LeaveMessage) {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()
	// check if channel exists
	id := string(msg.Channel)
	ch, ok := r.channels[id]
	if !ok {
		return
	}
	if ch.removeClient(c) {
		// channel is empty,delete it
		ch.stop()
		delete(r.channels, id)
	}
}

// unregisterClient takes a client and unregisters it from all channels the
// client is registered to. This function is usually called when a connection
// drops out.
func (r *Relay) unregisterClient(client *client) {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()

	for _, ch := range r.channels {
		ch.removeClient(client)
	}
}

// Stop closes all channel, and closes all connections to the Relay.
func (r *Relay) Stop() {
	r.channelsMut.Lock()
	defer r.channelsMut.Unlock()

	for _, ch := range r.channels {
		ch.stop()
	}
	r.listener.Stop()
}
