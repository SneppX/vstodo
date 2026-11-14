package main

import (
	"testing"

	"vstodo/internal/sha1"
)

func TestSHA1Size(t *testing.T) {
	if sha1.Size != 20 {
		t.Fatalf("sha1.Size = %d, want 20", sha1.Size)
	}
}

func TestSHA1Zero(t *testing.T) {
	var z sha1.Digest
	for i := 0; i < sha1.Size; i++ {
		if z[i] != 0 {
			t.Fatalf("zero digest byte %d = %d, want 0", i, z[i])
		}
	}
	if z.Hex() != "0000000000000000000000000000000000000000" {
		t.Fatalf("zero hex = %q", z.Hex())
	}
	if z.String() != z.Hex() {
		t.Fatalf("String() != Hex()")
	}
	if z.Prefix(2) != "00" {
		t.Fatalf("Prefix(2) = %q", z.Prefix(2))
	}
}

func TestSHA1FromHex(t *testing.T) {
	d, err := sha1.FromHex("ef0493b275aa2080237f676d2ef6559246f56636")
	if err != nil {
		t.Fatalf("FromHex: %v", err)
	}
	want := sha1.Digest{0xef, 0x04, 0x93, 0xb2, 0x75, 0xaa, 0x20, 0x80, 0x23, 0x7f, 0x67, 0x6d, 0x2e, 0xf6, 0x55, 0x92, 0x46, 0xf5, 0x66, 0x36}
	if d != want {
		t.Fatalf("FromHex = %x, want %x", d[:], want[:])
	}
	if d.Hex() != "ef0493b275aa2080237f676d2ef6559246f56636" {
		t.Fatalf("round-trip hex mismatch: %s", d.Hex())
	}

	if _, err := sha1.FromHex("not-hex"); err == nil {
		t.Fatalf("expected error for bad hex")
	}
	if _, err := sha1.FromHex("ef04"); err == nil {
		t.Fatalf("expected error for short hex")
	}
	if _, err := sha1.FromHex("gg0493b275aa2080237f676d2ef6559246f56636"); err == nil {
		t.Fatalf("expected error for invalid hex char")
	}

	d2, err := sha1.FromBytes(d.Bytes())
	if err != nil {
		t.Fatalf("FromBytes: %v", err)
	}
	if d2 != d {
		t.Fatalf("FromBytes round-trip mismatch")
	}
}

func TestSHA1Hasher(t *testing.T) {
	h := sha1.NewHasher()
	if _, err := h.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	d := h.SumDigest()
	if d.Hex() != "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d" {
		t.Fatalf("Hasher digest = %s, want aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d", d.Hex())
	}
	h.Reset()
	if _, err := h.Write([]byte("world")); err != nil {
		t.Fatalf("Write after reset: %v", err)
	}
	if d2 := h.SumDigest(); d2 == d {
		t.Fatalf("reset hasher should produce different digest")
	}
}
