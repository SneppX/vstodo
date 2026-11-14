package main

import (
	"strings"
	"testing"

	"vstodo/internal/object"
)

func TestTagCreatesLightweightTag(t *testing.T) {
	a, _, _, _ := checkoutFixture(t)
	head, err := a.Refs.Head()
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	if err := cmdTag(a, []string{"v1.0"}); err != nil {
		t.Fatalf("tag: %v", err)
	}
	got, err := a.Refs.ReadRef("refs/tags/v1.0")
	if err != nil || got != head {
		t.Fatalf("tag target = %s, err=%v; want %s", got.Hex(), err, head.Hex())
	}
}

func TestTagCreatesAnnotatedTagAndPeelsIt(t *testing.T) {
	a, _, _, _ := checkoutFixture(t)
	head, err := a.Refs.Head()
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	if err := cmdTag(a, []string{"-a", "release/v1", "-m", "first release"}); err != nil {
		t.Fatalf("annotated tag: %v", err)
	}
	tagID, err := a.Refs.ReadRef("refs/tags/release/v1")
	if err != nil {
		t.Fatalf("read tag ref: %v", err)
	}
	obj, err := a.Store.Load(tagID)
	if err != nil {
		t.Fatalf("load tag: %v", err)
	}
	if obj.Type != object.TagType {
		t.Fatalf("tag type = %s, want tag", obj.Type)
	}
	if !strings.Contains(string(obj.Data), "tag release/v1\n") || !strings.Contains(string(obj.Data), "first release") {
		t.Fatalf("annotated tag payload = %q", obj.Data)
	}
	peeled, err := peelAnnotatedTag(a, tagID)
	if err != nil || peeled != head {
		t.Fatalf("peeled tag = %s, err=%v; want %s", peeled.Hex(), err, head.Hex())
	}
}

func TestTagRejectsDuplicateAndCanDelete(t *testing.T) {
	a, _, _, _ := checkoutFixture(t)
	if err := cmdTag(a, []string{"v1"}); err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := cmdTag(a, []string{"v1"}); err == nil {
		t.Fatal("duplicate tag unexpectedly succeeded")
	}
	if err := cmdTag(a, []string{"-d", "v1"}); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	if _, err := a.Refs.ReadRef("refs/tags/v1"); err == nil {
		t.Fatal("tag ref still exists after deletion")
	}
}
