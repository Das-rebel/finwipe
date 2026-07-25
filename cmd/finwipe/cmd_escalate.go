package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
)

var escalateCmd = &cobra.Command{
	Use:   "escalate",
	Short: "Escalate a request to RBI Sachet, DPDP Board, or Consumer Forum",
	RunE:  runEscalate,
}

var (
	escChannel string
	escRef    string
	escNotes  string
)

func init() {
	escalateCmd.Flags().StringVar(&escRequestID, "request-id", "", "DPR-ID (required)")
	escalateCmd.Flags().StringVar(&escChannel, "to", "",
		"Escalation channel: rbi_sachet | dpd_board | consumer_forum | rbi_ombudsman (required)")
	escalateCmd.Flags().StringVar(&escRef, "ref", "", "Complaint reference number (if already filed)")
	escalateCmd.Flags().StringVar(&escNotes, "notes", "", "Summary of escalation reason")
	escalateCmd.MarkFlagRequired("request-id")
	escalateCmd.MarkFlagRequired("to")
}

func runEscalate(cmd *cobra.Command, args []string) error {
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	// Map friendly channel names to DB values
	channelMap := map[string]string{
		"rbi_sachet":       "RBI_SACHET",
		"dpd_board":        "DPDP_BOARD",
		"consumer_forum":   "CONSUMER_FORUM",
		"rbi_ombudsman":     "RBI_OMBUDSMAN",
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

	// Determine escalation level
	escLevel := map[string]int{
		"RBI_SACHET":       history.EscRBISachet,
		"DPDP_BOARD":        history.EscDPDPBoard,
		"CONSUMER_FORUM":   history.EscForum,
		"RBI_OMBUDSMAN":    history.EscForum,
	}[dbChannel]

	// Escalation level must not decrease
	if req.EscalationLevel > escLevel {
		return fmt.Errorf("cannot downgrade escalation from L%d to L%d",
			req.EscalationLevel, escLevel)
	}

	// Create escalation record
	summary := escNotes
	if summary == "" {
		summary = fmt.Sprintf("Escalated to %s", history.EscalationChannelLabel(dbChannel))
	}

	esc, err := hist.CreateEscalation(escRequestID, dbChannel, summary)
	if err != nil {
		return fmt.Errorf("create escalation: %w", err)
	}

	// Update complaint ref if provided
	if escRef != "" {
		hist.UpdateEscalation(esc.ID, "filed", escRef)
	}

	// Update escalation level if higher
	if escLevel > req.EscalationLevel {
		err = hist.SetEscalationLevel(escRequestID, req.EscalationLevel, escLevel,
			"CLI_USER", fmt.Sprintf("escalated to %s", history.EscalationChannelLabel(dbChannel)))
		if err != nil {
			return fmt.Errorf("set escalation level: %w", err)
		}
	}

	// Transition to ESCALATED if not already
	if req.LifecycleState != history.StateEscalated {
		err = hist.TransitionState(escRequestID, req.LifecycleState, history.StateEscalated,
			"CLI_USER", fmt.Sprintf("escalated to %s", history.EscalationChannelLabel(dbChannel)))
		if err != nil {
			// Not fatal — escalation was created
			fmt.Printf("⚠️  Escalation created but state transition failed: %v\n", err)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("  🔺 Escalation filed\n")
	fmt.Printf("  %s\n", req.RequestID)
	fmt.Printf("  Channel:  %s\n", history.EscalationChannelLabel(dbChannel))
	if escRef != "" {
		fmt.Printf("  Ref:      %s\n", escRef)
	}
	fmt.Printf("  Summary:  %s\n", summary)
	if escRef == "" {
		fmt.Println()
		fmt.Println("  ⚠️  IMPORTANT — File your complaint:")
		switch dbChannel {
		case "RBI_SACHET":
			fmt.Println("     1. Go to: https://sachet.rbi.org.in")
			fmt.Println("     2. Select: Non-Performing Assets → NBFC/Grievance against NBFC")
			fmt.Println("     3. File complaint and get reference number")
			fmt.Println("     4. Run: finwipe escalate --request-id " + escRequestID + " --to " + escChannel + " --ref <ref-number>")
		case "DPDP_BOARD":
			fmt.Println("     1. Go to: https://dpb.gov.in")
			fmt.Println("     2. File complaint under DPDP Act 2023")
			fmt.Println("     3. Run: finwipe escalate --request-id " + escRequestID + " --to " + escChannel + " --ref <ref-number>")
		case "CONSUMER_FORUM":
			fmt.Println("     1. File at: https://consumercourt.in or your local District Consumer Forum")
			fmt.Println("     2. Use the audit trail from: finwipe track --request-id " + escRequestID)
		}
	}
	fmt.Println()

	return nil
}
