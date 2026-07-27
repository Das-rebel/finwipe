package main

import (
	"os"
	"path/filepath"
)

func nbfcRegistryPath() string {
	paths := []string{
		filepath.Join(filepath.Dir(os.Args[0]), "data", "nbfcs.yaml"),
		"./data/nbfcs.yaml",
		"../data/nbfcs.yaml",
		filepath.Join(os.Getenv("HOME"), "go", "src", "github.com", "das-rebel", "finwipe", "data", "nbfcs.yaml"),
		"/Users/Subho/finwipe/data/nbfcs.yaml",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "./data/nbfcs.yaml" // fallback
}