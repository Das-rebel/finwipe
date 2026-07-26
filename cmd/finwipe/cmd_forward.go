package main

import (
	"crypto/sha256"
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
	Short: "Get your personal FinWipe inbox address",
	Long: `Get a dedicated FinWipe email address for passive financial discovery.

HOW IT WORKS (CRED / Fold.money model):

1. FinWipe gives you a unique forwarding address
2. You set up ONE Gmail/Outlook rule: forward all financial emails
3. Run 'finwipe check-inbox' to parse them and create deletion requests
4. Repeat periodically — FinWipe tracks what's new

No OAuth. No cloud parsing. Your emails never leave your machine.

GMAIL SETUP (2 minutes):
  finwipe setup-forward    ← get your address
  Then in Gmail:
  Settings → Filters → Create filter
  Has the words: "bank" OR "loan" OR "EMI" OR "credit" OR "insurance"
  Forward to: <your FinWipe address>
  Also: Settings → Filters → Forward email → Add forwarding address

WHAT TO FORWARD:
  • Bank alerts and statements (any bank)
  • Loan confirmation / disbursement emails
  • Credit card statements
  • Insurance policy documents
  • Fintech purchase receipts

Examples:
  finwipe setup-forward              # Get your inbox address
  finwipe check-inbox               # Parse forwarded emails
  finwipe check-inbox --dry-run     # Preview only`,
	RunE: runSetupForward,
}

var checkInboxCmd = &cobra.Command{
	Use:   "check-inbox",
	Short: "Parse forwarded emails and create deletion requests",
	Long: `Check your FinWipe inbox for forwarded emails, extract
financial institution names, and auto-create deletion requests.

Run after setting up email forwarding with 'finwipe setup-forward'.
Also parses any .eml/.mbox/.txt files in ~/.finwipe/forwarded/

Usage:
  finwipe check-inbox               # Parse emails and create requests
  finwipe check-inbox --dry-run    # Preview only
  finwipe check-inbox --import ./emails/   # Parse specific files`,
	RunE: runCheckInbox,
}

var (
	inboxWatch  bool
	inboxImport string
)

func init() {
	forwardCmd.AddCommand(checkInboxCmd)
	checkInboxCmd.Flags().BoolVar(&inboxWatch, "watch", false,
		"Monitor forwarded emails continuously")
	checkInboxCmd.Flags().StringVar(&inboxImport, "import", "",
		"Parse specific email files or directory")
}

func runSetupForward(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("profile incomplete (run finwipe init)")
	}

	// Generate a unique, privacy-preserving inbox address
	// Format: <hash>@inbox.finwipe.in
	// The hash is one-way so FinWipe can't identify users from the address alone
	inboxID := generateInboxID(cfg.Profile.Email)
	inboxAddr := inboxID + "@inbox.finwipe.in"

	// Save inbox address
	inboxPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox")
	if err := os.WriteFile(inboxPath, []byte(inboxAddr+"\n"+inboxID), 0600); err != nil {
		return fmt.Errorf("save inbox: %w", err)
	}

	// Save last-check timestamp
	lastCheckPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox_lastcheck")
	os.WriteFile(lastCheckPath, []byte("0"), 0600)

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║         FinWipe — Email Forwarding Setup                      ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║  CRED / Fold.money Style — Passive Financial Discovery       ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📧 Your FinWipe forwarding address:\n\n")
	fmt.Printf("     %s\n\n", inboxAddr)
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📋 SETUP STEPS (2 minutes):")
	fmt.Println()
	fmt.Println("  GMAIL (desktop browser):")
	fmt.Println("  1. Open Gmail → Settings (⚙️) → See all settings")
	fmt.Println("  2. Go to 'Filters' tab → 'Create a new filter'")
	fmt.Printf("  3. In 'Has the words': bank OR loan OR EMI OR credit OR insurance\n")
	fmt.Printf("  4. In 'Forward to': Add %s\n", inboxAddr)
	fmt.Println("  5. Check 'Forward it' → Create filter")
	fmt.Println()
	fmt.Println("  OUTLOOK (web):")
	fmt.Println("  1. Settings → Mail → Rules → New rule")
	fmt.Println("  2. Apply to all messages OR with specific words")
	fmt.Printf("  3. Forward → %s\n", inboxAddr)
	fmt.Println()
	fmt.Println("  APPLE MAIL:")
	fmt.Println("  1. Mail → Rules → Add Rule")
	fmt.Println("  2. Condition: Any recipient contains '@'")
	fmt.Printf("  3. Action: Forward to %s\n", inboxAddr)
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📬 WHAT TO FORWARD:")
	fmt.Println("     • Bank account alerts (HDFC, ICICI, SBI, etc.)")
	fmt.Println("     • Loan sanction / disbursement emails")
	fmt.Println("     • Credit card statements")
	fmt.Println("     • Insurance policy documents")
	fmt.Println("     • Investment confirmations (mutual funds, stocks)")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  After forwarding some emails, run:")
	fmt.Println()
	fmt.Println("    finwipe check-inbox --dry-run")
	fmt.Println()
	fmt.Println("  This parses all forwarded emails and shows what FIs were found.")
	fmt.Println()
	fmt.Printf("  📁 Inbox address saved: %s\n", inboxPath)
	fmt.Println()
	fmt.Println("  ═════════════════════════════════════════════════════════════════")

	return nil
}

func runCheckInbox(cmd *cobra.Command, args []string) error {
	profile, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if profile.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	// Load inbox address
	inboxPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox")
	inboxData, err := os.ReadFile(inboxPath)
	if err != nil {
		return fmt.Errorf("inbox not set up. Run: finwipe setup-forward")
	}
	lines := strings.Split(strings.TrimSpace(string(inboxData)), "\n")
	inboxAddr := lines[0]

	fmt.Println()
	fmt.Println("  📬 FinWipe — Check Inbox")
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Printf("  📧 Forwarding address: %s\n", inboxAddr)

	// Load entities
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	// Collect emails from multiple sources
	var allEmailText []string

	// Source 1: Forwarded emails directory (~/.finwipe/forwarded/)
	forwardedDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "forwarded")
	if _, err := os.Stat(forwardedDir); err == nil {
		emails := collectEmailsFromDir(forwardedDir)
		allEmailText = append(allEmailText, emails...)
		fmt.Printf("  📂 Forwarded dir: %d emails\n", len(emails))
	}

	// Source 2: Specific import path
	if inboxImport != "" {
		emails := collectEmailsFromDir(inboxImport)
		allEmailText = append(allEmailText, emails...)
		fmt.Printf("  📥 Import path: %d emails\n", len(emails))
	}

	// Source 3: Local mail spool (common locations)
	mailSpools := []string{
		"/var/mail/" + os.Getenv("USER"),
		filepath.Join(os.Getenv("HOME"), "Mail", "INBOX"),
		filepath.Join(os.Getenv("HOME"), "Library", "Mail", "V2", "IMAP-localhost"),
	}
	for _, spool := range mailSpools {
		if _, err := os.Stat(spool); err == nil {
			if emails := collectEmailsFromFile(spool); len(emails) > 0 {
				allEmailText = append(allEmailText, emails...)
				fmt.Printf("  📬 Mail spool: %d emails\n", len(emails))
			}
		}
	}

	if len(allEmailText) == 0 {
		fmt.Println()
		fmt.Println("  ⚠️  No forwarded emails found.")
		fmt.Println()
		fmt.Println("  To set up email forwarding:")
		fmt.Println("    finwipe setup-forward")
		fmt.Println()
		fmt.Println("  Then forward emails to your FinWipe address.")
		fmt.Println("  Or use: finwipe check-inbox --import ./emails/")
		fmt.Println()
		fmt.Println("  The next time you use any FinWipe command, it will also")
		fmt.Println("  automatically check ~/.finwipe/forwarded/ for new emails.")
		return nil
	}

	fmt.Printf("  ✅ Total: %d emails to parse\n\n", len(allEmailText))

	// Parse emails
	matches := parseEmailsForFIs(allEmailText, entities)

	if len(matches) == 0 {
		fmt.Println("  No registered FIs found in forwarded emails.")
		fmt.Println("  Try forwarding bank alerts and loan confirmation emails.")
		return nil
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  📊 DISCOVERED: %d financial institutions\n", len(matches))
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	for i, m := range matches {
		if i >= 25 {
			fmt.Printf("  ... and %d more\n", len(matches)-i)
			break
		}
		icon := "💳"
		if m.Entity.Category == nbfc.CatBANK {
			icon = "🏛️"
		} else if m.Entity.Category == nbfc.CatHFC {
			icon = "🏠"
		}
		fmt.Printf("  %2d. %s %-28s [%d emails]\n", i+1, icon,
			truncate(m.Entity.Name, 28), m.Count)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("  🔍 DRY RUN — No requests created")
		fmt.Printf("  Run: finwipe check-inbox --dry-run=false to create requests\n\n")
		return nil
	}

	// Create deletion requests
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	created := 0
	skipped := 0
	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	gen := letter.New(letterDir)

	for _, m := range matches {
		if m.Entity.GrievanceEmail == "" {
			skipped++
			continue
		}

		// Check if request already exists
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

		req, err := hist.CreateRequest(m.Entity.ID, m.Entity.Name,
			history.ChannelEmail,
			m.Entity.GrievanceEmail,
			profile.Profile.Email, profile.Profile.Name)
		if err != nil {
			fmt.Printf("  ⚠️  %-28s %v\n", m.Entity.Name, err)
			continue
		}

		gen.Generate(req.RequestID, m.Entity.Name, m.Entity.GrievanceEmail,
			profile.Profile, letter.DefaultDeletionCategories)

		fmt.Printf("  ✅ %-28s %s\n", m.Entity.Name, req.RequestID)
		created++
	}

	// Update last check timestamp
	lastCheckPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "inbox_lastcheck")
	os.WriteFile(lastCheckPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	fmt.Println()
	if created > 0 {
		fmt.Printf("  ✅ Created: %d deletion requests\n", created)
	}
	if skipped > 0 {
		fmt.Printf("  ⏭️  Skipped (already exists): %d\n", skipped)
	}
	fmt.Println()
	fmt.Println("  Next: finwipe send              # Dispatch to FIs")
	fmt.Println("       finwipe track --all       # Monitor acknowledgments")

	return nil
}

// generateInboxID creates a privacy-preserving anonymous inbox ID
func generateInboxID(email string) string {
	// One-way hash — FinWipe can't reverse-engineer your email from the address
	h := sha256.Sum256([]byte(email + "finwipe-salt-v1"))
	hash := hex.EncodeToString(h[:8])
	// Human-readable prefix from domain
	parts := strings.Split(email, "@")
	prefix := "u"
	if len(parts) > 0 && len(parts[0]) > 2 {
		// First 2 chars of email prefix
		for _, c := range parts[0][:2] {
			if c >= 'a' && c <= 'z' {
				prefix = prefix + string(c)
			}
		}
	}
	return prefix + hash[:10]
}

// emailMatch holds a discovered FI from emails
type emailMatch struct {
	Entity *nbfc.Entity
	Count  int
}

// parseEmailsForFIs extracts FI names from email text
func parseEmailsForFIs(emails []string, entities []nbfc.Entity) []emailMatch {
	seen := make(map[string]*emailMatch)

	// Build entity lookup
	entityByName := make(map[string]nbfc.Entity)
	entityByShort := make(map[string]nbfc.Entity)
	for i := range entities {
		e := &entities[i]
		entityByName[strings.ToLower(e.Name)] = *e
		if e.ShortName != "" {
			entityByShort[strings.ToLower(e.ShortName)] = *e
		}
	}

	// All FI name variants to search for
	fiKeywords := []string{
		// Full names (check first)
		"HDFC Bank", "ICICI Bank", "Axis Bank", "Kotak Mahindra Bank",
		"State Bank of India", "IndusInd Bank", "Yes Bank", "IDBI Bank",
		"Bank of Baroda", "Punjab National Bank", "Canara Bank",
		"Union Bank of India", "Federal Bank", "RBL Bank", "Bandhan Bank",
		"Bank of India", "Central Bank of India", "Indian Bank",
		"South Indian Bank", "Karur Vysya Bank", "City Union Bank",
		// NBFCs
		"Bajaj Finserv", "Bajaj Finance", "Tata Capital", "Tata Motors Finance",
		"Aditya Birla Capital", "Aditya Birla Finance",
		"L&T Finance", "L&T Housing Finance",
		"Muthoot Finance", "Muthoot Gold",
		"Cholamandalam Investment", "Cholamandalam Finance",
		"HDB Financial Services", "HDB Finance",
		"Kisht Consumer Finance", "KreditBee", "Stashfin",
		"Rupeek", "Navi Finserv", "OfBusiness", "EarlySalary",
		"Slice", "Uni", "Moneyview", "Lazee", "Moneymint",
		"Capital Float", "Kissht", "Kisht",
		// Fintech
		"PhonePe", "Paytm", "Razorpay", "CRED", "CRED Club",
		"Paisabazaar", "BankBazaar", "IndMoney", "Indmoney",
		"Groww", "Zerodha", "Upstox", "Angel One", "Dhan",
		"PolicyBazaar", "PolicyBazaar",
		"Amazon Pay", "Google Pay", "BHIM UPI",
		// Insurance
		"LIC", "HDFC Life", "SBI Life", "ICICI Prudential Life",
		"Bajaj Allianz Life", "Bajaj Allianz General",
		"TATA AIA Life", "Tata AIA", "Max Life", "Star Health",
		"Niva Bupa", "Reliance General", "Go Digit", "Digit Insurance",
		// UPI handles
		"PhonePe", "GPay", "Paytm Payments Bank",
		"Amazon Pay", "BHIM",
		// NBFC brands
		"Fino Payments Bank", "Paytm Payments Bank",
		"Finvo", "Finnitize",
	}

	for _, emailText := range emails {
		upper := strings.ToUpper(emailText)

		for _, keyword := range fiKeywords {
			if strings.Contains(upper, strings.ToUpper(keyword)) {
				// Try full name match first
				lower := strings.ToLower(keyword)
				if entity, ok := entityByName[lower]; ok {
					if _, exists := seen[entity.ID]; !exists {
						seen[entity.ID] = &emailMatch{Entity: &entity, Count: 1}
					} else {
						seen[entity.ID].Count++
					}
				} else if entity, ok := entityByShort[lower]; ok {
					if _, exists := seen[entity.ID]; !exists {
						seen[entity.ID] = &emailMatch{Entity: &entity, Count: 1}
					} else {
						seen[entity.ID].Count++
					}
				}
			}
		}
	}

	// Deduplicate and convert to slice
	var results []emailMatch
	for _, m := range seen {
		results = append(results, *m)
	}

	// Sort by count (most mentioned first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Count > results[i].Count {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// collectEmailsFromDir reads all email files from a directory
func collectEmailsFromDir(dir string) []string {
	var emails []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return emails
	}
	for _, e := range entries {
		if e.IsDir() {
			emails = append(emails, collectEmailsFromDir(filepath.Join(dir, e.Name()))...)
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".eml" || ext == ".mbox" || ext == ".txt" || ext == ".mail" {
			if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				emails = append(emails, string(data))
			}
		}
	}
	return emails
}

// collectEmailsFromFile reads emails from a file (MBOX or plain text)
func collectEmailsFromFile(path string) []string {
	var emails []string
	data, err := os.ReadFile(path)
	if err != nil {
		return emails
	}
	content := string(data)

	// MBOX format: starts with "From " lines
	if strings.Contains(content, "From ") && strings.Contains(content, "@") {
		// Split by "From " lines
		parts := strings.Split(content, "\nFrom ")
		for _, part := range parts {
			if strings.Contains(part, "@") && len(part) > 50 {
				emails = append(emails, part)
			}
		}
	} else if strings.Contains(content, "@") {
		// Single email
		emails = append(emails, content)
	}

	return emails
}
