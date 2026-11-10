package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/index"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/walk"
)

// Bisect state files.
const (
	bisectLog       = "BISECT_LOG"
	bisectStart     = "BISECT_START"
	bisectBad       = "BISECT_BAD"
	bisectGood      = "BISECT_GOOD"
	bisectTermsBad  = "BISECT_TERMS_BAD"
	bisectTermsGood = "BISECT_TERMS_GOOD"
)

// cmdBisect finds by binary search the change that introduced a bug.
func cmdBisect(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo bisect <start|bad|good|skip|reset|visualize|log|next|run>")
	}
	switch args[0] {
	case "start":
		return bisectStartCmd(a, args[1:])
	case "bad":
		return bisectMark(a, bisectBad, args[1:], "bad")
	case "good":
		return bisectMark(a, bisectGood, args[1:], "good")
	case "skip":
		return bisectSkip(a, args[1:])
	case "reset":
		return bisectReset(a, args[1:])
	case "log":
		return bisectLogCmd(a)
	case "visualize":
		return bisectVisualize(a)
	case "next":
		return bisectNext(a)
	case "run":
		return bisectRun(a, args[1:])
	default:
		return fmt.Errorf("unknown bisect subcommand: %s", args[0])
	}
}

func bisectDir(a *app.App) string {
	return a.Repo.GitDir
}

type bisectState struct {
	start sha1.Digest
	bad   sha1.Digest
	goods []sha1.Digest
	badT  string
	goodT string
}

func readBisectState(a *app.App) (*bisectState, error) {
	st := &bisectState{badT: "bad", goodT: "good"}
	if b, err := os.ReadFile(filepath.Join(bisectDir(a), bisectTermsBad)); err == nil {
		st.badT = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(bisectDir(a), bisectTermsGood)); err == nil {
		st.goodT = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(bisectDir(a), bisectStart)); err == nil {
		d, err := sha1.FromHex(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, err
		}
		st.start = d
	}
	if b, err := os.ReadFile(filepath.Join(bisectDir(a), bisectBad)); err == nil {
		d, err := sha1.FromHex(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, err
		}
		st.bad = d
	}
	if b, err := os.ReadFile(filepath.Join(bisectDir(a), bisectGood)); err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(b)))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			d, err := sha1.FromHex(line)
			if err != nil {
				continue
			}
			st.goods = append(st.goods, d)
		}
	}
	return st, nil
}

func writeBisectFile(a *app.App, name, value string) error {
	return os.WriteFile(filepath.Join(bisectDir(a), name), []byte(value+"\n"), 0o644)
}

func bisectStartCmd(a *app.App, args []string) error {
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("no commits to bisect")
	}

	bad := headID
	var goods []sha1.Digest
	idx := 0
	if len(args) > 0 {
		bad, err = resolveCommitish(a, args[0])
		if err != nil {
			return err
		}
		idx = 1
	}
	for ; idx < len(args); idx++ {
		g, err := resolveCommitish(a, args[idx])
		if err != nil {
			return err
		}
		goods = append(goods, g)
	}

	for _, f := range []string{bisectStart, bisectBad, bisectGood, bisectLog} {
		_ = os.Remove(filepath.Join(bisectDir(a), f))
	}
	if err := writeBisectFile(a, bisectStart, bad.Hex()); err != nil {
		return err
	}
	if err := writeBisectFile(a, bisectBad, bad.Hex()); err != nil {
		return err
	}
	var goodBuf strings.Builder
	for _, g := range goods {
		goodBuf.WriteString(g.Hex() + "\n")
	}
	if err := os.WriteFile(filepath.Join(bisectDir(a), bisectGood), []byte(goodBuf.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bisectDir(a), bisectTermsBad), []byte("bad\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bisectDir(a), bisectTermsGood), []byte("good\n"), 0o644); err != nil {
		return err
	}
	appendBisectLog(a, "bisect start\n")

	fmt.Printf("Bisecting: %d candidates (started from %s)\n", countCandidates(a, bad, goods), bad.Hex()[:7])
	return bisectNext(a)
}

func bisectMark(a *app.App, file string, args []string, term string) error {
	target := headOrArg(a, args)
	id, err := resolveCommitish(a, target)
	if err != nil {
		return err
	}
	if err := writeBisectFile(a, file, id.Hex()); err != nil {
		return err
	}
	appendBisectLog(a, fmt.Sprintf("bisect %s %s\n", term, id.Hex()))
	return bisectNext(a)
}

func headOrArg(a *app.App, args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return ""
	}
	return headID.Hex()
}

func bisectSkip(a *app.App, args []string) error {
	return bisectNext(a)
}

func bisectReset(a *app.App, args []string) error {
	st, err := readBisectState(a)
	if err != nil {
		return err
	}
	target := st.start
	if len(args) > 0 {
		id, err := resolveCommitish(a, args[0])
		if err != nil {
			return err
		}
		target = id
	}
	for _, f := range []string{bisectStart, bisectBad, bisectGood, bisectLog, bisectTermsBad, bisectTermsGood} {
		_ = os.Remove(filepath.Join(bisectDir(a), f))
	}
	if err := a.Refs.SetHead(target.Hex()); err != nil {
		return err
	}
	fmt.Printf("Bisect reset: HEAD at %s\n", target.Hex()[:7])
	return nil
}

func bisectLogCmd(a *app.App) error {
	b, err := os.ReadFile(filepath.Join(bisectDir(a), bisectLog))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No bisect log.")
			return nil
		}
		return err
	}
	fmt.Print(string(b))
	return nil
}

func bisectVisualize(a *app.App) error {
	st, err := readBisectState(a)
	if err != nil {
		return err
	}
	fmt.Printf("start: %s\n", st.start.Hex()[:7])
	fmt.Printf("bad:   %s\n", st.bad.Hex()[:7])
	for _, g := range st.goods {
		fmt.Printf("good:  %s\n", g.Hex()[:7])
	}
	return bisectNext(a)
}

func bisectNext(a *app.App) error {
	st, err := readBisectState(a)
	if err != nil {
		return err
	}
	if st.bad == sha1.Zero {
		return fmt.Errorf("bisect not in progress (use 'vstodo bisect start')")
	}

	walker := &walk.Walker{Load: func(id sha1.Digest) (*object.Object, error) { return a.Store.Load(id) }}
	badHist, err := walker.History(st.bad)
	if err != nil {
		return err
	}
	excluded := make(map[sha1.Digest]bool)
	for _, g := range st.goods {
		gHist, err := walker.History(g)
		if err != nil {
			return err
		}
		for _, h := range gHist {
			excluded[h] = true
		}
	}
	var candidates []sha1.Digest
	for _, h := range badHist {
		if !excluded[h] {
			candidates = append(candidates, h)
		}
	}

	if len(candidates) == 0 {
		fmt.Printf("%s is the first %s commit\n", st.bad.Hex()[:7], st.badT)
		return nil
	}

	mid := candidates[len(candidates)/2]
	if err := checkoutCommit(a, mid); err != nil {
		return err
	}

	fmt.Printf("Bisecting: %d candidates remaining, testing %s\n", len(candidates), mid.Hex()[:7])
	return nil
}

func bisectRun(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo bisect run <command>")
	}
	for {
		st, err := readBisectState(a)
		if err != nil {
			return err
		}
		if st.bad == sha1.Zero {
			return fmt.Errorf("bisect not in progress")
		}
		walker := &walk.Walker{Load: func(id sha1.Digest) (*object.Object, error) { return a.Store.Load(id) }}
		badHist, err := walker.History(st.bad)
		if err != nil {
			return err
		}
		excluded := make(map[sha1.Digest]bool)
		for _, g := range st.goods {
			gHist, err := walker.History(g)
			if err != nil {
				return err
			}
			for _, h := range gHist {
				excluded[h] = true
			}
		}
		var candidates []sha1.Digest
		for _, h := range badHist {
			if !excluded[h] {
				candidates = append(candidates, h)
			}
		}
		if len(candidates) == 0 {
			fmt.Printf("%s is the first %s commit\n", st.bad.Hex()[:7], st.badT)
			return nil
		}
		mid := candidates[len(candidates)/2]
		if err := checkoutCommit(a, mid); err != nil {
			return err
		}

		fmt.Printf("Running %s on %s\n", strings.Join(args, " "), mid.Hex()[:7])
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		status := cmd.Run()
		if err := markFromStatus(a, mid, status); err != nil {
			return err
		}
	}
}

func markFromStatus(a *app.App, mid sha1.Digest, status error) error {
	if status == nil {
		if err := writeBisectFile(a, bisectGood, mid.Hex()); err != nil {
			return err
		}
		appendBisectLog(a, fmt.Sprintf("bisect good %s\n", mid.Hex()))
	} else {
		exitCode := 1
		if ex, ok := status.(*exec.ExitError); ok {
			exitCode = ex.ExitCode()
		}
		if exitCode == 127 || exitCode == 128 {
			appendBisectLog(a, fmt.Sprintf("bisect skip %s\n", mid.Hex()))
		} else {
			if err := writeBisectFile(a, bisectBad, mid.Hex()); err != nil {
				return err
			}
			appendBisectLog(a, fmt.Sprintf("bisect bad %s\n", mid.Hex()))
		}
	}
	return bisectNext(a)
}

func checkoutCommit(a *app.App, id sha1.Digest) error {
	if err := a.Refs.SetHead(id.Hex()); err != nil {
		return err
	}
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
	for path := range a.Index.Entries() {
		_ = removeFile(a, path)
	}
	for path, bid := range tm {
		data, err := readBlob(a, bid)
		if err != nil {
			return err
		}
		if err := writeFile(a, path, data); err != nil {
			return err
		}
	}
	a.Index = indexFromTree(tm)
	return a.SaveIndex()
}

func indexFromTree(tm map[string]sha1.Digest) *index.Index {
	idx := index.NewIndex("")
	for p, d := range tm {
		idx.Add(p, d)
	}
	return idx
}

func appendBisectLog(a *app.App, line string) error {
	f, err := os.OpenFile(filepath.Join(bisectDir(a), bisectLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func countCandidates(a *app.App, bad sha1.Digest, goods []sha1.Digest) int {
	walker := &walk.Walker{Load: func(id sha1.Digest) (*object.Object, error) { return a.Store.Load(id) }}
	badHist, err := walker.History(bad)
	if err != nil {
		return 0
	}
	excluded := make(map[sha1.Digest]bool)
	for _, g := range goods {
		gHist, err := walker.History(g)
		if err != nil {
			continue
		}
		for _, h := range gHist {
			excluded[h] = true
		}
	}
	n := 0
	for _, h := range badHist {
		if !excluded[h] {
			n++
		}
	}
	return n
}

func resolveCommitish(a *app.App, s string) (sha1.Digest, error) {
	if d, err := sha1.FromHex(s); err == nil {
		if ok, _ := a.Store.Has(d); ok {
			return d, nil
		}
	}
	if d, err := a.Refs.ReadRef("refs/heads/" + s); err == nil {
		return d, nil
	}
	return sha1.Zero, fmt.Errorf("cannot resolve %s", s)
}
