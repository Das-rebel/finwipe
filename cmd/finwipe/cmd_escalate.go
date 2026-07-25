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
	Short: "Escalate a request to a higher authority",
	Long: `Escalate a tracked deletion request through the regulatory hierarchy.

Escalation path (in order):
  1. dpo       — Escalate to NBFC's Data Protection Officer / CEO
  2. dpd_board — Data Protection Board of India (PRIMARY for DPDPA issues)
  3. rbi_ombudsman — RBI Integrated Ombudsman (for RBI-regulated entities)
  4. consumer_forum — District Consumer Forum (CPA 2019 — compensation)
  5. legal     — Civil litigation / High Court

The council recommends exhausting the NBFC's internal mechanism first (DPO),
then escalating to the DPDP Board as the primary regulator for data protection
violations under DPDPA 2023. The RBI Ombudsman and Consumer Forum are
parallel paths for financial/commercial remedies.`,
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
		"Channel: dpo | dpd_board | rbi_ombudsman | consumer_forum | legal (required)")
	escalateCmd.Flags().StringVar(&escRef, "ref", "", "Complaint reference number (if already filed)")
	escalateCmd.Flags().StringVar(&escNotes, "notes", "", "Summary of escalation reason")
	escalateCmd.Flags().BoolVar(&escGenOnly, "generate-only", false,
		"Only generate complaint letter — don't record in DB")
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

	// Channel map: CLI name → DB channel value
	channelMap := map[string]string{
		"dpo":             "DPO",
		"dpd_board":       "DPDP_BOARD",
		"rbi_ombudsman":   "RBI_OMBUDSMAN",
		"rbi_sachet":      "RBI_SACHET",
		"consumer_forum":  "CONSUMER_FORUM",
		"legal":           "LEGAL",
	}
	dbChannel, ok := channelMap[escChannel]
	if !ok {
		return fmt.Errorf("invalid channel: %s\nValid: dpo | dpd_board | rbi_ombudsman | consumer_forum | legal", escChannel)
	}

	req, err := hist.GetByRequestID(escRequestID)
	if err != nil {
		return fmt.Errorf("request not found: %s", escRequestID)
	}

	if req.LifecycleState == history.StateClosed {
		return fmt.Errorf("request %s is already CLOSED — cannot escalate", escRequestID)
	}

	// Level map: DB channel → escalation level
	escLevelMap := map[string]int{
		"DPO":            history.EscDPO,
		"DPDP_BOARD":     history.EscDPDPBoard,
		"RBI_OMBUDSMAN":  history.EscRBIOmbu,
		"RBI_SACHET":     history.EscRBIOmbu,
		"CONSUMER_FORUM": history.EscConsumer,
		"LEGAL":          history.EscLegal,
	}
	escLevel := escLevelMap[dbChannel]

	if req.EscalationLevel > escLevel {
		return fmt.Errorf("cannot downgrade escalation from %s to %s",
			history.EscalationLevelLabel(req.EscalationLevel),
			history.EscalationLevelLabel(escLevel))
	}

	// Load NBFC entity
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

	// Generate complaint letter for regulatory channels
	letterGen := letter.New(filepath.Join(os.TempDir(), "finwipe_letters"))
	ageDays := int(time.Since(req.CreatedAt).Hours() / 24)

	var pdfPath string
	if dbChannel == "RBI_OMBUDSMAN" {
		pdfPath, _, err = letterGen.GenerateRBIComplaint(
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
		fmt.Printf("  🔺 Escalation filed — %s\n", history.EscalationChannelLabel(dbChannel))
		fmt.Printf("  %s\n", req.RequestID)
		fmt.Printf("  Level:    %s\n", history.EscalationLevelLabel(escLevel))
		if escRef != "" {
			fmt.Printf("  Ref:      %s\n", escRef)
		}
		if escNotes != "" {
			fmt.Printf("  Notes:    %s\n", escNotes)
		}
		fmt.Println()
	}

	// Filing instructions per channel
	fmt.Printf("  📋 HOW TO FILE:\n\n")
	switch dbChannel {
	case "DPO":
		// Escalate to NBFC CEO/DPO after grievance officer non-response
		fmt.Printf("  This step escalates within the NBFC itself — to the CEO or DPO.\n")
		fmt.Printf("  1. Draft a formal escalation email to the NBFC CEO/DPO\n")
		fmt.Printf("  2. Reference your original DPR-ID: %s\n", req.RequestID)
		fmt.Printf("  3. Copy the audit trail from: finwipe track --request-id %s\n", req.RequestID)
		fmt.Printf("  4. Email the CEO directly (find on company website)\n")
		fmt.Println()
		fmt.Println("  ⚠️  If no response in 7 days, proceed to DPDP Board (step 2)")
		emailPath := filepath.Join(os.TempDir(), "finwipe_dpo_email.txt")
		body := fmt.Sprintf(`To: CEO/DPO, %s
Subject: URGENT — Data Deletion Request Unacknowledged — Escalation Notice (Ref: %s)

Dear Sir/Madam,

I am escalating my data deletion request (Ref: %s) which was submitted to %s on %s.

Despite multiple follow-ups, the Grievance Officer has failed to acknowledge or process my request under Section 8(6) of the DPDP Act 2023.

I am escalating this matter to your office as the Chief Executive Officer / Data Protection Officer of %s.

I request immediate action to:
1. Acknowledge my data deletion request
2. Process the deletion of my personal data as mandated by law
3. Provide written confirmation of deletion

If no response is received within 7 days, I will escalate to the Data Protection Board of India (dpb.gov.in), the primary regulatory authority for DPDPA enforcement.

Reference: %s
Original grievance email: %s

Regards,
%s
%s | %s`,
			entity.Name, req.RequestID, req.RequestID, entity.Name,
			req.CreatedAt.Format("02 January 2006"),
			entity.Name, req.RequestID, entity.GrievanceEmail,
			cfg.Profile.Name, cfg.Profile.Email, cfg.Profile.Phone)
		os.WriteFile(emailPath, []byte(body), 0600)
		fmt.Printf("  📧 Pre-filled escalation email: %s\n", emailPath)

	case "DPDP_BOARD":
		// Primary regulatory path for DPDPA violations
		fmt.Printf("  The Data Protection Board of India is the PRIMARY statutory\n")
		fmt.Printf("  authority for enforcing DPDPA 2023. File here for any data\n")
		fmt.Printf("  protection violation by a Data Fiduciary (including NBFCs).\n\n")
		fmt.Printf("  1. Go to: https://dpb.gov.in\n")
		fmt.Printf("  2. Click: File Complaint → DPDP Act 2023\n")
		fmt.Printf("  3. Select: Section 8(6) — Right to Erasure\n")
		fmt.Printf("  4. Attach evidence from: finwipe track --request-id %s\n", req.RequestID)
		fmt.Println()
		fmt.Println("  ⚠️  This is the PRIMARY path for DPDPA violations.")
		fmt.Println("     Do this BEFORE RBI Ombudsman for data protection issues.")
		if entity.Category == "bank" || entity.Category == "nbfc" {
			fmt.Println("     For financial issues, you may also file with RBI Ombudsman (step 3).")
		}
		fmt.Println()
		fmt.Println("  After filing, update with reference number:")
		fmt.Printf("  finwipe escalate --request-id %s --to dpd_board --ref <ref>\n", req.RequestID)

	case "RBI_OMBUDSMAN":
		fmt.Printf("  For RBI-regulated entities (banks, NBFCs), the RBI Ombudsman\n")
		fmt.Printf("  can address data-related grievances framed as deficiency in\n")
		fmt.Printf("  service or regulatory non-compliance.\n\n")
		fmt.Printf("  OPTION A — Online (fastest): https://rbis.rbi.org.in\n")
		fmt.Println("  1. Login → Submit Complaint → Select: NBFC/Bank")
		fmt.Printf("  2. Attach PDF: %s\n", pdfPath)
		fmt.Println("  3. Submit and save reference number")
		fmt.Println()
		fmt.Println("  OPTION B — Email (free):")
		fmt.Println("  1. Email to: crpc@rbi.org.in")
		fmt.Printf("  2. Subject: Complaint — %s — DPDPA Data Deletion (Ref: %s)\n", entity.Name, req.RequestID)
		fmt.Println("  3. Copy body from: /tmp/finwipe_rbi_email_body.txt")
		fmt.Println()
		fmt.Println("  OPTION C — Physical:")
		fmt.Println("  1. Print PDF → Send to: CRPC, RBI, 4th Floor, SEA Bldg, Mumbai 400 001")
		fmt.Printf("\n  📎 PDF: %s\n", pdfPath)
		fmt.Printf("  📧 Email: /tmp/finwipe_rbi_email_body.txt\n")

	case "CONSUMER_FORUM":
		fmt.Printf("  Consumer Forum is for compensation claims under CPA 2019.\n")
		fmt.Printf("  Use when NBFC caused material harm through data misuse.\n\n")
		fmt.Println("  1. Go to: https://consumercourt.in")
		fmt.Println("     OR visit your District Consumer Forum (local civil court)")
		fmt.Println("  2. File under: Consumer Protection Act 2019")
		fmt.Println("     — Deficiency in service (data breach, unauthorized sharing)")
		fmt.Println("     — Unfair trade practice (selling user data without consent)")
		fmt.Printf("  3. Use audit trail: finwipe track --request-id %s\n", req.RequestID)
		fmt.Println("  4. Claim compensation for data harm (up to ₹1 crore in NCDRC)")
		fmt.Println()
		fmt.Println("  💡 Tip: Consumer Forum + DPDP Board = parallel paths, not sequential.")
		fmt.Printf("  finwipe escalate --request-id %s --to consumer_forum --ref <case>\n", req.RequestID)

	case "LEGAL":
		fmt.Printf("  Civil litigation is the final recourse for data protection harm.\n\n")
		fmt.Println("  1. Consult a lawyer specializing in DPDPA/data protection")
		fmt.Println("  2. File writ petition in High Court under Article 226")
		fmt.Println("     (right to privacy, data protection)")
		fmt.Println("  3. Alternatively, file suit for damages under DPDPA Section 37")
		fmt.Printf("  4. Use audit trail: finwipe track --request-id %s\n", req.RequestID)
	}

	fmt.Println()
	return nil
}
