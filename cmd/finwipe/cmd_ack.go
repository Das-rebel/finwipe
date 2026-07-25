package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
)

var ackCmd = &cobra.Command{
	Use:   "ack",
	Short: "Record NBFC acknowledgment of a deletion request",
	RunE:  runAck,
}

var (
	ackRef   string
	ackDate  string
	ackNotes string
)

func init() {
	ackCmd.Flags().StringVar(&ackRequestID, "request-id", "", "DPR-ID (required)")
	ackCmd.Flags().StringVar(&ackRef, "ref", "", "NBFC's ticket / acknowledgment reference number")
	ackCmd.Flags().StringVar(&ackDate, "date", "", "Date acknowledgment received (YYYY-MM-DD), default: now")
	ackCmd.Flags().StringVar(&ackNotes, "notes", "", "Additional notes")
	ackCmd.MarkFlagRequired("request-id")
}

func runAck(cmd *cobra.Command, args []string) error {
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	// Parse date
	var ackTime time.Time
	if ackDate != "" {
		ackTime, err = time.Parse("2006-01-02", ackDate)
		if err != nil {
			return fmt.Errorf("invalid date format: %s (use: YYYY-MM-DD)", ackDate)
		}
	} else {
		ackTime = time.Now()
	}

	// Idempotent: check if already acknowledged
	req, err := hist.GetByRequestID(ackRequestID)
	if err != nil {
		return fmt.Errorf("request not found: %s", ackRequestID)
	}

	if req.LifecycleState == history.StateAckReceived {
		if ackRef != "" && req.ExternalRef == ackRef {
			fmt.Printf("ℹ️  Already acknowledged with ref %s — idempotent skip\n", ackRef)
			return nil
		}
		if ackRef == "" {
			fmt.Printf("ℹ️  Already in ACK_RECEIVED state — idempotent skip\n")
			return nil
		}
		// Different ref — update
	}

	err = hist.RecordAck(ackRequestID, ackRef, "CLI_USER", ackTime)
	if err != nil {
		return fmt.Errorf("record ack: %w", err)
	}

	// Refresh to show updated state
	req, _ = hist.GetByRequestID(ackRequestID)

	fmt.Printf("\n")
	fmt.Printf("  ✅ Acknowledgment recorded\n")
	fmt.Printf("  %s\n", req.RequestID)
	fmt.Printf("  NBFC:      %s\n", req.NBFCName)
	fmt.Printf("  Ref:       %s\n", req.ExternalRef)
	fmt.Printf("  Acked at:  %s\n", req.AckReceivedAt.Format("02 Jan 2006 15:04 MST"))
	fmt.Printf("  New deadline: %s (30 days from ack)\n", req.ResponseDeadlineAt.Format("02 Jan 2006"))
	fmt.Println()

	return nil
}
