// ed25519 implements all the necessary constant time operations
// for scalars and  points on the ed25519 curve.
// This code is heavily inspired from https://github.com/agl/ed25519.
package lib

import (
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
// The private key has the same format as in NaCl (according to
// https://blog.mozilla.org/warner/2011/11/29/ed25519-keys/)
// Private = private key pk with bit/clearing (32bytes) || scalar (32bytes)
// Public  = pk * G
func NewKeyPair(reader io.Reader) (*PublicKey, *PrivateKey, error) {
	var seed [32]byte
	err := RandomBytes(reader, seed[:])
	if err != nil {
		return nil, nil, err
	}
	var privateKey PrivateKey
	var publicKey PublicKey

	digest := sha512.Sum512(seed[:])

	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	copy(privateKey[:], digest[:])

	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	copy(hBytes[:], digest[:32])
	edwards25519.GeScalarMultBase(&A, &hBytes)
	var pub32 [32]byte
	A.ToBytes(&pub32)

	copy(publicKey[:], pub32[:])
	return &publicKey, &privateKey, nil
}

func NewKeyPairReduced(reader io.Reader) (*PublicKey, *PrivateKey, error) {
	var seed [32]byte
	err := RandomBytes(reader, seed[:])
	if err != nil {
		return nil, nil, err
	}
	var privateKey PrivateKey
	var publicKey PublicKey

	digest := sha512.Sum512(seed[:])

	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	var privateReduced [32]byte

	copy(privateReduced[:], digest[:32])
	privateInt := nist.NewIntBytes(privateReduced[:], &primeOrder.V)
	privateInt.BO = nist.LittleEndian

	privateIntBuff := privateInt.LittleEndian(32, 32)
	if len(privateIntBuff) != 32 {
		panic("Aie")
	}

	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	copy(hBytes[:], privateIntBuff[:32])
	edwards25519.GeScalarMultBase(&A, &hBytes)
	var pub32 [32]byte
	A.ToBytes(&pub32)

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

// Bytes always return a 32 byte representation of this scalar.
func (s *Scalar) Bytes() []byte {
	return s.LittleEndian(32, 32)
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
// XXX HUGE: PrivateKey IS NOT USABLE TO SIGN
func (s *Scalar) Commit() (*PublicKey, error) {
	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	var publicKey PublicKey
	var pub32 [32]byte
	var buff = s.Bytes()

	copy(hBytes[:], buff[:])
	//fmt.Println("Commit() private:", hBytes)
	edwards25519.GeScalarMultBase(&A, &hBytes)
	A.ToBytes(&pub32)

	copy(publicKey[:], pub32[:])
	return &publicKey, nil
}
