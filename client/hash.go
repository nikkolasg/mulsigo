package client

import (
	"fmt"
	"hash"

	"github.com/dchest/blake2b"
)

var (
	Blake2 = "blake2"
)

var DefaultHash = Blake2

var SupportedHashes = []string{Blake2}

func NewHash(s string) (hash.Hash, error) {
	switch s {
	case Blake2:
		h, _ := blake2b.New(nil)
		return h, nil
	}
	return nil, fmt.Errorf("hash: %s not known", s)
}
