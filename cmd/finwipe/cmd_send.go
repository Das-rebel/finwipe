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
	"github.com/das-rebel/finwipe/internal/evidence"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Dispatch deletion requests via email or registered post",
	Long: `Send deletion requests to NBFCs, banks, and other financial entities.

A valid FinWipe request (created via finwipe new) must exist first.
SMTP must be configured (finwipe init) for email dispatch.

Examples:
  finwipe send                                    # Send all INITIATED requests
  finwipe send --request-id DPR-2026-000001      # Send specific request
  finwipe send --include bajaj-finserv,hdfc-bank  # By NBFC IDs
  finwipe send --category fintech                 # By category
  finwipe send --dry-run                         # Preview only`,
	RunE: runSend,
}

var (
	excludeCategories string
	includeIDs       string
	rateLimitMs      int
	sendRequestID    string
	sendChannel      string // email, post, cic
	sendLegalBasis   string // dpdp, rbi, both
)

func init() {
	sendCmd.Flags().StringVar(&excludeCategories, "exclude-category", "",
		"Exclude NBFCs by category (e.g., bank,hfc)")
	sendCmd.Flags().StringVar(&includeIDs, "include", "",
		"Send requests for specific NBFC IDs (comma-separated)")
	sendCmd.Flags().IntVar(&rateLimitMs, "rate-limit", 1000,
		"Milliseconds to wait between requests (default 1000)")
	sendCmd.Flags().StringVar(&sendRequestID, "request-id", "",
		"Send a specific request by DPR-ID")
	sendCmd.Flags().StringVar(&sendChannel, "channel", "email",
		"Dispatch channel: email, post, cic (default: email)")
	sendCmd.Flags().StringVar(&sendLegalBasis, "legal-basis", "withdrawal",
		"Legal basis: withdrawal (Sec 8(7)), erasure (Sec 8(6)), rbi, both, access (Sec 6)")
}

func runSend(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}
	nbfcMap := make(map[string]nbfc.Entity)
	for _, e := range entities {
		nbfcMap[e.ID] = e
	}

	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	evidenceBase := filepath.Join(os.Getenv("HOME"), ".finwipe", "evidence")
	evStore, _ := evidence.New(evidenceBase)

	var requests []*history.Request

	if sendRequestID != "" {
		req, err := hist.GetByRequestID(sendRequestID)
		if err != nil {
			return fmt.Errorf("request not found: %s", sendRequestID)
		}
		if req.LifecycleState != history.StateInitiated &&
			req.LifecycleState != history.StateDeliveryFailed {
			return fmt.Errorf("request %s is not in INITIATED or DELIVERY_FAILED state (current: %s)",
				req.RequestID, req.LifecycleState)
		}
		requests = append(requests, req)
	} else {
		all, err := hist.GetByState(history.StateInitiated)
		if err != nil {
			return fmt.Errorf("get initiated: %w", err)
		}
		failed, _ := hist.GetByState(history.StateDeliveryFailed)
		all = append(all, failed...)

		if includeIDs != "" {
			includeMap := make(map[string]bool)
			for _, id := range strings.Split(includeIDs, ",") {
				includeMap[strings.TrimSpace(id)] = true
			}
			for i := range all {
				if includeMap[all[i].NBFCID] {
					requests = append(requests, &all[i])
				}
			}
		} else {
			for i := range all {
				requests = append(requests, &all[i])
			}
		}
	}

	if len(requests) == 0 {
		fmt.Println("No requests to send.")
		return nil
	}

	if sendChannel == "email" && cfg.SMTP.Password == "" && !dryRun {
		return fmt.Errorf("SMTP not configured. Run: finwipe init\nOr use --channel post for registered post")
	}

	profile := cfg.Profile
	if profile.Name == "" {
		return fmt.Errorf("profile not configured. Run: finwipe init")
	}

	sender := email.New(&cfg.SMTP)
	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	letterGen := letter.New(letterDir)

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

		var categories []letter.DeletionCategory
		switch nbfcEntity.Category {
		case nbfc.CatNBFC, nbfc.CatFINTECH, nbfc.CatBANK, nbfc.CatHFC:
			categories = letter.DefaultDeletionCategories
		default:
			categories = letter.DefaultDeletionCategories
		}

		// Parse legal basis
		var legalBasis letter.LegalBasis
		switch sendLegalBasis {
		case "dpdp":
			legalBasis = letter.LegalBasisWithdrawal
		case "rbi":
			legalBasis = letter.LegalBasisRBI
		default:
			legalBasis = letter.LegalBasisBoth
		}

		var err error

		if sendChannel == "email" || req.Channel == history.ChannelEmail {
			emailBody := letter.GenerateEmailBody(req.RequestID, nbfcEntity.Name, profile, categories, legalBasis)

			// Retry with exponential backoff: 0s, 5s, 15s
			var sendErr error
			for attempt := 0; attempt < 3; attempt++ {
				if attempt > 0 {
					delay := []time.Duration{5, 15}[attempt-1] * time.Second
					fmt.Printf("  🔄 %s → retry %d/%d in %v...\n", req.RequestID, attempt+1, 3, delay)
					time.Sleep(delay)
				}
				sendErr = sender.Send(nbfcEntity, profile, emailBody)
				if sendErr == nil {
					break
				}
				fmt.Printf("  ⚠️  %s → attempt %d failed: %v\n", req.RequestID, attempt+1, sendErr)
			}

			if sendErr != nil {
				hist.TransitionState(req.RequestID, req.LifecycleState,
					history.StateDeliveryFailed, "SYSTEM",
					fmt.Sprintf("email send failed after 3 attempts: %v", sendErr))
				fmt.Printf("  ❌ %s → %s: %v\n", req.RequestID, req.NBFCName, sendErr)
				failed++
			} else {
				if evStore != nil {
					evStore.Save(req.RequestID, evidence.TypeEmailSent,
						"DeletionRequest_"+req.RequestID+".eml",
						io.NopCloser(strings.NewReader(emailBody)),
						"Sent: "+nbfcEntity.GrievanceEmail)
				}
				err = hist.Dispatch(req.RequestID, "", req.RequestID, history.ChannelEmail)
				if err != nil {
					fmt.Printf("  ⚠️  %s → %s: sent but record failed: %v\n",
						req.RequestID, req.NBFCName, err)
				} else {
					fmt.Printf("  ✅ %s → %s [%s]\n",
						req.RequestID, req.NBFCName, nbfcEntity.GrievanceEmail)
					sent++
				}
			}
		} else {
			letterPath, err := letterGen.Generate(req.RequestID, nbfcEntity.Name,
				nbfcEntity.GrievanceEmail, profile, categories, legalBasis)
			if err != nil {
				fmt.Printf("  ❌ %s → %s: letter generation failed: %v\n",
					req.RequestID, req.NBFCName, err)
				failed++
				continue
			}
			err = hist.Dispatch(req.RequestID, letterPath, "", history.ChannelPost)
			if err != nil {
				fmt.Printf("  ⚠️  %s → %s: letter generated but dispatch record failed: %v\n",
					req.RequestID, req.NBFCName, err)
			} else {
				fmt.Printf("  ✅ %s → %s [📄 %s]\n",
					req.RequestID, req.NBFCName, filepath.Base(letterPath))
				sent++
			}
		}

		if i < len(requests)-1 && rateLimitMs > 0 && !dryRun {
			time.Sleep(time.Duration(rateLimitMs) * time.Millisecond)
		}
	}

	fmt.Printf("\n")
	if dryRun {
		fmt.Printf("🔍 DRY RUN — No emails/letters actually sent\n")
	} else {
		fmt.Printf("✅ Dispatched: %d | ❌ Failed: %d\n", sent, failed)
	}

	if sent > 0 && !dryRun {
		fmt.Println("\nTimeline (per DPDP Act — no statutory timelines yet):")
		fmt.Println("  • Acknowledge: as soon as reasonable (Section 8(6), DPDP Act 2023)")
		fmt.Println("  • Erasure: as soon as reasonable (no prescribed period)")
		fmt.Println()
		fmt.Println("  finwipe track --all       # Monitor acknowledgments")
		fmt.Println("  finwipe cron --followup    # Auto-follow-up after reasonable period")
		fmt.Println("  finwipe report            # Compliance dashboard")
	}

	return nil
}
