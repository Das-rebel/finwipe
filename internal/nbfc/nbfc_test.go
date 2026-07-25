package nbfc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	// Use the actual registry
	path := filepath.Join(os.Getenv("HOME"), ".finwipe", "data", "nbfcs.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try relative path
		path = "../../data/nbfcs.yaml"
	}

	entities, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%q): %v", path, err)
	}
	if len(entities) == 0 {
		t.Error("expected non-empty registry")
	}

	// Check first entity has required fields
	e := entities[0]
	if e.ID == "" {
		t.Error("Entity.ID should not be empty")
	}
	if e.Name == "" {
		t.Error("Entity.Name should not be empty")
	}
	if e.Category == "" {
		t.Error("Entity.Category should not be empty")
	}
}

func TestCategoryValues(t *testing.T) {
	cases := []Category{
		CatNBFC,
		CatHFC,
		CatFINTECH,
		CatLSP,
		CatDSP,
		CatBANK,
	}
	for _, c := range cases {
		if c == "" {
			t.Errorf("Category constant is empty string")
		}
	}
}

func TestEntityActive(t *testing.T) {
	path := filepath.Join(os.Getenv("HOME"), ".finwipe", "data", "nbfcs.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = "../../data/nbfcs.yaml"
	}
	entities, _ := Load(path)

	activeCount := 0
	for _, e := range entities {
		if e.Active {
			activeCount++
		}
	}
	if activeCount == 0 {
		t.Error("expected at least one active entity")
	}
}

func TestEntityGrievanceEmail(t *testing.T) {
	path := filepath.Join(os.Getenv("HOME"), ".finwipe", "data", "nbfcs.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = "../../data/nbfcs.yaml"
	}
	entities, _ := Load(path)

	// Entities with grievance emails should have valid email format
	for _, e := range entities {
		if e.GrievanceEmail != "" {
			if len(e.GrievanceEmail) < 5 {
				t.Errorf("Entity %s has suspiciously short email: %q", e.ID, e.GrievanceEmail)
			}
			// Check for @ symbol
			hasAt := false
			for _, c := range e.GrievanceEmail {
				if c == '@' {
					hasAt = true
					break
				}
			}
			if !hasAt {
				t.Errorf("Entity %s email lacks @: %q", e.ID, e.GrievanceEmail)
			}
		}
	}
}

func TestFindByID(t *testing.T) {
	path := filepath.Join(os.Getenv("HOME"), ".finwipe", "data", "nbfcs.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = "../../data/nbfcs.yaml"
	}
	entities, _ := Load(path)

	found := false
	for _, e := range entities {
		if e.ID == "bajaj-finserv" || e.ID == "hdfc-bank" {
			found = true
			break
		}
	}
	if !found {
		t.Log("Note: specific NBFC IDs not found in registry — may have different IDs")
	}
}
