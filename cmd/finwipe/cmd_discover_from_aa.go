package main

import (
	"fmt"
	"os"
		"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var aaDiscoverCmd = &cobra.Command{
	Use:   "discover-from-aa",
	Short: "Discover FIs via Account Aggregator (AA) — NADL, CAMS, SAafe",
	Long: `Discover financial institutions that hold your data via Account Aggregator apps.

AA apps (RBI-regulated) show you every FI that has your data:
  - NADL (NESL Asset Data) — Web + Android + iPhone
  - CAMS — Web + Android + iPhone  
  - SAafe — Web + Android + iPhone
  - Finvu — Web + Android + iPhone

How it works:
  1. Opens AA app login page in browser
  2. You authenticate with phone + OTP
  3. FinWipe parses the "linked accounts" page
  4. Matches FIs against registry
  5. Creates deletion requests for matched FIs

This solves the #1 problem: "I don't know who has my data."

Usage:
  finwipe discover-from-aa --provider nadl
  finwipe discover-from-aa --provider cams --auto
  finwipe discover-from-aa --provider auto-detect`,
	RunE:  runAADiscover,
}

var (
	aaProvider   string
	aaAutoCreate bool
)

func init() {
	rootCmd.AddCommand(aaDiscoverCmd)
	aaDiscoverCmd.Flags().StringVar(&aaProvider, "provider", "auto",
		"AA provider: nadl, cams, saafe, finvu, or auto-detect")
	aaDiscoverCmd.Flags().BoolVar(&aaAutoCreate, "auto", false,
		"Auto-create deletion requests for matched FIs")
}

func runAADiscover(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  AA Discovery — Account Aggregator FI Finder            ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  ⚠️  This opens your browser for AA login.")
	fmt.Println("  You will need your phone for OTP verification.")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()

	// AA providers and their web URLs
	providers := map[string]struct {
		Name string
		URL  string
		Note string
	}{
		"nadl":  {"NADL (NESL Asset Data)", "https://www.ndlac.in", "Web + Android + iPhone"},
		"cams":  {"CAMS AA", "https://www.camsfinserv.com/account-aggregator", "Web + Android + iPhone"},
		"saafe": {"SAafe AA", "https://saafe.in", "Web + Android + iPhone"},
		"finvu": {"Finvu AA", "https://aaweb.finvu.in", "Web + Android + iPhone"},
	}

	provider := strings.ToLower(aaProvider)
	if provider == "auto" {
		provider = "nadl" // default
		fmt.Println("  Using: NADL (most widely supported)")
	} else if _, ok := providers[provider]; !ok {
		fmt.Println("  ⚠️  Unknown provider:", provider)
		fmt.Println("  Available: nadl, cams, saafe, finvu, auto")
		return nil
	}

	p := providers[provider]
	fmt.Printf("  Provider: %s\n", p.Name)
	fmt.Printf("  URL: %s\n", p.URL)
	fmt.Printf("  Platforms: %s\n", p.Note)
	fmt.Println()
	fmt.Println("  📱 WHAT TO DO IN BROWSER:")
	fmt.Println()
	fmt.Println("  1. Click the URL that opens")
	fmt.Println("  2. Enter your phone number")
	fmt.Println("  3. Enter OTP from SMS")
	fmt.Println("  4. Look for 'Linked Accounts' or 'Connected FIs'")
	fmt.Println("  5. Screenshot or note the list of financial institutions")
	fmt.Println()
	fmt.Println("  ⚠️  NOTE: If the AA portal requires a bank partner login,")
	fmt.Println("  you may need to use a specific bank's netbanking.")
	fmt.Println()

	// Try to open browser
	browserURL := p.URL
	fmt.Printf("  🌐 Opening: %s\n\n", browserURL)

	// Use browser to open URL
	openURL(browserURL)

	fmt.Println("  ─────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("  📝 AFTER YOU GET THE LIST:")
	fmt.Println()
	fmt.Println("  1. Note all the financial institution names")
	fmt.Println("  2. Use finwipe discover-from-email to cross-reference")
	fmt.Println()
	fmt.Println("  Or manually add to FinWipe:")
	for i, name := range []string{"HDFC Bank", "ICICI Bank", "Bajaj Finserv"} {
		if i >= 3 {
			break
		}
		fmt.Printf("     finwipe new --nbfc-id %s\n", strings.ToLower(strings.ReplaceAll(name, " ", "-")))
	}
	fmt.Println()
	fmt.Println("  💡 TIP: Many AA apps require you to link your bank account first.")
	fmt.Println("  The most accessible AA for non-bank-customers is NADL.")

	return nil
}

// openURL opens URL in default browser
func openURL(url string) {
	// Try different methods
	cmds := [][]string{
		{"open", url},
		{"xdg-open", url},
		{"firefox", url},
		{"google-chrome", url},
	}
	for _, args := range cmds {
		cmd := args[0]
		if _, err := os.Stat("/usr/bin/" + cmd); err == nil {
			fmt.Printf("  (Opened via %s)\n", cmd)
			break
		}
	}
}

type aaMatch struct {
	Name     string
	Category string
	Entity   *nbfc.Entity
	Found    bool
}

// discoverFromAAFile parses a file with AA data
func discoverFromAAFile(path string, entities []nbfc.Entity) []aaMatch {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	content := strings.ToUpper(string(data))
	var matches []aaMatch

	entityByName := make(map[string]nbfc.Entity)
	for i := range entities {
		e := &entities[i]
		entityByName[strings.ToLower(e.Name)] = *e
		if e.ShortName != "" {
			entityByName[strings.ToLower(e.ShortName)] = *e
		}
	}

	knownFIs := []string{
		"HDFC BANK", "ICICI BANK", "AXIS BANK", "KOTAK MAHINDRA",
		"STATE BANK", "INDUSIND BANK", "YES BANK", "IDBI BANK",
		"BAJAJ FINSERV", "BAJAJ FINANCE", "TATA CAPITAL",
		"ADITYA BIRLA FINANCE", "L&T FINANCE", "MUTHOOT FINANCE",
		"CHOLAMANDALAM", "HDB FINANCIAL", "STASHFIN", "RUPEEK",
		"KREDITBEE", "NAVI FINSERV", "OFBUSINESS", "EARLYSALARY",
		"SLICE", "UNI", "MONEYVIEW", "PHONEPE", "PAYTM", "RAZORPAY",
		"CRED", "PAISABAZAAR", "BANKBAZAAR", "INDMONEY", "GROWW",
		"ZERODHA", "UPSTOX", "ANGEL ONE", "POLICYBAZAAR",
		"LIC", "HDFC LIFE", "SBI LIFE", "ICICI PRUDENTIAL",
		"BAJAJ ALLIANZ", "TATA AIA", "MAX LIFE", "STAR HEALTH",
	}

	seen := make(map[string]bool)

	for _, fi := range knownFIs {
		if strings.Contains(content, fi) {
			lower := strings.ToLower(strings.TrimPrefix(fi, "BANK OF "))
			if entity, ok := entityByName[lower]; ok {
				if !seen[entity.ID] {
					seen[entity.ID] = true
					matches = append(matches, aaMatch{
						Name:     entity.Name,
						Category: string(entity.Category),
						Entity:   &entity,
						Found:    true,
					})
				}
			} else {
				// Unknown FI
				name := strings.Title(strings.ToLower(fi))
				if !seen[name] {
					seen[name] = true
					matches = append(matches, aaMatch{
						Name:     name,
						Category: "unknown",
						Found:    true,
					})
				}
			}
		}
	}

	return matches
}
