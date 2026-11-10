// Package sha1 implements the SHA-1 hash algorithm used by vstodo
// (and by git) for object addressing. Objects are addressed by the
// SHA-1 of their header-prefixed content: "<type> <size>\0<content>".
package sha1

import (
	"crypto/sha1"
	"fmt"
	"hash"
)

// Size is the byte length of a SHA-1 digest (20 bytes, 40 hex chars).
const Size = sha1.Size

// Digest is a 20-byte SHA-1 hash.
type Digest [Size]byte

// Zero is the empty digest.
var Zero Digest

// Hasher wraps crypto/sha1 to implement the vstodo Hasher interface.
type Hasher struct {
	h hash.Hash
}

// NewHasher returns a fresh Hasher.
func NewHasher() *Hasher { return &Hasher{h: sha1.New()} }

func (h *Hasher) Write(p []byte) (int, error) { return h.h.Write(p) }

func (h *Hasher) Sum(b []byte) []byte { return h.h.Sum(b) }

// Reset clears the hasher state.
func (h *Hasher) Reset() { h.h.Reset() }

// SumDigest returns the current digest.
func (h *Hasher) SumDigest() Digest {
	var d Digest
	b := h.h.Sum(nil)
	copy(d[:], b)
	return d
}

// Hex encodes the digest as a 40-character lowercase hex string.
func (d Digest) Hex() string { return fmt.Sprintf("%x", d[:]) }

// String returns the hex form.
func (d Digest) String() string { return d.Hex() }

// FromHex parses a 40-char hex string into a Digest.
func FromHex(s string) (Digest, error) {
	if len(s) != Size*2 {
		return Zero, fmt.Errorf("sha1: invalid hex length %d, want %d", len(s), Size*2)
	}
	var d Digest
	for i := 0; i < Size; i++ {
		hi, err := hexVal(s[2*i])
		if err != nil {
			return Zero, err
		}
		lo, err := hexVal(s[2*i+1])
		if err != nil {
			return Zero, err
		}
		d[i] = hi<<4 | lo
	}
	return d, nil
}

func hexVal(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	default:
		return 0, fmt.Errorf("sha1: invalid hex char %q", c)
	}
}

// FromBytes copies 20 bytes into a Digest.
func FromBytes(b []byte) (Digest, error) {
	if len(b) != Size {
		return Zero, fmt.Errorf("sha1: need %d bytes, got %d", Size, len(b))
	}
	var d Digest
	copy(d[:], b)
	return d, nil
}

// Bytes returns the raw 20 bytes.
func (d Digest) Bytes() []byte { return d[:] }

// Prefix returns the first n bytes used for loose-object directory naming.
func (d Digest) Prefix(n int) string { return d.Hex()[:n] }
