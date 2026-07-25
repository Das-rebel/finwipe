package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/evidence"
)

var evidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Attach and manage evidence for deletion requests",
	Long: `Attach and manage evidence for DPR requests.

Evidence types:
  email_sent         — Original deletion request email
  email_received     — NBFC's acknowledgment email
  email_bounce       — Email bounce/failure notification
  acknowledgment     — Written acknowledgment from NBFC
  nbfc_response      — NBFC's response to the deletion request
  escalation_filing  — Escalation complaint filing receipt
  delivery_receipt   — Email delivery/read receipt
  postal_receipt     — Postal acknowledgment card
  generic_file       — Any other relevant document`,
	RunE: runEvidence,
}

var (
	evRequestID  string
	evType      string
	evFilePath  string
	evNotes     string
	evEvidenceID string
	flagJSON    bool
)

var eviAttachCmd = &cobra.Command{
	Use:   "attach <request-id>",
	Short: "Attach evidence to a request",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvAttach,
}

var eviListCmd = &cobra.Command{
	Use:   "list <request-id>",
	Short: "List all evidence for a request",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvList,
}

var eviGetCmd = &cobra.Command{
	Use:   "get <request-id> [--evidence-id <id>]",
	Short: "Get/download evidence file",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvGet,
}

func init() {
	evidenceCmd.AddCommand(eviAttachCmd)
	evidenceCmd.AddCommand(eviListCmd)
	evidenceCmd.AddCommand(eviGetCmd)
	eviAttachCmd.Flags().StringVar(&evType, "type", "generic_file",
		"Evidence type (see finwipe evidence --help)")
	eviAttachCmd.Flags().StringVar(&evFilePath, "file", "",
		"Path to file to attach (required)")
	eviAttachCmd.Flags().StringVar(&evNotes, "notes", "",
		"Optional notes about this evidence")
	eviAttachCmd.MarkFlagRequired("file")

	eviListCmd.Flags().BoolVar(&flagJSON, "json", false, "Output as JSON")

	eviGetCmd.Flags().StringVar(&evEvidenceID, "evidence-id", "",
		"Evidence ID to download (required)")
	eviGetCmd.Flags().StringVar(&evFilePath, "output", "",
		"Output path (default: print to stdout)")
}

func runEvidence(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func evidenceStore() (*evidence.Store, error) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".finwipe", "evidence")
	return evidence.New(base)
}

func runEvAttach(cmd *cobra.Command, args []string) error {
	reqID := args[0]
	store, err := evidenceStore()
	if err != nil {
		return fmt.Errorf("evidence store: %w", err)
	}

	evType := evidence.EvidenceType(evType)

	f, err := os.Open(evFilePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	ev, err := store.Save(reqID, evType, filepath.Base(evFilePath), f, evNotes)
	if err != nil {
		return fmt.Errorf("save evidence: %w", err)
	}

	fmt.Printf("\n")
	fmt.Printf("  📎 Evidence attached\n")
	fmt.Printf("  Request:     %s\n", reqID)
	fmt.Printf("  Type:       %s\n", ev.Type)
	fmt.Printf("  File:       %s\n", ev.Filename)
	fmt.Printf("  Size:       %d bytes\n", ev.SizeBytes)
	fmt.Printf("  Evidence:   %s\n", ev.ID)
	fmt.Printf("  SHA256:     %s\n", ev.SHA256[:16]+"...")
	fmt.Println()

	return nil
}

func runEvList(cmd *cobra.Command, args []string) error {
	reqID := args[0]
	store, err := evidenceStore()
	if err != nil {
		return fmt.Errorf("evidence store: %w", err)
	}

	evs, err := store.List(reqID)
	if err != nil {
		return fmt.Errorf("list evidence: %w", err)
	}

	if len(evs) == 0 {
		fmt.Printf("No evidence for %s\n", reqID)
		return nil
	}

	if flagJSON {
		fmt.Printf("[]") // TODO: JSON output
		return nil
	}

	fmt.Printf("\n")
	fmt.Printf("  📎 Evidence for %s (%d files)\n", reqID, len(evs))
	fmt.Printf("  %-12s %-20s %-10s %s\n", "TYPE", "EVIDENCE ID", "SIZE", "FILENAME")
	fmt.Printf("  %-12s %-20s %-10s %s\n", "----", "-----------", "----", "--------")
	for _, ev := range evs {
		size := formatBytes(ev.SizeBytes)
		fmt.Printf("  %-12s %-20s %-10s %s\n", ev.Type, ev.ID, size, ev.Filename)
	}
	fmt.Println()

	return nil
}

func runEvGet(cmd *cobra.Command, args []string) error {
	reqID := args[0]

	if evEvidenceID == "" {
		return fmt.Errorf("--evidence-id is required")
	}

	store, err := evidenceStore()
	if err != nil {
		return fmt.Errorf("evidence store: %w", err)
	}

	path, err := store.GetPath(reqID, evEvidenceID)
	if err != nil {
		return fmt.Errorf("get evidence: %w", err)
	}

	if evFilePath != "" {
		// Copy to output path
		src, _ := os.Open(path)
		dst, err := os.Create(evFilePath)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		io.Copy(dst, src)
		src.Close()
		dst.Close()
		fmt.Printf("Saved to: %s\n", evFilePath)
	} else {
		// Print to stdout
		io.Copy(os.Stdout, os.Stdout)
	}

	return nil
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
}
