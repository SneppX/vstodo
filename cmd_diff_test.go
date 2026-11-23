package main

import (
	"strings"
	"testing"

	"vstodo/internal/commit"
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

func TestCmdDiffContentBetweenCommits(t *testing.T) {
	a, _, oursID, _ := makeMergeFixture(t,
		map[string]string{"file.txt": "line1\nline2\n"},
		map[string]string{"file.txt": "line1\nchanged\n"},
		map[string]string{"other.txt": "x\n"},
	)
	oursObj, err := a.Store.Load(oursID)
	if err != nil {
		t.Fatalf("load ours: %v", err)
	}
	oursCommit, err := commit.Parse(oursID, oursObj.Data)
	if err != nil {
		t.Fatalf("parse ours: %v", err)
	}
	if len(oursCommit.Parents) == 0 {
		t.Fatal("expected parent")
	}
	baseID := oursCommit.Parents[0]

	out, err := captureStdout(t, func() error {
		return cmdDiff(a, []string{baseID.Hex(), oursID.Hex()})
	})
	if err != nil {
		t.Fatalf("cmdDiff: %v", err)
	}
	if !strings.Contains(out, "diff --git a/file.txt b/file.txt") {
		t.Fatalf("missing diff header:\n%s", out)
	}
	if !strings.Contains(out, "--- a/file.txt") || !strings.Contains(out, "+++ b/file.txt") {
		t.Fatalf("missing unified headers:\n%s", out)
	}
	if !strings.Contains(out, "-line2") || !strings.Contains(out, "+changed") {
		t.Fatalf("missing change lines:\n%s", out)
	}
}

func TestCmdDiffIndexVsHead(t *testing.T) {
	a, _, oursID, _ := makeMergeFixture(t,
		map[string]string{"file.txt": "old\n"},
		map[string]string{"file.txt": "old\n"},
		map[string]string{"other.txt": "x\n"},
	)
	_ = oursID
	if err := ensureIndexFromHEAD(a); err != nil {
		t.Fatalf("ensureIndexFromHEAD: %v", err)
	}
	blobData := []byte("new\n")
	id, err := a.Store.WriteRaw("blob", blobData)
	if err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	a.Index.Add("file.txt", id)
	if err := a.SaveIndex(); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}
	out, err := captureStdout(t, func() error { return cmdDiff(a, nil) })
	if err != nil {
		t.Fatalf("cmdDiff: %v", err)
	}
	if !strings.Contains(out, "-old") || !strings.Contains(out, "+new") {
		t.Fatalf("expected unified diff, got:\n%s", out)
	}
}
