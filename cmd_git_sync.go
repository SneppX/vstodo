package main

import (
	"fmt"

	"vstodo/internal/migrate"
)

func cmdGitStatus(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: vstodo git-status")
	}
	report, err := migrate.InspectGit(".")
	if err != nil {
		return err
	}
	fmt.Printf("Git:    %s %s\n", report.GitBranch, report.GitCommit)
	fmt.Printf("VSTodo: %s %s\n", report.VSTodoBranch, report.VSTodoCommit)
	if !report.BranchMatch {
		fmt.Println("Branches differ. Check out matching branches before syncing.")
	} else if report.SameCommit {
		fmt.Println("Git and VSTodo are synchronized at this commit.")
	} else {
		fmt.Println("Branch tips differ. Run 'vstodo sync-git' to import Git-side commits safely, or 'vstodo export-git' if VSTodo is ahead.")
	}
	return nil
}

func cmdSyncGit(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: vstodo sync-git [branch]")
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}
	report, err := migrate.SyncGit(".", branch)
	if err != nil {
		return err
	}
	if report.Updated {
		fmt.Printf("Fast-forwarded VSTodo branch %s from %s to Git commit %s (%d Git objects imported). Git refs, config, index, and working files were not modified.\n",
			report.Branch, report.PreviousCommit, report.GitCommit, report.ObjectsImported)
		fmt.Println("The VSTodo index and working tree were preserved. Run 'vstodo status' to review their changes against the new HEAD.")
		return nil
	}
	if report.VSTodoAhead {
		fmt.Printf("VSTodo branch %s is ahead of Git. No refs were changed; run 'vstodo export-git' when you want to publish those commits to Git.\n", report.Branch)
		return nil
	}
	fmt.Printf("Git and VSTodo branch %s are already synchronized at %s.\n", report.Branch, report.GitCommit)
	return nil
}
