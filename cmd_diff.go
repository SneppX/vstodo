package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdDiff shows changes between two commits or index vs HEAD.
func cmdDiff(a *app.App, args []string) error {
	if len(args) == 0 {
		headID, err := currentCommitID(a)
		if err != nil || headID == sha1.Zero {
			fmt.Println("No commits yet.")
			return nil
		}
		obj, err := a.Store.Load(headID)
		if err != nil {
			return err
		}
		if obj.Type != object.CommitType {
			return fmt.Errorf("diff: %s is not a commit", headID.Hex())
		}
		c, err := commit.Parse(headID, obj.Data)
		if err != nil {
			return err
		}
		headMap, err := treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}
		indexMap := a.Index.Entries()
		for path, id := range indexMap {
			oldID := headMap[path]
			if oldID == id {
				continue
			}
			var oldData []byte
			if oldID != sha1.Zero {
				oldData, err = readBlob(a, oldID)
				if err != nil {
					return err
				}
			}
			newData, err := readBlob(a, id)
			if err != nil {
				return err
			}
			fmt.Printf("diff --git a/%s b/%s\n", path, path)
			if oldID == sha1.Zero {
				fmt.Printf("new file\n")
			} else {
				fmt.Printf("index %s..%s\n", oldID.Hex()[:7], id.Hex()[:7])
			}
			printUnifiedDiff(path, oldData, newData)
		}
		for path, oldID := range headMap {
			if _, ok := indexMap[path]; ok {
				continue
			}
			oldData, err := readBlob(a, oldID)
			if err != nil {
				return err
			}
			fmt.Printf("diff --git a/%s b/%s\n", path, path)
			fmt.Printf("deleted file\n")
			printUnifiedDiff(path, oldData, nil)
		}
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: vstodo diff [commit1 commit2]")
	}
	lid, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}
	rid, err := resolveCommitish(a, args[1])
	if err != nil {
		return err
	}
	lo, err := a.Store.Load(lid)
	if err != nil {
		return err
	}
	if lo.Type != object.CommitType {
		return fmt.Errorf("diff: %s is not a commit", lid.Hex())
	}
	ro, err := a.Store.Load(rid)
	if err != nil {
		return err
	}
	if ro.Type != object.CommitType {
		return fmt.Errorf("diff: %s is not a commit", rid.Hex())
	}
	lc, err := commit.Parse(lid, lo.Data)
	if err != nil {
		return err
	}
	rc, err := commit.Parse(rid, ro.Data)
	if err != nil {
		return err
	}
	lm, err := treeToMap(a, lc.Tree, "")
	if err != nil {
		return err
	}
	rm, err := treeToMap(a, rc.Tree, "")
	if err != nil {
		return err
	}
	for path, id := range rm {
		oldID := lm[path]
		if oldID == id {
			continue
		}
		var oldData []byte
		if oldID != sha1.Zero {
			oldData, err = readBlob(a, oldID)
			if err != nil {
				return err
			}
		}
		newData, err := readBlob(a, id)
		if err != nil {
			return err
		}
		fmt.Printf("diff --git a/%s b/%s\n", path, path)
		if oldID == sha1.Zero {
			fmt.Printf("new file\n")
		} else {
			fmt.Printf("index %s..%s\n", oldID.Hex()[:7], id.Hex()[:7])
		}
		printUnifiedDiff(path, oldData, newData)
	}
	for path, oldID := range lm {
		if _, ok := rm[path]; ok {
			continue
		}
		oldData, err := readBlob(a, oldID)
		if err != nil {
			return err
		}
		fmt.Printf("diff --git a/%s b/%s\n", path, path)
		fmt.Printf("deleted file\n")
		printUnifiedDiff(path, oldData, nil)
	}
	return nil
}

func isBinary(data []byte) bool {
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

type diffOp struct {
	kind byte
	line string
}

func lcsDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	if n*m > 4_000_000 {
		ops := make([]diffOp, 0, n+m)
		for _, l := range a {
			ops = append(ops, diffOp{kind: '-', line: l})
		}
		for _, l := range b {
			ops = append(ops, diffOp{kind: '+', line: l})
		}
		return ops
	}
	dp := make([]int, (n+1)*(m+1))
	stride := m + 1
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i*stride+j] = dp[(i+1)*stride+j+1] + 1
			} else if dp[(i+1)*stride+j] >= dp[i*stride+j+1] {
				dp[i*stride+j] = dp[(i+1)*stride+j]
			} else {
				dp[i*stride+j] = dp[i*stride+j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			ops = append(ops, diffOp{kind: ' ', line: a[i]})
			i++
			j++
		} else if dp[(i+1)*stride+j] >= dp[i*stride+j+1] {
			ops = append(ops, diffOp{kind: '-', line: a[i]})
			i++
		} else {
			ops = append(ops, diffOp{kind: '+', line: b[j]})
			j++
		}
	}
	for i < n {
		ops = append(ops, diffOp{kind: '-', line: a[i]})
		i++
	}
	for j < m {
		ops = append(ops, diffOp{kind: '+', line: b[j]})
		j++
	}
	return ops
}

func printUnifiedDiff(path string, oldData, newData []byte) {
	if isBinary(oldData) || isBinary(newData) {
		fmt.Printf("Binary files a/%s and b/%s differ\n", path, path)
		return
	}
	aLines := splitLines(oldData)
	bLines := splitLines(newData)
	ops := lcsDiff(aLines, bLines)
	changed := false
	for _, op := range ops {
		if op.kind != ' ' {
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	fmt.Printf("--- a/%s\n", path)
	fmt.Printf("+++ b/%s\n", path)
	const ctx = 3
	n := len(ops)
	i := 0
	for i < n {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start := i - ctx
		if start < 0 {
			start = 0
		}
		end := i
		for end < n {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			j := end
			for j < n && ops[j].kind == ' ' {
				j++
			}
			if j == n || j-end > 2*ctx {
				e := end + ctx
				if e > j {
					e = j
				}
				emitHunk(ops, start, e)
				i = e
				goto nextHunk
			}
			end = j
		}
		emitHunk(ops, start, n)
		i = n
	nextHunk:
	}
}

func emitHunk(ops []diffOp, start, end int) {
	var oldCount, newCount int
	for k := start; k < end; k++ {
		if ops[k].kind != '+' {
			oldCount++
		}
		if ops[k].kind != '-' {
			newCount++
		}
	}
	oldStart := 1
	newStart := 1
	for k := 0; k < start; k++ {
		if ops[k].kind != '+' {
			oldStart++
		}
		if ops[k].kind != '-' {
			newStart++
		}
	}
	fmt.Printf("@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
	for k := start; k < end; k++ {
		fmt.Printf("%c%s\n", ops[k].kind, ops[k].line)
	}
}
