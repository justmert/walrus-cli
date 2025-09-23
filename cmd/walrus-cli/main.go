package main

import (
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

// Version information - set by ldflags during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	// Check if configuration exists, if not and no command specified, run setup
	if !configExists() && (len(os.Args) == 1 || (len(os.Args) == 2 && os.Args[1] != "setup")) {
		// No config file exists and no specific command - run setup wizard
		color.Yellow("No configuration found. Starting setup wizard...\n")
		os.Args = []string{os.Args[0], "setup"}
	}

	// Check if we should use modern UI (default)
	useModern := true
	for _, arg := range os.Args {
		if arg == "--legacy" {
			useModern = false
			break
		}
	}

	if useModern {
		// Use modern Cobra-based CLI
		rootCmd := createRootCmd()
		if err := rootCmd.Execute(); err != nil {
			color.Red("Error: %v", err)
			os.Exit(1)
		}
	} else {
		// Fallback to legacy CLI (your existing code)
		// Remove --legacy from args
		newArgs := []string{}
		for _, arg := range os.Args {
			if arg != "--legacy" {
				newArgs = append(newArgs, arg)
			}
		}
		os.Args = newArgs

		// Call original main function
		mainLegacy()
	}
}

// configExists checks if a configuration file exists
func configExists() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	paths := []string{
		filepath.Join(home, ".config", "walrus-rclone", "config.yaml"),
		filepath.Join(home, ".walrus-rclone", "config.yaml"),
		"walrus-config.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}