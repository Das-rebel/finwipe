package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/cic"
	"github.com/das-rebel/finwipe/internal/config"
)

var cicCmd = &cobra.Command{
	Use:   "cic",
	Short: "Generate pre-filled CIC (CIBIL/Experian/Equifax/CRIF) dispute forms",
	RunE:  runCIC,
}

var (
	cicBureau string
)

func init() {
	cicCmd.Flags().StringVar(&cicBureau, "bureau", "CIBIL",
		"CIC bureau: CIBIL, Experian, Equifax, or CRIF")
}

func runCIC(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.Profile.Name == "" {
		return fmt.Errorf("profile not configured. Run: finwipe init")
	}

	bureau := cic.Bureau(cicBureau)
	if bureau != cic.CIBIL && bureau != cic.EXPERIAN && bureau != cic.EQUIFAX && bureau != cic.CRIF {
		return fmt.Errorf("invalid bureau: %s (use CIBIL, Experian, Equifax, or CRIF)", cicBureau)
	}

	// Output dir
	home, _ := os.UserHomeDir()
	outDir := filepath.Join(home, ".finwipe", "cic-disputes")

	// Entries: User must provide these manually (parsed from CIBIL PDF or entered)
	// For now, create an empty form — entries can be filled in later
	entries := []cic.Entry{
		{
			Type:        "account",
			Description: "[Enter the account or entry to dispute — from your CIBIL report]",
			NBFCName:    "[Institution name from your CIBIL report]",
			ControlNo:   "[Control number from your CIBIL report]",
		},
	}

	gen := cic.New(outDir)
	pdfPath, err := gen.Generate(bureau, cfg.Profile, entries)
	if err != nil {
		return fmt.Errorf("generate PDF: %w", err)
	}

	fmt.Printf("\n🏛️  %s Dispute Form Generated\n", bureau)
	fmt.Printf("   📄 %s\n\n", pdfPath)
	fmt.Println("⚠️  IMPORTANT — YOU MUST SUBMIT THIS MANUALLY:")
	fmt.Println("   1. Open the PDF above")
	fmt.Println("   2. Fill in the disputed entries from your CIBIL report")
	fmt.Println("   3. Visit cibil.com → Dispute Center → Submit dispute")
	fmt.Println("   4. Upload this form as supporting document")
	fmt.Println("   5. Note your dispute reference number")
	fmt.Println()
	fmt.Println("Timeline: As soon as reasonable per Section 8(6), DPDP Act 2023")
	fmt.Println("Updates: You'll receive email/SMS every 7 days")
	fmt.Println()

	return nil
}
