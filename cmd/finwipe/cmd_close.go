package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
)

var closeCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a deletion request with outcome",
	RunE:  runClose,
}

var (
	closeOutcome string
	closeNotes  string
)

func init() {
	closeCmd.Flags().StringVar(&closeRequestID, "request-id", "", "DPR-ID (required)")
	closeCmd.Flags().StringVar(&closeOutcome, "outcome", "",
		"Outcome: deleted | partial | exemption_claimed | no_response | rejected | withdrawn (required)")
	closeCmd.Flags().StringVar(&closeNotes, "notes", "", "Notes about the outcome")
	closeCmd.MarkFlagRequired("request-id")
	closeCmd.MarkFlagRequired("outcome")
}

func runClose(cmd *cobra.Command, args []string) error {
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	validOutcomes := map[string]bool{
		"deleted":            true,
		"partial":            true,
		"exemption_claimed":  true,
		"no_response":        true,
		"rejected":           true,
		"withdrawn":          true,
	}
	if !validOutcomes[closeOutcome] {
		return fmt.Errorf("invalid outcome: %s (use: deleted, partial, exemption_claimed, no_response, rejected, withdrawn)",
			closeOutcome)
	}

	req, err := hist.GetByRequestID(closeRequestID)
	if err != nil {
		return fmt.Errorf("request not found: %s", closeRequestID)
	}

	if req.LifecycleState == history.StateClosed {
		return fmt.Errorf("request %s is already CLOSED", closeRequestID)
	}

	fromState := req.LifecycleState
	err = hist.CloseRequest(closeRequestID, fromState, closeOutcome, closeNotes, "CLI_USER")
	if err != nil {
		return fmt.Errorf("close request: %w", err)
	}

	// Refresh
	req, _ = hist.GetByRequestID(closeRequestID)

	outcomeEmoji := map[string]string{
		"deleted":           "✅",
		"partial":           "⚠️",
		"exemption_claimed": "⚖️",
		"no_response":       "❌",
		"rejected":          "🚫",
		"withdrawn":         "➖",
	}[closeOutcome]

	fmt.Printf("\n")
	fmt.Printf("  %s Request closed\n", outcomeEmoji)
	fmt.Printf("  %s\n", req.RequestID)
	fmt.Printf("  NBFC:    %s\n", req.NBFCName)
	fmt.Printf("  State:   %s → CLOSED\n", fromState)
	fmt.Printf("  Outcome: %s\n", strings.ToUpper(closeOutcome))
	if closeNotes != "" {
		fmt.Printf("  Notes:   %s\n", closeNotes)
	}
	if !req.ClosedAt.IsZero() {
		fmt.Printf("  Closed:  %s\n", req.ClosedAt.Format("02 Jan 2006 15:04 MST"))
	}
	fmt.Println()

	return nil
}
