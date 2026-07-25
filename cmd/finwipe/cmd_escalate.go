package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var escalateCmd = &cobra.Command{
	Use:   "escalate",
	Short: "Escalate a request to RBI Sachet, DPDP Board, or Consumer Forum",
	Long: `Escalate a tracked deletion request to a higher authority.

Channels:
  rbi_sachet      RBI Sachet Portal (sachet.rbi.org.in) — for NBFC non-response
  rbi_ombudsman   RBI Integrated Ombudsman (Form-IV) — for unresolved complaints
  dpd_board       Data Protection Board of India (dpb.gov.in) — DPDPA enforcement
  consumer_forum  District Consumer Forum — civil remedy for data harm

With --generate-only flag, generates the pre-filled complaint PDF + email body
without recording the escalation in the DB (useful for review before committing).`,
	RunE: runEscalate,
}

var (
	escChannel string
	escRef     string
	escNotes   string
	escGenOnly bool
)

func init() {
	escalateCmd.Flags().StringVar(&escRequestID, "request-id", "", "DPR-ID (required)")
	escalateCmd.Flags().StringVar(&escChannel, "to", "",
		"Escalation channel: rbi_sachet | dpd_board | consumer_forum | rbi_ombudsman (required)")
	escalateCmd.Flags().StringVar(&escRef, "ref", "", "Complaint reference number (if already filed)")
	escalateCmd.Flags().StringVar(&escNotes, "notes", "", "Summary of escalation reason")
	escalateCmd.Flags().BoolVar(&escGenOnly, "generate-only", false,
		"Only generate complaint letter/email — don't record in DB")
	escalateCmd.MarkFlagRequired("request-id")
	escalateCmd.MarkFlagRequired("to")
}

func runEscalate(cmd *cobra.Command, args []string) error {
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	channelMap := map[string]string{
		"rbi_sachet":     "RBI_SACHET",
		"dpd_board":      "DPDP_BOARD",
		"consumer_forum":  "CONSUMER_FORUM",
		"rbi_ombudsman":  "RBI_OMBUDSMAN",
	}
	dbChannel, ok := channelMap[escChannel]
	if !ok {
		return fmt.Errorf("invalid channel: %s (use: rbi_sachet, dpd_board, consumer_forum, rbi_ombudsman)", escChannel)
	}

	req, err := hist.GetByRequestID(escRequestID)
	if err != nil {
		return fmt.Errorf("request not found: %s", escRequestID)
	}

	if req.LifecycleState == history.StateClosed {
		return fmt.Errorf("request %s is already CLOSED — cannot escalate", escRequestID)
	}

	escLevelMap := map[string]int{
		"RBI_SACHET":     history.EscRBISachet,
		"DPDP_BOARD":      history.EscDPDPBoard,
		"CONSUMER_FORUM":  history.EscForum,
		"RBI_OMBUDSMAN":  history.EscForum,
	}
	escLevel := escLevelMap[dbChannel]

	if req.EscalationLevel > escLevel {
		return fmt.Errorf("cannot downgrade escalation from L%d to L%d",
			req.EscalationLevel, escLevel)
	}

	// Load NBFC registry
	nbfcPath := filepath.Join(dataDir(), "nbfcs.yaml")
	entities, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	var entity nbfc.Entity
	for _, e := range entities {
		if history.SanitizeNBFCID(e.ID) == history.SanitizeNBFCID(req.NBFCID) {
			entity = e
			break
		}
	}

	// Generate complaint letter (RBI Ombudsman generates PDF + email)
	letterGen := letter.New(filepath.Join(os.TempDir(), "finwipe_letters"))
	ageDays := int(time.Since(req.CreatedAt).Hours() / 24)

	var pdfPath, emailBody string
	if dbChannel == "RBI_OMBUDSMAN" {
		pdfPath, emailBody, err = letterGen.GenerateRBIComplaint(
			req.RequestID, entity, cfg.Profile, ageDays)
		if err != nil {
			return fmt.Errorf("generate RBI complaint: %w", err)
		}
	}

	// DB update (skip if --generate-only)
	if !escGenOnly {
		summary := escNotes
		if summary == "" {
			summary = fmt.Sprintf("Escalated to %s", history.EscalationChannelLabel(dbChannel))
		}

		esc, err := hist.CreateEscalation(req.RequestID, dbChannel, summary)
		if err != nil {
			return fmt.Errorf("create escalation: %w", err)
		}

		if escRef != "" {
			hist.UpdateEscalation(esc.ID, "filed", escRef)
		}

		if escLevel > req.EscalationLevel {
			err = hist.SetEscalationLevel(req.RequestID, req.EscalationLevel, escLevel,
				"CLI_USER", fmt.Sprintf("escalated to %s", history.EscalationChannelLabel(dbChannel)))
			if err != nil {
				fmt.Printf("⚠️  Escalation created but level update failed: %v\n", err)
			}
		}

		if req.LifecycleState != history.StateEscalated {
			err = hist.TransitionState(req.RequestID, req.LifecycleState, history.StateEscalated,
				"CLI_USER", fmt.Sprintf("escalated to %s", history.EscalationChannelLabel(dbChannel)))
			if err != nil {
				fmt.Printf("⚠️  Escalation created but state transition failed: %v\n", err)
			}
		}
	}

	// Display results
	fmt.Printf("\n")
	if !escGenOnly {
		fmt.Printf("  🔺 Escalation filed\n")
		fmt.Printf("  %s\n", req.RequestID)
		fmt.Printf("  Channel:  %s\n", history.EscalationChannelLabel(dbChannel))
		if escRef != "" {
			fmt.Printf("  Ref:      %s\n", escRef)
		}
		if escNotes != "" {
			fmt.Printf("  Summary:  %s\n", escNotes)
		}
		fmt.Println()
	}

	// File complaint instructions
	fmt.Println("  📋 HOW TO FILE YOUR COMPLAINT:")
	fmt.Println()

	switch dbChannel {
	case "RBI_SACHET":
		fmt.Println("  1. Go to: https://sachet.rbi.org.in")
		fmt.Println("  2. Select category: Non-Banking Finance Companies (NBFCs)")
		fmt.Println("  3. Select sub-category: Grievance Against NBFC")
		fmt.Println("  4. Enter NBFC details and attach evidence")
		fmt.Println("  5. Submit and save reference number")
		fmt.Println("  6. Then update: finwipe escalate --request-id " + req.RequestID + " --to rbi_sachet --ref <ref>")

	case "RBI_OMBUDSMAN":
		fmt.Println("  OPTION A — Online (fastest):")
		fmt.Println("  1. Go to: https://rbis.rbi.org.in")
		fmt.Println("  2. Login → Submit Complaint → Select: NBFC")
		fmt.Println("  3. Attach PDF from below and submit")
		fmt.Println()
		fmt.Println("  OPTION B — Email (free):")
		fmt.Println("  1. Email to: crpc@rbi.org.in")
		fmt.Println("  2. Subject: Complaint — " + entity.Name + " — DPDPA Data Deletion")
		fmt.Println("  3. Copy email body from file below")
		fmt.Println()
		fmt.Println("  OPTION C — Physical:")
		fmt.Println("  1. Print the PDF below")
		fmt.Println("  2. Send to: The Centralised Receipt and Processing Centre,")
		fmt.Println("     RBI Ombudsman, 4th Floor, SEA Building, Mahatma Gandhi Road,")
		fmt.Println("     Fort, Mumbai — 400 001")
		fmt.Println()
		fmt.Println("  📎 Pre-filled complaint PDF:")
		fmt.Printf("     %s\n", pdfPath)
		fmt.Println()
		fmt.Println("  📧 Pre-filled email body:")
		emailPath := filepath.Join(os.TempDir(), "finwipe_rbi_email_body.txt")
		os.WriteFile(emailPath, []byte(emailBody), 0600)
		fmt.Printf("     %s\n", emailPath)

	case "DPDP_BOARD":
		fmt.Println("  1. Go to: https://dpb.gov.in")
		fmt.Println("  2. Click: File Complaint → DPDP Act 2023")
		fmt.Println("  3. Attach evidence of deletion request and NBFC non-response")
		fmt.Println("     (audit trail from: finwipe track --request-id " + req.RequestID + ")")
		fmt.Println("  4. Submit and save reference number")
		fmt.Println("  5. Then update: finwipe escalate --request-id " + req.RequestID + " --to dpd_board --ref <ref>")

	case "CONSUMER_FORUM":
		fmt.Println("  1. Go to: https://consumercourt.in")
		fmt.Println("     OR visit your District Consumer Forum (local civil court)")
		fmt.Println("  2. File under: Consumer Protection Act 2019 — Deficiency in Service")
		fmt.Println("     (data deletion is a service deficiency under CPA)")
		fmt.Println("  3. Use audit trail: finwipe track --request-id " + req.RequestID)
		fmt.Println("  4. Then update: finwipe escalate --request-id " + req.RequestID + " --to consumer_forum --ref <case-number>")
	}

	fmt.Println()
	return nil
}
