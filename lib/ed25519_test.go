package lib

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/alecthomas/assert"
	"github.com/stretchr/testify/require"
)

var _seed = []byte("The important thing is not to stop questioning. Curiosity has its own reason for existing.")

func seed() io.Reader {
	return bytes.NewBuffer(_seed)
}
func TestEd25519KeyPair(t *testing.T) {
	// mock reader
	pub, priv, err := NewKeyPair(seed())
	require.Nil(t, err)

	pub2, priv2, err := NewKeyPair(seed())
	require.Nil(t, err)

	assert.Equal(t, pub, pub2)
	assert.Equal(t, priv, priv2)

	pub2, priv2, err = NewKeyPair(nil)
	require.Nil(t, err)
	assert.NotEqual(t, pub, pub2)
	assert.NotEqual(t, priv, priv2)
}

func TestEd25519PublicPoint(t *testing.T) {
	defer func() {
		if e := recover(); e != nil {
			t.Error(e)
		}
	}()
	pub, _, err := NewKeyPair(nil)
	require.Nil(t, err)

	point := pub.Point()
	pubRec := point.Public()
	assert.Equal(t, pub, &pubRec)
}

func TestEd25519PrivateScalar(t *testing.T) {
	pub, priv, err := NewKeyPair(seed())
	require.Nil(t, err)
	scalar := priv.Scalar()
	pub2, err := scalar.Commit()
	pub3, _, err := NewKeyPairReduced(seed())
	require.Nil(t, err)
	require.Nil(t, err)
	fmt.Println("pub:", pub)
	fmt.Println("pub2:", pub2)
	fmt.Println("pub3:", pub3)

	assert.Equal(t, pub, pub2)
}
