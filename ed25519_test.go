package main

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, pub, pubRec)
}

func TestEd25519ScalarCommit(t *testing.T) {
	pub, priv, err := NewKeyPair(seed())
	require.Nil(t, err)
	scalar := priv.Scalar()
	pubp := scalar.Commit()
	pub2 := scalar.CommitPublic()

	assert.Equal(t, pub, pub2)
	assert.Equal(t, pub, pubp.Public())
}

func TestEd25519ScalarMarshalling(t *testing.T) {
	sc1 := NewScalar()

	rev := make([]byte, len(_seed))
	Reverse(_seed, rev)
	sc1.SetBytes(rev)

	buff, _ := sc1.MarshalBinary()
	buff2 := sc1.Bytes()

	sc2p := NewScalar()
	sc2p.SetBytes(buff2)
	sc2 := NewScalar()
	sc2.SetBytes(buff)

	sc3 := NewScalar()
	sc3.UnmarshalBinary(buff)

	sc4 := NewScalar()
	sc4.SetBytes(buff)

	assert.True(t, sc1.Equal(sc2.Int))
	assert.True(t, sc1.Equal(sc2.Int))
	assert.True(t, sc1.Equal(sc3.Int))
	assert.True(t, sc1.Equal(sc4.Int))
}

func TestEd25519SignVerify(t *testing.T) {
	pub, priv, err := NewKeyPair(nil)
	require.Nil(t, err)

	var msg = []byte("Hello World!\n")

	sig := SchnorrSign(priv.Scalar(), msg, nil)

	assert.True(t, SchnorrVerify(pub, msg, sig))

}
