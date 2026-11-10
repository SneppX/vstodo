package benchmark_test

import (
	"testing"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// BenchmarkHashBlob measures object content hashing (blobs).
func BenchmarkHashBlob(b *testing.B) {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = object.HashBlob(data)
	}
}

// BenchmarkHashCommit measures commit object hashing.
func BenchmarkHashCommit(b *testing.B) {
	data := []byte("tree 0000000000000000000000000000000000000000\nauthor test <test@test> 1 +0000\ncommitter test <test@test> 1 +0000\n\nmessage\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = object.HashCommit(data)
	}
}

// BenchmarkDigestHex measures Digest.Hex() encoding.
func BenchmarkDigestHex(b *testing.B) {
	var d sha1.Digest
	for i := 0; i < sha1.Size; i++ {
		d[i] = byte(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Hex()
	}
}
