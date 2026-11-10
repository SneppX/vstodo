// Package packfile implements the vstodo packfile (.pack) and pack index (.idx)
// formats, reverse-engineered from git.
//
// Git pack file format (reconstructed):
//
//	Pack file layout:
//	    12-byte header: "PACK" (4 bytes) | version uint32 BE (4) |
//	    num objects uint32 BE (4)
//	    N objects (each encoded below)
//	    20-byte trailer: SHA-1 checksum of all preceding bytes
//
//	Each object is a variable-length header followed by a zlib-compressed
//	payload:
//
//	    Header byte 0: bits 7..4 = object type (commit=1, tree=2, blob=3,
//	    tag=4, ofs-delta=6, ref-delta=7); bits 3..0 = low 4 bits of size.
//	    Following bytes (while high bit set): 7 bits of size each, LSB-first
//	    groups. High bit = continuation byte.
//
//	    Plain objects (commit/tree/blob/tag): zlib-compressed git object
//	        content ("<type> <size>\0<data>").
//	    OFS_DELTA: variable-length negative byte offset to the base object,
//	        then zlib-compressed delta instructions.
//	    REF_DELTA: 20-byte base object SHA-1, then zlib-compressed delta
//	        instructions.
//
//	Delta instructions (git delta format):
//
//	    Variable-length source size, then target size, then opcodes:
//	    - COPY (high bit set): offset encoded in 1-4 bytes selected by bits
//	      4-6 of the opcode, size encoded in 1-3 bytes selected by bits 0-3.
//	    - INSERT (high bit clear): low 7 bits = literal byte count, followed
//	      by that many literal bytes.
//
// Pack index (.idx) format version 2:
//
//	4-byte magic: \377tOc
//	4-byte version = 2
//	256 x 4-byte fan-out table (cumulative object counts by first ID byte)
//	N x 20-byte SHA-1 table (sorted object IDs)
//	N x 4-byte CRC32 table
//	N x 4-byte 32-bit offset table (bit31 set => large offset)
//	M x 8-byte large offset table
//	20-byte pack checksum (SHA-1 of pack file)
//	20-byte idx checksum (SHA-1 of everything above)
//
// Standard library only.
package packfile

import (
	"bytes"
	"compress/zlib"
	crypto_sha1 "crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"sort"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// hashWriter wraps a hash.Hash to satisfy hash.Hash (adds BlockSize).
type hashWriter struct {
	h hash.Hash
}

func (w *hashWriter) Write(p []byte) (int, error) { return w.h.Write(p) }
func (w *hashWriter) Sum(b []byte) []byte         { return w.h.Sum(b) }
func (w *hashWriter) Reset()                      { w.h.Reset() }
func (w *hashWriter) Size() int                   { return w.h.Size() }
func (w *hashWriter) BlockSize() int              { return 1 }

// ObjectType is the git pack numeric type code.
type ObjectType uint8

const (
	ObjCommit   ObjectType = 1
	ObjTree     ObjectType = 2
	ObjBlob     ObjectType = 3
	ObjTag      ObjectType = 4
	ObjOFSDelta ObjectType = 6
	ObjREFDelta ObjectType = 7
)

func (t ObjectType) String() string {
	switch t {
	case ObjCommit:
		return "commit"
	case ObjTree:
		return "tree"
	case ObjBlob:
		return "blob"
	case ObjTag:
		return "tag"
	case ObjOFSDelta:
		return "ofs-delta"
	case ObjREFDelta:
		return "ref-delta"
	default:
		return fmt.Sprintf("unknown-%d", int(t))
	}
}

// Pack is an in-memory pack with resolved objects and their pack offsets.
type Pack struct {
	Version  uint32
	Objects  []PackedObject // in pack file order
	Checksum sha1.Digest
	byID     map[sha1.Digest]*PackedObject
}

// PackedObject is a resolved object with its pack location.
type PackedObject struct {
	Type object.Type
	Data []byte
	ID   sha1.Digest
	// Offset is the byte offset of this object's header within the pack file.
	Offset uint64
	// Length is the total encoded byte length (header + payload).
	Length uint64
	// CRC32 of the object's packed (compressed) representation, for the index.
	CRC32 uint32
}

// PackWriter builds a pack from a set of objects.
type PackWriter struct {
	version uint32
	objs    []*object.Object
}

// NewPackWriter creates a writer for the given objects.
func NewPackWriter(objs ...*object.Object) *PackWriter {
	return &PackWriter{version: 2, objs: objs}
}

// SetVersion sets the pack version (2 or 3).
func (w *PackWriter) SetVersion(v uint32) { w.version = v }

// WritePack serializes objects into pack bytes, applying OFS_DELTA compression
// between similar objects where beneficial. Returns pack bytes and checksum.
func (w *PackWriter) WritePack() ([]byte, sha1.Digest, error) {
	sorted := make([]*object.Object, len(w.objs))
	copy(sorted, w.objs)
	sort.Slice(sorted, func(i, j int) bool {
		return bytes.Compare(sorted[i].ID.Bytes(), sorted[j].ID.Bytes()) < 0
	})

	var buf bytes.Buffer
	buf.WriteString("PACK")
	if err := binary.Write(&buf, binary.BigEndian, w.version); err != nil {
		return nil, sha1.Zero, err
	}
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(sorted))); err != nil {
		return nil, sha1.Zero, err
	}

	hw := &hashWriter{h: crypto_sha1.New()}
	hw.Write(buf.Bytes())

	var placedObjs []packedPlaced

	packObjs := make([]PackedObject, 0, len(sorted))
	for _, obj := range sorted {
		curOff := uint64(buf.Len())
		po, err := w.encodeObject(obj, placedObjs, curOff, &buf, hw)
		if err != nil {
			return nil, sha1.Zero, err
		}
		po.Offset = curOff
		po.Length = uint64(buf.Len()) - curOff
		start := curOff
		po.CRC32 = crc32.ChecksumIEEE(buf.Bytes()[start : start+po.Length])
		packObjs = append(packObjs, po)
		placedObjs = append(placedObjs, packedPlaced{off: curOff, obj: obj})
	}

	sumBytes := hw.h.Sum(nil)
	var sum sha1.Digest
	copy(sum[:], sumBytes)
	if _, err := buf.Write(sumBytes); err != nil {
		return nil, sha1.Zero, err
	}

	pack := &Pack{Version: w.version, Objects: packObjs, Checksum: sum}
	pack.byID = make(map[sha1.Digest]*PackedObject, len(packObjs))
	for i := range packObjs {
		pack.byID[packObjs[i].ID] = &packObjs[i]
	}
	return buf.Bytes(), sum, nil
}

// encodeObject writes a single object, choosing plain or OFS_DELTA storage.
func (w *PackWriter) encodeObject(obj *object.Object, placed []packedPlaced, curOff uint64, buf *bytes.Buffer, h hash.Hash) (PackedObject, error) {
	po := PackedObject{Type: obj.Type, Data: obj.Data, ID: obj.ID}

	// Try OFS_DELTA against already-placed objects of the same type.
	var baseOff uint64
	var deltaData []byte
	for i := len(placed) - 1; i >= 0; i-- {
		p := placed[i]
		if p.obj.Type != obj.Type {
			continue
		}
		if absInt(len(p.obj.Data)-len(obj.Data)) > len(obj.Data) {
			continue
		}
		d, err := computeDelta(p.obj.Data, obj.Data)
		if err != nil || len(d) == 0 {
			continue
		}
		comp, _ := zlibCompress(d)
		if uint64(len(comp))+uint64(varintLen(curOff-p.off)) >= uint64(len(obj.Data)) {
			continue
		}
		baseOff = p.off
		deltaData = d
		break
	}

	if deltaData != nil {
		offDiff := curOff - baseOff
		po.Type = obj.Type
		hdr := objHeader(ObjOFSDelta, uint64(len(deltaData)))
		if _, err := buf.Write(hdr); err != nil {
			return po, err
		}
		h.Write(hdr)
		offBytes := encodeOFSOffset(offDiff)
		if _, err := buf.Write(offBytes); err != nil {
			return po, err
		}
		h.Write(offBytes)
		comp, err := zlibCompress(deltaData)
		if err != nil {
			return po, err
		}
		if _, err := buf.Write(comp); err != nil {
			return po, err
		}
		h.Write(comp)
		po.ID = obj.ID
		return po, nil
	}

	// Plain object.
	raw := object.Marshal(obj.Type, obj.Data)
	comp, err := zlibCompress(raw)
	if err != nil {
		return po, err
	}
	hdr := objHeader(objTypeToGit(obj.Type), uint64(len(raw)))
	if _, err := buf.Write(hdr); err != nil {
		return po, err
	}
	h.Write(hdr)
	if _, err := buf.Write(comp); err != nil {
		return po, err
	}
	h.Write(comp)
	po.ID = obj.ID
	return po, nil
}

// packedPlaced records an object already written into the pack, used for OFS_DELTA
// base offset computation.
type packedPlaced struct {
	off uint64
	obj *object.Object
}

// objHeader produces the pack object header: type in high nibble of first byte,
// size as 7-bit groups (LSB first) with continuation high bit.
func objHeader(t ObjectType, size uint64) []byte {
	var out []byte
	first := byte(t<<4) | byte(size&0x7f)
	size >>= 7
	out = append(out, first)
	for size > 0 {
		b := byte(size & 0x7f)
		size >>= 7
		if size > 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}

// encodeOFSOffset encodes the OFS_DELTA offset (git stores offset-1).
func encodeOFSOffset(off uint64) []byte {
	v := off - 1
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			out = append(out, b|0x80)
		} else {
			out = append(out, b)
			return out
		}
	}
}

// decodeOFSOffset decodes the OFS_DELTA offset encoding back to offset.
// Returns the offset and the index just past the encoded bytes.
func decodeOFSOffset(data []byte, off int) (uint64, int, error) {
	if off >= len(data) {
		return 0, 0, errors.New("packfile: truncated ofs offset")
	}
	var offset uint64
	var shift uint
	for {
		b := data[off]
		off++
		offset |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			break
		}
		if off >= len(data) {
			return 0, 0, errors.New("packfile: truncated ofs offset continuation")
		}
	}
	return offset + 1, off, nil
}

// varintLen returns the encoded length of a 7-bit varint.
func varintLen(v uint64) int {
	n := 1
	for v > 0x7f {
		v >>= 7
		n++
	}
	return n
}

// zlibCompress returns the zlib-compressed form of data.
func zlibCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// zlibDecompressWithCount decompresses a zlib stream at the start of data and
// returns the output plus the number of input bytes consumed.
func zlibDecompressWithCount(data []byte) ([]byte, int, error) {
	cr := &countingReader{R: bytes.NewReader(data)}
	zr, err := zlib.NewReader(cr)
	if err != nil {
		return nil, 0, err
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, 0, err
	}
	if err := zr.Close(); err != nil {
		return nil, 0, err
	}
	return out, cr.N, nil
}

// countingReader wraps an io.Reader counting bytes read.
type countingReader struct {
	R io.Reader
	N int
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.R.Read(p)
	cr.N += n
	return n, err
}

// computeDelta produces git-format delta instructions turning src into dst.
func computeDelta(src, dst []byte) ([]byte, error) {
	const minMatch = 4
	if len(src) < minMatch || len(dst) < minMatch {
		return nil, nil
	}
	table := make(map[uint32][]int)
	for i := 0; i+minMatch <= len(src); i++ {
		key := hash4(src[i : i+minMatch])
		table[key] = append(table[key], i)
	}

	var ops bytes.Buffer
	i := 0
	copyStart := -1
	copyLen := 0
	insStart := 0

	flushCopy := func() {
		if copyStart < 0 {
			return
		}
		if copyStart > insStart {
			if err := emitInsert(&ops, dst[insStart:copyStart]); err != nil {
				panic(err)
			}
		}
		if err := emitCopy(&ops, copyStart-insStart, copyLen); err != nil {
			panic(err)
		}
		insStart = copyStart + copyLen
		copyStart = -1
		copyLen = 0
	}

	for i <= len(dst)-minMatch {
		key := hash4(dst[i : i+minMatch])
		candidates := table[key]
		bestLen := 0
		limit := len(candidates)
		if limit > 64 {
			limit = 64
		}
		for k := 0; k < limit; k++ {
			p := candidates[len(candidates)-1-k]
			l := matchLen(src[p:], dst[i:])
			if l > bestLen {
				bestLen = l
			}
		}
		if bestLen >= minMatch {
			if copyStart < 0 {
				copyStart = i
			}
			copyLen += bestLen
			i += bestLen
			continue
		}
		flushCopy()
		i++
	}
	flushCopy()
	if insStart < len(dst) {
		if err := emitInsert(&ops, dst[insStart:]); err != nil {
			return nil, err
		}
	}

	var hdr bytes.Buffer
	writeVarInt(&hdr, uint64(len(src)))
	writeVarInt(&hdr, uint64(len(dst)))
	out := append(hdr.Bytes(), ops.Bytes()...)
	return out, nil
}

// matchLen returns common prefix length of a and b.
func matchLen(a, b []byte) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// hash4 hashes the first 4 bytes.
func hash4(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// emitInsert appends INSERT opcodes (high bit clear) followed by literal bytes.
func emitInsert(out *bytes.Buffer, data []byte) error {
	for len(data) > 0 {
		n := len(data)
		if n > 127 {
			n = 127
		}
		out.WriteByte(byte(n)) // high bit clear = insert
		out.Write(data[:n])
		data = data[n:]
	}
	return nil
}

// emitCopy appends a COPY opcode with offset and size encodings.
func emitCopy(out *bytes.Buffer, offset, size int) error {
	var cmd byte = 0x80 // copy bit set
	if offset >= 0x1000000 {
		cmd |= 0x10 | 0x20 | 0x40
	} else if offset >= 0x10000 {
		cmd |= 0x20 | 0x40
	} else if offset >= 0x100 {
		cmd |= 0x40
	}
	if size >= 0x10000 {
		cmd |= 0x08
	} else if size >= 0x100 {
		cmd |= 0x04
	} else {
		cmd |= 0x02
	}
	out.WriteByte(cmd)
	if offset >= 0x1000000 {
		out.WriteByte(byte(offset >> 24))
	}
	if offset >= 0x10000 {
		out.WriteByte(byte(offset >> 16))
	}
	if offset >= 0x100 {
		out.WriteByte(byte(offset >> 8))
	}
	out.WriteByte(byte(offset))
	if size >= 0x10000 {
		out.WriteByte(byte(size >> 16))
	}
	if size >= 0x100 {
		out.WriteByte(byte(size >> 8))
	}
	out.WriteByte(byte(size))
	return nil
}

// writeVarInt writes a 7-bit little-endian varint.
func writeVarInt(buf *bytes.Buffer, v uint64) {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v > 0 {
			buf.WriteByte(b | 0x80)
		} else {
			buf.WriteByte(b)
			return
		}
	}
}

// absInt returns absolute value.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// objTypeToGit maps vstodo type to git pack code.
func objTypeToGit(t object.Type) ObjectType {
	switch t {
	case object.CommitType:
		return ObjCommit
	case object.TreeType:
		return ObjTree
	case object.BlobType:
		return ObjBlob
	case object.TagType:
		return ObjTag
	}
	return ObjBlob
}

// PackReader reads a pack file and resolves deltas.
type PackReader struct {
	Pack *Pack
}

// ReadPack parses pack bytes, verifies the checksum, resolves all deltas, and
// returns a Pack keyed by object ID.
func ReadPack(data []byte) (*Pack, error) {
	if len(data) < 12+20 {
		return nil, errors.New("packfile: data too short")
	}
	if string(data[:4]) != "PACK" {
		return nil, errors.New("packfile: bad magic")
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version != 2 && version != 3 {
		return nil, fmt.Errorf("packfile: unsupported version %d", version)
	}
	n := binary.BigEndian.Uint32(data[8:12])

	body := data[:len(data)-20]
	trailer := data[len(data)-20:]
	sumBytes := crypto_sha1.Sum(body)
	if !bytes.Equal(sumBytes[:], trailer) {
		return nil, errors.New("packfile: checksum mismatch")
	}

	off := 12
	rawObjs := make([]packedRaw, n)
	for i := uint32(0); i < n; i++ {
		cur := off
		typ, size, newOff, err := readHeader(data, off)
		if err != nil {
			return nil, fmt.Errorf("packfile: header object %d: %w", i, err)
		}
		off = newOff
		ro := packedRaw{off: uint64(cur), typ: typ, size: size}
		switch typ {
		case ObjCommit, ObjTree, ObjBlob, ObjTag:
			raw, consumed, err := zlibDecompressWithCount(data[off:])
			if err != nil {
				return nil, fmt.Errorf("packfile: object %d zlib: %w", i, err)
			}
			if uint64(len(raw)) != size {
				return nil, fmt.Errorf("packfile: object %d size mismatch", i)
			}
			nul := bytes.IndexByte(raw, 0)
			if nul < 0 {
				return nil, fmt.Errorf("packfile: object %d missing NUL", i)
			}
			vt, err := gitTypeToVstodo(typ)
			if err != nil {
				return nil, err
			}
			ro.vt = vt
			ro.data = raw[nul+1:]
			off += consumed
		case ObjOFSDelta:
			baseOff, newOff, err := decodeOFSOffset(data, off)
			if err != nil {
				return nil, fmt.Errorf("packfile: object %d ofs offset: %w", i, err)
			}
			ro.baseOff = baseOff
			off = newOff
			delta, consumed, err := zlibDecompressWithCount(data[off:])
			if err != nil {
				return nil, fmt.Errorf("packfile: object %d delta zlib: %w", i, err)
			}
			if uint64(len(delta)) != size {
				return nil, fmt.Errorf("packfile: object %d delta size mismatch", i)
			}
			ro.delta = delta
			off += consumed
		case ObjREFDelta:
			if off+20 > len(data) {
				return nil, fmt.Errorf("packfile: object %d truncated ref delta", i)
			}
			var baseID sha1.Digest
			copy(baseID[:], data[off:off+20])
			ro.baseID = baseID
			off += 20
			delta, consumed, err := zlibDecompressWithCount(data[off:])
			if err != nil {
				return nil, fmt.Errorf("packfile: object %d ref delta zlib: %w", i, err)
			}
			if uint64(len(delta)) != size {
				return nil, fmt.Errorf("packfile: object %d ref delta size mismatch", i)
			}
			ro.delta = delta
			off += consumed
		default:
			return nil, fmt.Errorf("packfile: object %d unknown type %d", i, typ)
		}
		ro.end = uint64(off)
		rawObjs[i] = ro
	}

	// Resolve iteratively: plain objects first, then deltas whose bases resolve.
	byOffset := make(map[uint64]int, n)
	for i := range rawObjs {
		byOffset[rawObjs[i].off] = i
	}
	byID := make(map[sha1.Digest]int, n)
	resolved := make([]*PackedObject, n)
	changed := true
	for changed {
		changed = false
		for i := range rawObjs {
			if resolved[i] != nil {
				continue
			}
			ro := &rawObjs[i]
			if ro.typ == ObjOFSDelta || ro.typ == ObjREFDelta {
				var base *PackedObject
				if ro.typ == ObjOFSDelta {
					j, ok := byOffset[ro.baseOff]
					if !ok || resolved[j] == nil {
						continue
					}
					base = resolved[j]
				} else {
					j, ok := byID[ro.baseID]
					if !ok || resolved[j] == nil {
						continue
					}
					base = resolved[j]
				}
				data, err := applyDelta(base.Data, ro.delta)
				if err != nil {
					return nil, fmt.Errorf("packfile: apply delta %d: %w", i, err)
				}
				po := &PackedObject{Type: base.Type, Data: data, Offset: ro.off, Length: ro.end - ro.off}
				po.ID = object.ComputeID(po.Type, po.Data)
				po.CRC32 = crc32.ChecksumIEEE(data)
				resolved[i] = po
				byID[po.ID] = i
				changed = true
			} else {
				po := &PackedObject{Type: ro.vt, Data: ro.data, Offset: ro.off, Length: ro.end - ro.off}
				po.ID = object.ComputeID(po.Type, po.Data)
				po.CRC32 = crc32.ChecksumIEEE(ro.data)
				resolved[i] = po
				byID[po.ID] = i
				changed = true
			}
		}
	}
	for i := range resolved {
		if resolved[i] == nil {
			return nil, fmt.Errorf("packfile: unresolved object %d", i)
		}
	}

	pack := &Pack{Version: version, Objects: make([]PackedObject, n)}
	for i, po := range resolved {
		pack.Objects[i] = *po
	}
	pack.byID = make(map[sha1.Digest]*PackedObject, n)
	for i := range pack.Objects {
		pack.byID[pack.Objects[i].ID] = &pack.Objects[i]
	}
	var sum sha1.Digest
	copy(sum[:], sumBytes[:])
	pack.Checksum = sum
	return pack, nil
}

// packedRaw holds an object as read before delta resolution.
type packedRaw struct {
	off     uint64
	end     uint64
	typ     ObjectType
	size    uint64
	vt      object.Type
	data    []byte
	baseID  sha1.Digest
	baseOff uint64
	delta   []byte
}

// readHeader reads the pack object header at off, returning (type, size, newOff).
func readHeader(data []byte, off int) (ObjectType, uint64, int, error) {
	if off >= len(data) {
		return 0, 0, 0, errors.New("packfile: truncated header")
	}
	b := data[off]
	off++
	typ := ObjectType((b >> 4) & 0x07)
	size := uint64(b & 0x7f)
	var shift uint = 7
	for b&0x80 != 0 {
		if off >= len(data) {
			return 0, 0, 0, errors.New("packfile: truncated header continuation")
		}
		b = data[off]
		off++
		size |= uint64(b&0x7f) << shift
		shift += 7
	}
	return typ, size, off, nil
}

// applyDelta applies git delta instructions in `delta` to `base`.
func applyDelta(base, delta []byte) ([]byte, error) {
	if len(delta) < 2 {
		return nil, errors.New("packfile: delta too short")
	}
	di := 0
	srcSize, n := readVarInt(delta, di)
	di += n
	tgtSize, n := readVarInt(delta, di)
	di += n
	if uint64(len(base)) != srcSize {
		return nil, fmt.Errorf("packfile: delta base size mismatch %d != %d", len(base), srcSize)
	}
	var out bytes.Buffer
	out.Grow(int(tgtSize))
	for di < len(delta) {
		cmd := delta[di]
		di++
		if cmd&0x80 != 0 {
			// Copy.
			var offset uint64
			var size uint64
			if cmd&0x10 != 0 {
				offset |= uint64(delta[di]) << 24
				di++
			}
			if cmd&0x20 != 0 {
				offset |= uint64(delta[di]) << 16
				di++
			}
			if cmd&0x40 != 0 {
				offset |= uint64(delta[di]) << 8
				di++
			}
			// offset low byte always present
			if di < len(delta) {
				offset |= uint64(delta[di])
				di++
			}
			if cmd&0x08 != 0 {
				size |= uint64(delta[di]) << 16
				di++
			}
			if cmd&0x04 != 0 {
				size |= uint64(delta[di]) << 8
				di++
			}
			if cmd&0x02 != 0 {
				size |= uint64(delta[di])
				di++
			}
			if size == 0 {
				size = 0x10000
			}
			if int(offset)+int(size) > len(base) {
				return nil, errors.New("packfile: delta copy out of range")
			}
			out.Write(base[offset : offset+size])
		} else {
			sz := int(cmd & 0x7f)
			if sz == 0 {
				return nil, errors.New("packfile: delta insert size 0")
			}
			if di+sz > len(delta) {
				return nil, errors.New("packfile: delta insert out of range")
			}
			out.Write(delta[di : di+sz])
			di += sz
		}
	}
	result := out.Bytes()
	if uint64(len(result)) != tgtSize {
		return nil, fmt.Errorf("packfile: delta result size mismatch %d != %d", len(result), tgtSize)
	}
	return result, nil
}

// readVarInt reads a 7-bit little-endian varint from data at di.
func readVarInt(data []byte, di int) (uint64, int) {
	var v uint64
	var shift uint
	for {
		b := data[di]
		di++
		v |= uint64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			return v, di
		}
	}
}

// gitTypeToVstodo maps git pack type to vstodo object.Type.
func gitTypeToVstodo(t ObjectType) (object.Type, error) {
	switch t {
	case ObjCommit:
		return object.CommitType, nil
	case ObjTree:
		return object.TreeType, nil
	case ObjBlob:
		return object.BlobType, nil
	case ObjTag:
		return object.TagType, nil
	}
	return "", fmt.Errorf("packfile: unknown object type %d", t)
}

// Lookup returns the resolved object by ID.
func (p *Pack) Lookup(id sha1.Digest) (*PackedObject, bool) {
	po, ok := p.byID[id]
	return po, ok
}

// ----------------------------------------------------------------------------
// PackIndex
// ----------------------------------------------------------------------------

// PackIndex is the git pack index (.idx v2) in memory.
type PackIndex struct {
	Version      uint32
	Fanout       [256]uint32
	IDs          []sha1.Digest // sorted
	CRCs         []uint32
	Offsets      []uint32 // bit31 set => large offset index
	LargeOffsets []uint64
	PackChecksum sha1.Digest
	IDXChecksum  sha1.Digest
}

const idxMagic = "\377tOc"

// WriteIndex serializes a PackIndex to .idx v2 bytes.
func WriteIndex(idx *PackIndex) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(idxMagic)
	if err := binary.Write(&buf, binary.BigEndian, idx.Version); err != nil {
		return nil, err
	}
	for i := 0; i < 256; i++ {
		if err := binary.Write(&buf, binary.BigEndian, idx.Fanout[i]); err != nil {
			return nil, err
		}
	}
	for _, id := range idx.IDs {
		buf.Write(id.Bytes())
	}
	for _, c := range idx.CRCs {
		if err := binary.Write(&buf, binary.BigEndian, c); err != nil {
			return nil, err
		}
	}
	li := 0
	for _, off := range idx.Offsets {
		if off&0x80000000 != 0 {
			if li >= len(idx.LargeOffsets) {
				return nil, errors.New("packfile: missing large offset")
			}
			if err := binary.Write(&buf, binary.BigEndian, idx.LargeOffsets[li]); err != nil {
				return nil, err
			}
			li++
		} else {
			if err := binary.Write(&buf, binary.BigEndian, off); err != nil {
				return nil, err
			}
		}
	}
	buf.Write(idx.PackChecksum.Bytes())
	pre := buf.Bytes()
	sum := crypto_sha1.Sum(pre)
	buf.Write(sum[:])
	return buf.Bytes(), nil
}

// ReadIndex parses .idx v2 bytes.
func ReadIndex(data []byte) (*PackIndex, error) {
	if len(data) < 8+256*4+20+20 {
		return nil, errors.New("packfile: idx too short")
	}
	if string(data[:4]) != idxMagic {
		return nil, errors.New("packfile: bad idx magic")
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version != 2 {
		return nil, fmt.Errorf("packfile: unsupported idx version %d", version)
	}
	off := 8
	var fanout [256]uint32
	for i := 0; i < 256; i++ {
		fanout[i] = binary.BigEndian.Uint32(data[off:])
		off += 4
	}
	n := fanout[255]
	ids := make([]sha1.Digest, n)
	for i := uint32(0); i < n; i++ {
		id, err := sha1.FromBytes(data[off:])
		if err != nil {
			return nil, err
		}
		ids[i] = id
		off += 20
	}
	crcs := make([]uint32, n)
	for i := uint32(0); i < n; i++ {
		crcs[i] = binary.BigEndian.Uint32(data[off:])
		off += 4
	}
	offsets := make([]uint32, n)
	var largeCount int
	for i := uint32(0); i < n; i++ {
		o := binary.BigEndian.Uint32(data[off:])
		off += 4
		if o&0x80000000 != 0 {
			largeCount++
		}
		offsets[i] = o
	}
	large := make([]uint64, largeCount)
	for i := 0; i < largeCount; i++ {
		if off+8 > len(data) {
			return nil, errors.New("packfile: truncated large offset table")
		}
		large[i] = binary.BigEndian.Uint64(data[off:])
		off += 8
	}
	var packCk, idxCk sha1.Digest
	copy(packCk[:], data[off:off+20])
	off += 20
	copy(idxCk[:], data[off:off+20])
	return &PackIndex{
		Version:      version,
		Fanout:       fanout,
		IDs:          ids,
		CRCs:         crcs,
		Offsets:      offsets,
		LargeOffsets: large,
		PackChecksum: packCk,
		IDXChecksum:  idxCk,
	}, nil
}

// Find returns the pack offset for id.
func (idx *PackIndex) Find(id sha1.Digest) (uint64, bool) {
	i := sort.Search(len(idx.IDs), func(j int) bool {
		return bytes.Compare(idx.IDs[j].Bytes(), id.Bytes()) >= 0
	})
	if i >= len(idx.IDs) || idx.IDs[i] != id {
		return 0, false
	}
	off := idx.Offsets[i]
	if off&0x80000000 != 0 {
		li := uint32(0)
		for j := uint32(0); j < uint32(i); j++ {
			if idx.Offsets[j]&0x80000000 != 0 {
				li++
			}
		}
		if li >= uint32(len(idx.LargeOffsets)) {
			return 0, false
		}
		return idx.LargeOffsets[li], true
	}
	return uint64(off), true
}

// BuildIndexFromPack builds a PackIndex from a resolved Pack.
func BuildIndexFromPack(pack *Pack) *PackIndex {
	ids := make([]sha1.Digest, 0, len(pack.Objects))
	crcs := make([]uint32, 0, len(pack.Objects))
	offsets := make([]uint32, 0, len(pack.Objects))
	for _, po := range pack.Objects {
		ids = append(ids, po.ID)
		crcs = append(crcs, po.CRC32)
		offsets = append(offsets, uint32(po.Offset))
	}
	sort.Slice(ids, func(i, j int) bool {
		return bytes.Compare(ids[i].Bytes(), ids[j].Bytes()) < 0
	})
	fanout := computeFanout(ids)
	return &PackIndex{
		Version:      2,
		Fanout:       fanout,
		IDs:          ids,
		CRCs:         crcs,
		Offsets:      offsets,
		PackChecksum: pack.Checksum,
		IDXChecksum:  sha1.Zero,
	}
}

func computeFanout(ids []sha1.Digest) [256]uint32 {
	var f [256]uint32
	for _, id := range ids {
		f[id[0]]++
	}
	var total uint32
	for i := 0; i < 256; i++ {
		total += f[i]
		f[i] = total
	}
	return f
}
