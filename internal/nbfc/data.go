package nbfc

import (
	"embed"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed nbfcs.yaml
var embeddedFS embed.FS

// Load returns NBFC entities from embedded data first, then falls back to filesystem.
// Priority: 1. explicit path  2. embedded  3. ~/.finwipe/nbfcs.yaml  4. executable dir
func Load(path string) ([]Entity, error) {
	// 1. If explicit path provided and exists, use it
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			return unmarshalYAML(data)
		}
	}

	// 2. Try embedded data (bundled with binary)
	if data, err := embeddedFS.ReadFile("nbfcs.yaml"); err == nil {
		return unmarshalYAML(data)
	}

	// 3. Try ~/.finwipe/nbfcs.yaml
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := filepath.Join(home, ".finwipe", "nbfcs.yaml")
		if data, err := os.ReadFile(homePath); err == nil {
			return unmarshalYAML(data)
		}
	}

	// 4. Try relative to executable
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeDataPath := filepath.Join(exeDir, "data", "nbfcs.yaml")
		if data, err := os.ReadFile(exeDataPath); err == nil {
			return unmarshalYAML(data)
		}
	}

	return nil, ErrNoDataFound
}

// ErrNoDataFound is returned when no NBFC data can be located.
var ErrNoDataFound = &DataNotFoundError{}

// DataNotFoundError indicates no NBFC registry data was found.
type DataNotFoundError struct{}

func (e *DataNotFoundError) Error() string {
	return "nbfc: no data found in embedded filesystem, ~/.finwipe/, or executable directory"
}

func unmarshalYAML(data []byte) ([]Entity, error) {
	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	var active []Entity
	for _, n := range reg.Entities {
		if n.Active {
			active = append(active, n)
		}
	}
	return active, nil
}
