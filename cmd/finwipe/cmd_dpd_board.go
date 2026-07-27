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

var (
	dpdBoardCmd = &cobra.Command{
		Use:   "dpd-board",
		Short: "File complaint with Data Protection Board of India (DPBB)",
		Long: `Generate a pre-filled complaint to the Data Protection Board of India
under Section 27(3), DPDP Act 2023.

The DPBB is the PRIMARY statutory authority for data protection violations.
Use when an entity has ignored your deletion request for 30+ days.

Legal basis:
  - DPDP Act 2023, Section 27(3): "Any person aggrieved..."
  - DPDP Act 2023, Section 8(6): Right to erasure

How to submit:
  1. Online: https://dpdpboard.gov.in (Form III)
  2. Email: complaints@dpdpboard.gov.in

Usage:
  finwipe dpd-board --request-id DPR-2026-000001
  finwipe dpd-board --nbfc-id bajaj-finserv --name "John" --email john@example.com`,
		RunE: runDPDBoard,
	}
	dpdRequestID   string
	dpdNBFCID     string
	dpdNBFCName   string
	dpdName       string
	dpdEmail      string
	dpdPhone      string
	dpdAddress    string
	dpdLetterOnly bool
)

func init() {
	rootCmd.AddCommand(dpdBoardCmd)
	dpdBoardCmd.Flags().StringVar(&dpdRequestID, "request-id", "",
		"FinWipe request ID (e.g., DPR-2026-000001)")
	dpdBoardCmd.Flags().StringVar(&dpdNBFCID, "nbfc-id", "",
		"NBFC entity ID from registry")
	dpdBoardCmd.Flags().StringVar(&dpdNBFCName, "nbfc-name", "",
		"NBFC name if not in registry")
	dpdBoardCmd.Flags().StringVar(&dpdName, "name", "",
		"Your full name")
	dpdBoardCmd.Flags().StringVar(&dpdEmail, "email", "",
		"Your email address")
	dpdBoardCmd.Flags().StringVar(&dpdPhone, "phone", "",
		"Your phone number")
	dpdBoardCmd.Flags().StringVar(&dpdAddress, "address", "",
		"Your address")
	dpdBoardCmd.Flags().BoolVar(&dpdLetterOnly, "letter-only", false,
		"Generate letter PDF only")
}

func runDPDBoard(cmd *cobra.Command, args []string) error {
	var entity *nbfc.Entity
	var req *history.Request

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}

	profileName := cfg.Profile.Name
	profileEmail := cfg.Profile.Email
	if dpdName != "" {
		profileName = dpdName
	}
	if dpdEmail != "" {
		profileEmail = dpdEmail
	}

	if profileName == "" || profileEmail == "" {
		return fmt.Errorf("name and email required. Use --name/--email or run finwipe init")
	}

	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	var timeline []string

	if dpdRequestID != "" {
		req, err = hist.GetByRequestID(dpdRequestID)
		if err != nil {
			return fmt.Errorf("request not found: %s", dpdRequestID)
		}
		dpdNBFCID = req.NBFCID
		timeline = buildTimeline(req)
	}

	for i := range entities {
		if entities[i].ID == dpdNBFCID {
			entity = &entities[i]
			break
		}
	}

	entityName := dpdNBFCName
	if entity != nil {
		entityName = entity.Name
	}

	if len(timeline) == 0 {
		timeline = []string{
			"Day 0: Data erasure request submitted to " + entityName,
			"Day 1-2: Statutory acknowledgment deadline (DPDP Rule 8)",
			"Day 30: Statutory completion deadline (Section 8(6), DPDP Act)",
			fmt.Sprintf("Day %d+: Filing this complaint (no compliance received)",
				int(time.Since(time.Now().AddDate(0, 0, -35)).Hours()/24)),
		}
	}

	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	os.MkdirAll(letterDir, 0700)

	letterPath := filepath.Join(letterDir,
		fmt.Sprintf("DPBB_complaint_%s_%s.pdf",
			sanitizeFilename(entityName),
			time.Now().Format("20060102")))

	gen := letter.New(letterDir)
	if err := gen.GenerateDPBB(entityName, entity, cfg.Profile,
		dpdRequestID, timeline, letterPath); err != nil {
		return fmt.Errorf("generate letter: %w", err)
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  DPBB Complaint Generated                               ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📄 PDF: %s\n\n", letterPath)
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📋 HOW TO FILE:")
	fmt.Println()
	fmt.Println("  OPTION 1 — Online:")
	fmt.Println("     1. https://dpdpboard.gov.in → File Complaint → Form III")
	fmt.Printf("     2. Attach: %s\n", filepath.Base(letterPath))
	fmt.Println()
	fmt.Println("  OPTION 2 — Email:")
	fmt.Println("     To: complaints@dpdpboard.gov.in")
	fmt.Println("     Subject: [COMPLAINT] Section 27(3) - Right to Erasure")
	fmt.Printf("     Attach: %s\n", filepath.Base(letterPath))
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  📜 Against: %s\n", entityName)
	fmt.Printf("  📧 You: %s <%s>\n", profileName, profileEmail)
	fmt.Println("  ⚖️  Law: Section 8(6), DPDP Act 2023")
	fmt.Println()

	if len(timeline) > 0 {
		fmt.Println("  📅 TIMELINE:")
		for _, t := range timeline {
			if len(t) > 0 {
				fmt.Printf("     • %s\n", t)
			}
		}
		fmt.Println()
	}

	fmt.Println("  ⚖️  DPBB POWERS:")
	fmt.Println("     • Order deletion: Section 27(3)(i)")
	fmt.Println("     • Penalty up to ₹250 crore: Section 33")
	fmt.Println("     • Investigate patterns: Section 27(4)")
	fmt.Println()

	if req != nil && !dpdLetterOnly {
		_ = hist.SetEscalationLevel(dpdRequestID, 1, 3, "user", "DPBB complaint filed")
	}

	return nil
}

func buildTimeline(req *history.Request) []string {
	var t []string
	t = append(t, fmt.Sprintf("Day 0: Deletion request sent (Ref: %s)", req.RequestID))
	t = append(t, fmt.Sprintf("Created: %s", req.CreatedAt.Format("02 Jan 2006")))

	if !req.DispatchedAt.IsZero() {
		t = append(t, fmt.Sprintf("Dispatched: %s", req.DispatchedAt.Format("02 Jan 2006")))
	}
	if !req.AckDeadlineAt.IsZero() {
		t = append(t, fmt.Sprintf("Ack Deadline: %s", req.AckDeadlineAt.Format("02 Jan 2006 15:04")))
	}
	if !req.AckReceivedAt.IsZero() {
		t = append(t, fmt.Sprintf("Acknowledgment: %s", req.AckReceivedAt.Format("02 Jan 2006")))
	} else {
		t = append(t, "Acknowledgment: NOT RECEIVED")
	}
	t = append(t, fmt.Sprintf("Current State: %s", req.LifecycleState))
	return t
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
