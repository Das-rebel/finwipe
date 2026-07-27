package main

import (
	"fmt"
	"time"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify if NBFC actually deleted your data",
	Long: `Verify that an entity has actually deleted your personal data.

Verification methods:
  1. TRY TO LOGIN — If account deleted, login should fail
  2. CHECK CONSENT — Try accessing the service
  3. SEND VERIFICATION EMAIL — "Confirm my data has been deleted"
  4. RECEIVE CERTIFICATE — Ask for deletion certificate

Why it matters:
  - Companies often say "data deleted" but don't actually delete
  - No way to know unless you verify
  - Proof of deletion = important for future disputes
  - Proof of non-deletion = evidence for DPBB complaint

Usage:
  finwipe verify --request-id DPR-2026-000001
  finwipe verify --request-id DPR-2026-000001 --method login
  finwipe verify --request-id DPR-2026-000001 --method email
  finwipe verify --request-id DPR-2026-000001 --certificate`,
	RunE:  runVerify,
}

var (
	verifyMethod     string
	verifyCertificate bool
)

func init() {
	verifyCmd.Flags().StringVar(&verifyMethod, "method", "email",
		"Verification method: email, login, consent, certificate")
	verifyCmd.Flags().BoolVar(&verifyCertificate, "certificate", false,
		"Request a deletion certificate")
}

func runVerify(cmd *cobra.Command, args []string) error {
	if dpdRequestID == "" {
		return fmt.Errorf("--request-id required")
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	req, err := hist.GetByRequestID(dpdRequestID)
	if err != nil {
		return fmt.Errorf("request not found: %s", dpdRequestID)
	}

	// Load entity
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	var entity *nbfc.Entity
	for i := range entities {
		if entities[i].ID == req.NBFCID {
			entity = &entities[i]
			break
		}
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  Data Deletion Verification                             ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📋 Request: %s\n", dpdRequestID)
	fmt.Printf("  🏢 Entity: %s\n", req.NBFCName)
	fmt.Printf("  📊 State: %s\n", req.LifecycleState)
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()

	// Check current state
	switch req.LifecycleState {
	case history.StateClosed:
		fmt.Println("  ✅ Status: CLOSED")
		fmt.Println("     Request was closed. Was data actually deleted?")
	case history.StateAckReceived:
		fmt.Println("  ⚠️  Status: Acknowledged but not closed")
		fmt.Println("     NBFC acknowledged but deletion may not be complete.")
	case history.StateDispatched:
		fmt.Println("  ⏳ Status: Dispatched, awaiting response")
		fmt.Println("     Too early to verify. Wait for acknowledgment.")
	default:
		fmt.Printf("  📊 Status: %s\n", req.LifecycleState)
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()

	// Generate verification request
	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	os.MkdirAll(letterDir, 0700)
	gen := letter.New(letterDir)

	method := strings.ToLower(verifyMethod)

	if verifyCertificate || method == "certificate" {
		// Generate deletion certificate request
		letterPath := filepath.Join(letterDir,
			fmt.Sprintf("Verification_%s_%s.pdf", req.NBFCID, time.Now().Format("20060102_150405")))

		if err := gen.GenerateVerification(req.RequestID, req.NBFCName, entity,
			cfg.Profile, "certificate", letterPath); err != nil {
			return fmt.Errorf("generate letter: %w", err)
		}

		fmt.Println("  📄 DELETION CERTIFICATE REQUEST")
		fmt.Printf("     Generated: %s\n", letterPath)
		fmt.Println()
		fmt.Println("  📋 WHAT TO DO:")
		fmt.Println("     1. Email the certificate request to the NBFC")
		fmt.Printf("        To: %s\n", func() string {
			if entity != nil {
				return entity.GrievanceEmail
			}
			return "their grievance email"
		}())
		fmt.Println("     2. Wait for their response (72 hours)")
		fmt.Println("     3. If they send a certificate → SAVE IT as proof")
		fmt.Println("     4. If they don't respond → evidence for DPBB complaint")
		fmt.Println()
	}

	if method == "email" || method == "" {
		letterPath := filepath.Join(letterDir,
			fmt.Sprintf("Verification_email_%s_%s.pdf", req.NBFCID, time.Now().Format("20060102_150405")))

		if err := gen.GenerateVerification(req.RequestID, req.NBFCName, entity,
			cfg.Profile, "email", letterPath); err != nil {
			return fmt.Errorf("generate letter: %w", err)
		}

		fmt.Println("  📧 VERIFICATION EMAIL")
		fmt.Printf("     Generated: %s\n", letterPath)
		fmt.Println()
		fmt.Println("  📋 WHAT TO DO:")
		fmt.Println("     1. Send this email asking to confirm deletion")
		fmt.Println("     2. Ask: 'Please confirm all my personal data has been deleted'")
		fmt.Println("     3. Keep any response as evidence")
		fmt.Println()
	}

	if method == "login" {
		fmt.Println("  🔐 LOGIN VERIFICATION")
		fmt.Println()
		fmt.Println("  📋 WHAT TO DO:")
		fmt.Println("     1. Try logging into the NBFC's app/website")
		fmt.Printf("        URL: %s\n", func() string {
			if entity != nil && entity.Website != "" {
				return entity.Website
			}
			return "(check NBFC website)"
		}())
		fmt.Println()
		fmt.Println("     2. If login fails with 'account deleted' → GOOD")
		fmt.Println("     3. If login succeeds → they still have your data")
		fmt.Println("     4. Screenshot any error/success messages")
		fmt.Println()
		fmt.Println("  ⚠️  NOTE: Some NBFCs keep your data even after 'deletion'")
		fmt.Println("     They may disable login but retain your data.")
		fmt.Println()
	}

	if method == "consent" {
		fmt.Println("  🤝 CONSENT VERIFICATION")
		fmt.Println()
		fmt.Println("  📋 WHAT TO DO:")
		fmt.Println("     1. Try to access the NBFC service (app/website)")
		fmt.Println("     2. If they ask for KYC/verification again → GOOD")
		fmt.Println("        (means old data was deleted)")
		fmt.Println("     3. If they know your previous details → BAD")
		fmt.Println("        (they still have your data)")
		fmt.Println()
	}

	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📁 EVIDENCE TO COLLECT:")
	fmt.Println("     • Screenshots of login attempts")
	fmt.Println("     • Email responses from NBFC")
	fmt.Println("     • Deletion certificate (if provided)")
	fmt.Println("     • Any acknowledgment of your request")
	fmt.Println()
	fmt.Println("  Attach evidence: finwipe evidence attach " + dpdRequestID)
	fmt.Println()

	return nil
}
