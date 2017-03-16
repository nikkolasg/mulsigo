package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testEventName = "TestEvent"

type TestEvent struct {
	Val int
}

func (tt *TestEvent) Name() string {
	return testEventName
}

type TestProcessor struct {
	chEvent chan Event
}

func newTestProcessor() *TestProcessor {
	return &TestProcessor{
		chEvent: make(chan Event),
	}
}

func (tp *TestProcessor) Receive(e Event) {
	tp.chEvent <- e
}

func TestSimpleDispatcherRegistration(t *testing.T) {
	d := NewSimpleDispatcher()
	proc := newTestProcessor()

	d.Register(testEventName, proc)
	assert.Equal(t, len(d.registered[testEventName]), 1)
	assert.Equal(t, d.registered[testEventName][0], proc)

	d.Register(testEventName, proc)
	assert.Equal(t, len(d.registered[testEventName]), 1)
	assert.Equal(t, d.registered[testEventName][0], proc)

	d.Register(testEventName+"a", proc)

	d.Unregister(testEventName, proc)

	assert.Equal(t, len(d.registered[testEventName]), 0)
	assert.Equal(t, len(d.registered[testEventName+"a"]), 1)
	assert.Equal(t, d.registered[testEventName+"a"][0], proc)
}

func TestSimpleDispatcherDispatch(t *testing.T) {
	d := NewSimpleDispatcher()
	proc := newTestProcessor()

	d.Register(testEventName, proc)

	go d.Publish(&TestEvent{10})

	select {
	case ev := <-proc.chEvent:
		tev, ok := ev.(*TestEvent)
		require.True(t, ok)
		assert.Equal(t, 10, tev.Val)
	case <-time.After(time.Millisecond * 50):
		t.Fail()
	}
}
