// This file implements the polynomial sharing code.
// Heavily inspired from github.com/dedis/crypto/poly/sharing.go
package lib

import (
	"errors"
	"io"
	"sync"
)

type Poly struct {
	// public key
	public Point
	// coeffs of the polynomial which are scalar on 32 bytes
	coeffs []Scalar
	// degree of the polynomial
	k uint32
}

type Share struct {
	// the committed secret of the polynomial, i.e. the public part.
	Public Point
	// the polynomial is evaluated at f(Xcoord)
	Xcoord uint32
	// the scalar representing the share
	Sc Scalar
}

// NewPoly creates a polynomial with
// - secret as the first coefficient
// - degree k
// - reader to pick random coef. If nil, pick crypto.Rand
func NewPoly(reader io.Reader, secret Scalar, public Point, k uint32) (*Poly, error) {
	var coeffs = make([]Scalar, k)
	coeffs[0] = secret
	for i := 1; i < int(k); i++ {
		var c [ScalarSize]byte
		err := RandomBytes(reader, c[:])
		if err != nil {
			return nil, err
		}
		sc := NewScalar()
		sc.SetBytes(c[:])
		if err != nil {
			return nil, err
		}
		coeffs[i] = sc
	}
	return &Poly{
		public: public,
		coeffs: coeffs,
		k:      k,
	}, nil
}

func (p *Poly) Secret() Scalar {
	return p.coeffs[0]
}

// Share evaluates the polynomial at i.
func (p *Poly) Share(i uint32) Share {
	acc := NewScalar()
	x := NewScalar()
	x.SetInt64(int64(1 + i))
	for j := int(p.k - 1); j >= 0; j-- {
		acc.Mul(acc.Int, x.Int)
		acc.Add(acc.Int, p.coeffs[j].Int)
	}
	return Share{
		Public: p.public,
		Xcoord: i + 1,
		Sc:     acc,
	}
}

// Reconstruct takes a list of shares, the threshold and the max number of
// shares and tries to reconstruct the original secret using lagrange
// interpolation
func Reconstruct(shares []Share, t, n uint32) (Scalar, error) {
	if len(shares) < int(t) {
		return Scalar{}, errors.New("Not enough shares to reconstruct")
	}
	sharesTaken := shares[:t]
	xCoords := make([]Scalar, len(sharesTaken))
	for i, sh := range sharesTaken {
		x := NewScalar()
		x.SetInt64(int64(sh.Xcoord))
		xCoords[i] = x
	}
	acc := NewScalar()
	acc.SetInt64(0)
	var num = NewScalar()
	var den = NewScalar()
	var tmp = NewScalar()
	// only need the first t shares
	for i, sh := range sharesTaken {
		num.Set(sh.Sc.Int)
		den = NewScalar()
		den.SetInt64(1)
		for j, _ := range sharesTaken {
			if i == j {
				continue
			}
			// j / (j -i)
			num.Mul(num.Int, xCoords[j].Int)
			den.Mul(den.Int, tmp.Sub(xCoords[j].Int, xCoords[i].Int))
		}
		acc.Add(acc.Int, num.Div(num.Int, den.Int))
	}
	return acc, nil
}

// XXX on HOLD
// DistPoly stands for Distributed Polynomial, just because I can't find a good
// struct name for what it does:
// - Create / Maintains a matrix of Poly
// - Generate all shares for this node to help construct the distributed secret
//      N1    N2    N3    N4 ... Nn
// N1 + s11   s12   s13   s14 .. s1n   Pub1
// N2 + s21   s22   s23   s24 .. s2n   Pub2
// ...
// NN + sn1   sn2   sn3   sn4 .. snn   PubN
// N1 = S1    S2    S3    S4     SN    Dist. Pub
// where S(n) is the share of the distributed secret.
type DistPoly struct {
	// Threshold to recover the distributed secret
	t uint32
	// number of participants
	n uint32
	// accumulator of share received needed to construct one distributed share
	shares []Share
	// the distributed share of this node
	disShare Share
	// the distributed public key
	disPublic Point
	// to make things thread safe
	disMut sync.Mutex
}

type DistShare struct {
}
