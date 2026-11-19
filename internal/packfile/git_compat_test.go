package packfile

import (
	"bytes"
	"testing"
)

func TestOFSOffsetUsesGitEncoding(t *testing.T) {
	cases := []struct {
		distance uint64
		encoded  []byte
	}{
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x00}},
		{129, []byte{0x80, 0x01}},
		{16511, []byte{0xff, 0x7f}},
		{16512, []byte{0x80, 0x80, 0x00}},
	}
	for _, tc := range cases {
		got := encodeOFSOffset(tc.distance)
		if !bytes.Equal(got, tc.encoded) {
			t.Fatalf("encodeOFSOffset(%d) = %x, want %x", tc.distance, got, tc.encoded)
		}
		decoded, next, err := decodeOFSOffset(got, 0)
		if err != nil {
			t.Fatalf("decodeOFSOffset(%x): %v", got, err)
		}
		if decoded != tc.distance || next != len(got) {
			t.Fatalf("decodeOFSOffset(%x) = (%d, %d), want (%d, %d)", got, decoded, next, tc.distance, len(got))
		}
	}
}

func TestApplyDeltaUsesGitCopyFieldLayout(t *testing.T) {
	base := []byte("hello world")
	delta := []byte{0x0b, 0x06, 0x93, 0x06, 0x00, 0x05, 0x01, '!'}
	got, err := applyDelta(base, delta)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("world!")) {
		t.Fatalf("applyDelta() = %q, want %q", got, "world!")
	}
}
