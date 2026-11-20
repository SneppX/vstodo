package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/transport"
)

// cmdPull fetches a remote branch and merges its tracking ref into the current branch.
func cmdPull(a *app.App, args []string) error {
	if len(args) > 2 {
		return fmt.Errorf("usage: vstodo pull [remote] [branch]")
	}
	remoteName := "origin"
	if len(args) > 0 {
		remoteName = args[0]
	}
	currentBranch, err := a.Refs.CurrentBranch()
	if err != nil {
		return err
	}
	if currentBranch == "" {
		return fmt.Errorf("pull: cannot pull while HEAD is detached")
	}
	branch := currentBranch
	if len(args) == 2 {
		branch = args[1]
	}
	if err := validateBranchName(branch); err != nil {
		return err
	}
	cfg, err := remoteConfig(a)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		if upstream := cfg.Get("branch." + currentBranch + ".remote"); upstream != "" {
			remoteName = upstream
		}
		if mergeRef := cfg.Get("branch." + currentBranch + ".merge"); mergeRef != "" {
			if !strings.HasPrefix(mergeRef, "refs/heads/") {
				return fmt.Errorf("pull: unsupported upstream merge ref %q", mergeRef)
			}
			branch = strings.TrimPrefix(mergeRef, "refs/heads/")
		}
	}
	remoteURL := cfg.Get("remote." + remoteName + ".url")
	if remoteURL == "" {
		return fmt.Errorf("remote: %q does not exist", remoteName)
	}
	if err := validateRemote(remoteName, remoteURL); err != nil {
		return err
	}
	token := os.Getenv("VSTODO_GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	advertised, err := transport.NewHTTPTransport().ListRefs(context.Background(), remoteURL, token)
	if err != nil {
		return err
	}
	remoteRef := "refs/heads/" + branch
	found := false
	for _, ref := range advertised {
		if ref.Name == remoteRef {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("pull: remote %q does not advertise branch %q", remoteName, branch)
	}
	if err := fetchRemote(a, remoteName, remoteURL); err != nil {
		return fmt.Errorf("pull: fetch failed: %w", err)
	}
	trackingBranch := remoteName + "/" + branch
	if err := cmdMerge(a, []string{trackingBranch}); err != nil {
		return fmt.Errorf("pull: fetched %s but merge failed: %w", trackingBranch, err)
	}
	fmt.Printf("Pulled %s from %s\n", branch, remoteName)
	return nil
}
