package main

import (
	"strings"
	"testing"
)

func TestCmdDiffUsage(t *testing.T) {
	err := cmdDiff(nil, []string{"only-one"})
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected usage error, got %v", err)
	}
	err = cmdDiff(nil, []string{"a", "b", "c"})
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestResolveCommitish(t *testing.T) {
	a, _, oursID, theirsID := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	head, err := resolveCommitish(a, "HEAD")
	if err != nil || head != oursID {
		t.Fatalf("HEAD: got %v %v, want %s", head, err, oursID.Hex())
	}
	feature, err := resolveCommitish(a, "feature")
	if err != nil || feature != theirsID {
		t.Fatalf("feature: got %v %v, want %s", feature, err, theirsID.Hex())
	}
	byHex, err := resolveCommitish(a, theirsID.Hex())
	if err != nil || byHex != theirsID {
		t.Fatalf("hex: got %v %v, want %s", byHex, err, theirsID.Hex())
	}
}
