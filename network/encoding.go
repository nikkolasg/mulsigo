package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"sync"

	"github.com/dedis/protobuf"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/ed25519"
)

// Suite used globally by this network library.
// For the moment, this will stay,as our focus is not on having the possibility
// to use any suite we want (the decoding stuff is much harder then, because we
// don't want to send the suite in the wire).
// It will surely change in futur releases so we can permit this behavior.
var Suite = ed25519.NewAES128SHA256Ed25519(false)

// Message is a type for any message that the user wants to send
type Message interface{}

var globalOrder = binary.BigEndian

// Encoder's role is to marshal and unmarshal messages from the network layer.
// Different encoding techniques can be easily used with this generic
// interface.
type Encoder interface {
	// Marshal takes a  message and returns the corresponding encoding.
	// The msg must be a POINTER to the message.
	Marshal(msg Message) ([]byte, error)
	// Unmarshal takes a slice of bytes and returns the corresponding message
	// and its type. The caller is responsible to give the right slice length so
	// the Encoder can decode. It returns a POINTER to the message.
	Unmarshal([]byte) (Message, error)
}

// SingleProtoEncoder is a struct that encodes and decodes a unique message using
// protobuf.  This encoder is useful when the whole message set can be contained
// in a single wrapper struct that protobuf can decode.
type SingleProtoEncoder struct {
	t reflect.Type
}

func NewSingleProtoEncoder(msg Message) *SingleProtoEncoder {
	t := getValueType(msg)
	return &SingleProtoEncoder{t}
}

func (m *SingleProtoEncoder) Marshal(msg Message) ([]byte, error) {
	if t := getValueType(msg); t != m.t {
		return nil, fmt.Errorf("monoencoder: can't encode %s", t.String())
	}
	return protobuf.Encode(msg)
}

func (m *SingleProtoEncoder) Unmarshal(buff []byte) (Message, error) {
	ptrVal := reflect.New(m.t)
	ptr := ptrVal.Interface()
	constructors := defaultConstructors(Suite)
	if err := protobuf.DecodeWithConstructors(buff, ptr, constructors); err != nil {
		return nil, err
	}

	return ptrVal.Interface(), nil
}

// DefaultConstructors gives a default constructor for protobuf out of the global suite
func defaultConstructors(suite abstract.Suite) protobuf.Constructors {
	constructors := make(protobuf.Constructors)
	var point abstract.Point
	var secret abstract.Scalar
	constructors[reflect.TypeOf(&point).Elem()] = func() interface{} { return suite.Point() }
	constructors[reflect.TypeOf(&secret).Elem()] = func() interface{} { return suite.Scalar() }
	return constructors
}

func getValueType(m Message) reflect.Type {
	val := reflect.ValueOf(m)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val.Type()
}

// MULTI STRUCT ENCODING PART:
type MultiProtoEncoder struct{}

func NewMultiProtoEncoder() Encoder {
	return &MultiProtoEncoder{}
}

func (m *MultiProtoEncoder) Marshal(msg Message) ([]byte, error) {
	var t = getValueType(msg)

	id, ok := registry.getId(t)
	if !ok {
		return nil, fmt.Errorf("multiprotoencoder: %s not registered", t.String())
	}

	b := new(bytes.Buffer)
	if err := binary.Write(b, globalOrder, id); err != nil {
		return nil, err
	}
	enc := SingleProtoEncoder{t}
	buff, err := enc.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(b.Bytes(), buff...), nil
}

func (m *MultiProtoEncoder) Unmarshal(buff []byte) (Message, error) {
	b := bytes.NewBuffer(buff)
	var id MessageID
	if err := binary.Read(b, globalOrder, &id); err != nil {
		return nil, err
	}
	typ, ok := registry.get(id)
	if !ok {
		return nil, fmt.Errorf("multiprotoencoder: type %d not registered", id)
	}

	enc := SingleProtoEncoder{typ}
	return enc.Unmarshal(b.Bytes())
}

type MessageID uint32

func RegisterMessage(id MessageID, msg Message) {
	msgType := getValueType(msg)
	registry.put(id, msgType)
}

var registry = newIDRegistry()

type idRegistry struct {
	types map[MessageID]reflect.Type
	inv   map[reflect.Type]MessageID
	sync.Mutex
}

func newIDRegistry() *idRegistry {
	return &idRegistry{
		types: make(map[MessageID]reflect.Type),
		inv:   make(map[reflect.Type]MessageID),
	}
}

// get returns the reflect.Type corresponding to the registered PacketTypeID
// an a boolean indicating if the type is actually registered or not.
func (tr *idRegistry) get(mid MessageID) (reflect.Type, bool) {
	tr.Lock()
	defer tr.Unlock()
	t, ok := tr.types[mid]
	return t, ok
}

func (tr *idRegistry) isRegistered(t reflect.Type) bool {
	_, ok := tr.getId(t)
	return ok
}

// put stores the given type in the idRegistry.
func (tr *idRegistry) put(mid MessageID, typ reflect.Type) {
	tr.Lock()
	defer tr.Unlock()
	tr.types[mid] = typ
	tr.inv[typ] = mid
}

func (tr *idRegistry) getId(t reflect.Type) (MessageID, bool) {
	tr.Lock()
	defer tr.Unlock()
	m, ok := tr.inv[t]
	return m, ok
}
