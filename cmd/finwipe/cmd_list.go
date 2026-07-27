package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered NBFCs",
	RunE:  runList,
}

var (
	listCategory string
	listSearch  string
	listJSON    bool
)

func init() {
	listCmd.Flags().StringVar(&listCategory, "category", "", "Filter by category: nbfc, hfc, fintech, lsp, dsp, bank")
	listCmd.Flags().StringVar(&listSearch, "search", "", "Search by name")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
}

func runList(cmd *cobra.Command, args []string) error {
	nbfcDataDir := dataDir()
	nbfcPath := filepath.Join(nbfcDataDir, "nbfcs.yaml")
	if _, err := os.Stat(nbfcPath); err != nil {
		exePath, _ := os.Executable()
		nbfcPath = filepath.Join(filepath.Dir(exePath), "data", "nbfcs.yaml")
	}

	nbfcs, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}

	// Filter by category
	if listCategory != "" {
		catMap := map[string]nbfc.Category{
			"nbfc": nbfc.CatNBFC, "hfc": nbfc.CatHFC,
			"fintech": nbfc.CatFINTECH, "lsp": nbfc.CatLSP,
			"dsp": nbfc.CatDSP, "bank": nbfc.CatBANK,
		}
		if cat, ok := catMap[listCategory]; ok {
			nbfcs = nbfc.FilterByCategory(nbfcs, []nbfc.Category{cat})
		}
	}

	// Filter by search
	if listSearch != "" {
		lower := strings.ToLower(listSearch)
		var filtered []nbfc.Entity
		for _, n := range nbfcs {
			if strings.Contains(strings.ToLower(n.Name), lower) ||
				strings.Contains(strings.ToLower(n.ShortName), lower) ||
				strings.Contains(strings.ToLower(n.ID), lower) {
				filtered = append(filtered, n)
			}
		}
		nbfcs = filtered
	}

	if listJSON {
		fmt.Printf("[\n")
		for i, n := range nbfcs {
			sep := ","
			if i == len(nbfcs)-1 {
				sep = ""
			}
			fmt.Printf(`  {"id":"%s","name":"%s","category":"%s","email":"%s"}%s`+"\n",
				n.ID, n.Name, n.Category, n.GrievanceEmail, sep)
		}
		fmt.Printf("]\n")
		return nil
	}

	fmt.Printf("\n📋 FinWipe NBFC Registry (%d entities)\n\n", len(nbfcs))

	byCat := make(map[nbfc.Category][]nbfc.Entity)
	for _, n := range nbfcs {
		byCat[n.Category] = append(byCat[n.Category], n)
	}

	catEmoji := map[nbfc.Category]string{
		nbfc.CatNBFC:    "🏦",
		nbfc.CatHFC:     "🏠",
		nbfc.CatFINTECH: "💳",
		nbfc.CatLSP:     "🔗",
		nbfc.CatDSP:     "📊",
		nbfc.CatBANK:    "🏛️",
	}

	for cat, items := range byCat {
		fmt.Printf("%s %s (%d)\n", catEmoji[cat], strings.ToUpper(string(cat)), len(items))
		fmt.Println(strings.Repeat("─", 60))
		for _, n := range items {
			email := n.GrievanceEmail
			if email == "" {
				email = "—"
			}
			fmt.Printf("  %-30s %s\n", n.Name, email)
		}
		fmt.Println()
	}

	fmt.Printf("Total: %d NBFCs\n\n", len(nbfcs))
	return nil
}
