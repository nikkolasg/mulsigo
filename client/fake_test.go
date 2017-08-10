// this file implements many interfaces defined with channels implementations so
// no connections / ports needs to be opened. real test will (?) be made with
// integration tests
package client

import (
	"crypto/rand"
	"errors"
	"strconv"
	"sync"

	"github.com/nikkolasg/mulsigo/relay"
)

var count int

func FakePrivateIdentity() (*Private, *Identity) {
	name := "John-" + strconv.Itoa(count)
	count++
	priv, id, err := NewPrivateIdentity(name, rand.Reader)
	if err != nil {
		panic(err)
	}
	return priv, id
}

func BatchPrivateIdentity(n int) ([]*Private, []*Identity) {
	var privs = make([]*Private, n)
	var pubs = make([]*Identity, n)
	for i := 0; i < n; i++ {
		privs[i], pubs[i] = FakePrivateIdentity()
	}
	return privs, pubs
}

type globalStreamFactory struct {
	//            id    stream
	streams map[string]*fakeStreamPair
	sync.Mutex
}

//
func NewGlobalStreamFactory() *globalStreamFactory {
	fsf := &globalStreamFactory{
		streams: make(map[string]*fakeStreamPair),
	}
	return fsf
}

type localStreamFactory struct {
	*globalStreamFactory
	own *Identity
}

func (f *globalStreamFactory) Sub(own *Identity) *localStreamFactory {
	return &localStreamFactory{
		globalStreamFactory: f,
		own:                 own,
	}
}

func (f *globalStreamFactory) streamPair(id string) *fakeStreamPair {
	f.Lock()
	defer f.Unlock()
	fsp, ok := f.streams[id]
	if !ok {
		fsp = &fakeStreamPair{Cond: sync.NewCond(&sync.Mutex{})}
		f.streams[id] = fsp
	}
	return fsp
}

//
func (f *localStreamFactory) newStream(to *Identity) (stream, error) {
	id, first := channelID(f.own, to)
	fsp := f.globalStreamFactory.streamPair(id)
	stream := &fakeStream{buffers: make(chan []byte, 10)}
	fsp.addAndWait(stream, first)
	return stream, nil
}

type fakeStreamPair struct {
	f1, f2 *fakeStream
	done   bool
	*sync.Cond
}

func (f *fakeStreamPair) addAndWait(s *fakeStream, first bool) {
	f.L.Lock()
	if first {
		f.f1 = s
		for f.f2 == nil {
			f.Wait()
		}
		s.remote = f.f2
	} else {
		f.f2 = s
		for f.f1 == nil {
			f.Wait()
		}
		s.remote = f.f1
	}

	f.L.Unlock()
	f.Signal()
}

type fakeStream struct {
	buffers chan []byte
	remote  *fakeStream
}

func NewFakeStreamsPair() (*fakeStream, *fakeStream) {
	f1 := &fakeStream{buffers: make(chan []byte, 10)}
	f2 := &fakeStream{buffers: make(chan []byte, 10)}
	f1.remote = f2
	f2.remote = f1
	return f1, f2
}

func (f *fakeStream) send(buff []byte) error {
	f.remote.buffers <- buff
	return nil
}

func (f *fakeStream) receive() ([]byte, error) {
	b, ok := <-f.buffers
	if !ok {
		return nil, errors.New("fakestream closed")
	}
	return b, nil
}

func (f *fakeStream) close() {
	//
}

type fakeChannel struct {
	addr   string
	own    chan relay.Egress
	remote *fakeChannel
}

func createFakeChannelPair() (relay.Channel, relay.Channel) {
	c1 := &fakeChannel{addr: cAddr1.String(), own: make(chan relay.Egress, 50)}
	c2 := &fakeChannel{addr: cAddr2.String(), own: make(chan relay.Egress, 50)}
	c1.remote = c2
	c2.remote = c1
	return c1, c2
}

func (f *fakeChannel) Send(b []byte) error {
	f.remote.own <- relay.Egress{
		Address: f.addr,
		Blob:    b,
	}
	return nil
}

func (f *fakeChannel) Receive() (string, []byte, error) {
	e := <-f.own
	return e.GetAddress(), e.GetBlob(), nil
}

func (f *fakeChannel) Id() string {
	return "fake"
}

func (f *fakeChannel) Close() {
	close(f.own)
}
