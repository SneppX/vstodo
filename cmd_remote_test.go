package main

import (
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/config"
)

func TestRemoteAddGetURLAndRemoveRoundTrip(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const remoteURL = "https://github.com/SneppX/vstodo.git"
	if err := cmdRemote(a, []string{"add", "origin", remoteURL}); err != nil {
		t.Fatal(err)
	}
	cfg, err := remoteConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Get("remote.origin.url"); got != remoteURL {
		t.Fatalf("remote URL = %q, want %q", got, remoteURL)
	}
	if err := cmdRemote(a, []string{"remove", "origin"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = remoteConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Get("remote.origin.url"); got != "" {
		t.Fatalf("remote URL after remove = %q, want empty", got)
	}
}

func TestRemoteRejectsUnsafeNamesAndCredentialURLs(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"../origin", "https://github.com/org/repo"},
		{"origin", "https://user:token@github.com/org/repo"},
		{"origin", "https://github.com/org/repo?token=secret"},
		{"origin", "file:///tmp/repo"},
	}
	for _, tc := range cases {
		if err := validateRemote(tc.name, tc.url); err == nil {
			t.Errorf("validateRemote(%q, %q) unexpectedly succeeded", tc.name, tc.url)
		}
	}
}

func TestRemoteConfigUsesRepositoryLocalStorage(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.New(a.Repo.GitPath("config"))
	if err := cfg.Load(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("remote.origin.url", "https://github.com/SneppX/vstodo"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded := config.New(a.Repo.GitPath("config"))
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get("remote.origin.url"); got != "https://github.com/SneppX/vstodo" {
		t.Fatalf("reloaded URL = %q", got)
	}
}
