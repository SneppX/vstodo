package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/config"
	"vstodo/internal/transport"
)

var remoteNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func remoteConfig(a *app.App) (*config.Config, error) {
	cfg := config.New(a.Repo.GitPath("config"))
	if err := cfg.Load(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateRemote(name, rawURL string) error {
	if !remoteNamePattern.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("remote: invalid remote name %q", name)
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("remote: %q is not an absolute HTTP or HTTPS URL", rawURL)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("remote: URL must not contain credentials, a query, or a fragment")
	}
	return nil
}

func cmdRemote(a *app.App, args []string) error {
	cfg, err := remoteConfig(a)
	if err != nil {
		return err
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "-v") {
		values := cfg.All()
		var names []string
		for key := range values {
			if strings.HasPrefix(key, "remote.") && strings.HasSuffix(key, ".url") {
				names = append(names, strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url"))
			}
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("%s\t%s (fetch)\n", name, values["remote."+name+".url"])
			fmt.Printf("%s\t%s (push)\n", name, values["remote."+name+".url"])
		}
		return nil
	}
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: vstodo remote add <name> <url>")
		}
		name, rawURL := args[1], args[2]
		if err := validateRemote(name, rawURL); err != nil {
			return err
		}
		key := "remote." + name + ".url"
		if cfg.Get(key) != "" {
			return fmt.Errorf("remote: %q already exists", name)
		}
		if err := cfg.Set(key, rawURL); err != nil {
			return err
		}
		return cfg.Save()
	case "remove", "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: vstodo remote remove <name>")
		}
		return cfg.UnsetAndSave("remote."+args[1]+".url")
	case "get-url":
		if len(args) != 2 {
			return fmt.Errorf("usage: vstodo remote get-url <name>")
		}
		value := cfg.Get("remote."+args[1]+".url")
		if value == "" {
			return fmt.Errorf("remote: %q does not exist", args[1])
		}
		fmt.Println(value)
		return nil
	default:
		return fmt.Errorf("usage: vstodo remote [add <name> <url>|remove <name>|get-url <name>|-v]")
	}
}

func cmdLSRemote(a *app.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vstodo ls-remote <remote-or-url>")
	}
	remoteURL := args[0]
	if !strings.Contains(remoteURL, "://") {
		cfg, err := remoteConfig(a)
		if err != nil {
			return err
		}
		remoteURL = cfg.Get("remote."+args[0]+".url")
		if remoteURL == "" {
			return fmt.Errorf("remote: %q does not exist", args[0])
		}
	}
	token := os.Getenv("VSTODO_GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	refs, err := transport.NewHTTPTransport().ListRefs(context.Background(), remoteURL, token)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		fmt.Printf("%s\t%s\n", ref.Hash.Hex(), ref.Name)
	}
	return nil
}
