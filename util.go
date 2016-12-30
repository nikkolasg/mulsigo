package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

func Fatal(txt string, formatter ...interface{}) {
	fmt.Fprintf(os.Stderr, txt, formatter)
	os.Exit(1)
}

// RandomBytes fills buff from the given reader or crypto.Rand(omness) if reader
// == nil. If there is any problem or the read did not take len(buff) bytes,
// it returns an error.
func RandomBytes(reader io.Reader, buff []byte) error {
	if reader == nil {
		return fillBuff(rand.Read, buff)
	}
	return fillBuff(reader.Read, buff)
}

func fillBuff(fn func(b []byte) (int, error), buff []byte) error {
	n, err := fn(buff)
	if err != nil {
		return err
	} else if n != len(buff) {
		return fmt.Errorf("Could not read %d bytes from reader")
	}
	return nil
}

// Reverse computes the reverse of src into dst. Both buffers can be the same
// but must not be overlapping otherwise.
func Reverse(src, dst []byte) error {
	if len(src) != len(dst) {
		return errors.New("Reverse can't operate on two different size buffer")
	}
	n := len(src)
	for i := 0; i < n; i++ {
		dst[n-i-1] = src[i]
	}
	return nil
}

func exists(fname string) bool {
	if _, err := os.Stat(fname); os.IsNotExist(err) {
		return false
	}
	return true
}
