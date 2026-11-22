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
	offset, crc, ok := decoded.FindEntry(wantIDs[0])
	if !ok || offset != 0x80000010 || crc != wantCRCs[0] {
		t.Fatalf("FindEntry large offset = (%d, %08x, %v), want (%d, %08x, true)", offset, crc, ok, uint64(0x80000010), wantCRCs[0])
	}
	offset, crc, ok = decoded.FindEntry(wantIDs[1])
	if !ok || offset != 200 || crc != wantCRCs[1] {
		t.Fatalf("FindEntry regular offset = (%d, %08x, %v), want (200, %08x, true)", offset, crc, ok, wantCRCs[1])
	}
	if _, _, ok := decoded.FindEntry(sha1.Digest{99}); ok {
		t.Fatal("FindEntry found an unknown object ID")
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

func TestPackIndexFindEntryUsesDirectLargeOffsetIndex(t *testing.T) {
	ids := []sha1.Digest{{1}, {2}, {3}}
	idx := &PackIndex{
		Version: 2,
		IDs: ids,
		CRCs: []uint32{11, 22, 33},
		Offsets: []uint32{0x80000001, 200, 0x80000000},
		LargeOffsets: []uint64{0x80000030, 0x80000020},
	}
	idx.Fanout = computeFanout(idx.IDs)
	encoded, err := WriteIndex(idx)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadIndex(encoded)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id sha1.Digest
		offset uint64
		crc uint32
	}{
		{ids[0], 0x80000020, 11},
		{ids[1], 200, 22},
		{ids[2], 0x80000030, 33},
	}
	for _, tc := range cases {
		offset, crc, ok := decoded.FindEntry(tc.id)
		if !ok || offset != tc.offset || crc != tc.crc {
			t.Fatalf("FindEntry(%s) = (%d, %d, %v), want (%d, %d, true)", tc.id.Hex(), offset, crc, ok, tc.offset, tc.crc)
		}
	}
}

func TestPackIndexFindEntryUsesFanoutRange(t *testing.T) {
	pack := &Pack{Objects: []PackedObject{
		{ID: sha1.Digest{1, 1}, Offset: 10, CRC32: 101},
		{ID: sha1.Digest{2, 1}, Offset: 20, CRC32: 201},
		{ID: sha1.Digest{2, 2}, Offset: 30, CRC32: 202},
		{ID: sha1.Digest{255, 1}, Offset: 40, CRC32: 2551},
	}}
	idx := BuildIndexFromPack(pack)
	for _, po := range pack.Objects {
		offset, crc, ok := idx.FindEntry(po.ID)
		if !ok || offset != po.Offset || crc != po.CRC32 {
			t.Fatalf("FindEntry(%s) = (%d, %d, %v), want (%d, %d, true)", po.ID.Hex(), offset, crc, ok, po.Offset, po.CRC32)
		}
	}
	if _, _, ok := idx.FindEntry(sha1.Digest{3}); ok {
		t.Fatal("FindEntry found an absent object")
	}
}
