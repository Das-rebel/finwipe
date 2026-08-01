package nbfc

type Category string

const (
	CatNBFC    Category = "nbfc"
	CatHFC            = "hfc"
	CatFINTECH        = "fintech"
	CatLSP            = "lsp"
	CatDSP            = "dsp"
	CatBANK           = "bank"
)

type Entity struct {
	ID             string   `yaml:"id"`
	Name           string   `yaml:"name"`
	ShortName      string   `yaml:"short_name"`
	Category       Category `yaml:"category"`
	GrievanceEmail string   `yaml:"grievance_email"`
	GrievancePhone string   `yaml:"grievance_phone"`
	Address        string   `yaml:"address"`
	DLAApp         string   `yaml:"dla_app"`
	Website        string   `yaml:"website"`
	Notes          string   `yaml:"notes"`
	Active         bool     `yaml:"active"`
}

type Registry struct {
	Entities []Entity `yaml:"nbfcs"`
}

func FilterByCategory(entities []Entity, cats []Category) []Entity {
	if len(cats) == 0 {
		return entities
	}
	catMap := make(map[Category]bool)
	for _, c := range cats {
		catMap[c] = true
	}
	var result []Entity
	for _, n := range entities {
		if catMap[n.Category] {
			result = append(result, n)
		}
	}
	return result
}
