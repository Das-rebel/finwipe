package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var massCmd = &cobra.Command{
	Use:   "mass-request",
	Short: "Send deletion requests to ALL entities in a category",
	Long: `Send deletion requests to all entities in a category with ONE command.

This is the "nuclear option" — sends requests to everyone.

Categories:
  bank     — 12 banks
  fintech  — 59 fintechs
  nbfc     — 18 NBFCs
  hfc      — 2 HFCs
  all      — ALL 91 entities

⚠️  WARNING: You will receive ~90 acknowledgment emails!

Usage:
  finwipe mass-request --category fintech --dry-run
  finwipe mass-request --category all --dry-run=false
  finwipe mass-request --category bank --exclude bajaj-finserv,hdfc-bank`,
	RunE:  runMassRequest,
}

var (
	massCategory  string
	massExclude  string
	massInclude  string
	massCount    int
	massLegalBasis string // dpdp, rbi, both
)

func init() {
	massCmd.Flags().StringVar(&massCategory, "category", "",
		"Category: bank, fintech, nbfc, hfc, all")
	massCmd.Flags().StringVar(&massExclude, "exclude", "",
		"Exclude NBFC IDs (comma-separated)")
	massCmd.Flags().StringVar(&massInclude, "include", "",
		"Include only these NBFC IDs (comma-separated)")
	massCmd.Flags().IntVar(&massCount, "count", 0,
		"Send to exactly N entities (picks randomly)")
	massCmd.Flags().StringVar(&massLegalBasis, "legal-basis", "both",
		"Legal basis: dpdp, rbi, both")
}

func runMassRequest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	category := strings.ToLower(massCategory)
	if category == "" {
		fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
		fmt.Println("  ║  Mass Deletion Request                                    ║")
		fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Println("  Usage: finwipe mass-request --category <name>")
		fmt.Println()
		fmt.Println("  Categories:")
		fmt.Println("    bank     — 12 banks")
		fmt.Println("    fintech  — 59 fintechs")
		fmt.Println("    nbfc     — 18 NBFCs")
		fmt.Println("    hfc      — 2 HFCs")
		fmt.Println("    all      — ALL 91 entities")
		fmt.Println()
		fmt.Println("  Examples:")
		fmt.Println("    finwipe mass-request --category fintech --dry-run")
		fmt.Println("    finwipe mass-request --category bank --exclude hdfc-bank")
		fmt.Println("    finwipe mass-request --category all -n 10 --dry-run")
		return nil
	}

	// Load entities
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	// Filter by category
	var targets []nbfc.Entity
	catMap := map[string]nbfc.Category{
		"bank":    nbfc.CatBANK,
		"fintech": nbfc.CatFINTECH,
		"nbfc":    nbfc.CatNBFC,
		"hfc":     nbfc.CatHFC,
	}

	for _, e := range entities {
		if category == "all" {
			targets = append(targets, e)
		} else if cat, ok := catMap[category]; ok {
			if e.Category == cat {
				targets = append(targets, e)
			}
		}
	}

	// Exclude
	excludeMap := make(map[string]bool)
	if massExclude != "" {
		for _, id := range strings.Split(massExclude, ",") {
			excludeMap[strings.TrimSpace(id)] = true
		}
	}

	var filtered []nbfc.Entity
	for _, e := range targets {
		if excludeMap[e.ID] {
			continue
		}
		if e.GrievanceEmail == "" {
			continue
		}
		filtered = append(filtered, e)
	}
	targets = filtered

	// Limit count
	if massCount > 0 && massCount < len(targets) {
		// Pick first N (deterministic)
		targets = targets[:massCount]
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  Mass Deletion Request                                    ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📋 Category: %s\n", category)
	fmt.Printf("  📊 Total targets: %d\n", len(targets))
	if massExclude != "" {
		fmt.Printf("  🚫 Excluded: %s\n", massExclude)
	}
	if massCount > 0 {
		fmt.Printf("  🎯 Limited to: %d\n", massCount)
	}
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")

	// Count by category
	catCounts := make(map[string]int)
	for _, e := range targets {
		catCounts[string(e.Category)]++
	}
	for cat, count := range catCounts {
		fmt.Printf("     %s: %d\n", cat, count)
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  ⚠️  WARNING: You are about to send emails to ALL these entities!")
	if len(targets) > 10 {
		fmt.Printf("     That's %d emails.\n", len(targets))
		fmt.Printf("     You will receive ~%d acknowledgment emails.\n", len(targets))
	}
	fmt.Println()

	if dryRun {
		fmt.Println("  🔍 DRY RUN — Preview (first 15):")
		fmt.Println()
		for i, e := range targets {
			if i >= 15 {
				fmt.Printf("  ... and %d more\n", len(targets)-i)
				break
			}
			icon := "💳"
			if e.Category == nbfc.CatBANK {
				icon = "🏛️"
			} else if e.Category == nbfc.CatHFC {
				icon = "🏠"
			}
			fmt.Printf("  %2d. %s %-28s %s\n", i+1, icon, truncate(e.Name, 28), e.GrievanceEmail)
		}
		fmt.Println()
		fmt.Println("  Run with --dry-run=false to actually send.")
		return nil
	}

	// Create requests
	fmt.Println("  🚀 Creating deletion requests...")
	fmt.Println()

	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	os.MkdirAll(letterDir, 0700)

	// Parse legal basis
	var legalBasis letter.LegalBasis
	switch massLegalBasis {
	case "dpdp":
		legalBasis = letter.LegalBasisDPDP
	case "rbi":
		legalBasis = letter.LegalBasisRBI
	default:
		legalBasis = letter.LegalBasisBoth
	}

	gen := letter.New(letterDir)

	created := 0
	skipped := 0
	existingCount := 0

	for i, e := range targets {
		// Check if already exists
		existing, _ := hist.GetByNBFCID(e.ID)
		isDup := false
		for _, req := range existing {
			if req.LifecycleState != history.StateClosed &&
				req.LifecycleState != history.StateDeliveryFailed {
				isDup = true
				break
			}
		}

		if isDup {
			existingCount++
			skipped++
			continue
		}

		req, err := hist.CreateRequest(
			e.ID, e.Name,
			history.ChannelEmail,
			e.GrievanceEmail,
			cfg.Profile.Email, cfg.Profile.Name)
		if err != nil {
			fmt.Printf("  ⚠️  %-28s %v\n", e.Name, err)
			skipped++
			continue
		}

		gen.Generate(req.RequestID, e.Name, e.GrievanceEmail,
			cfg.Profile, letter.DefaultDeletionCategories, legalBasis)

		icon := "💳"
		if e.Category == nbfc.CatBANK {
			icon = "🏛️"
		}
		fmt.Printf("  ✅ %2d/%d %s %-25s %s\n", i+1, len(targets), icon, e.Name, req.RequestID)
		created++

		// Rate limit
		if i < len(targets)-1 {
			timeSleep(200 * time.Millisecond)
		}
	}

	fmt.Println()
	fmt.Printf("  ✅ Created: %d | Skipped (exists): %d\n", created, existingCount)
	fmt.Println()
	fmt.Println("  📋 NEXT STEPS:")
	fmt.Println()
	fmt.Println("  1. finwipe send --dry-run  # Preview emails")
	fmt.Println("  2. finwipe send           # Actually send emails")
	fmt.Println("  3. finwipe track --all    # Monitor acknowledgments")
	fmt.Println("  4. finwipe cron --followup # Auto follow-up after 48h")
	fmt.Println()

	return nil
}

func timeSleep(d time.Duration) {
	time.Sleep(d)
}
