package relay

import (
	"errors"
	"sync"
)

// channel holds a list of all participants registered to a channel designated
// by an ID. Each participant can broadcast message to a channel and receive
// message from others on the same channel.
type channel struct {
	id string
	// all clients registered to that channel
	clients map[string]*client
	sync.Mutex

	out chan messageInfo
}

// newChannel returns a new channel identified by "id". It launches the
// processing routine.
func newChannel(id string) *channel {
	ch := &channel{
		id:      id,
		clients: make(map[string]*client),
		out:     make(chan messageInfo, ChannelQueueSize),
	}
	go ch.process()
	return ch
}

func (ch *channel) broadcast(c *client, msg []byte) {
	ch.out <- messageInfo{msg, c.address}
}

// process reads continuously from the inner channel (Golang) of the channel
// (struct). Each message read, is broadcasted to every participant, except for
// the sender.
func (ch *channel) process() {
	for info := range ch.out {
		msg := info.msg
		addr := info.address
		toBroadcast := ChannelOutgoingMessage{
			Channel: ch.id,
			Address: addr,
			Blob:    msg,
		}

		ch.Lock()
		if _, ok := ch.clients[addr]; !ok {
			// message coming from a non-registered user: just drop.
			ch.Unlock()
			continue
		}

		for _, c := range ch.clients {
			if c.address == addr {
				continue
			}
			c.out <- toBroadcast
		}
		ch.Unlock()
	}
}

// addClient takes a client and adds it to the list of registered client for
// this channel. It returns an error if the channel is already full (i.e. more
// than ChannelSize), or if the client is already registered. Otherwise, it
// returns nil.
func (ch *channel) addClient(client *client) error {
	ch.Lock()
	defer ch.Unlock()

	if len(ch.clients) >= ChannelSize {
		return errors.New("channel: can't join a full channel")
	}

	for _, c := range ch.clients {
		if c.address == client.address {
			return errors.New("channel: already registered")
		}
	}
	ch.clients[client.address] = client
	return nil
}

// removeClient takes a client and removes it from the list of registered client
// for this channel. It returns true if the channel can be deleted as there is
// no more client left for this channel. It returns false otherwise.
func (ch *channel) removeClient(client *client) bool {
	ch.Lock()
	defer ch.Unlock()
	delete(ch.clients, client.address)
	if len(ch.clients) == 0 {
		return true
	}
	return false
}

// stop makes the channel stop for processing messages.
func (ch *channel) stop() {
	ch.Lock()
	defer ch.Unlock()
	close(ch.out)
	ch.clients = nil
}

// messageInfo is a simple wrapper to wrap the sender of a message to the
// message in question.
type messageInfo struct {
	msg     []byte
	address string
}
