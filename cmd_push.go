package main

import (
	"context"
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/packfile"
	"vstodo/internal/sha1"
	"vstodo/internal/transport"
	"vstodo/internal/tree"
)

// cmdPush publishes the current branch to an HTTP smart-transport remote.
// It supports fast-forward branch updates only; force pushes and tag pushes are
// intentionally excluded until their safety and recovery semantics are complete.
func cmdPush(a *app.App, args []string) error {
	if len(args) > 2 {
		return fmt.Errorf("usage: vstodo push [remote] [branch]")
	}
	remoteName := "origin"
	if len(args) > 0 {
		remoteName = args[0]
	}
	branch, err := a.Refs.CurrentBranch()
	if err != nil {
		return err
	}
	if len(args) == 2 {
		branch = args[1]
	}
	if branch == "" {
		return fmt.Errorf("push: detached HEAD; specify a branch")
	}
	if err := validateBranchName(branch); err != nil {
		return err
	}
	cfg, err := remoteConfig(a)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		if configured := cfg.Get("branch." + branch + ".pushRemote"); configured != "" {
			remoteName = configured
		} else if configured := cfg.Get("branch." + branch + ".remote"); configured != "" {
			remoteName = configured
		}
	}
	remoteURL := remotePushURL(cfg, remoteName)
	if remoteURL == "" {
		return fmt.Errorf("remote: %q does not exist", remoteName)
	}
	if err := validateRemote(remoteName, remoteURL); err != nil {
		return err
	}

	localRef := "refs/heads/" + branch
	localHead, err := a.Refs.ReadRef(localRef)
	if err != nil {
		return fmt.Errorf("push: read local branch: %w", err)
	}
	if localHead == sha1.Zero {
		return fmt.Errorf("push: branch %q has no commit", branch)
	}

	token := os.Getenv("VSTODO_GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	httpTransport := transport.NewHTTPTransport()
	remoteRefs, err := httpTransport.ListPushRefs(context.Background(), remoteURL, token)
	if err != nil {
		return err
	}
	remoteRef := "refs/heads/" + branch
	remoteHead := sha1.Zero
	for _, ref := range remoteRefs {
		if ref.Name == remoteRef {
			remoteHead = ref.Hash
			break
		}
	}
	if remoteHead == localHead {
		fmt.Printf("Everything up-to-date (%s)\n", branch)
		return nil
	}
	if remoteHead != sha1.Zero {
		ancestor, err := isCommitAncestor(a, remoteHead, localHead)
		if err != nil {
			return fmt.Errorf("push: remote tip %s is not available locally; run `vstodo fetch %s` before pushing: %w", remoteHead.Hex(), remoteName, err)
		}
		if !ancestor {
			return fmt.Errorf("push: rejected non-fast-forward update to %s; fetch and integrate remote changes first", remoteRef)
		}
	}

	objects, err := collectReachableObjects(a, localHead)
	if err != nil {
		return fmt.Errorf("push: collect reachable objects: %w", err)
	}
	packBytes, _, err := packfile.NewPackWriter(objects...).WritePack()
	if err != nil {
		return fmt.Errorf("push: build pack: %w", err)
	}
	if err := httpTransport.PushPack(context.Background(), remoteURL, token, remoteHead, localHead, remoteRef, packBytes); err != nil {
		return err
	}
	if err := a.Refs.WriteRef("refs/remotes/"+remoteName+"/"+branch, localHead); err != nil {
		return fmt.Errorf("push succeeded but local tracking ref update failed: %w", err)
	}
	fmt.Printf("Pushed %s to %s (%d objects)\n", branch, remoteName, len(objects))
	return nil
}

func collectReachableObjects(a *app.App, tip sha1.Digest) ([]*object.Object, error) {
	queue := []sha1.Digest{tip}
	seen := make(map[sha1.Digest]struct{})
	objects := make([]*object.Object, 0)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		obj, err := a.Store.Load(id)
		if err != nil {
			return nil, err
		}
		objects = append(objects, obj)
		switch obj.Type {
		case object.CommitType:
			parsed, err := commit.Parse(id, obj.Data)
			if err != nil {
				return nil, err
			}
			queue = append(queue, parsed.Tree)
			queue = append(queue, parsed.Parents...)
		case object.TreeType:
			parsed, err := tree.Parse(id, obj.Data)
			if err != nil {
				return nil, err
			}
			for _, entry := range parsed.Entries {
				if entry.Mode != "160000" {
					queue = append(queue, entry.ID)
				}
			}
		}
	}
	return objects, nil
}
