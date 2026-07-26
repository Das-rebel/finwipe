package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new deletion request (returns DPR-ID for tracking)",
	Long: `Create one or more deletion requests and get a DPR-ID for each.

A DPR-ID (DPR-2026-000001) is your tracking reference. Use it to:
  - Track acknowledgment:  finwipe track --request-id DPR-2026-000001
  - Follow up:           finwipe followup --request-id DPR-2026-000001
  - Escalate:             finwipe escalate --request-id DPR-2026-000001

Examples:
  finwipe new --nbfc bajaj-finserv --name "John Doe" --email john@mail.com
  finwipe new --category fintech --name "John" --email john@mail.com
  finwipe new --nbfc lic --request-type insurance --policy-number ABC123
  finwipe new --batch fintech --dry-run
  finwipe new --nbfc xyz-bank --categories marketing,third_party`,
	RunE:  runNew,
}

var (
	newNBFCID       string
	newCategory     string
	newBatch       string
	newRequestType string
	newPolicyNumber string
	newName         string
	newEmail        string
	newCategories   string // comma-separated deletion categories
	newDryRun       bool
)

func init() {
	newCmd.Flags().StringVar(&newNBFCID, "nbfc", "",
		"NBFC ID (e.g., bajaj-finserv, hdfc-bank, lic)")
	newCmd.Flags().StringVar(&newCategory, "category", "",
		"Category: fintech, nbfc, bank, hfc, lsp, dsp")
	newCmd.Flags().StringVar(&newBatch, "batch", "",
		"Create requests for ALL entities in category (e.g., fintech, bank, nbfc, all)")
	newCmd.Flags().StringVar(&newRequestType, "request-type", "financial-data",
		"Type: financial-data, insurance, telecom, edtech")
	newCmd.Flags().StringVar(&newPolicyNumber, "policy-number", "",
		"Policy/Account number (for insurance/bank requests)")
	newCmd.Flags().StringVar(&newName, "name", "",
		"Your full legal name (or set in finwipe init)")
	newCmd.Flags().StringVar(&newEmail, "email", "",
		"Your registered email (or set in finwipe init)")
	newCmd.Flags().StringVar(&newCategories, "categories", "",
		"Comma-separated deletion categories: marketing,third_party,behavioral,app_usage,medical,nominee,employment")
	newCmd.Flags().BoolVar(&newDryRun, "dry-run", false,
		"Preview requests without creating")
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, args []string) error {
	// Load profile
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Get name and email from flags or config
	name := newName
	emailAddr := newEmail
	if name == "" {
		name = cfg.Profile.Name
	}
	if emailAddr == "" {
		emailAddr = cfg.Profile.Email
	}
	if name == "" || emailAddr == "" {
		return fmt.Errorf("name and email required. Set via --name/--email or run finwipe init")
	}


	// Load NBFCs
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}

	// Determine categories
	categories := parseCategories(newCategories, newRequestType)

	// Build target list
	var targets []nbfc.Entity
	if newNBFCID != "" {
		found := false
		for _, e := range entities {
			if e.ID == newNBFCID || e.ID == strings.TrimSpace(newNBFCID) {
				targets = append(targets, e)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("NBFC not found: %s (try: finwipe list --search %s)",
				newNBFCID, newNBFCID)
		}
	} else if newCategory != "" {
		catMap := map[string]nbfc.Category{
			"nbfc":   nbfc.CatNBFC,
			"hfc":    nbfc.CatHFC,
			"fintech": nbfc.CatFINTECH,
			"lsp":    nbfc.CatLSP,
			"dsp":    nbfc.CatDSP,
			"bank":   nbfc.CatBANK,
		}
		cat, ok := catMap[newCategory]
		if !ok {
			return fmt.Errorf("invalid category: %s (use: fintech, nbfc, bank, hfc, lsp, dsp)",
				newCategory)
		}
		for _, e := range entities {
			if e.Category == cat && e.GrievanceEmail != "" && e.Active {
				targets = append(targets, e)
			}
		}
	} else if newBatch != "" {
		for _, e := range entities {
			if e.GrievanceEmail == "" || !e.Active {
				continue
			}
			if newBatch == "all" {
				targets = append(targets, e)
			} else if e.Category == nbfc.CatFINTECH && newBatch == "fintech" {
				targets = append(targets, e)
			} else if e.Category == nbfc.CatNBFC && newBatch == "nbfc" {
				targets = append(targets, e)
			} else if e.Category == nbfc.CatBANK && newBatch == "bank" {
				targets = append(targets, e)
			} else if e.Category == nbfc.CatHFC && newBatch == "hfc" {
				targets = append(targets, e)
			}
		}
	} else {
		return fmt.Errorf(" specify --nbfc, --category, or --batch")
	}

	if len(targets) == 0 {
		return fmt.Errorf("no matching NBFCs found")
	}

	fmt.Println()
	fmt.Printf("  🆕 Creating %d deletion request(s)\n", len(targets))
	fmt.Printf("  👤 Name: %s\n", name)
	fmt.Printf("  📧 Email: %s\n", emailAddr)
	fmt.Printf("  📋 Categories: %s\n", categories)
	if newDryRun {
		fmt.Println("  🔍 DRY RUN — No requests created")
	}

	// Print targets
	fmt.Println()
	for i, e := range targets {
		if i >= 20 {
			fmt.Printf("  ... and %d more\n", len(targets)-20)
			break
		}
		email := e.GrievanceEmail
		if email == "" {
			email = "(no email — use registered post)"
		}
		fmt.Printf("  %2d. %-30s %s\n", i+1, truncate(e.Name, 30), email)
	}
	fmt.Println()

	// Filter to those with emails
	var emailable []nbfc.Entity
	for _, e := range targets {
		if e.GrievanceEmail != "" {
			emailable = append(emailable, e)
		}
	}

	if len(emailable) == 0 {
		return fmt.Errorf("none of the selected NBFCs have grievance emails")
	}

	if newDryRun {
		fmt.Printf("  Would create %d requests\n", len(emailable))
		return nil
	}

	// Create requests
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	created := 0
	failed := 0

	fmt.Println("  Creating requests...")
	for _, e := range emailable {
		req, err := hist.CreateRequest(e.ID, e.Name, history.ChannelEmail,
			e.GrievanceEmail, emailAddr, name)
		if err != nil {
			fmt.Printf("  ⚠️  %-30s %v\n", e.Name, err)
			failed++
			continue
		}
		fmt.Printf("  ✅ %-30s %s\n", e.Name, req.RequestID)
		created++
	}

	fmt.Println()
	fmt.Printf("  ✅ Created: %d | ⚠️  Failed: %d\n", created, failed)

	if created > 0 {
		fmt.Println()
		if cfg.SMTP.Password != "" {
			fmt.Println("  Next: finwipe send --dry-run  # Preview emails")
			fmt.Println("       finwipe send             # Send all requests")
		} else {
			fmt.Println("  ⚠️  SMTP not configured — set up email for sending:")
			fmt.Println("       finwipe init")
			fmt.Println()
			fmt.Println("  Or generate letters for registered post:")
			fmt.Printf("       finwipe letter --output ~/.finwipe/letters/\n")
		}
	}

	return nil
}

func parseCategories(catStr, requestType string) []letter.DeletionCategory {
	var result []letter.DeletionCategory

	if catStr != "" {
		// User-specified categories
		for _, c := range strings.Split(catStr, ",") {
			c = strings.TrimSpace(strings.ToLower(c))
			switch c {
			case "marketing", "marketing_data":
				result = append(result, letter.CatMarketing)
			case "third_party", "third_party_shared":
				result = append(result, letter.CatThirdParty)
			case "behavioral", "behavioral_analytics":
				result = append(result, letter.CatBehavioral)
			case "app_usage", "app_usage_metadata":
				result = append(result, letter.CatAppUsage)
			case "call_records", "call_interaction_logs":
				result = append(result, letter.CatCallRecords)
			case "sms", "sms_logs":
				result = append(result, letter.CatSMSLogs)
			case "location", "location_metadata":
				result = append(result, letter.CatLocation)
			case "employment", "employment_proof":
				result = append(result, letter.CatEmployment)
			case "medical", "health_records":
				result = append(result, letter.CatMedical)
			case "nominee", "nominee_data":
				result = append(result, letter.CatNominee)
			case "credit", "credit_profile":
				result = append(result, letter.CatCreditProfile)
			case "kyc", "kyc_supplements":
				result = append(result, letter.CatKYCSupplements)
			case "all", "all_non_essential":
				return []letter.DeletionCategory{
					letter.CatMarketing,
					letter.CatThirdParty,
					letter.CatBehavioral,
					letter.CatAppUsage,
					letter.CatMarketingPref,
				}
			}
		}
	}

	if len(result) == 0 {
		// Default based on request type
		switch requestType {
		case "insurance":
			return letter.InsuranceDeletionCategories
		case "telecom":
			return letter.TelecomDeletionCategories
		default:
			return letter.DefaultDeletionCategories
		}
	}

	return result
}

