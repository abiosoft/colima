package shautil

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
)

// SHA is a sha computation
type SHA interface {
	String() string
	Bytes() []byte
}

type s1 [20]byte

func (s s1) String() string { return fmt.Sprintf("%x", s[:]) }
func (s s1) Bytes() []byte  { return s[:] }

type s256 [32]byte

func (s s256) String() string { return fmt.Sprintf("%x", s[:]) }
func (s s256) Bytes() []byte  { return s[:] }

// SHA256Hash computes a sha256sum of a string.
func SHA256(s string) SHA {
	return s256(sha256.Sum256([]byte(s)))
}

// SHA256Hash computes a sha256sum of a string.
func SHA1(s string) SHA {
	return s1(sha1.Sum([]byte(s)))
}
