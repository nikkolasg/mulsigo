package network

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wrongMessage struct {
	Val int
}

type TestRegisterS struct {
	I int
}

func TestEncoderRegistry(t *testing.T) {
	if registry.isRegistered(getValueType(&TestRegisterS{})) {
		t.Error("TestRegister should not yet be there")
	}

	RegisterMessage(10, &TestRegisterS{})
	if !registry.isRegistered(getValueType(&TestRegisterS{})) {
		t.Error("TestRegister should be there")
	}
	ty, ok := registry.get(10)
	assert.True(t, ok)
	if ttt := reflect.TypeOf(TestRegisterS{}); ttt != ty {
		t.Error("wrong type" + ttt.String() + ty.String())
	}

	id, ok := registry.getId(ty)
	assert.Equal(t, id, MessageID(10))
	assert.True(t, ok)
}

func TestEncoderSingleProto(t *testing.T) {
	enc := NewSingleProtoEncoder(basicMessage{})

	msg := &basicMessage{10}
	_, err := enc.Marshal(&wrongMessage{})
	assert.Error(t, err)

	b, err := enc.Marshal(msg)
	assert.Nil(t, err)
	assert.NotNil(t, b)

	m, err := enc.Unmarshal(b)
	assert.Nil(t, err)
	bm, ok := m.(*basicMessage)
	assert.True(t, ok)
	assert.Equal(t, msg.Value, bm.Value)

	_, err = enc.Unmarshal(append([]byte{}, []byte{1, 2, 3, 4}...))
	assert.Error(t, err)
}

func TestEncoderMultiProto(t *testing.T) {
	enc := NewMultiProtoEncoder()
	buff, err := enc.Marshal(&basicMessage{10})
	assert.Nil(t, buff)
	assert.Error(t, err)
	var id MessageID = 10
	RegisterMessage(id, &basicMessage{})

	buff, err = enc.Marshal(&basicMessage{10})
	assert.NotNil(t, buff)
	assert.Nil(t, err)

	wrongBuff := make([]byte, len(buff))
	m, err := enc.Unmarshal(wrongBuff)
	assert.Nil(t, m)
	assert.Error(t, err)

	m, err = enc.Unmarshal(buff)
	assert.Nil(t, err)
	bm, ok := m.(*basicMessage)
	require.True(t, ok)
	assert.Equal(t, 10, bm.Value)
}
