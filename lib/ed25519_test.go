package lib

import (
	"bytes"
	"testing"

	"github.com/alecthomas/assert"
)

func TestKeyPair(t *testing.T) {
	// mock reader
	seed := []byte("The important thing is not to stop questioning. Curiosity has its own reason for existing.")
	read := bytes.NewBuffer(seed)
	pub, priv, err := NewKeyPair(read)
	assert.Nil(t, err)
	read = bytes.NewBuffer(seed)
	pub2, priv2, err := NewKeyPair(read)
	assert.Nil(t, err)

	assert.Equal(t, pub, pub2)
	assert.Equal(t, priv, priv2)

	pub2, priv2, err = NewKeyPair(nil)
	assert.NotEqual(t, pub, pub2)
	assert.NotEqual(t, priv, priv2)
}
