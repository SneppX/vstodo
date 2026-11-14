// Package main is the vstodo CLI entry point.
package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	if cmd == "demo" {
		if err := cmdDemo(args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if cmd == "init" {
		if err := cmdInit(args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if cmd == "clone" {
		if err := cmdClone(args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
		return
	}

	a, err := app.New(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "vstodo: not a vstodo repository (or any parent directory): .vstodo\n")
		os.Exit(1)
	}

	switch cmd {
	case "clone":
		if err := cmdClone(args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "init":
		if err := cmdInit(args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "add":
		if err := cmdAdd(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "commit":
		if err := cmdCommit(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := cmdStatus(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "log":
		if err := cmdLog(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "cat-file":
		if err := cmdCatFile(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "hash-object":
		if err := cmdHashObject(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "branch":
		if err := cmdBranch(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "checkout":
		if err := cmdCheckout(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "diff":
		if err := cmdDiff(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "show":
		if err := cmdShow(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "merge":
		if err := cmdMerge(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "tag":
		if err := cmdTag(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "reset":
		if err := cmdReset(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "rm":
		if err := cmdRm(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "rebase":
		if err := cmdRebase(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "blame":
		if err := cmdBlame(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "bisect":
		if err := cmdBisect(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "stash":
		if err := cmdStash(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "cherry-pick":
		if err := cmdCherryPick(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "notes":
		if err := cmdNotes(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if err := cmdConfig(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "gc":
		if err := cmdGC(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "worktree":
		if err := cmdWorktree(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "bundle":
		if err := cmdBundle(a, args); err != nil {
			fmt.Fprintf(os.Stderr, "vstodo: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "vstodo: '%s' is not a vstodo command. See 'vstodo help'.\n", cmd)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vstodo - a simple version control system (reverse-engineered from git, extended beyond it)

Usage: vstodo <command> [options] [args]

Core commands:
  demo         Create a sample repository demonstrating VSTodo features
  clone        Clone a repository into a new directory
  init         Initialize a new vstodo repository
  config       Get and set repository configuration
  add          Add file contents to the staging index
  commit       Record changes to the repository
  status       Show the working tree status
  log          Show commit history
  branch       List, create, or delete branches
  checkout     Switch branches or restore files
  diff         Show changes between commits, the index, or the working tree
  show         Show a commit, tag, or tree
  merge        Merge changes from another branch into the current branch
  tag          Create, list, or delete tags
  reset        Reset current HEAD to the specified state
  rm           Remove files from the working tree and the index
  rebase       Reapply commits on top of another base tip

Advanced commands:
  blame        Show what revision and author last modified each line
  bisect       Find by binary search the change that introduced a bug
  stash        Stash changes in a dirty working tree away
  cherry-pick  Apply the changes introduced by some existing commits
  notes        Add or inspect object notes
  gc           Cleanup unnecessary files and optimize the repository
  worktree     Manage multiple working trees
  bundle       Move objects and refs by archive

Plumbing:
  hash-object   Compute the object ID for a file
  cat-file      Provide the content or type of repository objects

Run 'vstodo help <command>' for command-specific help.`)
}
