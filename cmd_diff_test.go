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
