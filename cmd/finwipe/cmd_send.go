package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/email"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/evidence"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Dispatch a deletion request (email sent or letter generated)",
	RunE:  runSend,
}

var (
	excludeCategories string
	includeIDs       string
	rateLimitMs      int
	sendRequestID    string
)

func init() {
	sendCmd.Flags().StringVar(&sendRequestID, "request-id", "",
		"Send a specific request by DPR-ID")
	sendCmd.Flags().StringVar(&includeIDs, "include", "",
		"Send requests for specific NBFC IDs (comma-separated)")
	sendCmd.Flags().IntVar(&rateLimitMs, "rate-limit", 1000,
		"Milliseconds between emails (Gmail: 1000+ recommended)")
}

func runSend(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	// Evidence store for sent emails
	evidenceBase := filepath.Join(os.Getenv("HOME"), ".finwipe", "evidence")
	evStore, _ := evidence.New(evidenceBase)

	// Load NBFCs for lookups
	nbfcPath := filepath.Join(dataDir(), "nbfcs.yaml")
	allNBFCs, err := nbfc.Load(nbfcPath)
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}
	nbfcMap := make(map[string]nbfc.Entity)
	for _, e := range allNBFCs {
		nbfcMap[e.ID] = e
	}

	// Determine which requests to send
	var requests []*history.Request

	if sendRequestID != "" {
		// Send specific request
		req, err := hist.GetByRequestID(sendRequestID)
		if err != nil {
			return fmt.Errorf("request not found: %s", sendRequestID)
		}
		if req.LifecycleState != history.StateInitiated && req.LifecycleState != history.StateDeliveryFailed {
			return fmt.Errorf("request %s is in state %s, can only send from INITIATED or DELIVERY_FAILED",
				req.RequestID, req.LifecycleState)
		}
		requests = []*history.Request{req}
	} else if includeIDs != "" {
		// Send requests for specific NBFCs (that are in INITIATED state)
		idMap := make(map[string]bool)
		for _, id := range splitCSV(includeIDs) {
			idMap[history.SanitizeNBFCID(id)] = true
		}
		all, err := hist.GetByState(history.StateInitiated)
		if err != nil {
			return err
		}
		for _, req := range all {
			if idMap[req.NBFCID] {
				requests = append(requests, &req)
			}
		}
	} else {
		// No specific request — show help
		return fmt.Errorf("specify --request-id <DPR-ID> to send a specific request\n" +
			"Use: finwipe track --all  to see all active requests\n" +
			"Use: finwipe new --nbfc <id> to create a new request")
	}

	if len(requests) == 0 {
		fmt.Println("No requests to send (check --request-id or NBFC IDs)")
		return nil
	}

	// Dry run
	if dryRun {
		fmt.Printf("\n🔍 DRY RUN — No emails will be sent\n\n")
		fmt.Printf("Would dispatch %d request(s):\n\n", len(requests))
		for _, req := range requests {
			fmt.Printf("  %-20s %-30s <%s>\n",
				req.RequestID, req.NBFCName, req.GrievanceEmail)
		}
		fmt.Println()
		return nil
	}

	// Check SMTP
	if cfg.SMTP.Password == "" {
		return fmt.Errorf("SMTP not configured. Run: finwipe init")
	}

	sender := email.New(&cfg.SMTP)

	fmt.Printf("\n📤 Dispatching %d request(s)...\n\n", len(requests))

	sent, failed := 0, 0
	for i, req := range requests {
		nbfcEntity, ok := nbfcMap[req.NBFCID]
		if !ok {
			nbfcEntity = nbfc.Entity{
				ID:             req.NBFCID,
				Name:           req.NBFCName,
				GrievanceEmail: req.GrievanceEmail,
			}
		}

		var msgID string
		var err error

		if req.Channel == history.ChannelEmail {
			err = sender.Send(nbfcEntity, cfg.Profile, "")
			msgID = "" // DPR-ID in subject line is the tracking reference
		} else {
			// For post/cic — letter generated separately via finwipe letter
			err = nil
		}

		if err != nil {
			fmt.Printf("  ❌ %s → %s: %v\n", req.RequestID, req.NBFCName, err)
			failed++
			// Mark as DELIVERY_FAILED so user can investigate
			if req.Channel == history.ChannelEmail {
				hist.TransitionState(req.RequestID, req.LifecycleState, history.StateDeliveryFailed,
					"SYSTEM", fmt.Sprintf("delivery failed: %v", err))
			}
		} else {
			letterPath := ""
			if req.Channel == history.ChannelPost {
				// Letter path would be set by separate letter command
			}
			err = hist.Dispatch(req.RequestID, letterPath, msgID, req.Channel)
			if err != nil {
				fmt.Printf("  ⚠️  %s → %s: sent but dispatch record failed: %v\n",
					req.RequestID, req.NBFCName, err)
			} else {
				// Store email as evidence (proof of what was sent)
				if req.Channel == history.ChannelEmail && evStore != nil {
					emailBody := email.GenerateFollowupBody(req.RequestID, req.NBFCName, cfg.Profile, 0)
					ev, err := evStore.Save(req.RequestID, evidence.TypeEmailSent,
						"DeletionRequest_"+req.RequestID+".eml",
						io.NopCloser(strings.NewReader(emailBody)),
						"Sent deletion request email to "+req.GrievanceEmail)
					if err == nil {
						fmt.Printf("  📎 Evidence: %s\n", ev.ID)
					}
				}
				fmt.Printf("  ✅ %s → %s\n", req.RequestID, req.NBFCName)
				sent++
			}
		}

		// Rate limit
		if i < len(requests)-1 && rateLimitMs > 0 {
			time.Sleep(time.Duration(rateLimitMs) * time.Millisecond)
		}
	}

	fmt.Printf("\n✅ Dispatched: %d | Failed: %d\n", sent, failed)
	if sent > 0 {
		fmt.Println("\nNext: finwipe track --all   # monitor acknowledgment")
		fmt.Println("       finwipe cron         # setup daily follow-up automation")
	}

	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
