package packfile

import (
	"bytes"
	"testing"

	"vstodo/internal/sha1"
)

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

func TestPackIndexKeepsMetadataAlignedWhenSortingIDs(t *testing.T) {
	pack := &Pack{
		Checksum: sha1.Digest{9},
		Objects: []PackedObject{
			{ID: sha1.Digest{3}, Offset: 300, CRC32: 0x33333333},
			{ID: sha1.Digest{1}, Offset: 0x80000010, CRC32: 0x11111111},
			{ID: sha1.Digest{2}, Offset: 200, CRC32: 0x22222222},
		},
	}
	idx := BuildIndexFromPack(pack)
	if idx == nil {
		t.Fatal("BuildIndexFromPack returned nil")
	}
	wantIDs := []sha1.Digest{{1}, {2}, {3}}
	wantCRCs := []uint32{0x11111111, 0x22222222, 0x33333333}
	for i := range wantIDs {
		if idx.IDs[i] != wantIDs[i] || idx.CRCs[i] != wantCRCs[i] {
			t.Fatalf("entry %d = (%s, %08x), want (%s, %08x)", i, idx.IDs[i].Hex(), idx.CRCs[i], wantIDs[i].Hex(), wantCRCs[i])
		}
	}
	if idx.Offsets[0] != 0x80000000 || idx.LargeOffsets[0] != 0x80000010 || idx.Offsets[1] != 200 || idx.Offsets[2] != 300 {
		t.Fatalf("offset tables do not match sorted object IDs: offsets=%x large=%v", idx.Offsets, idx.LargeOffsets)
	}
	encoded, err := WriteIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadIndex(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for i := range wantIDs {
		if decoded.IDs[i] != wantIDs[i] || decoded.CRCs[i] != wantCRCs[i] {
			t.Fatalf("decoded entry %d lost ID/CRC association", i)
		}
	}
	if got, ok := decoded.Find(wantIDs[0]); !ok || got != 0x80000010 {
		t.Fatalf("Find large offset = %d, %v; want %d, true", got, ok, uint64(0x80000010))
	}
}

func TestPackIndexReadRejectsCorruptionAndTruncation(t *testing.T) {
	idx := BuildIndexFromPack(&Pack{})
	encoded, err := WriteIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), encoded...)
	corrupt[len(corrupt)-1] ^= 0xff
	if _, err := ReadIndex(corrupt); err == nil {
		t.Fatal("ReadIndex accepted a bad checksum")
	}
	if _, err := ReadIndex(encoded[:len(encoded)-1]); err == nil {
		t.Fatal("ReadIndex accepted a truncated index")
	}
	if !bytes.Equal(encoded[:4], []byte(idxMagic)) {
		t.Fatal("unexpected index magic")
	}
}

func TestPackIndexWriteRejectsMismatchedTables(t *testing.T) {
	idx := BuildIndexFromPack(&Pack{Objects: []PackedObject{{ID: sha1.Digest{1}, Offset: 12, CRC32: 4}}})
	idx.CRCs = nil
	if _, err := WriteIndex(idx); err == nil {
		t.Fatal("WriteIndex accepted mismatched object tables")
	}
}
