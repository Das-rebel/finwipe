package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new deletion request (returns DPR-ID for tracking)",
	Long: `Create one or more tracked deletion requests.

Single:  finwipe new --nbfc bajaj-finserv
Batch:   finwipe new --batch --category fintech

Each request gets a unique DPR-ID (DPR-2026-NNNNNN) for full lifecycle tracking.`,
	RunE: runNew,
}

var (
	newNBFCID   string
	newChannel  string
	newDataCats string
	newBatch    bool
	newCategory string
)

func init() {
	newCmd.Flags().StringVar(&newNBFCID, "nbfc", "", "NBFC ID from registry (required if not --batch)")
	newCmd.Flags().BoolVar(&newBatch, "batch", false, "Create requests for all matching NBFCs")
	newCmd.Flags().StringVar(&newCategory, "category", "",
		"Filter by category (nbfc, hfc, fintech, lsp, dsp, bank) — use with --batch")
	newCmd.Flags().StringVar(&newChannel, "channel", "email",
		"Delivery channel: email | post | cic")
	newCmd.Flags().StringVar(&newDataCats, "data-categories",
		"marketing,third_party,behavioral,call_recordings,preapproved",
		"Comma-separated data categories")
}

func runNew(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	nbfcPath := filepath.Join(dataDir(), "nbfcs.yaml")
	entities, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	if newBatch {
		return runNewBatch(hist, entities, cfg)
	}

	// ── Single NBFC ─────────────────────────────────────────────
	if newNBFCID == "" {
		return fmt.Errorf("--nbfc is required (or use --batch to create all)")
	}

	sanitizedID := history.SanitizeNBFCID(newNBFCID)
	var target nbfc.Entity
	for _, e := range entities {
		if history.SanitizeNBFCID(e.ID) == sanitizedID {
			target = e
			break
		}
	}
	if target.ID == "" {
		return fmt.Errorf("NBFC not found: %s\nUse: finwipe list --search <name>", newNBFCID)
	}

	if target.GrievanceEmail == "" {
		return fmt.Errorf("NBFC %s has no grievance email — use registered post instead", target.Name)
	}

	req, err := hist.CreateRequest(target.ID, target.Name, newChannel, target.GrievanceEmail, cfg.Profile.Email, cfg.Profile.Name)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	fmt.Printf("\n")
	fmt.Printf("  🆕 Request created: %s\n", req.RequestID)
	fmt.Printf("  📦 NBFC:           %s\n", req.NBFCName)
	fmt.Printf("  📮 Channel:        %s\n", history.ChannelLabel(req.Channel))
	fmt.Printf("  ⏰ Ack deadline:   %s\n", req.AckDeadlineAt.Format("02 Jan 2006 15:04 MST"))
	fmt.Printf("  📧 To:             %s\n", req.GrievanceEmail)
	fmt.Println()
	fmt.Printf("Next step:\n")
	switch req.Channel {
	case history.ChannelEmail:
		fmt.Printf("  finwipe send --request-id %s\n", req.RequestID)
	case history.ChannelPost:
		fmt.Printf("  finwipe letter --request-id %s   # generate signed PDF\n", req.RequestID)
		fmt.Printf("  finwipe send --request-id %s    # mark as dispatched\n", req.RequestID)
	default:
		fmt.Printf("  finwipe cic --bureau CIBIL\n")
	}
	fmt.Println()
	return nil
}

// runNewBatch creates requests for all NBFCs matching --category filter
func runNewBatch(hist *history.DB, entities []nbfc.Entity, cfg *config.Config) error {
	catMap := map[string]nbfc.Category{
		"nbfc":    nbfc.CatNBFC,
		"hfc":     nbfc.CatHFC,
		"fintech": nbfc.CatFINTECH,
		"lsp":     nbfc.CatLSP,
		"dsp":     nbfc.CatDSP,
		"bank":    nbfc.CatBANK,
	}

	var targets []nbfc.Entity
	if newCategory != "" {
		cat, ok := catMap[newCategory]
		if !ok {
			return fmt.Errorf("invalid category: %s (use: nbfc, hfc, fintech, lsp, dsp, bank)", newCategory)
		}
		for _, e := range entities {
			if e.Category == cat && e.GrievanceEmail != "" && e.Active {
				targets = append(targets, e)
			}
		}
	} else {
		for _, e := range entities {
			if e.GrievanceEmail != "" && e.Active {
				targets = append(targets, e)
			}
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("no NBFCs match the filter")
	}

	fmt.Printf("\n")
	fmt.Printf("  📋 Batch create: %d NBFCs\n", len(targets))
	if newCategory != "" {
		fmt.Printf("  🏷️  Category:     %s\n", newCategory)
	}
	fmt.Printf("  📧 Channel:       %s\n", newChannel)
	fmt.Printf("  ⏰ Ack deadline:  72 hours from dispatch\n\n")
	fmt.Printf("Creating requests...\n\n")

	var created []string
	for _, e := range targets {
		req, err := hist.CreateRequest(e.ID, e.Name, newChannel, e.GrievanceEmail, cfg.Profile.Email, cfg.Profile.Name)
		if err != nil {
			fmt.Printf("  ⚠️  %-35s %v\n", e.Name, err)
			continue
		}
		created = append(created, req.RequestID)
		fmt.Printf("  ✅ %-35s %s\n", e.Name, req.RequestID)
	}

	fmt.Printf("\n")
	if len(created) > 0 {
		fmt.Printf("  %d requests created\n", len(created))
		fmt.Println()
		fmt.Printf("Next step:\n")
		fmt.Printf("  finwipe send --request-id %s   # send first one\n", created[0])
		if len(created) > 1 {
			fmt.Printf("  finwipe send --include %s   # send all %d at once\n",
				strings.Join(created[:3], ",")+"...", len(created))
		}
		fmt.Println()
	}

	// Write request IDs to file for scripting
	idsFile := filepath.Join(os.TempDir(), "finwipe_batch_ids.txt")
	if len(created) > 0 {
		os.WriteFile(idsFile, []byte(strings.Join(created, "\n")), 0600)
		fmt.Printf("Request IDs saved to: %s\n", idsFile)
	}

	return nil
}
