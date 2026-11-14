// Package index reads and writes Git-compatible version 2 index files.
// VSTodo currently stores only stage-zero entries; stat data is zeroed so Git
// refreshes it from the working tree before relying on it.
package index

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vstodo/internal/sha1"
)

const (
	indexSignature = "DIRC"
	indexVersion   = uint32(2)
	fixedEntrySize = 62
	defaultMode    = uint32(0100644)
)

// Index maps repository-relative paths to staged blob digests.
type Index struct {
	entries map[string]sha1.Digest
	modes   map[string]uint32
	path    string
}

func NewIndex(path string) *Index {
	return &Index{
		entries: make(map[string]sha1.Digest),
		modes:   make(map[string]uint32),
		path:    path,
	}
}

func (idx *Index) Load() error {
	data, err := os.ReadFile(idx.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	idx.entries = make(map[string]sha1.Digest)
	idx.modes = make(map[string]uint32)
	if len(data) >= 4 && string(data[:4]) == indexSignature {
		return idx.loadBinary(data)
	}
	return idx.loadLegacyText(data)
}

func (idx *Index) loadLegacyText(data []byte) error {
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		parts := bytes.SplitN(line, []byte("\t"), 2)
		if len(parts) != 2 {
			continue
		}
		id, err := sha1.FromHex(string(parts[0]))
		if err != nil {
			continue
		}
		path := string(parts[1])
		idx.entries[path] = id
		idx.modes[path] = defaultMode
	}
	return nil
}

func (idx *Index) loadBinary(data []byte) error {
	if len(data) < 12+20 {
		return fmt.Errorf("index: truncated DIRC file")
	}
	checksumOffset := len(data) - 20
	sum := sha1.Sum(data[:checksumOffset])
	if !bytes.Equal(sum[:], data[checksumOffset:]) {
		return fmt.Errorf("index: checksum mismatch")
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version != indexVersion {
		return fmt.Errorf("index: unsupported version %d", version)
	}
	count := binary.BigEndian.Uint32(data[8:12])
	cursor := 12
	for i := uint32(0); i < count; i++ {
		start := cursor
		if cursor+fixedEntrySize > checksumOffset {
			return fmt.Errorf("index: truncated entry %d", i)
		}
		mode := binary.BigEndian.Uint32(data[cursor+24 : cursor+28])
		id, err := sha1.FromBytes(data[cursor+40 : cursor+60])
		if err != nil {
			return fmt.Errorf("index: invalid object ID in entry %d: %w", i, err)
		}
		flags := binary.BigEndian.Uint16(data[cursor+60 : cursor+62])
		if flags&0x3000 != 0 {
			return fmt.Errorf("index: conflict stages are not supported")
		}
		cursor += fixedEntrySize
		nameEnd := bytes.IndexByte(data[cursor:checksumOffset], 0)
		if nameEnd < 0 {
			return fmt.Errorf("index: unterminated path in entry %d", i)
		}
		path := string(data[cursor : cursor+nameEnd])
		if path == "" {
			return fmt.Errorf("index: empty path in entry %d", i)
		}
		idx.entries[path] = id
		if mode == 0 {
			mode = defaultMode
		}
		idx.modes[path] = mode
		cursor += nameEnd + 1
		entryLength := cursor - start
		padding := (8 - entryLength%8) % 8
		cursor += padding
		if cursor > checksumOffset {
			return fmt.Errorf("index: entry %d exceeds file boundary", i)
		}
	}
	// Version 2 optional extensions are safely ignored; mandatory extensions
	// (lowercase signature) are rejected rather than silently misinterpreted.
	for cursor < checksumOffset {
		if cursor+8 > checksumOffset {
			return fmt.Errorf("index: truncated extension header")
		}
		signature := data[cursor : cursor+4]
		size := binary.BigEndian.Uint32(data[cursor+4 : cursor+8])
		cursor += 8
		if uint64(cursor)+uint64(size) > uint64(checksumOffset) {
			return fmt.Errorf("index: extension exceeds file boundary")
		}
		if signature[0] >= 'a' && signature[0] <= 'z' {
			return fmt.Errorf("index: unsupported mandatory extension %q", string(signature))
		}
		cursor += int(size)
	}
	return nil
}

func (idx *Index) Save() error {
	if err := os.MkdirAll(filepath.Dir(idx.path), 0o755); err != nil {
		return err
	}
	paths := make([]string, 0, len(idx.entries))
	for path := range idx.entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var buf bytes.Buffer
	buf.WriteString(indexSignature)
	_ = binary.Write(&buf, binary.BigEndian, indexVersion)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(paths)))
	for _, path := range paths {
		start := buf.Len()
		for i := 0; i < 10; i++ {
			_ = binary.Write(&buf, binary.BigEndian, uint32(0))
		}
		id := idx.entries[path]
		buf.Write(id.Bytes())
		nameLength := len(path)
		flagsLength := nameLength
		if flagsLength > 0x0fff {
			flagsLength = 0x0fff
		}
		flags := uint16(flagsLength)
		_ = binary.Write(&buf, binary.BigEndian, flags)
		buf.WriteString(path)
		buf.WriteByte(0)
		entryLength := buf.Len() - start
		padding := (8 - entryLength%8) % 8
		for i := 0; i < padding; i++ {
			buf.WriteByte(0)
		}
	}
	sum := sha1.Sum(buf.Bytes())
	buf.Write(sum[:])
	return os.WriteFile(idx.path, buf.Bytes(), 0o644)
}

func (idx *Index) Add(path string, d sha1.Digest) {
	idx.AddMode(path, d, defaultMode)
}

func (idx *Index) AddMode(path string, d sha1.Digest, mode uint32) {
	idx.entries[path] = d
	if mode == 0 {
		mode = defaultMode
	}
	idx.modes[path] = mode
}

func (idx *Index) Mode(path string) uint32 {
	if mode := idx.modes[path]; mode != 0 {
		return mode
	}
	return defaultMode
}

func (idx *Index) Remove(path string) {
	delete(idx.entries, path)
	delete(idx.modes, path)
}

func (idx *Index) Get(path string) sha1.Digest { return idx.entries[path] }

func (idx *Index) Entries() map[string]sha1.Digest { return idx.entries }

func (idx *Index) Contains(path string) bool {
	_, ok := idx.entries[path]
	return ok
}
