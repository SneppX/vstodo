// vstodo-installer is VSTodo's cross-platform, user-scoped installer.
package main

import (
	"flag"
	"fmt"
	"os"

	"vstodo/internal/installer"
)

var Version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "vstodo-installer: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("a command is required")
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("vstodo-installer %s\n", Version)
		return nil
	case "install", "upgrade", "repair":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		source := fs.String("source", "", "path to the VSTodo executable")
		installerSource := fs.String("installer-source", "", "path to the VSTodo installer executable (defaults to the running installer)")
		root := fs.String("root", "", "custom installation root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *source == "" {
			return fmt.Errorf("--source is required")
		}
		manifest, err := installer.Install(installer.Options{
			Root: *root,
			Source: *source,
			InstallerSource: *installerSource,
			Version: Version,
		})
		if err != nil {
			return err
		}
		paths, err := installer.ResolvePaths(*root)
		if err != nil {
			return err
		}
		if err := installer.AddToUserPath(paths.BinDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update PATH automatically: %v\n", err)
			fmt.Fprintf(os.Stderr, "add this directory to PATH manually: %s\n", paths.BinDir)
		}
		fmt.Printf("VSTodo %s installed successfully. Open a new terminal to use 'vstodo'.\n", manifest.Version)
		fmt.Printf("Installation directory: %s\n", paths.Root)
		return nil
	case "verify":
		fs := flag.NewFlagSet("verify", flag.ContinueOnError)
		root := fs.String("root", "", "custom installation root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		manifest, err := installer.Verify(*root)
		if err != nil {
			return err
		}
		fmt.Printf("VSTodo %s installation is healthy (%d files verified).\n", manifest.Version, len(manifest.Files))
		return nil
	case "uninstall":
		fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
		root := fs.String("root", "", "custom installation root")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		paths, err := installer.ResolvePaths(*root)
		if err != nil {
			return err
		}
		if _, err := installer.Uninstall(*root); err != nil {
			return err
		}
		if err := installer.RemoveFromUserPath(paths.BinDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove PATH entry automatically: %v\n", err)
		}
		fmt.Println("VSTodo was uninstalled. Repository data was not modified.")
		return nil
	case "help", "--help", "-h":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Print(`vstodo-installer - VSTodo's cross-platform installer

Usage:
  vstodo-installer install --source <vstodo-binary> [--root <directory>]
  vstodo-installer upgrade --source <vstodo-binary> [--root <directory>]
  vstodo-installer repair --source <vstodo-binary> [--root <directory>]
  vstodo-installer verify [--root <directory>]
  vstodo-installer uninstall [--root <directory>]
  vstodo-installer version
`)
}
