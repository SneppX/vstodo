package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/sha1"
	"vstodo/internal/transport"
)

func cmdClone(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: vstodo clone <src> <dst>")
	}
	src, dst := args[0], args[1]
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return cloneHTTPRemote(src, dst)
	}
	srcApp, err := app.New(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	dstApp, err := app.Init(dst)
	if err != nil {
		return fmt.Errorf("init dst: %w", err)
	}
	if err := transport.NewLocalTransport().Clone(srcApp, dstApp); err != nil {
		return fmt.Errorf("clone: %w", err)
	}
	fmt.Printf("Cloned '%s' into '%s'\n", src, dst)
	return nil
}

func cloneHTTPRemote(remoteURL, destination string) error {
	if err := validateRemote("origin", remoteURL); err != nil {
		return err
	}
	dstApp, err := app.Init(destination)
	if err != nil {
		return fmt.Errorf("init destination: %w", err)
	}
	cfg, err := remoteConfig(dstApp)
	if err != nil {
		return err
	}
	if err := cfg.Set("remote.origin.url", remoteURL); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	if err := fetchRemote(dstApp, "origin", remoteURL); err != nil {
		return fmt.Errorf("clone: fetch origin: %w", err)
	}
	head, err := dstApp.Refs.ReadRef("refs/remotes/origin/HEAD")
	if err != nil {
		return fmt.Errorf("clone: remote did not advertise a usable HEAD: %w", err)
	}
	remoteRoot := dstApp.Repo.GitPath(filepath.Join("refs", "remotes", "origin"))
	var candidates []string
	err = filepath.WalkDir(remoteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(remoteRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "HEAD" || strings.HasPrefix(relative, "tags/") {
			return nil
		}
		id, err := dstApp.Refs.ReadRef("refs/remotes/origin/" + relative)
		if err != nil {
			return err
		}
		if id == head {
			candidates = append(candidates, relative)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("clone: inspect remote branches: %w", err)
	}
	if len(candidates) == 0 {
		return fmt.Errorf("clone: remote HEAD %s does not match an advertised branch", head.Hex())
	}
	sort.Strings(candidates)
	branch := candidates[0]
	for _, preferred := range []string{"main", "master"} {
		for _, candidate := range candidates {
			if candidate == preferred {
				branch = candidate
				break
			}
		}
		if branch == preferred {
			break
		}
	}
	if err := dstApp.Refs.SetHead(sha1.Zero.Hex()); err != nil {
		return err
	}
	if err := dstApp.Refs.WriteRef("refs/heads/"+branch, head); err != nil {
		return fmt.Errorf("clone: create local branch: %w", err)
	}
	if err := cmdCheckout(dstApp, []string{branch}); err != nil {
		return fmt.Errorf("clone: checkout %s: %w", branch, err)
	}
	fmt.Printf("Cloned '%s' into '%s' (branch '%s')\n", remoteURL, destination, branch)
	return nil
}
