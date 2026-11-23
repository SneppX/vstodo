package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

func TestCmdLogLimit(t *testing.T) {
	a, _, _, _ := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	out, err := captureStdout(t, func() error { return cmdLog(a, []string{"-n", "1"}) })
	if err != nil {
		t.Fatalf("cmdLog: %v", err)
	}
	if strings.Count(out, "commit ") != 1 {
		t.Fatalf("expected 1 commit, got output:\n%s", out)
	}
}

func TestCmdLogInvalidLimit(t *testing.T) {
	a, _, _, _ := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	_, err := captureStdout(t, func() error { return cmdLog(a, []string{"-n", "bad"}) })
	if err == nil {
		t.Fatal("expected error")
	}
}
