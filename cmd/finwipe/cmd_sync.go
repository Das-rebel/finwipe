package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync email forwarding discoveries from FinWipe cloud",
	Long: `Pull discovered financial institutions from FinWipe cloud and
optionally create deletion requests for them.

This command fetches FI discoveries made via email forwarding.
Set up forwarding with: finwipe setup-forward

Usage:
  finwipe sync                    # Show discoveries
  finwipe sync --auto            # Auto-create deletion requests for new FIs
  finwipe sync --dry-run=false   # Create requests without preview
  finwipe sync --cloud https://fw.finwipe.in  # Custom cloud endpoint`,
	RunE:  runSync,
}

var (
	syncAuto      bool
	syncCloudURL  string
	syncImport    string // Import from local forwarded emails (offline mode)
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVar(&syncAuto, "auto", false,
		"Automatically create deletion requests for new FIs")
	syncCmd.Flags().StringVar(&syncCloudURL, "cloud", "https://fw.finwipe.in",
		"FinWipe cloud API endpoint")
	syncCmd.Flags().StringVar(&syncImport, "import", "",
		"Import from local forwarded emails (overrides cloud)")
}

func runSync(cmd *cobra.Command, args []string) error {
	profile, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if profile.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	// Load inbox config
	inboxPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox")
	inboxData, err := os.ReadFile(inboxPath)
	if err != nil {
		return fmt.Errorf("inbox not set up. Run: finwipe setup-forward")
	}
	lines := strings.Split(strings.TrimSpace(string(inboxData)), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("inbox config corrupted. Run: finwipe setup-forward")
	}
	inboxAddr := lines[0]
	userHash := lines[1]

	// Load API key (stored separately)
	apiKeyPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "cloud_api_key")
	apiKey, _ := os.ReadFile(apiKeyPath)
	apiKeyStr := strings.TrimSpace(string(apiKey))

	fmt.Println()
	fmt.Println("  📬 FinWipe — Cloud Sync")
	fmt.Println("  ─────────────────────────────────────────────────────────────")
	fmt.Printf("  📧 Inbox: %s\n", inboxAddr)
	fmt.Printf("  🔗 Cloud: %s\n", syncCloudURL)

	// If --import specified, do local parse instead
	if syncImport != "" {
		return runLocalSync(syncImport, profile.Profile, userHash)
	}

	// Fetch from cloud
	apiURL := fmt.Sprintf("%s/api/discoveries?user_id=%s", strings.TrimSuffix(syncCloudURL, "/"), userHash)
	if apiKeyStr != "" {
		apiURL += "&api_key=" + url.QueryEscape(apiKeyStr)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		// Cloud unavailable — fall back to local
		fmt.Println()
		fmt.Println("  ⚠️  Cloud unreachable. Using local forwarded emails.")
		fmt.Println("  Run 'finwipe check-inbox --import ~/.finwipe/forwarded/'")
		fmt.Println()
		return runLocalFallback(profile.Profile, userHash)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloud API error: %s", string(body))
	}

	var result struct {
		UserID      string `json:"userId"`
		Count       int    `json:"count"`
		Discoveries []struct {
			Name       string `json:"name"`
			Category   string `json:"category"`
			Domain     string `json:"domain"`
			MatchType  string `json:"matchType"`
			Subject    string `json:"subject"`
			FirstSeen  string `json:"firstSeen"`
			LastSeen   string `json:"lastSeen"`
			Count      int    `json:"count"`
		} `json:"discoveries"`
		Timestamp string `json:"timestamp"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse cloud response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println()
		fmt.Println("  📭 No discoveries yet.")
		fmt.Println()
		fmt.Println("  Make sure your Gmail filter is forwarding emails to:")
		fmt.Printf("     %s\n", inboxAddr)
		fmt.Println()
		fmt.Println("  Then check back after receiving some financial emails.")
		return nil
	}

	fmt.Println()
	fmt.Printf("  ✅ Found %d financial institutions from email forwarding\n\n", result.Count)

	// Load registry for matching
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}
	entityMap := make(map[string]nbfc.Entity)
	for _, e := range entities {
		entityMap[e.ID] = e
	}

	// Categorize
	type match struct {
		Discovery interface{} // json discovery
		Entity   *nbfc.Entity
		Disc     struct {
			Name, Category, Domain, Subject string
			MatchType string
			Count     int
		}
	}
	var matched []match
	var unknown []struct{ Name, Domain, Category string; Count int }

	for _, d := range result.Discoveries {
		lower := strings.ToLower(d.Name)
		var entity *nbfc.Entity
		for _, e := range entities {
			if strings.Contains(lower, strings.ToLower(e.Name)) ||
				strings.Contains(strings.ToLower(e.Name), lower) {
				entity = &e
				break
			}
			if e.ShortName != "" && strings.Contains(lower, strings.ToLower(e.ShortName)) {
				entity = &e
				break
			}
		}

		if entity != nil {
			matched = append(matched, match{
				Entity: entity,
				Disc: struct {
					Name, Category, Domain, Subject string
					MatchType string
					Count     int
				}{
					Name:      d.Name,
					Category:  d.Category,
					Domain:    d.Domain,
					Subject:   d.Subject,
					MatchType: d.MatchType,
					Count:     d.Count,
				},
			})
		} else {
			unknown = append(unknown, struct{ Name, Domain, Category string; Count int }{
				Name: d.Name, Domain: d.Domain, Category: d.Category, Count: d.Count,
			})
		}
	}

	// Show matched
	if len(matched) > 0 {
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  ✅ MATCHED IN REGISTRY")
		fmt.Println("  ─────────────────────────────────────────────────────")
		for i, m := range matched {
			if i >= 30 {
				fmt.Printf("  ... and %d more\n", len(matched)-i)
				break
			}
			icon := "💳"
			if m.Entity.Category == nbfc.CatBANK {
				icon = "🏛️"
			} else if m.Entity.Category == nbfc.CatHFC {
				icon = "🏠"
			} else if m.Entity.Category == nbfc.CatFINTECH {
				icon = "📱"
			}
			grievance := m.Entity.GrievanceEmail
			if grievance == "" {
				grievance = "—"
			}
			fmt.Printf("  %2d. %s %-25s %s\n", i+1, icon, truncate(m.Entity.Name, 25),
				truncate(grievance, 25))
		}
		fmt.Println()
	}

	// Show unknown
	if len(unknown) > 0 {
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  ❓ NOT IN REGISTRY (may need investigation)")
		fmt.Println("  ─────────────────────────────────────────────────────")
		for i, u := range unknown {
			if i >= 15 {
				fmt.Printf("  ... and %d more\n", len(unknown)-i)
				break
			}
			fmt.Printf("  %2d. %-28s (%s)\n", i+1, truncate(u.Name, 28), u.Domain)
		}
		fmt.Println()
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Total: %d | Matched: %d | Unknown: %d\n", result.Count, len(matched), len(unknown))
	fmt.Println("  ═══════════════════════════════════════════════════════════════")

	// Auto-create requests
	if syncAuto || !dryRun {
		if len(matched) == 0 {
			fmt.Println()
			fmt.Println("  No matched FIs to create requests for.")
			return nil
		}

		fmt.Println()
		fmt.Println("  🚀 Creating deletion requests...")

		hist, err := history.New(dbPath())
		if err != nil {
			return fmt.Errorf("open history: %w", err)
		}
		defer hist.Close()

		letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
		gen := letter.New(letterDir)

		created := 0
		skipped := 0

		for _, m := range matched {
			if m.Entity.GrievanceEmail == "" {
				skipped++
				continue
			}

			// Check if already exists
			existing, _ := hist.GetByNBFCID(m.Entity.ID)
			isDup := false
			for _, e := range existing {
				if e.LifecycleState != history.StateClosed &&
					e.LifecycleState != history.StateDeliveryFailed {
					isDup = true
					break
				}
			}
			if isDup {
				skipped++
				continue
			}

			req, err := hist.CreateRequest(
				m.Entity.ID, m.Entity.Name,
				history.ChannelEmail,
				m.Entity.GrievanceEmail,
				profile.Profile.Email, profile.Profile.Name,
			)
			if err != nil {
				fmt.Printf("  ⚠️  %-25s %v\n", m.Entity.Name, err)
				continue
			}

			gen.Generate(req.RequestID, m.Entity.Name, m.Entity.GrievanceEmail,
				profile.Profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)

			fmt.Printf("  ✅ %-25s %s\n", m.Entity.Name, req.RequestID)
			created++
		}

		fmt.Println()
		if created > 0 {
			fmt.Printf("  ✅ Created: %d deletion requests\n", created)
		}
		if skipped > 0 {
			fmt.Printf("  ⏭️  Skipped (already exists): %d\n", skipped)
		}
		fmt.Println()
		fmt.Println("  Next: finwipe send              # Dispatch to FIs")
	} else {
		fmt.Println()
		fmt.Println("  🔍 DRY RUN — No requests created")
		fmt.Println("  Run with --auto or --dry-run=false to create requests")
	}

	return nil
}

// runLocalFallback checks local forwarded emails when cloud is unavailable
func runLocalFallback(profile config.Profile, userHash string) error {
	forwardedDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "forwarded")
	if _, err := os.Stat(forwardedDir); os.IsNotExist(err) {
		fmt.Println("  Run 'finwipe check-inbox' to parse local emails first.")
		return nil
	}

	// Reuse check-inbox logic via import
	return runLocalSync(forwardedDir, profile, userHash)
}

// runLocalSync parses local forwarded emails (offline/cloud-unavailable mode)
func runLocalSync(dirPath string, profile config.Profile, userHash string) error {
	// Collect email files
	var emailFiles []string
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".eml" || ext == ".mbox" || ext == ".txt" || ext == ".mail" {
			emailFiles = append(emailFiles, filepath.Join(dirPath, e.Name()))
		}
	}

	if len(emailFiles) == 0 {
		fmt.Println("  📭 No email files found in " + dirPath)
		return nil
	}

	fmt.Printf("  📂 Parsing %d local email files...\n", len(emailFiles))

	// Read all email content
	var allText []string
	for _, f := range emailFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		allText = append(allText, string(data))
	}

	// Load entities
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	// Parse (reuse check-inbox parsing logic via a helper)
	found := parseEmailsForFIsOffline(allText, entities)

	if len(found) == 0 {
		fmt.Println("  📭 No financial institutions found in emails.")
		return nil
	}

	fmt.Printf("  ✅ Found %d unique FIs\n\n", len(found))

	// Show results
	type match struct {
		Name   string
		Entity *nbfc.Entity
		Count  int
	}
	var matched []match

	seen := make(map[string]bool)
	for _, f := range found {
		if seen[f.Entity.Name] {
			continue
		}
		seen[f.Entity.Name] = true
		matched = append(matched, match{
			Name:   f.Entity.Name,
			Entity: f.Entity,
			Count:  f.Count,
		})
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	for i, m := range matched {
		if i >= 25 {
			fmt.Printf("  ... and %d more\n", len(matched)-i)
			break
		}
		icon := "💳"
		if m.Entity.Category == nbfc.CatBANK {
			icon = "🏛️"
		}
		fmt.Printf("  %2d. %s %-25s [%d emails]\n", i+1, icon,
			truncate(m.Entity.Name, 25), m.Count)
	}
	fmt.Println("  ═══════════════════════════════════════════════════════════════")

	if !syncAuto && dryRun {
		fmt.Println()
		fmt.Println("  🔍 DRY RUN — No requests created")
		return nil
	}

	// Create requests
	if len(matched) == 0 {
		return nil
	}

	fmt.Println()
	fmt.Println("  🚀 Creating deletion requests...")

	hist, err := history.New(dbPath())
	if err != nil {
		return err
	}
	defer hist.Close()

	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	gen := letter.New(letterDir)

	created := 0
	for _, m := range matched {
		if m.Entity.GrievanceEmail == "" {
			continue
		}
		existing, _ := hist.GetByNBFCID(m.Entity.ID)
		isDup := false
		for _, e := range existing {
			if e.LifecycleState != history.StateClosed &&
				e.LifecycleState != history.StateDeliveryFailed {
				isDup = true
				break
			}
		}
		if isDup {
			continue
		}

		req, err := hist.CreateRequest(m.Entity.ID, m.Entity.Name,
			history.ChannelEmail,
			m.Entity.GrievanceEmail,
			profile.Email, profile.Name)
		if err != nil {
			continue
		}

		gen.Generate(req.RequestID, m.Entity.Name, m.Entity.GrievanceEmail,
			profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)

		fmt.Printf("  ✅ %-25s %s\n", m.Entity.Name, req.RequestID)
		created++
	}

	fmt.Printf("\n  ✅ Created: %d deletion requests\n", created)
	return nil
}

// offlineEmailMatch mirrors emailMatch but without cloud dependency
type offlineEmailMatch struct {
	Entity *nbfc.Entity
	Count  int
}

// parseEmailsForFIsOffline parses local email files for FI names
func parseEmailsForFIsOffline(emails []string, entities []nbfc.Entity) []offlineEmailMatch {
	seen := make(map[string]*offlineEmailMatch)

	entityByName := make(map[string]nbfc.Entity)
	for i := range entities {
		e := &entities[i]
		entityByName[strings.ToLower(e.Name)] = *e
		if e.ShortName != "" {
			entityByName[strings.ToLower(e.ShortName)] = *e
		}
	}

	fiKeywords := []string{
		"HDFC Bank", "ICICI Bank", "Axis Bank", "Kotak Mahindra",
		"State Bank", "IndusInd Bank", "Yes Bank", "IDBI Bank",
		"Bank of Baroda", "Punjab National Bank", "Canara Bank",
		"Federal Bank", "RBL Bank", "Bandhan Bank",
		"Bajaj Finserv", "Bajaj Finance", "Tata Capital",
		"Aditya Birla Finance", "L&T Finance", "Muthoot Finance",
		"Cholamandalam", "HDB Financial", "Kisht", "Stashfin",
		"Rupeek", "KreditBee", "Navi Finserv", "OfBusiness",
		"EarlySalary", "Slice", "Uni", "Moneyview",
		"PhonePe", "Paytm", "Razorpay", "CRED",
		"Paisabazaar", "BankBazaar", "IndMoney", "Groww",
		"Zerodha", "Upstox", "Angel One", "PolicyBazaar",
		"LIC", "HDFC Life", "SBI Life", "ICICI Prudential",
		"Bajaj Allianz", "TATA AIA", "Max Life", "Star Health",
	}

	for _, email := range emails {
		upper := strings.ToUpper(email)
		for _, keyword := range fiKeywords {
			if strings.Contains(upper, strings.ToUpper(keyword)) {
				lower := strings.ToLower(keyword)
				if entity, ok := entityByName[lower]; ok {
					if _, exists := seen[entity.ID]; !exists {
						seen[entity.ID] = &offlineEmailMatch{Entity: &entity, Count: 1}
					} else {
						seen[entity.ID].Count++
					}
				}
			}
		}
	}

	var results []offlineEmailMatch
	for _, m := range seen {
		results = append(results, *m)
	}
	return results
}

// hashEmail creates a privacy-preserving hash of email for user ID
func hashEmail(email string) string {
	h := sha256.Sum256([]byte(email + "finwipe-salt-v1"))
	return hex.EncodeToString(h[:8])
}
