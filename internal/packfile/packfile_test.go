package packfile

import "testing"

func TestObjectHeaderRoundTripsTypeAndSize(t *testing.T) {
	for _, size := range []uint64{0, 1, 15, 16, 127, 128, 255, 4095, 4096, 1 << 20} {
		header := objHeader(ObjBlob, size)
		gotType, gotSize, next, err := readHeader(header, 0)
		if err != nil {
			t.Fatalf("size %d: readHeader: %v", size, err)
		}
		if gotType != ObjBlob || gotSize != size || next != len(header) {
			t.Fatalf("size %d: got type=%v size=%d next=%d, header=%x", size, gotType, gotSize, next, header)
		}
	}
}
