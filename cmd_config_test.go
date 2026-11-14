package main

import (
	"os"
	"testing"

	"vstodo/internal/app"
)

func TestConfigPersistsUserIdentity(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil { t.Fatalf("app.Init: %v", err) }
	if err := cmdConfig(a, []string{"user.name", "Ammar Dev"}); err != nil { t.Fatalf("set name: %v", err) }
	if err := cmdConfig(a, []string{"user.email", "ammar@example.test"}); err != nil { t.Fatalf("set email: %v", err) }
	if got := commitIdentity(a); got != "Ammar Dev <ammar@example.test>" { t.Fatalf("identity = %q", got) }
	if err := cmdConfig(a, []string{"--unset", "user.email"}); err != nil { t.Fatalf("unset email: %v", err) }
	if got := commitIdentity(a); got != "Ammar Dev <vstodo@local>" { t.Fatalf("fallback identity = %q", got) }
	if _, err := os.Stat(a.Repo.GitPath("config")); err != nil { t.Fatalf("config file missing: %v", err) }
}
