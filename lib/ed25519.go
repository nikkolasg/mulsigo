// ed25519 implements all the necessary constant time operations
// for scalars and  points on the ed25519 curve.
// This code is heavily inspired from https://github.com/agl/ed25519.
package lib

import (
	"crypto/sha512"
	"io"

	"github.com/agl/ed25519/edwards25519"
	"github.com/dedis/crypto/nist"
)

const (
	PublicSize  = 32
	PrivateSize = 64
	ScalarSize  = 32
)

type Public [PublicSize]byte

type Private [PrivateSize]byte

// prime order of base point = 2^252 + 27742317777372353535851937790883648493
var primeOrder, _ = new(nist.Int).SetString("7237005577332262213973186563042994240857116359379907606001950938285454250989", "", 10)

// NewKeyPair returns a newly fresh ed25519 key pair using the reader
// given in argument. If the reader is nil, it takes the default crypto.Rand.
func NewKeyPair(reader io.Reader) (*Public, *Private, error) {
	var privateKey Private
	var publicKey Public
	var pub32 [32]byte
	err := RandomBytes(reader, privateKey[:32])
	if err != nil {
		return nil, nil, err
	}

	h := sha512.New()
	h.Write(privateKey[:32])
	digest := h.Sum(nil)

	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	copy(hBytes[:], digest)
	edwards25519.GeScalarMultBase(&A, &hBytes)
	A.ToBytes(&pub32)

	copy(privateKey[32:], pub32[:])
	copy(publicKey[:], pub32[:])
	return &publicKey, &privateKey, nil
}

// Scalar returns the scalar part of the private key
func (p *Private) Scalar() Scalar {
	sc, err := NewScalarFromBytes(p[:32])
	if err != nil {
		panic("Wrong private key")
	}
	return sc
}

type Scalar struct {
	*nist.Int
}

func NewScalar() Scalar {
	i := nist.NewInt64(0, &primeOrder.V)
	i.BO = nist.LittleEndian
	return Scalar{i}
}
func NewScalarFromBytes(buff []byte) (Scalar, error) {
	// put the buff in big endian as the default in nist.Int
	var cop = make([]byte, len(buff))
	copy(cop, buff)
	err := Reverse(cop, cop)
	if err != nil {
		return Scalar{}, err
	}
	i := nist.NewIntBytes(cop, &primeOrder.V)
	i.BO = nist.LittleEndian
	return Scalar{i}, nil
}
