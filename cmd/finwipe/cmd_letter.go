package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var letterCmd = &cobra.Command{
	Use:   "letter",
	Short: "Generate professional PDF deletion letters",
	RunE:  runLetter,
}

var (
	letterOutputDir string
	letterNBFCs    string // comma-separated IDs
	letterLegalBasis string // dpdp, rbi, both
)

func init() {
	letterCmd.Flags().StringVar(&letterOutputDir, "output", "", "Output directory for PDFs (default: ~/.finwipe/letters/)")
	letterCmd.Flags().StringVar(&letterNBFCs, "nbfcs", "", "Generate letters for specific NBFC IDs (comma-separated)")
	letterCmd.Flags().StringVar(&letterLegalBasis, "legal-basis", "withdrawal",
		"Legal basis: dpdp, rbi, both")
}

func runLetter(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.Profile.Name == "" {
		return fmt.Errorf("profile not configured. Run: finwipe init")
	}

	// Load NBFCs
	exePath, _ := os.Executable()
	nbfcPath := filepath.Join(filepath.Dir(exePath), "data", "nbfcs.yaml")
	if _, err := os.Stat(nbfcPath); err != nil {
		nbfcPath = "./data/nbfcs.yaml"
	}

	allNBFCs, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}

	var targetNBFCs []nbfc.Entity
	if letterNBFCs != "" {
		idMap := make(map[string]bool)
		for _, id := range strings.Split(letterNBFCs, ",") {
			idMap[id] = true
		}
		for _, n := range allNBFCs {
			if idMap[n.ID] {
				targetNBFCs = append(targetNBFCs, n)
			}
		}
	} else {
		targetNBFCs = allNBFCs
	}

	// Output dir
	outDir := letterOutputDir
	if outDir == "" {
		home, _ := os.UserHomeDir()
		outDir = filepath.Join(home, ".finwipe", "letters")
	}

	// Parse legal basis
	var legalBasis letter.LegalBasis
	switch letterLegalBasis {
	case "dpdp":
		legalBasis = letter.LegalBasisWithdrawal
	case "rbi":
		legalBasis = letter.LegalBasisRBI
	default:
		legalBasis = letter.LegalBasisBoth
	}

	gen := letter.New(outDir)
	generated, failed := gen.GenerateBatch(targetNBFCs, cfg.Profile, letter.DefaultDeletionCategories, legalBasis)

	fmt.Printf("\n📄 Generated: %d PDFs\n", generated)
	fmt.Printf("📁 Output: %s/\n", outDir)
	if len(failed) > 0 {
		fmt.Printf("\n❌ Failed (%d):\n", len(failed))
		for _, f := range failed {
			fmt.Printf("  %s\n", f)
		}
	}
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Print the PDFs")
	fmt.Println("  2. Sign each letter")
	fmt.Println("  3. Send via registered post to the addresses in each letter")
	fmt.Println("  4. Track delivery via India Post tracking")

	return nil
}
