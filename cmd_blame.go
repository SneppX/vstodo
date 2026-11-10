package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/sha1"
)

// cmdBlame shows what revision and author last modified each line of a file.
func cmdBlame(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo blame <file>")
	}
	rel := filepath.ToSlash(args[0])

	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("no commits to blame")
	}

	// Walk first-parent history from HEAD.
	var history []sha1.Digest
	seen := make(map[sha1.Digest]bool)
	cur := headID
	for cur != sha1.Zero && !seen[cur] {
		seen[cur] = true
		history = append(history, cur)
		obj, err := a.Store.Load(cur)
		if err != nil {
			return err
		}
		c, err := commit.Parse(cur, obj.Data)
		if err != nil {
			return err
		}
		if len(c.Parents) > 0 {
			cur = c.Parents[0]
		} else {
			cur = sha1.Zero
		}
	}

	// Load each commit's tree map and resolve the file's blob id at that commit.
	type rev struct {
		id   sha1.Digest
		blob sha1.Digest
	}
	revs := make([]rev, 0, len(history))
	for _, id := range history {
		obj, err := a.Store.Load(id)
		if err != nil {
			return err
		}
		c, err := commit.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		tm, err := treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}
		revs = append(revs, rev{id: id, blob: tm[rel]})
	}

	// The most recent commit that has the file is the starting point.
	startIdx := -1
	for i := len(revs) - 1; i >= 0; i-- {
		if revs[i].blob != sha1.Zero {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return fmt.Errorf("file %q not found in history", rel)
	}

	// Blame: attribute each line to the newest commit that last changed it.
	// Walk from newest (startIdx) down to oldest. For each commit, compare its
	// file content to its parent's file content; any line that differs at the
	// same position gets attributed to the child commit.
	blame := make([]sha1.Digest, 0)

	childData := []byte(nil)
	cob, err := a.Store.Load(revs[startIdx].blob)
	if err != nil {
		return err
	}
	childData = cob.Data

	for i := startIdx; i > 0; i-- {
		parentBlob := revs[i-1].blob
		parentData := []byte(nil)
		if parentBlob != sha1.Zero {
			po, err := a.Store.Load(parentBlob)
			if err != nil {
				return err
			}
			parentData = po.Data
		}
		childLines := splitLines(childData)
		parentLines := splitLines(parentData)
		if len(blame) == 0 {
			blame = make([]sha1.Digest, len(childLines))
			for j := range blame {
				blame[j] = revs[i].id
			}
		}
		for j := 0; j < len(childLines); j++ {
			var pl string
			if j < len(parentLines) {
				pl = parentLines[j]
			}
			if childLines[j] != pl {
				if j < len(blame) {
					blame[j] = revs[i].id
				}
			}
		}
		childData = parentData
	}

	// If there's only one commit (startIdx == 0), blame lines to it.
	if len(blame) == 0 {
		co, err := a.Store.Load(revs[startIdx].blob)
		if err != nil {
			return err
		}
		lines := splitLines(co.Data)
		blame = make([]sha1.Digest, len(lines))
		for j := range blame {
			blame[j] = revs[startIdx].id
		}
	}

	// Re-resolve the final content to print (content at startIdx).
	curObj, err := a.Store.Load(revs[startIdx].blob)
	if err != nil {
		return err
	}
	curLines := splitLines(curObj.Data)

	// Pad blame if shorter than curLines (shouldn't happen).
	for len(blame) < len(curLines) {
		blame = append(blame, revs[startIdx].id)
	}

	width := 7
	for i, line := range curLines {
		id := blame[i]
		if id == sha1.Zero {
			id = revs[startIdx].id
		}
		fmt.Printf("%s %d: %s\n", id.Hex()[:width], i+1, line)
	}
	return nil
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.Split(string(bytes.TrimRight(data, "\n")), "\n")
}
