package main

import (
	"os"
	"path/filepath"
)

// truncate truncates a string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}

// nbfcRegistryPath returns the path to the NBFC registry YAML
func nbfcRegistryPath() string {
	paths := []string{
		// Built binary location
		filepath.Join(filepath.Dir(os.Args[0]), "data", "nbfcs.yaml"),
		// Relative to current working directory
		"./data/nbfcs.yaml",
		// Relative to repo root (for development)
		"../data/nbfcs.yaml",
		// Home directory
		filepath.Join(os.Getenv("HOME"), "go", "src", "github.com", "das-rebel", "finwipe", "data", "nbfcs.yaml"),
		// Absolute paths from common setups
		"/Users/Subho/finwipe/data/nbfcs.yaml",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Default to first path
	return paths[0]
}
