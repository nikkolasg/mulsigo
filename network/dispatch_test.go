package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type basicProcessor struct {
	envChan chan Message
}

func (bp *basicProcessor) Process(from Address, msg Message) {
	bp.envChan <- msg
}

type basicMessage struct {
	Value int
}

func TestBlockingDispatcher(t *testing.T) {

	dispatcher := NewBlockingDispatcher()
	processor := &basicProcessor{make(chan Message, 1)}
	addr := NewLocalAddress("blou")
	err := dispatcher.Dispatch(addr, &basicMessage{10})

	if err == nil {
		t.Error("Dispatcher should have returned an error")
	}

	dispatcher.RegisterProcessor(processor, basicMessage{})
	dispatcher.Dispatch(addr, &basicMessage{10})

	select {
	case m := <-processor.envChan:
		msg, ok := m.(*basicMessage)
		assert.True(t, ok)
		assert.Equal(t, msg.Value, 10)
	default:
		t.Error("No message received")
	}

	var found bool
	dispatcher.RegisterProcessorFunc(basicMessage{}, func(from Address, msg Message) {
		found = true
	})
	dispatcher.Dispatch(addr, basicMessage{10})

	if !found {
		t.Error("ProcessorFunc should have set to true")
	}
}

func TestRoutineDispatcher(t *testing.T) {

	dispatcher := NewRoutineDispatcher()
	if dispatcher == nil {
		t.Fatal("nil dispatcher")
	}
	processor := &basicProcessor{make(chan Message, 1)}

	addr := NewLocalAddress("blou")
	err := dispatcher.Dispatch(addr, basicMessage{10})

	if err == nil {
		t.Error("Dispatcher should have returned an error")
	}

	dispatcher.RegisterProcessor(processor, &basicMessage{})
	dispatcher.Dispatch(addr, basicMessage{10})

	select {
	case m := <-processor.envChan:
		msg, ok := m.(basicMessage)
		assert.True(t, ok)
		assert.Equal(t, msg.Value, 10)
	case <-time.After(100 * time.Millisecond):
		t.Error("No message received")

	}
}

func TestDefaultProcessor(t *testing.T) {
	var okCh = make(chan bool, 1)
	pr := defaultProcessor{func(from Address, msg Message) {
		okCh <- true
	}}

	pr.Process(NewLocalAddress("blou"), &basicMessage{})
	select {
	case <-okCh:
	default:
		t.Error("no ack received...")
	}
}
