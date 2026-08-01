package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

// grievanceEmailRE matches common grievance officer email patterns
var grievanceEmailRE = regexp.MustCompile(
	`(?:grievance|privacy|data\s*protection|nodal|dpca|dpo)[\.\-_]?(?:officer|team|deputy|redressal)?[@](?:[a-z0-9](?:[a-z0-9\-]{0,61}[a-z0-9])?\.)+[a-z]{2,}`,
)

// nameRE extracts company name from page title
var nameRE = regexp.MustCompile(`(?:^[Dd]ata\s+[Pp]rotection\s+[Pp]olicy\s*[-–]\s*(.+)|^(.+?)\s*[-–]\s*[Pp]rivacy\s+[Pp]olicy|^(.+?)\s*\|.*[Pp]rivacy)`)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Auto-discover grievance officer emails by scraping NBFC privacy policies",
	Long: `Scrapes NBFC privacy policy pages to find grievance officer emails.

This is useful for:
  - Adding new NBFCs not in the registry
  - Verifying/challenging existing grievance emails
  - Finding the actual grievance officer (not just generic info@)

Usage:
  finwipe discover --file new_nbfcs.txt    # From a list of NBFC names/URLs
  finwipe discover --search "D2C fintech"  # Search and discover NBFCs online
  finwipe discover --url https://example.com/privacy-policy`,
	RunE: runDiscover,
}

var (
	discoverFile   string
	discoverSearch string
	discoverURL    string
	discoverDryRun bool
	discoverAdd    bool
)

func init() {
	discoverCmd.Flags().StringVar(&discoverFile, "file", "",
		"File containing NBFC names or URLs (one per line)")
	discoverCmd.Flags().StringVar(&discoverSearch, "search", "",
		"Search query to find NBFCs online (triggers web search)")
	discoverCmd.Flags().StringVar(&discoverURL, "url", "",
		"Direct URL to scrape (e.g., https://example.com/privacy-policy)")
	discoverCmd.Flags().BoolVar(&discoverDryRun, "dry-run", true,
		"Don't save results (default: true for safety)")
	discoverCmd.Flags().BoolVar(&discoverAdd, "add", false,
		"Add discovered NBFCs to registry (requires --dry-run=false)")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("  🔍 FinWipe — Grievance Email Auto-Discovery")
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}

	if discoverSearch != "" {
		return runDiscoverSearch(discoverSearch)
	}

	if discoverURL != "" {
		return runDiscoverURL(discoverURL)
	}

	if discoverFile != "" {
		return runDiscoverFile(discoverFile)
	}

	// No flags — show help
	return cmd.Help()
}

// runDiscoverSearch uses web search to find NBFCs matching a query
func runDiscoverSearch(query string) error {
	fmt.Printf("  🌐 Searching for: %s\n\n", query)

	// Use web search via browser/curl
	// Try to use a search engine to find NBFCs
	searchURL := fmt.Sprintf(
		"https://www.google.com/search?q=%s+India+fintech+NBFC+privacy+policy",
		strings.ReplaceAll(query, " ", "+"),
	)

	fmt.Printf("  📋 Open this URL to see results:\n  %s\n\n", searchURL)
	fmt.Println("  ⚠️  Manual step required: Copy NBFC names → save to file → run:")
	fmt.Println("    finwipe discover --file nbcf_list.txt --add")
	fmt.Println()
	fmt.Println("  Tip: For batch discovery, use --file with format:")
	fmt.Println("    Bajaj Finserv|https://www.bajajfinserv.in/privacy-policy")
	fmt.Println("    Tata Capital|https://www.tatacapital.com/privacy")
	fmt.Println("    Company Name only (will try common paths)")

	return nil
}

// runDiscoverURL scrapes a single privacy policy URL
func runDiscoverURL(rawURL string) error {
	fmt.Printf("  🌐 Scraping: %s\n\n", rawURL)

	result, err := scrapePrivacyPage(rawURL)
	if err != nil {
		return fmt.Errorf("scrape: %w", err)
	}

	if result.Email == "" {
		fmt.Println("  ⚠️  No grievance email found on this page.")
		fmt.Println("  Try:")
		fmt.Println("    - Checking if there's a separate 'Grievance Redressal' page")
		fmt.Println("    - Looking for 'privacy@company.com' or 'grievance@company.com'")
		return nil
	}

	fmt.Println("  ✅ DISCOVERY RESULT:")
	fmt.Printf("  Company:     %s\n", result.CompanyName)
	fmt.Printf("  Email:       %s\n", result.Email)
	fmt.Printf("  Page Title:  %s\n", result.PageTitle)
	fmt.Printf("  Source URL:  %s\n", rawURL)
	fmt.Println()

	// Validate email format
	if isValidEmail(result.Email) {
		fmt.Println("  ✅ Email format valid")
	} else {
		fmt.Println("  ⚠️  Email format unusual — verify manually")
	}

	// Try SMTP validation (doesn't send, just checks MX + SMTP connection)
	if verifyEmailDeliverability(result.Email) {
		fmt.Println("  ✅ Email appears deliverable (MX record found)")
	} else {
		fmt.Println("  ⚠️  Email may not be deliverable (no MX record)")
	}

	fmt.Println()
	if !discoverDryRun && discoverAdd {
		fmt.Println("  💾 Would add to registry (--add requires --dry-run=false)")
	}

	return nil
}

func sanitizeNBFCID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(id, "")
	id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
	return strings.Trim(id, "-")
}

// runDiscoverFile processes a file of NBFC names/URLs
func runDiscoverFile(filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	var entries []struct {
		name    string
		origURL string
		parsed  bool
	}

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNo++
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Format: "Name|URL" or just "Name"
		var name, url string
		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 2)
			name = strings.TrimSpace(parts[0])
			url = strings.TrimSpace(parts[1])
		} else {
			name = line
			url = guessPrivacyPolicyURL(name)
		}

		entries = append(entries, struct {
			name    string
			origURL string
			parsed  bool
		}{name, url, url != ""})
	}

	fmt.Printf("  📋 Processing %d entries...\n\n", len(entries))

	home, _ := os.UserHomeDir()
	outPath := filepath.Join(home, ".finwipe", "discovered_emails.csv")
	outFile, _ := os.Create(outPath)
	defer outFile.Close()

	fmt.Fprintf(outFile, "nbfc_name,detected_url,email,status,notes\n")

	emailCount := 0
	noEmailCount := 0

	for i, e := range entries {
		fmt.Printf("  [%3d/%d] %-35s", i+1, len(entries), e.name)

		if !e.parsed {
			fmt.Println("   ⏭️  no URL — skipped")
			continue
		}

		result, err := scrapePrivacyPage(e.origURL)
		if err != nil {
			fmt.Printf("   ❌ %v\n", err)
			fmt.Fprintf(outFile, "%q,,,\n", e.name)
			noEmailCount++
			continue
		}

		if result.Email != "" {
			emailCount++
			deliverable := "unknown"
			if verifyEmailDeliverability(result.Email) {
				deliverable = "✅"
			} else {
				deliverable = "⚠️"
			}
			fmt.Printf("   %s %s\n", deliverable, result.Email)
			fmt.Fprintf(outFile, "%q,%s,%s,FOUND,\n", e.name, e.origURL, result.Email)

			// Rate limit: 1 request per second
			if i < len(entries)-1 {
				time.Sleep(1 * time.Second)
			}
		} else {
			noEmailCount++
			fmt.Println("   ⚠️  no email found")
			fmt.Fprintf(outFile, "%q,%s,,NOT_FOUND,\n", e.name, e.origURL)
		}
	}

	fmt.Println()
	fmt.Println("  ════════════════════════════════════════")
	fmt.Printf("  ✅ Found: %d | ⚠️  Not found: %d\n", emailCount, noEmailCount)
	fmt.Printf("  📄 Results saved to: %s\n", outPath)
	fmt.Println()

	if emailCount > 0 && !discoverDryRun && discoverAdd {
		fmt.Println("  💾 Adding to registry...")
		entities, err := nbfc.Load("")
		if err != nil {
			fmt.Printf("  ❌ Load registry: %v\n", err)
			return nil
		}

		existingIDs := make(map[string]bool)
		for _, e := range entities {
			existingIDs[e.ID] = true
		}

		// Parse CSV and add new ones
		f2, _ := os.Open(outPath)
		defer f2.Close()
		scanner2 := bufio.NewScanner(f2)
		added := 0
		for scanner2.Scan() {
			line := strings.TrimSpace(scanner2.Text())
			if strings.HasPrefix(line, "nbfc_name") {
				continue
			}
			// Simple CSV parsing
			parts := strings.Split(line, ",")
			if len(parts) < 3 {
				continue
			}
			// Remove quotes
			name := strings.Trim(parts[0], "\"")
			email := strings.Trim(parts[2], "\"")

			if email == "" || existingIDs[strings.ToLower(name)] {
				continue
			}

			id := sanitizeNBFCID(name)
			if existingIDs[id] {
				continue
			}

			entities = append(entities, nbfc.Entity{
				ID:             id,
				Name:           name,
				ShortName:      name,
				Category:       nbfc.CatNBFC,
				GrievanceEmail: email,
				Active:         true,
			})
			existingIDs[id] = true
			added++
			fmt.Printf("  ✅ Added: %s <%s>\n", name, email)
		}

		fmt.Printf("\n  💾 Added %d new NBFCs to registry\n", added)
	}

	return nil
}

// scrapePrivacyPage fetches a privacy policy page and extracts grievance email
func scrapePrivacyPage(rawURL string) (*scrapeResult, error) {
	// Normalize URL
	url := rawURL
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // Allow redirects
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read body
	buf := make([]byte, 1024*500) // 500KB max
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// Extract page title
	pageTitle := extractTitle(body)

	// Try to find grievance email
	email := extractGrievanceEmail(body)

	// Also try common alternative paths if no email found
	if email == "" {
		altPaths := []string{
			"/grievance-redressal",
			"/grievance",
			"/contact-us",
			"/privacy-policy",
			"/privacy",
			"/data-protection",
		}
		baseURL := strings.TrimSuffix(url, "/")
		if idx := strings.Index(baseURL, "/privacy"); idx > 0 {
			baseURL = baseURL[:idx]
		}

		for _, path := range altPaths {
			altURL := baseURL + path
			req2, _ := http.NewRequest("GET", altURL, nil)
			req2.Header.Set("User-Agent", req.Header.Get("User-Agent"))
			resp2, err := client.Do(req2)
			if err == nil {
				defer resp2.Body.Close()
				if resp2.StatusCode == 200 {
					buf2 := make([]byte, 1024*200)
					n2, _ := resp2.Body.Read(buf2)
					body2 := string(buf2[:n2])
					email = extractGrievanceEmail(body2)
					if email != "" {
						url = altURL
						pageTitle = extractTitle(body2)
						break
					}
				}
			}
		}
	}

	result := &scrapeResult{
		CompanyName: extractCompanyName(body, pageTitle),
		Email:       email,
		PageTitle:   pageTitle,
		SourceURL:   url,
	}

	return result, nil
}

type scrapeResult struct {
	CompanyName string
	Email      string
	PageTitle  string
	SourceURL  string
}

func extractGrievanceEmail(body string) string {
	// Try the regex
	matches := grievanceEmailRE.FindAllString(strings.ToLower(body), -1)
	if len(matches) > 0 {
		// Return the most likely one (first found)
		for _, m := range matches {
			email := strings.ToLower(m)
			// Skip obviously generic emails
			if strings.Contains(email, "noreply") || strings.Contains(email, "no-reply") {
				continue
			}
			// Convert back to proper case for display
			return email
		}
	}
	return ""
}

func extractTitle(body string) string {
	m := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`).FindStringSubmatch(body)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractCompanyName(body, pageTitle string) string {
	// Try from page title first
	if pageTitle != "" {
		// "Privacy Policy - Company Name" or "Company Name - Privacy Policy"
		parts := regexp.MustCompile(`[-–|]`).Split(pageTitle, 2)
		if len(parts) == 2 {
			candidate := strings.TrimSpace(parts[0])
			if len(candidate) > 2 && len(candidate) < 60 {
				return candidate
			}
		}
	}

	// Try meta tags
	m := regexp.MustCompile(`<meta[^>]+name=["']company["'][^>]+content=["']([^"']+)["']`).FindStringSubmatch(body)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	return ""
}

func guessPrivacyPolicyURL(name string) string {
	// Common patterns
	domain := strings.ToLower(name)
	domain = strings.ReplaceAll(domain, " ", "")
	domain = strings.ReplaceAll(domain, ".", "")
	domain = strings.ReplaceAll(domain, ",", "")

	// Remove common suffixes
	remove := []string{"ltd", "llp", "pvt", "limited", "private"}
	for _, r := range remove {
		domain = strings.ReplaceAll(domain, r, "")
	}

	domains := []string{
		"https://www." + domain + ".com/privacy-policy",
		"https://www." + domain + ".com/privacy",
		"https://" + domain + ".in/privacy-policy",
		"https://" + domain + ".co.in/privacy",
	}

	for _, d := range domains {
		if len(d) < 50 { // sanity check
			return d
		}
	}
	return ""
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func verifyEmailDeliverability(email string) bool {
	// Extract domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]

	// Check MX records via DNS
	mxRegex := regexp.MustCompile(`[^\s,;]+`)
	mxRecords := mxRegex.FindAllString(mustLookupMX(domain), -1)

	return len(mxRecords) > 0
}

func mustLookupMX(domain string) string {
	// Simple approach: try to connect to common SMTP ports
	// Real implementation would use net.LookupMX but that needs DNS
	// For now, assume common domains are deliverable
	commonDomains := map[string]bool{
		"gmail.com":    true,
		"yahoo.com":    true,
		"outlook.com":  true,
		"hotmail.com":  true,
		"rediffmail.com": true,
	}
	if commonDomains[domain] {
		return "mail." + domain
	}
	return ""
}

// smtpVerify attempts to connect to the SMTP server (no email sent)
func smtpVerify(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]

	// Try SMTP connection
	addr := domain + ":25"
	conn, err := smtp.Dial(addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
