// ed25519 implements all the necessary constant time operations
// for scalars and  points on the ed25519 curve.
// This code is heavily inspired from https://github.com/agl/ed25519 and from
// https://github.com/dedis/crypto.
package main

import (
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/agl/ed25519/edwards25519"
	"github.com/dedis/crypto/nist"
)

const (
	PublicSize    = 32
	PrivateSize   = 64
	ScalarSize    = 32
	SignatureSize = 64
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

	privateInt := NewScalar()
	privateInt.SetBytes(privateReduced[:])
	privateIntBuff, _ := privateInt.MarshalBinary()
	if len(privateIntBuff) != 32 {
		panic("Aie")
	}
	copy(privateKey[:], privateIntBuff[:32])
	copy(privateKey[32:], digest[32:])
	fmt.Println("NewKeyPairReduced() ", hex.EncodeToString(privateKey[:]))

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
	sc := NewScalar()
	sc.SetBytes(p[:32])
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

func NewPoint() Point {
	ege := new(edwards25519.ExtendedGroupElement)
	return Point{ege}
}

func (p *Point) Public() *PublicKey {
	var pub [PublicSize]byte
	p.ToBytes(&pub)
	pk := PublicKey(pub)
	return &pk
}

func (p *Point) Bytes() []byte {
	var pub = p.Public()
	return pub[:]
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

// Commit returns s * G with G being the base point of the curve.
func (s *Scalar) Commit() Point {
	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	var buff = s.Bytes()

	copy(hBytes[:], buff[:])
	edwards25519.GeScalarMultBase(&A, &hBytes)

	return Point{&A}
}

func (s *Scalar) CommitPublic() *PublicKey {
	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	var buff = s.Bytes()
	var pub32 [32]byte

	copy(hBytes[:], buff[:])
	edwards25519.GeScalarMultBase(&A, &hBytes)
	A.ToBytes(&pub32)

	pk := PublicKey(pub32)
	return &pk
}

// SchnorrSign signs the message using the given secret scalar.
// The signature algorithm follows the ideas from
// + Eddsa https://tools.ietf.org/html/draft-irtf-cfrg-eddsa-05#page-12
// + Xeddsa https://whispersystems.org/docs/specifications/xeddsa/
// + CoSi https://arxiv.org/pdf/1503.08768v4.pdf
// It's a non-deterministic signature scheme.
// random must be able to generate 64 bytes of random bytes. If it's nil,
// crypto.Rand is used instead.
func SchnorrSign(secret Scalar, msg []byte, random io.Reader) [SignatureSize]byte {
	var Z [64]byte
	var sig [64]byte
	var public = secret.CommitPublic()
	RandomBytes(random, Z[:])
	// r = H( secret || A  || Z )
	h1 := sha512.New()
	h1.Write(secret.Bytes())
	h1.Write(public[:])
	h1.Write(Z[:])
	r := NewScalar()
	r.SetBytes(h1.Sum(nil))
	R := r.Commit()

	// k = H( R || A || M)
	h2 := sha512.New()
	h2.Write(R.Bytes())
	h2.Write(public[:])
	h2.Write(msg)
	k := NewScalar()
	k.SetBytes(h2.Sum(nil))

	// s = r + k * secret
	// TODO replace by const time
	s := k.Mul(k.Int, secret.Int).Add(k.Int, r.Int)

	// sig = R || s
	copy(sig[:32], R.Bytes())
	copy(sig[32:], s.Bytes())
	return sig
}

func SchnorrVerify(public *PublicKey, msg []byte, sig [SignatureSize]byte) bool {

	if sig[63]&224 != 0 {
		fmt.Println("Something's wrong or not")
		return false
	}
	// Reconstruct R & s
	var R [32]byte
	var s [32]byte
	var A = NewPoint()

	copy(R[:], sig[:32])
	copy(s[:], sig[32:])

	// Set A to its negative
	ab := [32]byte(*public)
	if !A.FromBytes(&ab) {
		return false
	}
	edwards25519.FeNeg(&A.X, &A.X)
	edwards25519.FeNeg(&A.T, &A.T)

	// reconstruct k = H( R || A || Msg)
	h := sha512.New()
	h.Write(R[:])
	h.Write(public[:])
	h.Write(msg)
	var digest [64]byte
	copy(digest[:], h.Sum(nil))
	var kReduced [32]byte
	edwards25519.ScReduce(&kReduced, &digest)

	// checks s*B + k*(-A) = Rcheck =?= R
	var RCheck = new(edwards25519.ProjectiveGroupElement)
	edwards25519.GeDoubleScalarMultVartime(RCheck, &kReduced, A.ExtendedGroupElement, &s)
	var RCheckBuff [32]byte
	RCheck.ToBytes(&RCheckBuff)

	return subtle.ConstantTimeCompare(RCheckBuff[:], R[:]) == 1
}
