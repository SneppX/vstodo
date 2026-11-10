package benchmark_test

import (
	"os"
	"testing"

	"vstodo/internal/index"
	"vstodo/internal/sha1"
)

var _ = os.Remove

// BenchmarkIndexAdd measures staging entries in the index.
func BenchmarkIndexAdd(b *testing.B) {
	idx := index.NewIndex(os.DevNull)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Add(pathFor(i), sha1.Zero)
	}
}

// BenchmarkIndexGet measures reading staged digests.
func BenchmarkIndexGet(b *testing.B) {
	idx := index.NewIndex(os.DevNull)
	for i := 0; i < 1000; i++ {
		idx.Add(pathFor(i), sha1.Zero)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Get(pathFor(i))
	}
}

// BenchmarkIndexContains measures the Contains check.
func BenchmarkIndexContains(b *testing.B) {
	idx := index.NewIndex(os.DevNull)
	for i := 0; i < 1000; i++ {
		idx.Add(pathFor(i), sha1.Zero)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Contains(pathFor(i))
	}
}

// BenchmarkIndexSave measures persisting the index to disk.
func BenchmarkIndexSave(b *testing.B) {
	idx := index.NewIndex(b.TempDir() + "/index")
	for i := 0; i < 1000; i++ {
		idx.Add(pathFor(i), sha1.Zero)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.Save(); err != nil {
			b.Fatal(err)
		}
	}
}

func pathFor(i int) string {
	return "dir/sub/file" + itoa(i) + ".txt"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [10]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
