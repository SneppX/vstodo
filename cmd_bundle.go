package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdBundle moves objects and refs by archive.
func cmdBundle(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo bundle <create|unbundle|list-heads> ...")
	}
	switch args[0] {
	case "create":
		return bundleCreate(a, args[1:])
	case "unbundle":
		return bundleUnbundle(a, args[1:])
	case "list-heads":
		return bundleListHeads(a, args[1:])
	default:
		return fmt.Errorf("vstodo: unknown bundle subcommand '%s'", args[0])
	}
}

const bundleMagic = "VSTODO BUNDLE"

func bundleCreate(a *app.App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: vstodo bundle create <file> [...]")
	}
	outPath := args[0]
	refs := args[1:]
	if len(refs) == 0 {
		all, err := a.Refs.ListRefs()
		if err != nil {
			return err
		}
		for name := range all {
			refs = append(refs, name)
		}
	}

	collected := make(map[sha1.Digest]bool)
	var order []sha1.Digest
	var add func(id sha1.Digest)
	add = func(id sha1.Digest) {
		if id == sha1.Zero || collected[id] {
			return
		}
		obj, err := a.Store.Load(id)
		if err != nil {
			return
		}
		collected[id] = true
		order = append(order, id)
		switch obj.Type {
		case object.CommitType:
			c, err := commit.Parse(id, obj.Data)
			if err != nil {
				return
			}
			for _, p := range c.Parents {
				add(p)
			}
			add(c.Tree)
		case object.TreeType:
			addTreeRefs(a, obj.Data, add)
		}
	}
	for _, r := range refs {
		id, err := a.Refs.ReadRef(r)
		if err != nil {
			continue
		}
		add(id)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()

	fmt.Fprintln(w, bundleMagic)
	for _, r := range refs {
		id, err := a.Refs.ReadRef(r)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "REF %s %s\n", r, id.Hex())
	}
	fmt.Fprintln(w, "END-REFS")

	for _, id := range order {
		obj, err := a.Store.Load(id)
		if err != nil {
			continue
		}
		comp, err := object.Compress(obj.Type, obj.Data)
		if err != nil {
			return err
		}
		enc := base64.StdEncoding.EncodeToString(comp)
		fmt.Fprintf(w, "OBJECT %s %s\n", obj.Type, id.Hex())
		for i := 0; i < len(enc); i += 76 {
			end := i + 76
			if end > len(enc) {
				end = len(enc)
			}
			fmt.Fprintln(w, enc[i:end])
		}
		fmt.Fprintln(w, "END-OBJECT")
	}
	w.Flush()
	fmt.Printf("Created bundle %s with %d objects and %d refs\n", outPath, len(order), len(refs))
	return nil
}

func addTreeRefs(a *app.App, data []byte, add func(sha1.Digest)) {
	rest := data
	for len(rest) > 0 {
		nul := -1
		for i, b := range rest {
			if b == 0 {
				nul = i
				break
			}
		}
		if nul < 0 {
			return
		}
		rest = rest[nul+1:]
		if len(rest) < 20 {
			return
		}
		id, err := sha1.FromBytes(rest[:20])
		if err != nil {
			return
		}
		rest = rest[20:]
		add(id)
		obj, err := a.Store.Load(id)
		if err != nil {
			continue
		}
		if obj.Type == object.TreeType {
			addTreeRefs(a, obj.Data, add)
		}
	}
}

func bundleUnbundle(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo bundle unbundle <file>")
	}
	inPath := args[0]
	f, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if !sc.Scan() {
		return fmt.Errorf("empty bundle")
	}
	if sc.Text() != bundleMagic {
		return fmt.Errorf("not a vstodo bundle")
	}

	type refEntry struct {
		name string
		id   sha1.Digest
	}
	var refs []refEntry
	objectData := make(map[sha1.Digest][]byte) // compressed
	objectTypes := make(map[sha1.Digest]object.Type)
	inObject := false
	var curType object.Type
	var curID sha1.Digest
	var curBuf strings.Builder

	for sc.Scan() {
		line := sc.Text()
		if inObject {
			if line == "END-OBJECT" {
				comp, err := base64.StdEncoding.DecodeString(strings.TrimSpace(curBuf.String()))
				if err != nil {
					return err
				}
				objectData[curID] = comp
				objectTypes[curID] = curType
				inObject = false
				curBuf.Reset()
			} else {
				curBuf.WriteString(line)
			}
			continue
		}
		if strings.HasPrefix(line, "REF ") {
			parts := strings.SplitN(line[4:], " ", 2)
			if len(parts) != 2 {
				continue
			}
			id, err := sha1.FromHex(parts[1])
			if err != nil {
				continue
			}
			refs = append(refs, refEntry{name: parts[0], id: id})
		} else if strings.HasPrefix(line, "OBJECT ") {
			parts := strings.SplitN(line[7:], " ", 2)
			if len(parts) != 2 {
				return fmt.Errorf("malformed OBJECT line")
			}
			curType = object.Type(parts[0])
			id, err := sha1.FromHex(parts[1])
			if err != nil {
				return err
			}
			curID = id
			inObject = true
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	for _, comp := range objectData {
		raw, err := object.Decompress(comp)
		if err != nil {
			return err
		}
		typ, data, err := object.Unmarshal(raw)
		if err != nil {
			return err
		}
		if _, err := a.Store.Store(&object.Object{Type: typ, Data: data}); err != nil {
			return err
		}
	}
	for _, r := range refs {
		if err := a.Refs.WriteRef(r.name, r.id); err != nil {
			return err
		}
	}
	fmt.Printf("Unbundled %d objects and %d refs from %s\n", len(objectData), len(refs), inPath)
	return nil
}

func bundleListHeads(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo bundle list-heads <file>")
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return err
	}
	if sc.Text() != bundleMagic {
		return fmt.Errorf("not a vstodo bundle")
	}
	for sc.Scan() {
		line := sc.Text()
		if line == "END-REFS" {
			break
		}
		if strings.HasPrefix(line, "REF ") {
			fmt.Println(line[4:])
		}
	}
	return sc.Err()
}
