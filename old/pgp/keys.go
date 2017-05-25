package pgp

import (
	"bytes"
	"crypto/sha512"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"

	kyber "gopkg.in/dedis/crypto.v0/abstract"

	dedisEd25519 "gopkg.in/dedis/crypto.v0/ed25519"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/openpgp/packet"
)

type GpgKeyInfo struct {
	Name    string
	Comment string
	Email   string
}

func (g *GpgKeyInfo) Batch() string {
	// longish way to do it but need to print % correctly.. ??
	b := new(bytes.Buffer)
	b.WriteString(`%` + "no-protection\n")
	b.WriteString("Key-Type: eddsa\n")
	b.WriteString("Key-Curve: Ed25519\n")
	b.WriteString("Name-Real: " + g.Name + "\n")
	b.WriteString("Name-Comment: " + g.Comment + "\n")
	b.WriteString("Name-Email: " + g.Email + "\n")
	b.WriteString("Expire-Date: 1y\n")
	b.WriteString(`%` + "commit\n")
	b.WriteString(`%` + "echo gpg: mulsigo keygen done")
	return b.String()
}

func createKey(homedir string, g *GpgKeyInfo) ([]byte, error) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(g.Batch()); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	var out bytes.Buffer

	args := []string{"--homedir", homedir, "--expert", "--full-gen-key", "--batch", f.Name()}
	cmd := exec.Command("gpg", args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	return out.Bytes(), err
}

func readKey(homedir string) (*packet.PrivateKey, error) {
	readPrivateKeyArgs := []string{"--homedir", homedir, "--export-secret-keys"}
	cmd := exec.Command("gpg", readPrivateKeyArgs...)
	out := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	p, err := packet.Read(out)
	if err != nil {
		return nil, err
	}

	privKey, ok := p.(*packet.PrivateKey)
	if !ok {
		return nil, errors.New("it's not a private key")
	}
	return privKey, nil
}

var suite = dedisEd25519.NewAES128SHA256Ed25519(false)

// scalarFromSeed hash the seed and does the bit twiddling, and put that as a
// kyber.Scalar (already modulo-d)
func scalarFromSeed(priv ed25519.PrivateKey) kyber.Scalar {
	digest := sha512.Sum512(priv[:32])
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64
	scalar := suite.Scalar()
	//err := scalar.UnmarshalBinary(digest[:])
	scalar.SetBytes(digest[:32])
	return scalar
}
