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

// dataDir returns the directory containing the NBFC registry.
// Tries multiple locations in order: installed binary dir, CWD, home dir.
func dataDir() string {
	paths := []string{
		// Binary installed location (homebrew, go install)
		filepath.Join(filepath.Dir(os.Args[0]), "data"),
		// Current working directory (dev mode)
		"./data",
		// Relative to repo root
		"../data",
		// Explicit HOME path
		filepath.Join(os.Getenv("HOME"), ".finwipe", "data"),
		// Explicit absolute path for this repo
		"/Users/Subho/finwipe/data",
	}
	for _, p := range paths {
		// Check if the directory exists OR the nbfcs.yaml file exists in it
		if _, err := os.Stat(p); err == nil {
			return p
		}
		nbfcFile := filepath.Join(p, "nbfcs.yaml")
		if _, err := os.Stat(nbfcFile); err == nil {
			return p
		}
	}
	// Default: return CWD data dir
	return "./data"
}

// nbfcRegistryPath returns the path to the NBFC registry YAML.
// Tries multiple known locations to find the registry.
func nbfcRegistryPath() string {
	paths := []string{
		// Installed binary location
		filepath.Join(filepath.Dir(os.Args[0]), "data", "nbfcs.yaml"),
		// CWD
		"./data/nbfcs.yaml",
		// Relative to repo root
		"../data/nbfcs.yaml",
		// Home dir
		filepath.Join(os.Getenv("HOME"), ".finwipe", "data", "nbfcs.yaml"),
		// Explicit path
		"/Users/Subho/finwipe/data/nbfcs.yaml",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Default to binary location
	return paths[0]
}
