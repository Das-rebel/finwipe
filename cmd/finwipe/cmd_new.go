package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new deletion request (returns DPR-ID for tracking)",
	RunE:  runNew,
}

var (
	newNBFCID   string
	newChannel  string
	newDataCats string
)

func init() {
	newCmd.Flags().StringVar(&newNBFCID, "nbfc", "", "NBFC ID from registry (required)")
	newCmd.Flags().StringVar(&newChannel, "channel", "email",
		"Delivery channel: email | post | cic")
	newCmd.Flags().StringVar(&newDataCats, "data-categories",
		"marketing,third_party,behavioral,call_recordings,preapproved",
		"Comma-separated data categories to request deletion for")
	newCmd.MarkFlagRequired("nbfc")
}

func runNew(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Load NBFC registry
	nbfcPath := filepath.Join(dataDir(), "nbfcs.yaml")
	entities, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Find the NBFC
	sanitizedID := history.SanitizeNBFCID(newNBFCID)
	var target nbfc.Entity
	found := false
	for _, e := range entities {
		if history.SanitizeNBFCID(e.ID) == sanitizedID {
			target = e
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("NBFC not found: %s\nUse: finwipe list --search <name>", newNBFCID)
	}

	if target.GrievanceEmail == "" {
		return fmt.Errorf("NBFC %s has no grievance email — try registered post instead", target.Name)
	}

	if newChannel != history.ChannelEmail && newChannel != history.ChannelPost && newChannel != history.ChannelCIC {
		return fmt.Errorf("invalid channel: %s (use: email, post, cic)", newChannel)
	}

	// Init history DB
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	// Create request
	req, err := hist.CreateRequest(
		target.ID,
		target.Name,
		newChannel,
		target.GrievanceEmail,
		cfg.Profile.Email,
		cfg.Profile.Name,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Show result
	fmt.Printf("\n")
	fmt.Printf("  🆕 Request created: %s\n", req.RequestID)
	fmt.Printf("  📦 NBFC:           %s\n", req.NBFCName)
	fmt.Printf("  📮 Channel:        %s\n", history.ChannelLabel(req.Channel))
	fmt.Printf("  ⏰ Ack deadline:    %s\n", req.AckDeadlineAt.Format("02 Jan 2006 15:04 MST"))
	fmt.Printf("  📧 To:              %s\n", req.GrievanceEmail)
	fmt.Println()
	fmt.Printf("Next step:\n")
	if req.Channel == history.ChannelEmail {
		fmt.Printf("  finwipe send --request-id %s\n", req.RequestID)
	} else if req.Channel == history.ChannelPost {
		fmt.Printf("  finwipe letter --request-id %s   # generates signed PDF\n", req.RequestID)
		fmt.Printf("  finwipe send --request-id %s    # mark as dispatched\n", req.RequestID)
	} else {
		fmt.Printf("  finwipe cic --bureau CIBIL      # generate CIC dispute form\n")
	}
	fmt.Println()

	return nil
}
