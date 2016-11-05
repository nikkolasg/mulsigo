// ed25519 implements all the necessary constant time operations
// for scalars and  points on the ed25519 curve.
// This code is heavily inspired from https://github.com/agl/ed25519.
package lib

import (
	"bytes"
	"crypto/sha512"
	"crypto/subtle"
	"io"

	"github.com/agl/ed25519/edwards25519"
	"github.com/dedis/crypto/nist"
)

const (
	PublicSize  = 32
	PrivateSize = 64
	ScalarSize  = 32
)

type PublicKey [PublicSize]byte

type PrivateKey [PrivateSize]byte

// prime order of base point = 2^252 + 27742317777372353535851937790883648493
var primeOrder, _ = new(nist.Int).SetString("7237005577332262213973186563042994240857116359379907606001950938285454250989", "", 10)

// NewKeyPair returns a newly fresh ed25519 key pair using the reader
// given in argument. If the reader is nil, it takes the default crypto.Rand.
func NewKeyPair(reader io.Reader) (*PublicKey, *PrivateKey, error) {
	var privateKey PrivateKey
	err := RandomBytes(reader, privateKey[:32])
	if err != nil {
		return nil, nil, err
	}
	var publicKey PublicKey
	var pub32 [32]byte

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
func (p *PrivateKey) Scalar() Scalar {
	sc, err := NewScalarFromBytes(p[:32])
	if err != nil {
		panic("Wrong private key")
	}
	return sc
}

func (p *PrivateKey) Equal(p2 *PrivateKey) bool {
	return subtle.ConstantTimeCompare(p[:], p2[:]) == 1
}

func (p *PublicKey) Point() Point {
	ge := new(edwards25519.ExtendedGroupElement)
	b := [32]byte(*p)

	ok := ge.FromBytes(&b)
	if !ok {
		panic("This public key is wrong, don't do anything with it.")
	}
	return Point{ge}
}

func (p *PublicKey) Equal(p2 *PublicKey) bool {
	return subtle.ConstantTimeCompare(p[:], p2[:]) == 1
}

// Point represents a point on the ed25519 curve
type Point struct {
	*edwards25519.ExtendedGroupElement
}

func (p *Point) Public() PublicKey {
	var pub [PublicSize]byte
	p.ToBytes(&pub)
	return PublicKey(pub)
}

// Scalar is a scalar represented on 32 bytes and is always reduced modulo
// prime order of base point = 2^252 + 27742317777372353535851937790883648493
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

// KeyPair returns a ed25519/eddsa public / private key pair from the scalar.
// Namely, it treats the scalar as the output of the prng.
func (s *Scalar) KeyPair() (*PublicKey, *PrivateKey, error) {
	buff := s.Bytes()
	return NewKeyPair(bytes.NewBuffer(buff))
}
