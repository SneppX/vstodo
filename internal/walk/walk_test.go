package walk

import (
	"errors"
	"testing"

	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

func makeCommit(t *testing.T, parents []sha1.Digest, msg string) (sha1.Digest, []byte) {
	t.Helper()
	tree := sha1.Digest{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	raw := commit.Build(tree, parents, "Test <test@example.com>", "Test <test@example.com>", msg)
	return object.HashCommit(raw), raw
}

func TestHistoryLinear(t *testing.T) {
	idA, rawA := makeCommit(t, nil, "A")
	idB, rawB := makeCommit(t, []sha1.Digest{idA}, "B")
	idC, rawC := makeCommit(t, []sha1.Digest{idB}, "C")
	objs := map[sha1.Digest][]byte{idA: rawA, idB: rawB, idC: rawC}
	w := &Walker{Load: func(id sha1.Digest) (*object.Object, error) {
		raw, ok := objs[id]
		if !ok {
			return nil, errors.New("missing")
		}
		return &object.Object{Type: object.CommitType, Data: raw}, nil
	}}
	got, err := w.History(idC)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	want := []sha1.Digest{idC, idB, idA}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%s want %s", i, got[i].Hex(), want[i].Hex())
		}
	}
}

func TestHistoryDiamond(t *testing.T) {
	idA, rawA := makeCommit(t, nil, "A")
	idB, rawB := makeCommit(t, []sha1.Digest{idA}, "B")
	idC, rawC := makeCommit(t, []sha1.Digest{idA}, "C")
	idD, rawD := makeCommit(t, []sha1.Digest{idB, idC}, "D")
	objs := map[sha1.Digest][]byte{idA: rawA, idB: rawB, idC: rawC, idD: rawD}
	w := &Walker{Load: func(id sha1.Digest) (*object.Object, error) {
		raw, ok := objs[id]
		if !ok {
			return nil, errors.New("missing")
		}
		return &object.Object{Type: object.CommitType, Data: raw}, nil
	}}
	got, err := w.History(idD)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("len=%d want 4: %v", len(got), got)
	}
	if got[0] != idD {
		t.Fatalf("first=%s want %s", got[0].Hex(), idD.Hex())
	}
	seen := make(map[sha1.Digest]bool)
	for _, id := range got {
		if seen[id] {
			t.Fatalf("duplicate %s", id.Hex())
		}
		seen[id] = true
	}
	for _, want := range []sha1.Digest{idA, idB, idC, idD} {
		if !seen[want] {
			t.Fatalf("missing %s", want.Hex())
		}
	}
}

func TestHistoryNilLoad(t *testing.T) {
	w := &Walker{}
	if _, err := w.History(sha1.Digest{1}); err == nil {
		t.Fatal("expected error")
	}
}
