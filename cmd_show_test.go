package main

import (
	"strings"
	"testing"
)

func TestCmdShowResolvesCommitish(t *testing.T) {
	a, _, oursID, _ := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	out, err := captureStdout(t, func() error {
		return cmdShow(a, []string{"HEAD"})
	})
	if err != nil {
		t.Fatalf("cmdShow HEAD: %v", err)
	}
	if !strings.Contains(out, "commit "+oursID.Hex()) {
		t.Fatalf("expected HEAD commit in output:\n%s", out)
	}
}
