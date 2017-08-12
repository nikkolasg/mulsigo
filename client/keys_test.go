package client

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/agl/ed25519/edwards25519"
	"golang.org/x/crypto/ed25519"

	"github.com/stretchr/testify/require"
)

func TestKeys(t *testing.T) {
	var name = "winston smith"
	priv, id, err := NewPrivateIdentity(name, rand.Reader)
	require.Nil(t, err)
	require.NotNil(t, priv)
	require.NotNil(t, id)

	// test public private relationship
	buffFromSeed := (*priv.seed)[32:]
	buffFromPublic := id.Key
	require.Equal(t, []byte(buffFromSeed), []byte(buffFromPublic))
	privScalar := priv.Scalar()
	pubPoint := Group.Point()
	pubPoint.Mul(privScalar, nil)
	buffFromPoint, err := pubPoint.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, []byte(buffFromSeed), buffFromPoint)

	// test conversion to curve25519
	pubCurveFromPriv := priv.PublicCurve25519()
	pubCurveFromPub := id.PublicCurve25519()
	require.Equal(t, pubCurveFromPub, pubCurveFromPriv)

}

func TestKeysConversionKyber(t *testing.T) {
	pub, privEd, err := ed25519.GenerateKey(rand.Reader)
	pprivEd := &privEd
	require.Nil(t, err)

	h := sha512.New()
	h.Write((*pprivEd)[:32])
	digest := h.Sum(nil)

	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	v1Scalar := Group.Scalar()
	var reducedDigest [32]byte
	var digest32 [32]byte
	copy(digest32[:], digest)

	require.Nil(t, v1Scalar.UnmarshalBinary(digest32[:]))
	v1Public := Group.Point()
	v1Public = v1Public.Mul(v1Scalar, nil)
	buff, err := v1Public.MarshalBinary()

	var ege edwards25519.ExtendedGroupElement
	edwards25519.GeScalarMultBase(&ege, &reducedDigest)
	var buffPublic [32]byte
	ege.ToBytes(&buffPublic)
	fmt.Println("agl:" + hex.EncodeToString(pub))
	fmt.Println("v1: " + hex.EncodeToString(buff))
	require.Equal(t, hex.EncodeToString(buff), hex.EncodeToString(pub))

}
