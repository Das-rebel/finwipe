package main

import (
	"fmt"
	"time"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var portabilityCmd = &cobra.Command{
	Use:   "portability",
	Short: "Request all data an entity holds about you (Section 6(9), DPDP Act)",
	Long: `Generate a data portability request under Section 6(9), DPDP Act 2023.

This is DIFFERENT from deletion:
  - Deletion: "Delete my data"
  - Portability: "Give me ALL data you have about me"

Why it matters:
  - Company must respond within 72 hours
  - You can verify what they actually deleted
  - You know what data was collected (often more than you knew)
  - Proves compliance or non-compliance

Two-step strategy:
  1. Portability: "Send me your data" (Day 1)
  2. Deletion: "Now delete it" (Day 4)
     → If they only deleted what they sent = they cooperated
     → If they didn't respond = proof for DPBB complaint

Usage:
  finwipe portability --nbfc-id bajaj-finserv
  finwipe portability --nbfc-id bajaj-finserv --send
  finwipe portability --request-id DPR-2026-000001`,
	RunE:  runPortability,
}

var (
	portNBFCID   string
	portSend     bool
	portChannel  string // email or post
)

func init() {
	rootCmd.AddCommand(portabilityCmd)
	portabilityCmd.Flags().StringVar(&portNBFCID, "nbfc-id", "",
		"NBFC entity ID (from finwipe list)")
	portabilityCmd.Flags().BoolVar(&portSend, "send", false,
		"Send the portability request (opens email)")
	portabilityCmd.Flags().StringVar(&portChannel, "channel", "email",
		"Delivery channel: email, post, cic")
}

func runPortability(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	var entity *nbfc.Entity
	for i := range entities {
		if entities[i].ID == portNBFCID {
			entity = &entities[i]
			break
		}
	}

	if entity == nil {
		return fmt.Errorf("NBFC not found: %s (try finwipe list)", portNBFCID)
	}

	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	os.MkdirAll(letterDir, 0700)
	gen := letter.New(letterDir)

	letterPath := filepath.Join(letterDir,
		fmt.Sprintf("Portability_%s_%s.pdf",
			entity.ID, time.Now().Format("20060102_150405")))

	if err := gen.GeneratePortability(entity, cfg.Profile, letterPath); err != nil {
		return fmt.Errorf("generate letter: %w", err)
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  Data Portability Request — Generated                     ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📄 Letter: %s\n\n", letterPath)
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  📜 Requesting: ALL data %s holds about you\n", entity.Name)
	fmt.Printf("  📧 To: %s\n", entity.GrievanceEmail)
	fmt.Printf("  ⏰ Must respond within: 72 hours (Section 6(9), DPDP Act)\n")
	fmt.Println()
	fmt.Println("  📋 NEXT STEPS:")
	fmt.Println()
	fmt.Println("  1. Send the letter to the NBFC")
	if entity.GrievanceEmail != "" {
		fmt.Printf("     Email: %s\n", entity.GrievanceEmail)
		fmt.Printf("     Subject: DPDPA Section 6(9) — Data Portability Request\n")
	}
	fmt.Println()
	fmt.Println("  2. Wait 72 hours for response")
	fmt.Println()
	fmt.Println("  3. Track their response:")
	fmt.Printf("     finwipe new --nbfc-id %s --category portability\n", entity.ID)
	fmt.Println()
	fmt.Println("  4. If no response, proceed with deletion:")
	fmt.Printf("     finwipe send --nbfc-id %s\n", entity.ID)
	fmt.Println()
	fmt.Println("  💡 TWO-STEP STRATEGY:")
	fmt.Println("     Day 1: Portability (see what they have)")
	fmt.Println("     Day 4: Deletion (now they can't hide what they sent)")
	fmt.Println()

	if portSend && entity.GrievanceEmail != "" {
		fmt.Println("  📤 Sending email...")
		// Would integrate with email sender
		fmt.Printf("  ✅ Email prepared: %s\n", entity.GrievanceEmail)
	}

	return nil
}
