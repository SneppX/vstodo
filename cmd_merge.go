package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdMerge performs a fast-forward or a conservative three-way merge.
// Overlapping edits are reported without changing HEAD or the working tree.
func cmdMerge(a *app.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vstodo merge <branch>")
	}
	branch := args[0]
	currentID, err := currentCommitID(a)
	if err != nil || currentID == sha1.Zero {
		return fmt.Errorf("nothing to merge into")
	}
	currentBranch, err := a.Refs.CurrentBranch()
	if err != nil || currentBranch == "" {
		return fmt.Errorf("cannot merge while HEAD is detached")
	}
	otherID, err := a.Refs.ReadRef("refs/heads/" + branch)
	if err != nil {
		return fmt.Errorf("no such branch: %s", branch)
	}
	if otherID == currentID {
		fmt.Println("Already up to date.")
		return nil
	}

	currentObj, err := a.Store.Load(currentID)
	if err != nil {
		return err
	}
	if currentObj.Type != object.CommitType {
		return fmt.Errorf("merge: current HEAD is not a commit")
	}
	currentCommit, err := commit.Parse(currentID, currentObj.Data)
	if err != nil {
		return err
	}
	otherObj, err := a.Store.Load(otherID)
	if err != nil {
		return err
	}
	if otherObj.Type != object.CommitType {
		return fmt.Errorf("merge: branch %q does not point to a commit", branch)
	}
	otherCommit, err := commit.Parse(otherID, otherObj.Data)
	if err != nil {
		return err
	}
	if err := requireCleanWorkingTree(a, currentCommit.Tree); err != nil {
		return fmt.Errorf("merge: %w", err)
	}

	currentTree, err := treeToMap(a, currentCommit.Tree, "")
	if err != nil {
		return err
	}
	otherTree, err := treeToMap(a, otherCommit.Tree, "")
	if err != nil {
		return err
	}

	currentIsAncestor, err := isCommitAncestor(a, currentID, otherID)
	if err != nil {
		return err
	}
	if currentIsAncestor {
		if err := applyMergedTree(a, currentTree, otherTree); err != nil {
			return err
		}
		if err := a.Refs.WriteRef("refs/heads/"+currentBranch, otherID); err != nil {
			return err
		}
		fmt.Printf("Fast-forward %s to %s\n", currentBranch, otherID.Hex()[:7])
		return nil
	}
	otherIsAncestor, err := isCommitAncestor(a, otherID, currentID)
	if err != nil {
		return err
	}
	if otherIsAncestor {
		fmt.Println("Already up to date.")
		return nil
	}

	baseID, err := findMergeBase(a, currentID, otherID)
	if err != nil {
		return err
	}
	baseObj, err := a.Store.Load(baseID)
	if err != nil {
		return err
	}
	baseCommit, err := commit.Parse(baseID, baseObj.Data)
	if err != nil {
		return err
	}
	baseTree, err := treeToMap(a, baseCommit.Tree, "")
	if err != nil {
		return err
	}

	mergedTree, mergedContents, conflicts, err := mergeTrees(a, baseTree, currentTree, otherTree)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("merge conflicts in %s; no files or refs were changed", strings.Join(conflicts, ", "))
	}
	for path, data := range mergedContents {
		id, err := a.Store.WriteRaw(object.BlobType, data)
		if err != nil {
			return fmt.Errorf("merge: store merged blob %s: %w", path, err)
		}
		mergedTree[path] = id
	}
	treeID, err := buildTreeFromIndex(a, mergedTree, "")
	if err != nil {
		return err
	}
	commitData := commit.Build(treeID, []sha1.Digest{currentID, otherID}, commitIdentity(a), commitIdentity(a), "Merge branch '"+branch+"'")
	mergeID := object.HashCommit(commitData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: commitData}); err != nil {
		return err
	}
	if err := applyMergedTree(a, currentTree, mergedTree); err != nil {
		return err
	}
	if err := a.Refs.WriteRef("refs/heads/"+currentBranch, mergeID); err != nil {
		return err
	}
	fmt.Printf("Merge made %s\n", mergeID.Hex()[:7])
	return nil
}

func applyMergedTree(a *app.App, current, target map[string]sha1.Digest) error {
	contents := make(map[string][]byte, len(target))
	for path, id := range target {
		if _, err := worktreePath(a, path); err != nil {
			return err
		}
		data, err := readBlob(a, id)
		if err != nil {
			return fmt.Errorf("merge: read %s: %w", path, err)
		}
		contents[path] = data
	}
	if err := validateCheckoutTreePaths(target); err != nil {
		return err
	}
	for path := range current {
		if _, exists := target[path]; exists {
			continue
		}
		if err := removeFile(a, path); err != nil {
			return fmt.Errorf("merge: remove %s: %w", path, err)
		}
		if err := pruneEmptyParentDirs(a, path); err != nil {
			return err
		}
	}
	for path, data := range contents {
		if err := writeFile(a, path, data); err != nil {
			return fmt.Errorf("merge: write %s: %w", path, err)
		}
	}
	for path := range a.Index.Entries() {
		a.Index.Remove(path)
	}
	for path, id := range target {
		a.Index.Add(path, id)
	}
	return a.SaveIndex()
}

func isCommitAncestor(a *app.App, ancestor, descendant sha1.Digest) (bool, error) {
	distances, err := commitAncestorDistances(a, descendant)
	if err != nil {
		return false, err
	}
	_, ok := distances[ancestor]
	return ok, nil
}

func findMergeBase(a *app.App, ours, theirs sha1.Digest) (sha1.Digest, error) {
	oursAncestors, err := commitAncestorDistances(a, ours)
	if err != nil {
		return sha1.Zero, err
	}
	theirsAncestors, err := commitAncestorDistances(a, theirs)
	if err != nil {
		return sha1.Zero, err
	}
	best := sha1.Zero
	bestScore := int(^uint(0) >> 1)
	for id, oursDistance := range oursAncestors {
		if theirsDistance, ok := theirsAncestors[id]; ok {
			score := oursDistance + theirsDistance
			if score < bestScore {
				best = id
				bestScore = score
			}
		}
	}
	if best == sha1.Zero {
		return sha1.Zero, fmt.Errorf("merge: histories have no common ancestor")
	}
	return best, nil
}

func commitAncestorDistances(a *app.App, start sha1.Digest) (map[sha1.Digest]int, error) {
	distances := map[sha1.Digest]int{start: 0}
	queue := []sha1.Digest{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		obj, err := a.Store.Load(id)
		if err != nil {
			return nil, err
		}
		if obj.Type != object.CommitType {
			return nil, fmt.Errorf("merge: object %s is not a commit", id.Hex())
		}
		parsed, err := commit.Parse(id, obj.Data)
		if err != nil {
			return nil, err
		}
		for _, parent := range parsed.Parents {
			if _, visited := distances[parent]; visited {
				continue
			}
			distances[parent] = distances[id] + 1
			queue = append(queue, parent)
		}
	}
	return distances, nil
}

func mergeTrees(a *app.App, base, ours, theirs map[string]sha1.Digest) (map[string]sha1.Digest, map[string][]byte, []string, error) {
	all := make(map[string]bool, len(base)+len(ours)+len(theirs))
	for path := range base {
		all[path] = true
	}
	for path := range ours {
		all[path] = true
	}
	for path := range theirs {
		all[path] = true
	}
	paths := make([]string, 0, len(all))
	for path := range all {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	merged := make(map[string]sha1.Digest)
	mergedContents := make(map[string][]byte)
	var conflicts []string
	for _, path := range paths {
		baseID, baseExists := base[path]
		oursID, oursExists := ours[path]
		theirsID, theirsExists := theirs[path]

		if sameTreeEntry(oursID, oursExists, theirsID, theirsExists) {
			if oursExists {
				merged[path] = oursID
			}
			continue
		}
		if sameTreeEntry(oursID, oursExists, baseID, baseExists) {
			if theirsExists {
				merged[path] = theirsID
			}
			continue
		}
		if sameTreeEntry(theirsID, theirsExists, baseID, baseExists) {
			if oursExists {
				merged[path] = oursID
			}
			continue
		}
		if !oursExists || !theirsExists {
			conflicts = append(conflicts, path)
			continue
		}

		var baseData []byte
		var err error
		if baseExists {
			baseData, err = readBlob(a, baseID)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		oursData, err := readBlob(a, oursID)
		if err != nil {
			return nil, nil, nil, err
		}
		theirsData, err := readBlob(a, theirsID)
		if err != nil {
			return nil, nil, nil, err
		}
		mergedData, ok := mergeText3Way(baseData, oursData, theirsData)
		if !ok {
			conflicts = append(conflicts, path)
			continue
		}
		mergedContents[path] = mergedData
	}
	return merged, mergedContents, conflicts, nil
}

func sameTreeEntry(a sha1.Digest, aExists bool, b sha1.Digest, bExists bool) bool {
	return aExists == bExists && (!aExists || a == b)
}

type lineEdit struct {
	start       int
	end         int
	replacement []string
}

func mergeText3Way(base, ours, theirs []byte) ([]byte, bool) {
	if bytes.IndexByte(base, 0) >= 0 || bytes.IndexByte(ours, 0) >= 0 || bytes.IndexByte(theirs, 0) >= 0 {
		return nil, false
	}
	if bytes.Equal(ours, theirs) {
		return append([]byte(nil), ours...), true
	}
	if bytes.Equal(ours, base) {
		return append([]byte(nil), theirs...), true
	}
	if bytes.Equal(theirs, base) {
		return append([]byte(nil), ours...), true
	}
	baseLines := splitMergeLines(base)
	oursEdit := makeLineEdit(baseLines, splitMergeLines(ours))
	theirsEdit := makeLineEdit(baseLines, splitMergeLines(theirs))
	if editsOverlap(oursEdit, theirsEdit) {
		return nil, false
	}
	edits := []lineEdit{oursEdit, theirsEdit}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start == edits[j].start {
			return edits[i].end > edits[j].end
		}
		return edits[i].start > edits[j].start
	})
	merged := append([]string(nil), baseLines...)
	for _, edit := range edits {
		next := make([]string, 0, len(merged)-(edit.end-edit.start)+len(edit.replacement))
		next = append(next, merged[:edit.start]...)
		next = append(next, edit.replacement...)
		next = append(next, merged[edit.end:]...)
		merged = next
	}
	return []byte(strings.Join(merged, "")), true
}

func splitMergeLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	return strings.SplitAfter(string(data), "\n")
}

func makeLineEdit(base, changed []string) lineEdit {
	prefix := 0
	for prefix < len(base) && prefix < len(changed) && base[prefix] == changed[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(base)-prefix && suffix < len(changed)-prefix &&
		base[len(base)-1-suffix] == changed[len(changed)-1-suffix] {
		suffix++
	}
	return lineEdit{
		start:       prefix,
		end:         len(base) - suffix,
		replacement: append([]string(nil), changed[prefix:len(changed)-suffix]...),
	}
}

func editsOverlap(a, b lineEdit) bool {
	if a.start == a.end && b.start == b.end {
		return a.start == b.start
	}
	if a.start == a.end {
		return a.start >= b.start && a.start < b.end
	}
	if b.start == b.end {
		return b.start >= a.start && b.start < a.end
	}
	return a.start < b.end && b.start < a.end
}

// requireCleanWorkingTree rejects staged, unstaged, deleted, and untracked
// files before operations that need a stable HEAD snapshot.
func requireCleanWorkingTree(a *app.App, headTree sha1.Digest) error {
	if err := ensureIndexFromHEAD(a); err != nil {
		return err
	}
	tracked := make(map[string]sha1.Digest)
	if headTree != sha1.Zero {
		var err error
		tracked, err = treeToMap(a, headTree, "")
		if err != nil {
			return err
		}
	}
	staged := a.Index.Entries()
	if len(staged) != len(tracked) {
		return fmt.Errorf("staged changes are present; commit or unstage them first")
	}
	for path, id := range tracked {
		if staged[path] != id {
			return fmt.Errorf("staged changes are present; commit or unstage them first")
		}
	}
	files, err := walkFiles(a.Repo.Root)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(files))
	for _, path := range files {
		id, exists := tracked[path]
		if !exists {
			return fmt.Errorf("untracked file %q would be at risk; move or remove it first", path)
		}
		fullPath, err := worktreePath(a, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		if object.HashBlob(data) != id {
			return fmt.Errorf("local changes in %q; commit or restore them first", path)
		}
		seen[path] = true
	}
	for path := range tracked {
		if !seen[path] {
			return fmt.Errorf("tracked file %q is missing; restore it first", path)
		}
	}
	return nil
}
