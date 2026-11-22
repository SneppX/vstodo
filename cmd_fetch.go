package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/object"
	"vstodo/internal/packfile"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
	"vstodo/internal/transport"
)

// cmdFetch downloads objects from a configured HTTP remote and updates remote-tracking refs.
func cmdFetch(a *app.App, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: vstodo fetch [remote]")
	}
	name := "origin"
	if len(args) == 1 {
		name = args[0]
	}
	cfg, err := remoteConfig(a)
	if err != nil {
		return err
	}
	remoteURL := cfg.Get("remote." + name + ".url")
	if remoteURL == "" {
		return fmt.Errorf("remote: %q does not exist", name)
	}
	return fetchRemote(a, name, remoteURL)
}

func fetchRemote(a *app.App, name, remoteURL string) error {
	if err := validateRemote(name, remoteURL); err != nil {
		return err
	}
	httpTransport := transport.NewHTTPTransport()
	token := os.Getenv("VSTODO_GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	ctx := context.Background()
	refs, err := httpTransport.ListRefs(ctx, remoteURL, token)
	if err != nil {
		return err
	}

	var tracking []fetchedTrackingRef
	wants := make([]sha1.Digest, 0, len(refs))
	seen := make(map[sha1.Digest]struct{}, len(refs))
	var head sha1.Digest
	for _, remoteRef := range refs {
		switch {
		case remoteRef.Name == "HEAD":
			head = remoteRef.Hash
		case strings.HasPrefix(remoteRef.Name, "refs/heads/"):
			suffix := strings.TrimPrefix(remoteRef.Name, "refs/heads/")
			if !validRemoteRefSuffix(suffix) {
				return fmt.Errorf("remote: server advertised an invalid branch ref %q", remoteRef.Name)
			}
			tracking = append(tracking, fetchedTrackingRef{name: "refs/remotes/" + name + "/" + suffix, id: remoteRef.Hash})
		case strings.HasPrefix(remoteRef.Name, "refs/tags/"):
			suffix := strings.TrimPrefix(remoteRef.Name, "refs/tags/")
			if !validRemoteRefSuffix(suffix) {
				return fmt.Errorf("remote: server advertised an invalid tag ref %q", remoteRef.Name)
			}
			tracking = append(tracking, fetchedTrackingRef{name: "refs/remotes/" + name + "/tags/" + suffix, id: remoteRef.Hash})
		default:
			continue
		}
		if _, ok := seen[remoteRef.Hash]; !ok {
			seen[remoteRef.Hash] = struct{}{}
			wants = append(wants, remoteRef.Hash)
		}
	}
	if len(wants) == 0 {
		return fmt.Errorf("remote: %q advertised no fetchable refs", name)
	}

	for i := range tracking {
		expected, err := readRefExpectation(a, tracking[i].name)
		if err != nil {
			return fmt.Errorf("remote: snapshot %s: %w", tracking[i].name, err)
		}
		tracking[i].expected = expected
	}
	var headExpected *sha1.Digest
	if head != sha1.Zero {
		headExpected, err = readRefExpectation(a, "refs/remotes/"+name+"/HEAD")
		if err != nil {
			return fmt.Errorf("remote: snapshot remote HEAD: %w", err)
		}
	}

	haves, err := localHaveCommits(a)
	if err != nil {
		return fmt.Errorf("remote: collect local negotiation commits: %w", err)
	}
	packBytes, err := httpTransport.FetchPackWithHaves(ctx, remoteURL, token, wants, haves)
	if err != nil {
		return err
	}
	pack, err := packfile.ReadPack(packBytes)
	if err != nil {
		return fmt.Errorf("remote: parse received pack: %w", err)
	}
	for _, packed := range pack.Objects {
		id, err := a.Store.Store(&object.Object{Type: packed.Type, Data: packed.Data})
		if err != nil {
			return fmt.Errorf("remote: store object %s: %w", packed.ID.Hex(), err)
		}
		if id != packed.ID {
			return fmt.Errorf("remote: object identity mismatch: pack has %s, storage computed %s", packed.ID.Hex(), id.Hex())
		}
	}
	for _, fetched := range tracking {
		if err := a.Refs.UpdateRef(fetched.name, fetched.expected, fetched.id); err != nil {
			return fmt.Errorf("remote: update %s (remote-tracking ref changed concurrently; retry fetch): %w", fetched.name, err)
		}
	}
	if head != sha1.Zero {
		if err := a.Refs.UpdateRef("refs/remotes/"+name+"/HEAD", headExpected, head); err != nil {
			return fmt.Errorf("remote: update remote HEAD (changed concurrently; retry fetch): %w", err)
		}
	}
	fmt.Printf("Fetched %d objects from %s\n", len(pack.Objects), name)
	return nil
}

func validRemoteRefSuffix(suffix string) bool {
	if suffix == "" || strings.HasPrefix(suffix, "/") || strings.HasSuffix(suffix, "/") || strings.Contains(suffix, "..") || strings.Contains(suffix, "//") {
		return false
	}
	for _, component := range strings.Split(suffix, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
		for _, r := range component {
			if r <= 0x20 || strings.ContainsRune("~^:?*[\\", r) || r == 0x7f {
				return false
			}
		}
	}
	return true
}

type fetchedTrackingRef struct {
	name     string
	id       sha1.Digest
	expected *sha1.Digest
}

func readRefExpectation(a *app.App, name string) (*sha1.Digest, error) {
	id, err := a.Refs.ReadRef(name)
	if err != nil {
		if errors.Is(err, ref.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

func updateFetchedTrackingRef(a *app.App, fetched fetchedTrackingRef) error {
	return a.Refs.UpdateRef(fetched.name, fetched.expected, fetched.id)
}

func localHaveCommits(a *app.App) ([]sha1.Digest, error) {
	refs, err := a.Refs.ListAllRefs()
	if err != nil {
		return nil, err
	}
	seen := make(map[sha1.Digest]struct{})
	var haves []sha1.Digest
	add := func(id sha1.Digest) {
		if id == sha1.Zero {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		obj, err := a.Store.Load(id)
		if err != nil || obj.Type != object.CommitType {
			return
		}
		seen[id] = struct{}{}
		haves = append(haves, id)
	}
	for _, id := range refs {
		add(id)
	}
	if head, err := a.Refs.Head(); err == nil {
		add(head)
	}
	sort.Slice(haves, func(i, j int) bool { return haves[i].Hex() < haves[j].Hex() })
	return haves, nil
}
