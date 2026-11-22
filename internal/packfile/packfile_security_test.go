package packfile

import (
	"bytes"
	"compress/zlib"
	cryptosha1 "crypto/sha1"
	"encoding/binary"
	"testing"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

func sealTestPack(body []byte) []byte {
	sum := cryptosha1.Sum(body)
	return append(append([]byte(nil), body...), sum[:]...)
}

func TestReadPackRejectsUntrustedObjectCountBeforeAllocation(t *testing.T) {
	body := make([]byte, 12)
	copy(body[:4], "PACK")
	binary.BigEndian.PutUint32(body[4:8], 2)
	binary.BigEndian.PutUint32(body[8:12], ^uint32(0))
	if _, err := ReadPack(sealTestPack(body)); err == nil {
		t.Fatal("ReadPack accepted an impossible object count")
	}
}

func TestReadPackRejectsDecompressionBeyondDeclaredSize(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(bytes.Repeat([]byte("x"), 1024)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 12)
	copy(body[:4], "PACK")
	binary.BigEndian.PutUint32(body[4:8], 2)
	binary.BigEndian.PutUint32(body[8:12], 1)
	body = append(body, objHeader(ObjBlob, 1)...)
	body = append(body, compressed.Bytes()...)
	if _, err := ReadPack(sealTestPack(body)); err == nil {
		t.Fatal("ReadPack accepted decompressed content larger than its declared size")
	}
}

func TestReadPackRejectsTrailingBytesAfterDeclaredObjects(t *testing.T) {
	payload := []byte("small payload")
	obj := &object.Object{Type: object.BlobType, Data: payload}
	valid, _, err := NewPackWriter(obj).WritePack()
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(nil), valid[:len(valid)-20]...)
	body = append(body, 0xff)
	if _, err := ReadPack(sealTestPack(body)); err == nil {
		t.Fatal("ReadPack accepted trailing bytes after its declared objects")
	}
}

func TestReadHeaderRejectsSizeOverflow(t *testing.T) {
	malformed := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x00}
	if _, _, _, err := readHeader(malformed, 0); err == nil {
		t.Fatal("readHeader accepted an overflowing object size")
	}
}

func TestPackWriterComputesMissingObjectID(t *testing.T) {
	obj := &object.Object{Type: object.BlobType, Data: []byte("content")}
	data, _, err := NewPackWriter(obj).WritePack()
	if err != nil {
		t.Fatal(err)
	}
	pack, err := ReadPack(data)
	if err != nil {
		t.Fatal(err)
	}
	want := object.ComputeID(object.BlobType, obj.Data)
	if len(pack.Objects) != 1 || pack.Objects[0].ID != want {
		t.Fatalf("pack object ID = %v, want %s", pack.Objects, want.Hex())
	}
}

func TestPackWriterRejectsIncorrectObjectIDAndVersion(t *testing.T) {
	obj := &object.Object{Type: object.BlobType, Data: []byte("content"), ID: sha1.Digest{1}}
	if _, _, err := NewPackWriter(obj).WritePack(); err == nil {
		t.Fatal("PackWriter accepted an incorrect object ID")
	}
	writer := NewPackWriter(&object.Object{Type: object.BlobType, Data: []byte("content")})
	writer.SetVersion(99)
	if _, _, err := writer.WritePack(); err == nil {
		t.Fatal("PackWriter accepted an unsupported version")
	}
}
