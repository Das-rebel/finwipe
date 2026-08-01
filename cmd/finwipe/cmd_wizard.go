package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/evidence"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)


// sanitizeHeader strips CR, LF, and null bytes to prevent CRLF header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

var wizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive guided deletion request flow",
	Long: `Step-by-step interactive wizard to create and send deletion requests.

Run without flags to start the guided flow:
  finwipe wizard`,
	RunE: runWizard,
}

var (
	wizardCategory string
	wizardDryRun  bool
)

func init() {
	wizardCmd.Flags().BoolVar(&wizardDryRun, "dry-run", false, "Preview without sending")
	wizardCmd.Flags().StringVar(&wizardCategory, "category", "",
		"Pre-filter by category: fintech, nbfc, bank, hfc, lsp, dsp")
}

func runWizard(cmd *cobra.Command, args []string) error {
	r := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════╗")
	fmt.Println("  ║         FinWipe — Data Deletion Request Wizard         ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Load or create config
	fmt.Println("  ▶ Step 1: Loading your profile...")
	cfg, err := config.Load(cfgFile)
	if err != nil || cfg.Profile.Name == "" {
		fmt.Println("  ⚠️  Profile not configured. Running quick setup...")
		cfg, err = runInitWizard(r)
		if err != nil {
			return fmt.Errorf("profile setup: %w", err)
		}
	} else {
		fmt.Printf("  ✅ Profile: %s <%s>\n", cfg.Profile.Name, cfg.Profile.Email)
	}
	fmt.Println()

	// Step 2: Load NBFC registry
	fmt.Println("  ▶ Step 2: Loading NBFC registry...")
	nbfcPath := nbfcRegistryPath()
	entities, err := nbfc.Load(nbfcPath)
	if err != nil {
		// Registry not found — offer to fetch it
		fmt.Println("  ⚠️  NBFC registry not found.")
		fmt.Println("  Running finwipe update-registry to fetch the latest list...")
		fmt.Println()
		// Try to fetch via update-registry (sets dry-run=false)
		// Since we can't call another command directly, tell the user
		fmt.Println("  Run this first:")
		fmt.Println("    finwipe update-registry")
		fmt.Println("  Then re-run this wizard:")
		fmt.Println("    finwipe wizard")
		fmt.Println()
		return fmt.Errorf("run 'finwipe update-registry' first, then re-run 'finwipe wizard'")
	}
	fmt.Printf("  ✅ %d NBFCs loaded\n", len(entities))
	fmt.Println()

	// Step 3: Select NBFCs
	fmt.Println("  ▶ Step 3: Select NBFCs to target...")
	targets, err := selectNBFCs(r, entities)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Println("  No NBFCs selected. Goodbye!")
		return nil
	}
	fmt.Printf("  Selected %d NBFCs\n", len(targets))
	fmt.Println()

	// Step 4: Review
	fmt.Println("  ▶ Step 4: Review & Confirm")
	fmt.Println("  ──────────────────────────────────────────────────────────")
	for i, e := range targets {
		email := e.GrievanceEmail
		if email == "" {
			email = "(use registered post)"
		}
		fmt.Printf("  %2d. %-35s %s\n", i+1, e.Name, email)
	}
	fmt.Println()

	fmt.Print("  Send deletion requests to these NBFCs? [Y/n]: ")
	yn, _ := r.ReadString('\n')
	yn = strings.TrimSpace(strings.ToLower(yn))
	if yn == "n" {
		fmt.Println("  Cancelled.")
		return nil
	}

	// Step 5: Create requests
	fmt.Println()
	fmt.Println("  ▶ Step 5: Creating requests...")
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("history db: %w", err)
	}
	defer hist.Close()

	evBase := filepath.Join(os.Getenv("HOME"), ".finwipe", "evidence")
	evStore, _ := evidence.New(evBase)

	var created []string
	for _, e := range targets {
		req, err := hist.CreateRequest(e.ID, e.Name, history.ChannelEmail,
			e.GrievanceEmail, cfg.Profile.Email, cfg.Profile.Name)
		if err != nil {
			fmt.Printf("  ⚠️  %-35s %v\n", e.Name, err)
			continue
		}
		created = append(created, req.RequestID)

		// Store deletion request email as evidence
		if evStore != nil && e.GrievanceEmail != "" {
			emailBody := buildDeletionEmail(req.RequestID, e, cfg.Profile)
			evStore.Save(req.RequestID, evidence.TypeEmailSent,
				"DeletionRequest_"+req.RequestID+".eml",
				io.NopCloser(strings.NewReader(emailBody)),
				"Deletion request email")
		}

		fmt.Printf("  ✅ %-35s %s\n", e.Name, req.RequestID)
	}
	fmt.Println()

	if len(created) == 0 {
		fmt.Println("  ❌ No requests created.")
		return nil
	}

	fmt.Printf("  📋 %d requests created\n", len(created))
	fmt.Println()

	// Step 6: Send or show next steps
	fmt.Println("  ▶ Step 6: Next steps")
	fmt.Println("  ──────────────────────────────────────────────────────────")
	if wizardDryRun {
		fmt.Println("  🔍 DRY RUN — No emails sent")
	} else {
		if cfg.SMTP.Password != "" {
			fmt.Println("  📤 SMTP configured — ready to send emails")
		} else {
			fmt.Println("  ⚠️  SMTP not configured — use registered post")
		}
	}
	fmt.Println()
	fmt.Println("  Commands to run next:")
	fmt.Printf("    finwipe send --include %s\n", strings.Join(created[:3], ","))
	if len(created) > 3 {
		fmt.Printf("    # ...and %d more\n", len(created)-3)
	}
	fmt.Println("    finwipe track --all")
	fmt.Println("    finwipe report")
	fmt.Println()

	return nil
}

// selectNBFCs interactively selects NBFCs
func selectNBFCs(r *bufio.Reader, entities []nbfc.Entity) ([]nbfc.Entity, error) {
	catMap := map[string]nbfc.Category{
		"nbfc":   nbfc.CatNBFC,
		"hfc":    nbfc.CatHFC,
		"fintech": nbfc.CatFINTECH,
		"lsp":    nbfc.CatLSP,
		"dsp":    nbfc.CatDSP,
		"bank":   nbfc.CatBANK,
	}

	var selectable []nbfc.Entity
	if wizardCategory != "" {
		if cat, ok := catMap[wizardCategory]; ok {
			for _, e := range entities {
				if e.Category == cat && e.GrievanceEmail != "" && e.Active {
					selectable = append(selectable, e)
				}
			}
			fmt.Printf("  Filtered to %d %s entities\n", len(selectable), wizardCategory)
		}
	} else {
		for _, e := range entities {
			if e.GrievanceEmail != "" && e.Active {
				selectable = append(selectable, e)
			}
		}
	}

	if len(selectable) == 0 {
		return nil, nil
	}

	fmt.Println("  Leave blank and press Enter to select ALL, or type to search:")
	fmt.Print("  > ")
	query, _ := r.ReadString('\n')
	query = strings.TrimSpace(strings.ToLower(query))

	var targets []nbfc.Entity
	if query == "" {
		targets = selectable
	} else {
		fmt.Println()
		var matches []nbfc.Entity
		for _, e := range selectable {
			if strings.Contains(strings.ToLower(e.Name), query) ||
				strings.Contains(strings.ToLower(e.ID), query) {
				matches = append(matches, e)
			}
		}
		if len(matches) == 0 {
			fmt.Println("  No matches found.")
			return nil, nil
		}

		fmt.Println("  Search results:")
		for i, e := range matches {
			if i >= 30 {
				fmt.Printf("  ... and %d more\n", len(matches)-30)
				break
			}
			fmt.Printf("  %3d. %-40s %s\n", i+1, e.Name, e.GrievanceEmail)
		}
		fmt.Println()
		fmt.Print("  Enter numbers to select (e.g. 1,3,5) or 'all': ")
		sel, _ := r.ReadString('\n')
		sel = strings.TrimSpace(strings.ToLower(sel))

		if sel == "all" {
			targets = matches
		} else {
			selMap := make(map[int]bool)
			for _, s := range strings.Split(sel, ",") {
				s = strings.TrimSpace(s)
				if n, err := strconv.Atoi(s); err == nil && n > 0 {
					selMap[n-1] = true
				}
			}
			for i, e := range matches {
				if selMap[i] {
					targets = append(targets, e)
				}
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []nbfc.Entity
	for _, t := range targets {
		if !seen[t.ID] {
			seen[t.ID] = true
			unique = append(unique, t)
		}
	}
	return unique, nil
}

// runInitWizard runs a simplified profile setup
func runInitWizard(r *bufio.Reader) (*config.Config, error) {
	cfg := &config.Config{
		Profile: config.Profile{},
		SMTP:    config.SMTP{},
	}

	fmt.Println("  ── Profile Setup ──────────────────────────────────────────")
	fmt.Print("  Your full name: ")
	name, _ := r.ReadString('\n')
	cfg.Profile.Name = strings.TrimSpace(name)

	fmt.Print("  Your email: ")
	email, _ := r.ReadString('\n')
	cfg.Profile.Email = strings.TrimSpace(email)

	fmt.Print("  Your phone (optional): ")
	cfg.Profile.Phone = strings.TrimSpace(mustRead(r))

	fmt.Print("  Your address (optional): ")
	cfg.Profile.Address = strings.TrimSpace(mustRead(r))

	fmt.Println()
	fmt.Println("  ── SMTP Setup ─────────────────────────────────────────────")
	fmt.Println("  Leave blank to skip email (use registered post instead)")
	fmt.Print("  SMTP host (e.g., smtp.gmail.com): ")
	host := strings.TrimSpace(mustRead(r))
	cfg.SMTP.Host = host

	if cfg.SMTP.Host != "" {
		cfg.SMTP.Port = 587
		cfg.SMTP.UseTLS = true
		fmt.Print("  Username (email): ")
		cfg.SMTP.Username = strings.TrimSpace(mustRead(r))
		fmt.Print("  App password: ")
		cfg.SMTP.Password = strings.TrimSpace(mustRead(r))
		cfg.SMTP.From = cfg.Profile.Email
	}

	cfgPath := config.DefaultPath()
	os.MkdirAll(filepath.Dir(cfgPath), 0700)
	if err := cfg.Save(cfgPath); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("  ✅ Config saved to: %s\n", cfgPath)

	return cfg, nil
}

func mustRead(r *bufio.Reader) string {
	s, _ := r.ReadString('\n')
	return s
}

// buildDeletionEmail creates the deletion request email body
func buildDeletionEmail(reqID string, entity nbfc.Entity, profile config.Profile) string {
	safeName := sanitizeHeader(profile.Name)
	safeEmail := sanitizeHeader(profile.Email)
	safeReqID := sanitizeHeader(reqID)
	return fmt.Sprintf(
		"From: %s <%s>\r\n"+
		"To: %s <%s>\r\n"+
		"Subject: DPDPA Section 8(6) Data Deletion Request \u2014 %s\r\n"+
		"Date: %s\r\n"+
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
		"Message-ID: <finwipe-request>\r\n"+
		"\r\n"+
		"Dear Grievance Officer,\n\n"+
		"I, %s, exercising my right to erasure under Section 8(6)\n"+
		"of the Digital Personal Data Protection Act, 2023 (DPDP Act),\n"+
		"hereby request deletion of the following categories of personal data\n"+
		"held by %s:\n\n"+
		"  \u25a1 Marketing and promotional data\n"+
		"  \u25a1 Third-party shared data\n"+
		"  \u25a1 Behavioral/usage data through your app/website\n"+
		"  \u25a1 Pre-approved loan offer profiles\n"+
		"  \u25a1 Call recordings and customer service interaction logs\n\n"+
		"I request acknowledgment as soon as reasonably practicable and completion of\n"+
		"deletion no statutory timeline per DPDP Act of this notice.\n\n"+
		"This request does not extend to KYC documents, transaction\n"+
		"records, or data required by law.\n\n"+
		"Request Reference: %s\n\n"+
		"Regards,\n"+
		"%s\n"+
		"Email: %s | Phone: %s\n\n"+
		"Generated by FinWipe | github.com/das-rebel/finwipe",
		safeEmail, entity.GrievanceEmail,
		entity.Name, safeName,
		time.Now().Format(time.RFC1123Z), safeReqID,
		safeName, entity.Name,
		safeReqID,
		safeName, safeEmail, profile.Phone)
}


