package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/email"
	"github.com/das-rebel/finwipe/internal/evidence"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var discoverCibilCmd = &cobra.Command{
	Use:   "discover-from-cibil [flags]",
	Short: "Parse CIBIL report (PDF/text) and auto-generate deletion requests for ALL discovered institutions",
	Long: `Extract financial institutions from a CIBIL credit report and generate deletion requests.

This solves the #1 problem: most people don't know who has their financial data.

The CIBIL report shows:
- Every loan account (active and closed)
- Every institution that queried your report
- Every credit card
- Payment history

Each institution listed = an entity that holds or has held your financial data.

Usage:
  finwipe discover-from-cibil --file CIBIL_Report.pdf
  finwipe discover-from-cibil --file report.txt --format text
  finwipe discover-from-cibil --file report.csv --format csv

Supported formats:
  - PDF from CIBIL, Paisabazaar, UMANG, or any source
  - Text export from Paisabazaar credit report
  - CSV export if available

The tool will:
  1. Parse the report and extract all institution names
  2. Cross-reference with FinWipe's NBFC registry (91 entities)
  3. Identify UNKNOWN institutions not in the registry
  4. Generate deletion requests for ALL discovered institutions
  5. Create tracked requests in FinWipe's history database
  6. Generate a CSV report of what was found`,
	RunE: runDiscoverFromCibil,
}

var (
	cibilFile    string
	cibilFormat  string // pdf, text, csv
	cibilDryRun bool
	cibilEmail  string // User's email for requests
	cibilName   string // User's name for requests
)

func init() {
	discoverCibilCmd.Flags().StringVar(&cibilFile, "file", "", "Path to CIBIL report (PDF, text, or CSV)")
	discoverCibilCmd.Flags().StringVar(&cibilFormat, "format", "auto",
		"Format: auto (detect), pdf, text, csv")
	discoverCibilCmd.Flags().BoolVar(&cibilDryRun, "dry-run", true,
		"Preview only — don't create requests (default: true)")
	discoverCibilCmd.Flags().StringVar(&cibilEmail, "email", "",
		"Your email address (required for requests)")
	discoverCibilCmd.Flags().StringVar(&cibilName, "name", "",
		"Your full name as per records (required for requests)")
	discoverCibilCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(discoverCibilCmd)
}

func runDiscoverFromCibil(cmd *cobra.Command, args []string) error {
	if cibilEmail == "" || cibilName == "" {
		// Try to load from config
		cfg, err := config.Load(cfgFile)
		if err == nil && cfg.Profile.Email != "" {
			cibilEmail = cfg.Profile.Email
			cibilName = cfg.Profile.Name
		}
		if cibilEmail == "" {
			return fmt.Errorf("--email is required (or run finwipe init to set profile)")
		}
		if cibilName == "" {
			return fmt.Errorf("--name is required (or run finwipe init to set profile)")
		}
	}

	profile := config.Profile{
		Name:  cibilName,
		Email: cibilEmail,
		Phone: "",
	}

	fmt.Println()
	fmt.Println("  🔍 FinWipe — CIBIL Discovery Engine")
	fmt.Println("  ─────────────────────────────────────────────────────────────")
	fmt.Printf("  📄 File: %s\n", cibilFile)
	fmt.Printf("  👤 Name: %s\n", cibilName)
	fmt.Printf("  📧 Email: %s\n", cibilEmail)
	fmt.Println()

	// Detect format
	format := cibilFormat
	if format == "auto" {
		format = detectFormat(cibilFile)
	}
	fmt.Printf("  🔬 Detected format: %s\n\n", format)

	// Read file
	var text string
	var err error
	switch format {
	case "pdf":
		text, err = extractTextFromPDF(cibilFile)
	case "csv":
		text, err = extractTextFromCSV(cibilFile)
	case "text":
		var data []byte
		data, err = os.ReadFile(cibilFile)
		text = string(data)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Parse institutions from CIBIL report
	fmt.Println("  📋 Parsing CIBIL report...")
	institutions, err := parseCIBILReport(text)
	if err != nil {
		return fmt.Errorf("parse CIBIL: %w", err)
	}

	fmt.Printf("  ✅ Found %d institutions in CIBIL report\n\n", len(institutions))

	// Load FinWipe NBFC registry
	fmt.Println("  📦 Loading FinWipe NBFC registry...")
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}
	fmt.Printf("  ✅ Loaded %d registered NBFCs/Banks\n\n", len(entities))

	// Cross-reference
	type match struct {
		InstitutionName string
		Entity          *nbfc.Entity
		Source          string // "registry", "unknown"
		AccountInfo     string
	}

	var matches []match
	unknownByName := make(map[string]string) // name → account info

	for _, inst := range institutions {
		if inst.Name == "" {
			continue
		}
		normalized := normalize(inst.Name)

		// Check against registry
		found := false
		for _, e := range entities {
			if normalize(e.Name) == normalized ||
				strings.Contains(normalized, normalize(e.Name)) ||
				strings.Contains(normalize(e.Name), normalized) {
				matches = append(matches, match{
					InstitutionName: inst.Name,
					Entity:          &e,
					Source:         "registry",
					AccountInfo:    inst.Account,
				})
				found = true
				break
			}

			// Also check short names
			if e.ShortName != "" {
				if strings.Contains(normalized, normalize(e.ShortName)) ||
					strings.Contains(normalize(e.ShortName), normalized) {
					matches = append(matches, match{
						InstitutionName: inst.Name,
						Entity:          &e,
						Source:         "registry",
						AccountInfo:    inst.Account,
					})
					found = true
					break
				}
			}
		}

		if !found {
			unknownByName[inst.Name] = inst.Account
		}
	}

	// Sort
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Entity.Name < matches[j].Entity.Name
	})

	// Report
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Println("  📊 DISCOVERY RESULTS")
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Println()

	fmt.Printf("  ✅ IN FINWIPE REGISTRY: %d\n", len(matches))
	fmt.Println("  ──────────────────────────────────────────────────────────")
	for i, m := range matches {
		fmt.Printf("  %2d. %-30s %s [%s]\n", i+1, truncate(m.Entity.Name, 30),
			m.Entity.GrievanceEmail, m.AccountInfo)
	}
	fmt.Println()

	if len(unknownByName) > 0 {
		fmt.Printf("  ❓ NOT IN REGISTRY (need investigation): %d\n", len(unknownByName))
		fmt.Println("  ──────────────────────────────────────────────────────────")
		i := 0
		for name, account := range unknownByName {
			i++
			if i > 20 {
				fmt.Printf("  ... and %d more unknown institutions\n", len(unknownByName)-20)
				break
			}
			fmt.Printf("  %2d. %-35s [%s]\n", i, truncate(name, 35), account)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Total institutions in CIBIL:  %d\n", len(institutions))
	fmt.Printf("  Matched in FinWipe registry:  %d\n", len(matches))
	fmt.Printf("  Unknown (need investigation): %d\n", len(unknownByName))
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Println()

	if cibilDryRun {
		fmt.Println("  🔍 DRY RUN — No requests created")
		fmt.Println("  To create requests, run with --dry-run=false")
		fmt.Println()
		fmt.Println("  Example:")
		fmt.Printf("    finwipe discover-from-cibil --file %s --email %s --name %q --dry-run=false\n",
			cibilFile, cibilEmail, cibilName)
		return nil
	}

	// Actually create requests
	fmt.Println("  🚀 Creating deletion requests...")

	cfg, _ := config.Load(cfgFile)
	hist, err := history.New(dbPath())
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	evidenceBase := filepath.Join(os.Getenv("HOME"), ".finwipe", "evidence")
	evStore, _ := evidence.New(evidenceBase)

	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	letterGen := letter.New(letterDir)

	created := 0
	failed := 0
	smtpConfigured := cfg.SMTP.Password != ""

	// Create requests for matched entities
	for _, m := range matches {
		req, err := hist.CreateRequest(m.Entity.ID, m.Entity.Name,
			history.ChannelEmail,
			m.Entity.GrievanceEmail,
			cibilEmail, cibilName)
		if err != nil {
			fmt.Printf("  ⚠️  %-30s %v\n", m.Entity.Name, err)
			failed++
			continue
		}

		// Store evidence
		emailBody := letter.GenerateEmailBody(req.RequestID, m.Entity.Name, profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)

		if smtpConfigured {
			sender := email.New(&cfg.SMTP)
			err = sender.Send(*m.Entity, profile, "")
			if err == nil {
				hist.Dispatch(req.RequestID, "", "", history.ChannelEmail)
				if evStore != nil {
					evStore.Save(req.RequestID, evidence.TypeEmailSent,
						"DeletionRequest_"+req.RequestID+".eml",
						io.NopCloser(strings.NewReader(emailBody)),
						"Sent: "+m.Entity.GrievanceEmail)
				}
				fmt.Printf("  ✅ %-30s → %s [%s]\n",
					m.Entity.Name, req.RequestID, "email sent")
			} else {
				fmt.Printf("  ⚠️  %-30s %v\n", m.Entity.Name, err)
				failed++
			}
		} else {
			// Just create the request — letter generated separately
			fmt.Printf("  ✅ %-30s → %s [%s]\n",
				m.Entity.Name, req.RequestID, "request created")
		}

		// Generate letter PDF
		letterPath, _ := letterGen.Generate(req.RequestID, m.Entity.Name,
			m.Entity.GrievanceEmail, profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)
		_ = letterPath // stored in history

		created++
	}

	// Save unknown institutions to CSV for manual review
	if len(unknownByName) > 0 {
		unknownCSV := filepath.Join(os.Getenv("HOME"), ".finwipe", "unknown_institutions.csv")
		f, _ := os.Create(unknownCSV)
		w := csv.NewWriter(f)
		w.Write([]string{"Institution Name", "Account/Product Info", "Grievance Email (TBD)"})
		for name, account := range unknownByName {
			w.Write([]string{name, account, ""})
		}
		w.Flush()
		f.Close()
		fmt.Printf("\n  📄 Unknown institutions saved to: %s\n", unknownCSV)
		fmt.Println("  → Fill in grievance emails manually, then run:")
		fmt.Println("    finwipe new --nbfc-file unknown_institutions.csv")
	}

	fmt.Println()
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  ✅ Created: %d | ❌ Failed: %d\n", created, failed)
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    finwipe track --all        # Monitor acknowledgments")
	fmt.Println("    finwipe report            # View compliance dashboard")
	if smtpConfigured {
		fmt.Println("    finwipe cron --followup   # Auto-follow-up automation")
	}

	return nil
}

// institution represents a financial institution found in a CIBIL report
type institution struct {
	Name    string // e.g., "HDFC Bank Ltd"
	Account string // e.g., "XXXX1234" or "Auto Loan"
	Type    string // e.g., " Secured Loan", " Credit Card"
	Status  string // e.g., "Active", "Closed"
}

// parseCIBILReport extracts all institutions from a CIBIL report text
func parseCIBILReport(text string) ([]institution, error) {
	var results []institution

	lines := strings.Split(text, "\n")

	// Patterns that indicate institution names in CIBIL reports:
	// 1. "HDFC BANK LIMITED" — bank name lines
	// 2. "BAJAJ FINANCE LTD" — NBFC names
	// 3. Lines with "Account Number:" nearby

	// Known institution patterns
	knownPatterns := []string{
		// Banks
		"HDFC BANK", "ICICI BANK", "AXIS BANK", "KOTAK MAHINDRA BANK",
		"STATE BANK", "INDUSIND", "YES BANK", "IDBI BANK", "BANK OF BARODA",
		"PUNJAB NATIONAL", "CANARA BANK", "UNION BANK", "UNION BANK OF INDIA",
		"SBI", "CENTRAL BANK", "INDIAN BANK", "INDIAN OVERSEAS",
		// NBFCs
		"BAJAJ FINANCE", "TATA CAPITAL", "ADITYA BIRLA", "L&T FINANCE",
		"MUTHOOT", "SHIRAM", "CHOLAMANDALAM", "MAHINDRA FINANCE",
		"HDB FINANCIAL", "KISHT", "STASHFIN", "LAZEE", "RUPEEK",
		"FINSERV", "NAVI", "MONEYWIDE", "KREDITBEE", "KRAZEN",
		"RUBICON", "BERYL", "IVORY", "KASHIV", "ANANTA",
		// Fintech
		"PHONEPE", "PAYTM", "RAZORPAY", "SLICE", "UNI", "CRED",
		"CARDekho", "PAISABAZAAR", "BANKBAZAAR", "INDMONEY", "ZOPPO",
		"OF BUSINESS", "PROSTAGE", "AYE FINANCE", "UGRO",
		// Insurance
		"LIC", "HDFC LIFE", "ICICI PRUDENTIAL", "SBI LIFE", "MAX LIFE",
		"BAJAJ ALLIANZ", "TATA AIA", "NIVA BUPA", "STAR HEALTH",
		"ManipalCigna", "CARE HEALTH",
	}

	// Build a map of found institutions
	found := make(map[string]bool)

	// Pattern 1: Look for known institution names in the text
	for _, line := range lines {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if upper == "" {
			continue
		}

		for _, pattern := range knownPatterns {
			if strings.Contains(upper, pattern) {
				// Clean up the name
				name := cleanInstitutionName(line)
				if name != "" && !found[name] {
					found[name] = true
					results = append(results, institution{
						Name:    name,
						Account: extractAccountInfo(line),
						Type:    extractAccountType(line),
						Status:  extractStatus(line),
					})
				}
				break
			}
		}
	}

	// Pattern 2: Look for "Member:" or institution name before "Account Number"
	accountPatterns := regexp.MustCompile(`(?i)(member|institution|bank|nbfc|finance|credit)\s*[:\-]?\s*([A-Za-z\s]+?)(?:\n|account|branch|\d{4})`)
	matches := accountPatterns.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) > 2 {
			name := strings.TrimSpace(m[2])
			name = cleanInstitutionName(name)
			if name != "" && !found[name] && len(name) > 3 {
				found[name] = true
				results = append(results, institution{Name: name})
			}
		}
	}

	// Pattern 3: Capitalized words that look like company names
	// (less reliable, used to catch anything we missed)
	companyRE := regexp.MustCompile(`\b([A-Z][A-Z\s]+(?:BANK|FINANCE|LENDING|CREDIT|FINSERV|NBFC|LIFE|INSURANCE|BANKING)[\w\s]*)\b`)
	cmatches := companyRE.FindAllString(text, -1)
	for _, m := range cmatches {
		name := cleanInstitutionName(m)
		if name != "" && !found[name] && len(name) > 5 {
			found[name] = true
			results = append(results, institution{Name: name})
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var deduped []institution
	for _, r := range results {
		if !seen[r.Name] {
			seen[r.Name] = true
			deduped = append(deduped, r)
		}
	}

	return deduped, nil
}

func cleanInstitutionName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToUpper(name)
	// Remove extra whitespace
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	// Remove common prefixes/suffixes
	name = strings.TrimPrefix(name, "M/S ")
	name = strings.TrimPrefix(name, "M/S. ")
	name = strings.TrimPrefix(name, "M/S. ")
	// Remove if too short
	if len(name) < 4 {
		return ""
	}
	return name
}

func extractAccountInfo(line string) string {
	// Try to find account number patterns
	accRE := regexp.MustCompile(`(XXXX|X{2,})\d{4,}|\d{4,}[\-X]\d{3,}[\-X]\d{3,}`)
	m := accRE.FindString(line)
	return m
}

func extractAccountType(line string) string {
	line = strings.ToUpper(line)
	if strings.Contains(line, "LOAN") || strings.Contains(line, "LAP") {
		return "Loan"
	}
	if strings.Contains(line, "CREDIT CARD") || strings.Contains(line, "CARD") {
		return "Credit Card"
	}
	if strings.Contains(line, "OVERDRAFT") {
		return "Overdraft"
	}
	return ""
}

func extractStatus(line string) string {
	line = strings.ToUpper(line)
	if strings.Contains(line, "ACTIVE") || strings.Contains(line, "CURRENT") {
		return "Active"
	}
	if strings.Contains(line, "CLOSED") || strings.Contains(line, "Settled") {
		return "Closed"
	}
	if strings.Contains(line, "WRITTEN OFF") {
		return "Written Off"
	}
	return ""
}

func normalize(s string) string {
	s = strings.ToUpper(s)
	s = regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(s, "")
	return s
}


func detectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".csv":
		return "csv"
	case ".txt", ".text":
		return "text"
	default:
		// Try to detect from content
		f, err := os.Open(path)
		if err != nil {
			return "text"
		}
		defer f.Close()

		// Read first few KB
		buf := make([]byte, 4096)
		n, _ := f.Read(buf)
		content := string(buf[:n])

		// Check for PDF signature
		if bytes.HasPrefix(buf, []byte("%PDF")) {
			return "pdf"
		}

		// Check for CSV
		if strings.Contains(content, ",") && strings.Count(content, ",") > 3 {
			return "csv"
		}

		return "text"
	}
}

func extractTextFromPDF(path string) (string, error) {
	// Try multiple approaches

	// 1. If it's a ZIP (some PDF exports are zipped)
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return extractTextFromZipPDF(path)
	}

	// 2. Try plain text extraction
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Check if PDF
	if bytes.HasPrefix(data, []byte("%PDF")) {
		// Basic PDF text extraction — finds text between BT/ET markers
		text := ""
		re := regexp.MustCompile(`BT\s*(.*?)\s*ET`)
		matches := re.FindAllSubmatch(data, -1)
		for _, m := range matches {
			str := string(m[1])
			// Remove formatting
			str = regexp.MustCompile(`\[|\]|\(|\)|\{|\}`).ReplaceAllString(str, " ")
			str = regexp.MustCompile(`\\[\w]`).ReplaceAllString(str, "")
			str = regexp.MustCompile(`\s+`).ReplaceAllString(str, " ")
			text += str + "\n"
		}
		return text, nil
	}

	// Already plain text
	return string(data), nil
}

func extractTextFromZipPDF(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".pdf") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			// Extract text
			text := ""
			re := regexp.MustCompile(`BT\s*(.*?)\s*ET`)
			matches := re.FindAllSubmatch(data, -1)
			for _, m := range matches {
				str := string(m[1])
				str = regexp.MustCompile(`\[|\]|\(|\)|\{|\}`).ReplaceAllString(str, " ")
				str = regexp.MustCompile(`\\[\w]`).ReplaceAllString(str, "")
				text += str + "\n"
			}
			return text, nil
		}
	}
	return "", fmt.Errorf("no PDF found in zip")
}

func extractTextFromCSV(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var text strings.Builder
	r := csv.NewReader(f)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		text.WriteString(strings.Join(rec, " ") + "\n")
	}
	return text.String(), nil
}

