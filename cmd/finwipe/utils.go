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
	exePath, _ := os.Executable()
	nbfcPath := filepath.Join(filepath.Dir(exePath), "data", "nbfcs.yaml")
	if _, err := os.Stat(nbfcPath); err != nil {
		nbfcPath = "./data/nbfcs.yaml"
	}
	return nbfcPath
}
