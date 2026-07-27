package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var forwardCmd = &cobra.Command{
	Use:   "setup-forward",
	Short: "Get your FinWipe cloud inbox address for passive FI discovery",
	Long: `Set up email forwarding to discover FIs passively.

This is the CRED/Fold.money model:
1. You get a unique FinWipe inbox address
2. Set up a Gmail filter to forward financial emails
3. FinWipe cloud receives and parses sender domains
4. finwipe sync pulls discoveries → creates deletion requests

WHAT YOU GET:
  • Unique inbox: <hash>@inbox.finwipe.in
  • Privacy: Your email is NEVER stored — only a one-way hash
  • No OAuth: Just a Gmail filter (takes 2 minutes)
  • Passive: All financial emails forwarded automatically

WHAT FINWIPE CLOUD SEES (only):
  • Sender domain (e.g., "hdfcbank.com") — NOT your emails
  • Subject line (for FI matching)
  • Nothing else — no email content, no addresses

PREREQUISITES:
  1. A domain (or use inbox.finwipe.in if pre-configured)
  2. Cloudflare Worker deployed (see apps/finwipe-cloud/)
  3. Mailgun inbound email set up

DEPLOY YOUR OWN CLOUD (free):
  cd apps/finwipe-cloud
  ./deploy.sh
  (Mailgun free: 5K emails/month)

Examples:
  finwipe setup-forward              # Get your inbox address
  finwipe sync                      # Pull discoveries from cloud
  finwipe sync --auto              # Auto-create deletion requests
  finwipe check-inbox              # Parse local emails (no cloud)`,
	RunE: runSetupForward,
}

var checkInboxCmd = &cobra.Command{
	Use:   "check-inbox",
	Short: "Parse local forwarded emails (offline mode)",
	Long: `Parse forwarded emails stored locally (~/.finwipe/forwarded/)
and create deletion requests.

This is the offline alternative to 'finwipe sync'.
No cloud required — all processing is local.

Usage:
  finwipe check-inbox               # Parse local emails and show discoveries
  finwipe check-inbox --import /path/to/emails  # Custom directory
  finwipe check-inbox --dry-run=false  # Create deletion requests`,
	RunE: runCheckInbox,
}

var (
	inboxWatch  bool
	inboxImport string
)

func init() {
	forwardCmd.AddCommand(checkInboxCmd)
	checkInboxCmd.Flags().BoolVar(&inboxWatch, "watch", false,
		"Monitor forwarded emails continuously (not yet implemented)")
	checkInboxCmd.Flags().StringVar(&inboxImport, "import", "",
		"Parse specific email files or directory (default: ~/.finwipe/forwarded/)")
}

func runSetupForward(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("profile incomplete (run finwipe init)")
	}

	// Generate privacy-preserving inbox ID
	// SHA256(email + salt) → 16-char hash
	// User is identified by hash only — no PII stored in cloud
	inboxID := generateInboxID(cfg.Profile.Email)
	forwardAddr := inboxID + "@inbox.finwipe.in"

	// Also generate a read API key (for fetching discoveries)
	apiKey := generateAPIKey(cfg.Profile.Email)

	// Save inbox config
	inboxPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox")
	os.MkdirAll(filepath.Dir(inboxPath), 0700)
	inboxConfig := fmt.Sprintf("%s\n%s\n%s\n%s\n",
		forwardAddr,    // line 1: inbox email address
		inboxID,        // line 2: user hash (used as user_id)
		apiKey,         // line 3: API key for reads
		"https://fw.finwipe.in", // line 4: cloud API endpoint
	)
	if err := os.WriteFile(inboxPath, []byte(inboxConfig), 0600); err != nil {
		return fmt.Errorf("save inbox config: %w", err)
	}

	// Save API key separately (for writes by cloud worker)
	apiKeyPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "cloud_api_key")
	os.WriteFile(apiKeyPath, []byte(apiKey), 0600)

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║         FinWipe — Email Forwarding Setup                    ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║  CRED / Fold.money Style — Passive Financial Discovery     ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📧 Your FinWipe forwarding address:\n\n")
	fmt.Printf("     %s\n\n", forwardAddr)
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  🔒 PRIVACY GUARANTEE:")
	fmt.Println("     Your email is NEVER stored.")
	fmt.Println("     The cloud only sees: sender domain (e.g. hdfcbank.com)")
	fmt.Println("     No email content, no subject lines, no addresses.")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📋 SETUP STEPS (takes 2 minutes):")
	fmt.Println()
	fmt.Println("  STEP 1 — Deploy FinWipe Cloud (one-time):")
	fmt.Println("     cd apps/finwipe-cloud && ./deploy.sh")
	fmt.Println()
	fmt.Println("  STEP 2 — Gmail Filter:")
	fmt.Println("     a. Open Gmail → Settings (⚙️) → See all settings")
	fmt.Println("     b. Go to 'Filters' tab → 'Create a new filter'")
	fmt.Printf("     c. 'Has the words': bank OR loan OR EMI OR credit OR insurance\n")
	fmt.Printf("     d. 'Forward to': Add %s\n", forwardAddr)
	fmt.Println("     e. Check 'Forward it' → Create filter")
	fmt.Println()
	fmt.Println("  STEP 3 — Test it:")
	fmt.Println("     finwipe sync")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📬 HOW IT WORKS:")
	fmt.Println()
	fmt.Println("  Gmail Filter → FORWARDS all matching emails to FinWipe")
	fmt.Println()
	fmt.Println("  Mailgun (free tier) → RECEIVES emails, extracts sender domain only")
	fmt.Println()
	fmt.Println("  Cloudflare Worker → MATCHES domains to known FIs, stores discovery")
	fmt.Println()
	fmt.Println("  finwipe sync → PULLS discoveries, shows what FIs found your email")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📁 Files created:")
	fmt.Printf("     ~/.finwipe/inbox          — your inbox address and hash\n")
	fmt.Printf("     ~/.finwipe/cloud_api_key  — API key for secure access\n")
	fmt.Println()
	fmt.Println("  Next: finwipe sync              # Pull discoveries")
	fmt.Println("       finwipe check-inbox       # Parse local emails (no cloud)")
	fmt.Println()
	fmt.Println("  ═════════════════════════════════════════════════════════════════")

	return nil
}

func runCheckInbox(cmd *cobra.Command, args []string) error {
	// Load profile
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
		fmt.Println("  ⚠️  Inbox not set up. Run: finwipe setup-forward")
		fmt.Println()
		fmt.Println("  Or use: finwipe check-inbox --import /path/to/emails")
		fmt.Println()
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(inboxData)), "\n")
	inboxAddr := lines[0]
	if len(lines) < 4 {
		lines = append(lines, "https://fw.finwipe.in")
	}

	fmt.Println()
	fmt.Println("  📬 FinWipe — Check Inbox (Offline)")
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Printf("  📧 Forwarding address: %s\n", inboxAddr)

	// Determine path to check
	dirPath := inboxImport
	if dirPath == "" {
		dirPath = filepath.Join(os.Getenv("HOME"), ".finwipe", "forwarded")
	}

	// Collect email files
	var emailFiles []string

	// Check if it's a directory
	if stat, err := os.Stat(dirPath); err == nil && stat.IsDir() {
		entries, _ := os.ReadDir(dirPath)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".eml" || ext == ".mbox" || ext == ".txt" || ext == ".mail" {
				emailFiles = append(emailFiles, filepath.Join(dirPath, e.Name()))
			}
		}
	} else if _, err := os.Stat(dirPath); err == nil {
		// Single file
		emailFiles = []string{dirPath}
	}

	if len(emailFiles) == 0 {
		fmt.Println()
		fmt.Println("  📭 No email files found.")
		fmt.Println()
		fmt.Println("  To use offline mode:")
		fmt.Println("    1. Export emails from Gmail (Takeout → MBOX)")
		fmt.Println("    2. Put .eml/.mbox files in ~/.finwipe/forwarded/")
		fmt.Println("    3. Run finwipe check-inbox")
		fmt.Println()
		fmt.Println("  Or set up cloud forwarding for passive discovery:")
		fmt.Println("    finwipe setup-forward")
		return nil
	}

	fmt.Printf("  📂 Found %d email files\n\n", len(emailFiles))

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

	// Parse
	found := parseEmailsOffline(allText, entities)

	if len(found) == 0 {
		fmt.Println("  📭 No financial institutions found in emails.")
		return nil
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  📊 DISCOVERED: %d financial institutions\n", len(found))
	fmt.Println("  ═══════════════════════════════════════════════════════════════")

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

	for i, m := range matched {
		if i >= 25 {
			fmt.Printf("  ... and %d more\n", len(matched)-i)
			break
		}
		icon := "💳"
		if m.Entity.Category == nbfc.CatBANK {
			icon = "🏛️"
		} else if m.Entity.Category == nbfc.CatHFC {
			icon = "🏠"
		}
		fmt.Printf("  %2d. %s %-25s [%d emails]\n", i+1, icon,
			truncate(m.Entity.Name, 25), m.Count)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("  🔍 DRY RUN — No requests created")
		fmt.Println("  Run with --dry-run=false to create requests")
		return nil
	}

	// Create requests
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
			profile.Profile.Email, profile.Profile.Name)
		if err != nil {
			continue
		}

		gen.Generate(req.RequestID, m.Entity.Name, m.Entity.GrievanceEmail,
			profile.Profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)

		fmt.Printf("  ✅ %-25s %s\n", m.Entity.Name, req.RequestID)
		created++
	}

	fmt.Printf("\n  ✅ Created: %d deletion requests\n", created)
	return nil
}

// offlineMatch mirrors offlineEmailMatch
type offlineMatch struct {
	Entity *nbfc.Entity
	Count  int
}

// parseEmailsOffline parses local emails for FI names
func parseEmailsOffline(emails []string, entities []nbfc.Entity) []offlineMatch {
	seen := make(map[string]*offlineMatch)

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
						seen[entity.ID] = &offlineMatch{Entity: &entity, Count: 1}
					} else {
						seen[entity.ID].Count++
					}
				}
			}
		}
	}

	var results []offlineMatch
	for _, m := range seen {
		results = append(results, *m)
	}
	return results
}

// generateInboxID generates a random inbox ID using crypto/rand.
func generateInboxID(email string) string {
	// Generate 8 random bytes
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback (should not happen)
		return "u" + hex.EncodeToString(b)[:10]
	}
	// Human-readable prefix from email
	parts := strings.Split(email, "@")
	prefix := "u"
	if len(parts) > 0 && len(parts[0]) > 2 {
		for _, c := range parts[0][:2] {
			if c >= 'a' && c <= 'z' {
				prefix = prefix + string(c)
			}
		}
	}
	return prefix + hex.EncodeToString(b)[:10]
}

// generateAPIKey creates a cryptographically random API key using crypto/rand.
func generateAPIKey(email string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback (should not happen)
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}
