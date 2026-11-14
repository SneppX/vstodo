package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/config"
)

func repositoryConfig(a *app.App) (*config.Config, error) {
	cfg := config.New(a.Repo.GitPath("config"))
	if err := cfg.Load(); err != nil { return nil, err }
	return cfg, nil
}

func commitIdentity(a *app.App) string {
	cfg, err := repositoryConfig(a)
	name, email := "", ""
	if err == nil {
		name = cfg.Get("user.name")
		email = cfg.Get("user.email")
	}
	if value := os.Getenv("GIT_COMMITTER_NAME"); value != "" { name = value }
	if value := os.Getenv("GIT_COMMITTER_EMAIL"); value != "" { email = value }
	if name == "" { name = "VSTodo" }
	if email == "" { email = "vstodo@local" }
	return fmt.Sprintf("%s <%s>", name, email)
}
