package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

// scrapeCmd handles both: finwipe discover --url and finwipe scrape
var scrapeCmd = &cobra.Command{
	Use:   "scrape [privacy policy URL]",
	Short: "Scrape grievance officer email from privacy policy URL",
	Long: `Extract grievance officer email from an NBFC's privacy policy page.

This command:
1. Fetches the privacy policy page
2. Searches for "Grievance Officer" / "DPO" / "Data Protection" sections
3. Extracts the email address
4. Optionally generates a deletion request letter

Example:
  finwipe scrape https://www.bajajfinserv.in/privacy-policy
  finwipe scrape https://www.cred.club/privacy --create-request`,
	RunE: runScrape,
}

var (
	scrapeCreateRequest bool
	scrapeOutputDir   string
	scrapeLegalBasis  string
)

func init() {
	scrapeCmd.Flags().BoolVar(&scrapeCreateRequest, "create-request",
		false, "Create deletion request after scraping")
	scrapeCmd.Flags().StringVar(&scrapeOutputDir, "output", "",
		"Output directory for letter PDF (default: ~/.finwipe/letters/)")
	scrapeCmd.Flags().StringVar(&scrapeLegalBasis, "legal-basis", "both",
		"Legal basis: dpdp, rbi, both")
}

func runScrape(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("URL required: finwipe scrape <privacy-policy-url>")
	}

	targetURL := args[0]
	fmt.Printf("🔍 Scraping: %s\n\n", targetURL)

	// Fetch the page
	resp, err := http.Get(targetURL)
	if err != nil {
		return fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, targetURL)
	}

	// Read content (limit to 5MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	content := string(body)

	// Extract emails
	emails := extractEmailsFromText(content)

	// Find grievance-related emails
	var grievanceEmail, privacyEmail string

	for _, email := range emails {
		lower := strings.ToLower(email)
		if strings.Contains(lower, "grievance") || strings.Contains(lower, "grive") {
			if grievanceEmail == "" {
				grievanceEmail = email
			}
		}
		if strings.Contains(lower, "dpo") || strings.Contains(lower, "dataprotection") || strings.Contains(lower, "privacy") {
			if privacyEmail == "" {
				privacyEmail = email
			}
		}
	}

	// Also look for DPO mentions without email
	dpoMention := extractDPOMention(content)

	// Parse domain for entity name
	parsedURL, _ := url.Parse(targetURL)
	domain := parsedURL.Host
	entityName := domainToEntityName(domain)

	fmt.Printf("📋 Entity: %s\n", entityName)
	fmt.Printf("🌐 Domain: %s\n\n", domain)

	if grievanceEmail != "" {
		fmt.Printf("✅ Grievance Officer Email: %s\n", grievanceEmail)
	} else {
		fmt.Printf("❌ Grievance Officer Email: NOT FOUND\n")
	}

	if privacyEmail != "" && privacyEmail != grievanceEmail {
		fmt.Printf("🔐 Privacy/DPO Email: %s\n", privacyEmail)
	}

	if dpoMention != "" {
		fmt.Printf("📝 DPO Mention: %s\n", dpoMention)
	}

	if len(emails) > 0 {
		fmt.Printf("\n📧 All emails found on page (%d):\n", len(emails))
		shown := emails
		if len(shown) > 10 {
			shown = emails[:10]
			fmt.Printf("  (showing 10 of %d)\n", len(emails))
		}
		for _, e := range shown {
			flag := ""
			if strings.ToLower(e) == strings.ToLower(grievanceEmail) {
				flag = " ← Grievance Officer"
			}
			if strings.ToLower(e) == strings.ToLower(privacyEmail) {
				flag = " ← Privacy/DPO"
			}
			fmt.Printf("  • %s%s\n", e, flag)
		}
	}

	// Try common privacy policy URLs if none found
	if grievanceEmail == "" {
		fmt.Printf("\n🔍 Trying common privacy policy URLs...\n")
		alternateURLs := tryAlternatePrivacyURLs(domain)
		for _, altURL := range alternateURLs {
			fmt.Printf("  Trying: %s\n", altURL)
			email := tryFetchGrievanceEmail(altURL)
			if email != "" {
				fmt.Printf("  ✅ Found: %s\n", email)
				grievanceEmail = email
				break
			}
		}
	}

	if grievanceEmail == "" {
		fmt.Printf("\n⚠️  Could not find grievance officer email.\n")
		fmt.Printf("   Try manually searching for:\n")
		fmt.Printf("   - %s/privacy-policy\n", domain)
		fmt.Printf("   - %s/privacy\n", domain)
		fmt.Printf("   - %s/legal\n", domain)
	}

	// Create deletion request if requested
	if scrapeCreateRequest && grievanceEmail != "" {
		fmt.Printf("\n📄 Generating deletion request...\n")

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w (run finwipe init)", err)
		}
		if cfg.Profile.Name == "" {
			return fmt.Errorf("profile incomplete: run finwipe init first")
		}

		// Find or create entity
		id := domainToID(domain)
		entity := &nbfc.Entity{
			ID:             id,
			Name:           entityName,
			ShortName:      entityName,
			Category:       guessCategory(entityName),
			GrievanceEmail: grievanceEmail,
			Active:         true,
		}

		// Parse legal basis
		var lb letter.LegalBasis
		switch scrapeLegalBasis {
		case "dpdp":
			lb = letter.LegalBasisDPDP
		case "rbi":
			lb = letter.LegalBasisRBI
		default:
			lb = letter.LegalBasisBoth
		}

		// Generate letter
		outDir := scrapeOutputDir
		if outDir == "" {
			home, _ := os.UserHomeDir()
			outDir = filepath.Join(home, ".finwipe", "letters")
		}
		os.MkdirAll(outDir, 0755)

		gen := letter.New(outDir)
		letterPath, err := gen.Generate(
			fmt.Sprintf("DPR-%d-SCRAPE", 2026),
			entity.Name,
			entity.GrievanceEmail,
			cfg.Profile,
			letter.DefaultDeletionCategories,
			lb,
		)
		if err != nil {
			return fmt.Errorf("generate letter: %w", err)
		}

		fmt.Printf("✅ Letter generated: %s\n", letterPath)
	}

	return nil
}

// extractEmailsFromText extracts all email addresses from text
func extractEmailsFromText(text string) []string {
	re := regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	matches := re.FindAllString(text, -1)

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		lower := strings.ToLower(m)
		// Filter obvious non-contact emails
		if strings.Contains(lower, "example") ||
		   strings.Contains(lower, "test") ||
		   strings.Contains(lower, "noreply") ||
		   strings.Contains(lower, "no-reply") ||
		   strings.Contains(lower, "support@linkedin") { // LinkedIn tracking
			continue
		}
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, m)
		}
	}
	return unique
}

// extractDPOMention finds paragraphs mentioning DPO but without email
func extractDPOMention(content string) string {
	patterns := []string{
		`[Dd]ata\s+[Pp]rotection\s+[Oo]fficer[^.<]{0,100}`,
		`[Dd]ata\s+[Pp]rotection\s+[Oo]fficer[^.<\n]{0,200}`,
		`[Dd]ata\s+[Pp]rivacy\s+[Oo]fficer[^.<]{0,100}`,
		`[Gg]rievance\s+[Oo]fficer[^.<]{0,100}`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match := re.FindString(content)
		if match != "" {
			// Clean it up
			match = strings.TrimSpace(match)
			match = regexp.MustCompile(`\s+`).ReplaceAllString(match, " ")
			if len(match) > 150 {
				match = match[:150] + "..."
			}
			return match
		}
	}
	return ""
}

// tryAlternatePrivacyURLs generates likely privacy policy URLs
func tryAlternatePrivacyURLs(domain string) []string {
	paths := []string{
		"/privacy-policy",
		"/privacy",
		"/legal/privacy-policy",
		"/legal/privacy",
		"/legal",
		"/grievance",
		"/contact-us",
		"/support/privacy",
	}

	var urls []string
	for _, path := range paths {
		urls = append(urls, "https://"+domain+path)
	}
	return urls
}

// tryFetchGrievanceEmail tries to fetch a URL and extract grievance email
func tryFetchGrievanceEmail(targetURL string) string {
	resp, err := http.Get(targetURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return ""
	}

	content := string(body)
	emails := extractEmailsFromText(content)

	for _, email := range emails {
		lower := strings.ToLower(email)
		if strings.Contains(lower, "grievance") ||
		   strings.Contains(lower, "dpo") ||
		   strings.Contains(lower, "dataprotection") ||
		   strings.Contains(lower, "privacy") {
			return email
		}
	}

	return ""
}

// domainToEntityName converts a domain to a readable entity name
func domainToEntityName(domain string) string {
	// Remove common prefixes and TLD
	name := domain
	name = strings.TrimPrefix(name, "www.")
	name = strings.TrimSuffix(name, ".com")
	name = strings.TrimSuffix(name, ".in")
	name = strings.TrimSuffix(name, ".co.in")
	name = strings.TrimSuffix(name, ".org")

	// Convert dashes and underscores to spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Title case
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// domainToID converts a domain to URL-safe ID
func domainToID(domain string) string {
	id := strings.ToLower(domain)
	id = strings.TrimPrefix(id, "www.")
	id = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(id, "-")
	id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
	return id
}

// guessCategory tries to guess the entity category from name
func guessCategory(name string) nbfc.Category {
	lower := strings.ToLower(name)

	bankWords := []string{"bank", "sbi", "hdfc", "icici", "axis", "kotak", "pnb", "union bank"}
	for _, w := range bankWords {
		if strings.Contains(lower, w) {
			return nbfc.CatBANK
		}
	}

	insWords := []string{"insurance", "life", "bajaj", "hdfc ergo", "tata aia", "max life", "reliance"}
	for _, w := range insWords {
		if strings.Contains(lower, w) {
			return nbfc.CatFINTECH
		}
	}

	lendWords := []string{"lending", "loan", "credit", "finance", "kap给自己", "cred", "slice", "uni"}
	for _, w := range lendWords {
		if strings.Contains(lower, w) {
			return nbfc.CatFINTECH
		}
	}

	payWords := []string{"pay", "upi", "phonepe", "razorpay", "paytm", "mobikwik", "amazon"}
	for _, w := range payWords {
		if strings.Contains(lower, w) {
			return nbfc.CatFINTECH
		}
	}

	return nbfc.CatFINTECH
}

// fetchWithCmdHeadless uses cmd-headless to fetch JS-heavy pages
func fetchWithCmdHeadless(targetURL string) (string, error) {
	// This would use the sota-browser MCP to fetch JS-heavy pages
	// For now, fall back to simple HTTP
	return "", fmt.Errorf("use direct fetch")
}

// parseHTMLForGrievanceEmail parses HTML content for grievance email patterns
func parseHTMLForGrievanceEmail(html string) string {
	// Remove script and style content
	html = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(html, "")

	// Look for grievance-related text
	patterns := []string{
		`(?:Grievance|Data\s+Protection|Privacy)\s*(?:Officer|Contact|Email)[^<>\n]{0,50}([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})`,
		`(?:Grievance|Data\s+Protection|Privacy)\s*(?:Officer|Contact)[^<\n]{0,200}?([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 1 {
				email := strings.TrimSpace(match[1])
				if validateEmail(email) {
					return email
				}
			}
		}
	}

	return ""
}

// validateEmail does basic email format validation
func validateEmail(email string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`, email)
	return matched
}
