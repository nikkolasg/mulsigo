package relay

import (
	"errors"
	"sync"

	"github.com/dedis/onet/log"
	net "github.com/nikkolasg/mulsigo/network"
)

// channel holds a list of all participants registered to a channel designated
// by an ID. Each participant can broadcast message to a channel and receive
// message from others on the same channel.
type channel struct {
	id string
	// all clients registered to that channel
	clients []net.Address
	sync.Mutex

	router *net.Router
	out    chan messageInfo
}

// newChannel returns a new channel identified by "id". It launches the
// processing routine.
func newChannel(r *net.Router, id string) *channel {
	ch := &channel{
		id:     id,
		out:    make(chan messageInfo, ChannelQueueSize),
		router: r,
	}
	go ch.process()
	return ch
}

func (ch *channel) broadcast(from net.Address, msg []byte) {
	ch.out <- messageInfo{msg, from}
}

// process reads continuously from the inner channel (Golang) of the channel
// (struct). Each message read, is broadcasted to every participant, except for
// the sender.
func (ch *channel) process() {
	for info := range ch.out {
		msg := info.msg
		addr := info.address
		toBroadcast := &RelayMessage{
			Outgoing: &ChannelOutgoingMessage{
				Channel: ch.id,
				Address: addr.String(),
				Blob:    msg,
			},
		}

		ch.Lock()
		var found bool
		for _, c := range ch.clients {
			if c == addr {
				found = true
			}
		}
		if !found {
			// message coming from a non-registered user: just drop.
			ch.Unlock()
			continue
		}

		for _, c := range ch.clients {
			if c == addr {
				continue
			}
			// XXX change to a more abstract way
			if err := ch.router.Send(c, toBroadcast); err != nil {
				log.Errorf("channel %d: %s: %s", ch.id, c, err)
			}
		}
		ch.Unlock()
	}
}

// addClient takes a client and adds it to the list of registered client for
// this channel. It returns an error if the channel is already full (i.e. more
// than ChannelSize), or if the client is already registered. Otherwise, it
// returns nil.
func (ch *channel) addClient(client net.Address) error {
	ch.Lock()
	defer ch.Unlock()

	if len(ch.clients) >= ChannelSize {
		return errors.New("channel: can't join a full channel")
	}

	for _, c := range ch.clients {
		if c == client {
			return errors.New("channel: already registered")
		}
	}
	ch.clients = append(ch.clients, client)
	return nil
}

// removeClient takes a client and removes it from the list of registered client
// for this channel. It returns true if the channel can be deleted as there is
// no more client left for this channel. It returns false otherwise.
func (ch *channel) removeClient(client net.Address) bool {
	ch.Lock()
	defer ch.Unlock()
	nClients := ch.clients[:0]
	for _, c := range ch.clients {
		if c == client {
			continue
		}
		nClients = append(nClients, c)
	}
	ch.clients = nClients
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
	address net.Address
}
