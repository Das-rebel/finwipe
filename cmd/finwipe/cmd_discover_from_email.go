package main

import (
	"archive/zip"
	
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var emailDiscoveryCmd = &cobra.Command{
	Use:   "discover-from-email [email export file or Gmail export zip]",
	Short: "Parse email exports to find financial institutions",
	Long: `Extract financial institutions from email exports.

Sources:
- Gmail Takeout (all emails as HTML/mbox)
- Thunderbird export (.mbox or individual .eml files)
- Any email client export containing From/To headers

How it works:
1. Reads email files and extracts From: addresses
2. Maps email domains to known Indian financial institutions
3. Shows all discovered FIs (banks, NBFCs, fintechs, insurers)
4. Creates deletion requests for matched entities

IMPORTANT: Your emails never leave your machine. This is a local parser.
No OAuth, no cloud, no third-party access.

Usage:
  finwipe discover-from-email gmail-export.zip
  finwipe discover-from-email ./emails/ --format mbox
  finwipe discover-from-email --file export.csv --format csv`,
	RunE: runDiscoverFromEmail,
}

var (
	emailFile   string
	emailFormat string // auto, mbox, eml, csv, html
)

func init() {
	emailDiscoveryCmd.Flags().StringVar(&emailFile, "file", "", "Path to email export (ZIP, mbox, CSV, or directory)")
	emailDiscoveryCmd.Flags().StringVar(&emailFormat, "format", "auto",
		"Format: auto, mbox, eml, csv, html")
	rootCmd.AddCommand(emailDiscoveryCmd)
}

func runDiscoverFromEmail(cmd *cobra.Command, args []string) error {
	if emailFile == "" && len(args) == 0 {
		return fmt.Errorf("--file required")
	}
	path := emailFile
	if path == "" {
		path = args[0]
	}

	profile, err := loadProfile()
	if err != nil {
		return err
	}

	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	// Build domain → entity map
	domainMap := buildEmailDomainMap(entities)

	fmt.Println()
	fmt.Println("  📧 FinWipe — Email Discovery")
	fmt.Println("  ─────────────────────────────────────────────────────────────")
	fmt.Printf("  📄 Source: %s\n", path)
	fmt.Println()

	// Extract all emails
	emails, err := extractEmails(path, emailFormat)
	if err != nil {
		return fmt.Errorf("extract emails: %w", err)
	}
	fmt.Printf("  ✅ Processed %d emails\n\n", len(emails))

	// Find financial institution emails
	found := findFIEmails(emails, domainMap, entities)

	// Categorize
	type match struct {
		Name     string
		Domain   string
		Evidence string
		Entity   *nbfc.Entity
	}
	var matched []match
	var unknown []match

	seen := make(map[string]bool)
	for name, info := range found {
		if seen[name] {
			continue
		}
		seen[name] = true

		// Cross-reference with registry
		lower := strings.ToLower(name)
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
				Name:     entity.Name,
				Domain:   info.domain,
				Evidence: fmt.Sprintf("%d emails", info.count),
				Entity:   entity,
			})
		} else {
			unknown = append(unknown, match{
				Name:     name,
				Domain:   info.domain,
				Evidence: fmt.Sprintf("%d emails", info.count),
			})
		}
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  ✅ IN REGISTRY: %d\n", len(matched))
	fmt.Println("  ──────────────────────────────────────────────────────────")
	for i, m := range matched {
		if i >= 30 {
			fmt.Printf("  ... and %d more\n", len(matched)-30)
			break
		}
		fmt.Printf("  %2d. %-28s %s (%s)\n", i+1, truncate(m.Name, 28), m.Domain, m.Evidence)
	}
	fmt.Println()

	if len(unknown) > 0 {
		fmt.Printf("  ❓ NOT IN REGISTRY: %d\n", len(unknown))
		fmt.Println("  ──────────────────────────────────────────────────────────")
		for i, u := range unknown {
			if i >= 20 {
				fmt.Printf("  ... and %d more\n", len(unknown)-20)
				break
			}
			fmt.Printf("  %2d. %-28s %s\n", i+1, truncate(u.Name, 28), u.Domain)
		}
		fmt.Println()
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Total found: %d | Registry: %d | Unknown: %d\n",
		len(found), len(matched), len(unknown))
	fmt.Println("  ═══════════════════════════════════════════════════════════════")

	if dryRun {
		fmt.Println("\n  🔍 DRY RUN — No requests created")
		return nil
	}

	fmt.Println("\n  🚀 Creating deletion requests...")
	hist, err := history.New(dbPath())
	if err != nil {
		return err
	}
	defer hist.Close()

	created := 0
	for _, m := range matched {
		if m.Entity.GrievanceEmail == "" {
			continue
		}
		req, err := hist.CreateRequest(m.Entity.ID, m.Entity.Name,
			history.ChannelEmail,
			m.Entity.GrievanceEmail,
			profile.Email, profile.Name)
		if err != nil {
			fmt.Printf("  ⚠️  %-28s %v\n", m.Entity.Name, err)
			continue
		}
		letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
		gen := letter.New(letterDir)
		gen.Generate(req.RequestID, m.Entity.Name, m.Entity.GrievanceEmail,
			profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)
		fmt.Printf("  ✅ %-28s %s\n", m.Entity.Name, req.RequestID)
		created++
	}

	fmt.Printf("\n  ✅ Created: %d deletion requests\n", created)
	return nil
}

// emailInfo tracks what we found about an entity
type emailInfo struct {
	domain string
	count  int
}

// buildEmailDomainMap maps known financial institution email domains
func buildEmailDomainMap(entities []nbfc.Entity) map[string]nbfc.Entity {
	m := make(map[string]nbfc.Entity)

	// Primary domains
	primaryDomains := map[string]nbfc.Entity{
		// Banks
		"hdfcbank.com":         {ID: "hdfc-bank", Name: "HDFC Bank", Category: nbfc.CatBANK},
		"icicibank.com":        {ID: "icici-bank", Name: "ICICI Bank", Category: nbfc.CatBANK},
		"axisbank.com":         {ID: "axis-bank", Name: "Axis Bank", Category: nbfc.CatBANK},
		"kotak.com":            {ID: "kotak-bank", Name: "Kotak Mahindra Bank", Category: nbfc.CatBANK},
		"sbi.co.in":            {ID: "sbi", Name: "State Bank of India", Category: nbfc.CatBANK},
		"yesbank.in":           {ID: "yesbank", Name: "Yes Bank", Category: nbfc.CatBANK},
		"indusind.com":         {ID: "indusind-bank", Name: "IndusInd Bank", Category: nbfc.CatBANK},
		"idbibank.com":         {ID: "idbi-bank", Name: "IDBI Bank", Category: nbfc.CatBANK},
		"bankofbaroda.in":      {ID: "bob", Name: "Bank of Baroda", Category: nbfc.CatBANK},
		"pnb.co.in":            {ID: "pnb", Name: "Punjab National Bank", Category: nbfc.CatBANK},
		"canarabank.com":       {ID: "canara", Name: "Canara Bank", Category: nbfc.CatBANK},
		"unionbankofindia.co.in": {ID: "union-bank", Name: "Union Bank of India", Category: nbfc.CatBANK},
		"centralbankofindia.co.in": {ID: "cbi", Name: "Central Bank of India", Category: nbfc.CatBANK},
		"bankofindia.co.in":    {ID: "boi", Name: "Bank of India", Category: nbfc.CatBANK},
		"indianbank.in":        {ID: "indian-bank", Name: "Indian Bank", Category: nbfc.CatBANK},
		"rblbank.com":          {ID: "rbl", Name: "RBL Bank", Category: nbfc.CatBANK},
		"federalbank.co.in":    {ID: "federal", Name: "Federal Bank", Category: nbfc.CatBANK},
		"bandhanbank.com":      {ID: "bandhan", Name: "Bandhan Bank", Category: nbfc.CatBANK},
		"bankofmaharashtra.com": {ID: "bom", Name: "Bank of Maharashtra", Category: nbfc.CatBANK},
		// Fintech
		"phonepe.com":          {ID: "phonepe", Name: "PhonePe Pvt Ltd", Category: nbfc.CatFINTECH},
		"paytm.com":            {ID: "paytm", Name: "Paytm", Category: nbfc.CatFINTECH},
		"razorpay.com":         {ID: "razorpay", Name: "Razorpay", Category: nbfc.CatFINTECH},
		"cred.club":            {ID: "cred", Name: "CRED", Category: nbfc.CatFINTECH},
		"paisabazaar.com":      {ID: "paisabazaar", Name: "Paisabazaar", Category: nbfc.CatFINTECH},
		"bankbazaar.com":       {ID: "bankbazaar", Name: "BankBazaar", Category: nbfc.CatFINTECH},
		"indmoney.com":         {ID: "indmoney", Name: "IndMoney", Category: nbfc.CatFINTECH},
		"groww.in":             {ID: "groww", Name: "Groww", Category: nbfc.CatFINTECH},
		"zerodha.com":          {ID: "zerodha", Name: "Zerodha", Category: nbfc.CatFINTECH},
		"upstox.com":           {ID: "upstox", Name: "Upstox", Category: nbfc.CatFINTECH},
		"dhan.in":              {ID: "dhan", Name: "Dhan", Category: nbfc.CatFINTECH},
		"policybazaar.com":     {ID: "policybazaar", Name: "PolicyBazaar", Category: nbfc.CatFINTECH},
		// NBFCs
		"bajajfinserv.in":     {ID: "bajaj-finserv", Name: "Bajaj Finserv Ltd", Category: nbfc.CatNBFC},
		"tatacapital.com":     {ID: "tata-capital", Name: "Tata Capital Ltd", Category: nbfc.CatNBFC},
		"adityabirlacapital.com": {ID: "aditya-birla", Name: "Aditya Birla Finance", Category: nbfc.CatNBFC},
		"ltfs.com":             {ID: "ltfs", Name: "L&T Finance", Category: nbfc.CatNBFC},
		"muthootfinance.com":   {ID: "muthoot", Name: "Muthoot Finance", Category: nbfc.CatNBFC},
		"chola.murugappa.com":  {ID: "chola", Name: "Cholamandalam", Category: nbfc.CatNBFC},
		"hdbank.com":           {ID: "hdb", Name: "HDB Financial Services", Category: nbfc.CatNBFC},
		"kisht.com":            {ID: "kisht", Name: "Kisht Consumer Finance", Category: nbfc.CatNBFC},
		"stashfin.com":        {ID: "stashfin", Name: "Stashfin", Category: nbfc.CatNBFC},
		"rupeek.com":          {ID: "rupeek", Name: "Rupeek", Category: nbfc.CatNBFC},
		"navi.com":             {ID: "navi", Name: "Navi Finserv", Category: nbfc.CatNBFC},
		"ofbusiness.com":       {ID: "ofbusiness", Name: "OfBusiness Financial Technologies", Category: nbfc.CatNBFC},
		"kreditbee.in":         {ID: "kreditbee", Name: "KreditBee", Category: nbfc.CatNBFC},
		"sliceit.com":         {ID: "slice", Name: "Slice", Category: nbfc.CatNBFC},
		"uni.cards":           {ID: "uni", Name: "Uni (CardPay)", Category: nbfc.CatNBFC},
		"moneyview.in":         {ID: "moneyview", Name: "Moneyview", Category: nbfc.CatNBFC},
		"earlysalary.com":      {ID: "earlysalary", Name: "EarlySalary", Category: nbfc.CatNBFC},
		// Insurance
		"licindia.in":         {ID: "lic", Name: "LIC", Category: nbfc.CatFINTECH},
		"hdfclife.com":        {ID: "hdfc-life", Name: "HDFC Life Insurance", Category: nbfc.CatFINTECH},
		"iciciprulife.com":   {ID: "icici-pru", Name: "ICICI Prudential Life", Category: nbfc.CatFINTECH},
		"sbilife.co.in":       {ID: "sbi-life", Name: "SBI Life Insurance", Category: nbfc.CatFINTECH},
		"bajajallianzlife.in": {ID: "bajaj-allianz-life", Name: "Bajaj Allianz Life", Category: nbfc.CatFINTECH},
		"tataaia.com":         {ID: "tata-aia", Name: "TATA AIA Life", Category: nbfc.CatFINTECH},
		"maxlifeinsurance.com": {ID: "max-life", Name: "Max Life Insurance", Category: nbfc.CatFINTECH},
		"starhealth.in":       {ID: "star-health", Name: "Star Health Insurance", Category: nbfc.CatFINTECH},
		"nivabupa.com":         {ID: "niva-bupa", Name: "Niva Bupa Health Insurance", Category: nbfc.CatFINTECH},
		"reliancegeneral.in":  {ID: "reliance-general", Name: "Reliance General Insurance", Category: nbfc.CatFINTECH},
		"digit.insure":         {ID: "go-digi", Name: "Go Digit Insurance", Category: nbfc.CatFINTECH},
	}

	for domain, entity := range primaryDomains {
		m[domain] = entity
	}

	// Add from registry
	for _, e := range entities {
		if e.GrievanceEmail != "" {
			// Extract domain from grievance email
			at := strings.Index(e.GrievanceEmail, "@")
			if at >= 0 {
				domain := strings.ToLower(e.GrievanceEmail[at+1:])
				if _, exists := m[domain]; !exists {
					m[domain] = e
				}
			}
		}
	}

	return m
}

// extractedEmail holds parsed email data
type extractedEmail struct {
	From    string
	Subject string
	Date    string
	Body    string
}

// extractEmails reads email files and extracts From addresses
func extractEmails(path string, format string) ([]extractedEmail, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var emails []extractedEmail

	if stat.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			extracted, _ := extractEmails(filepath.Join(path, e.Name()), format); emails = append(emails, extracted...)
		}
		return emails, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".zip" {
		return extractEmailsFromZip(path)
	}
	if ext == ".mbox" || ext == ".eml" || ext == ".txt" || ext == ".csv" {
		return extractEmailsFromFile(path)
	}
	if ext == ".html" || ext == ".htm" {
		return extractEmailsFromHTML(path)
	}

	// Try auto-detect
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	if strings.Contains(content, "From ") && strings.Contains(content, "@") {
		return parseMbox(content), nil
	}
	if strings.Contains(content, "<html") || strings.Contains(content, "<HTML") {
		return parseHTMLForEmails(data), nil
	}
	return nil, fmt.Errorf("unknown format: %s", ext)
}

func extractEmailsFromZip(path string) ([]extractedEmail, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var emails []extractedEmail
	for _, f := range r.File {
		if f.Mode().IsRegular() {
			ext := strings.ToLower(filepath.Ext(f.Name))
			if ext == ".mbox" || ext == ".eml" || ext == ".txt" || ext == ".html" || ext == ".htm" {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					continue
				}
				content := string(data)
				if strings.Contains(content, "From ") && strings.Contains(content, "@") {
					emails = append(emails, parseMbox(content)...)
				} else if strings.Contains(content, "<html") || strings.Contains(content, "<HTML") {
					emails = append(emails, parseHTMLForEmails(data)...)
				}
			}
		}
	}
	return emails, nil
}

func extractEmailsFromFile(path string) ([]extractedEmail, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	if strings.Contains(content, "From ") && strings.Contains(content, "@") {
		return parseMbox(content), nil
	}
	if strings.Contains(content, "<html") || strings.Contains(content, "<HTML") {
		return parseHTMLForEmails(data), nil
	}
	return nil, fmt.Errorf("no email content found")
}

func extractEmailsFromHTML(path string) ([]extractedEmail, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseHTMLForEmails(data), nil
}

// parseMbox parses mbox format email files (Gmail Takeout)
func parseMbox(content string) []extractedEmail {
	var emails []extractedEmail
	lines := strings.Split(content, "\n")
	var current *extractedEmail

	for _, line := range lines {
		if strings.HasPrefix(line, "From ") && strings.Contains(line, "@") {
			if current != nil && current.From != "" {
				emails = append(emails, *current)
			}
			current = &extractedEmail{}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "from:") {
			current.From = extractEmailAddr(strings.TrimPrefix(line, "From:"))
		} else if strings.HasPrefix(strings.ToLower(line), "subject:") {
			current.Subject = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		} else if strings.HasPrefix(strings.ToLower(line), "date:") {
			current.Date = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") &&
			strings.Contains(line, ":") && !strings.Contains(line, "@") {
			// Header line without known prefix — reset body
			if current.Body == "" && current.From != "" {
				current.Body = "header"
			}
		}
	}

	if current != nil && current.From != "" {
		emails = append(emails, *current)
	}
	return emails
}

// parseHTMLForEmails extracts From addresses from HTML email exports
func parseHTMLForEmails(data []byte) []extractedEmail {
	var emails []extractedEmail
	content := string(data)

	// Find all mailto: links
	mailtoRe := regexp.MustCompile(`(?i)mailto:([^\s"']+@[^"\s']+)`)
	for _, m := range mailtoRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			emails = append(emails, extractedEmail{
				From: m[1],
			})
		}
	}

	// Find email addresses in text
	emailRe := regexp.MustCompile(`([a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})`)
	for _, m := range emailRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			addr := strings.ToLower(m[1])
			// Filter obvious non-FI emails
			if !strings.Contains(addr, "noreply") && !strings.Contains(addr, "no-reply") &&
				!strings.Contains(addr, "donotreply") && !strings.Contains(addr, "support@") &&
				!strings.Contains(addr, "help@") && !strings.Contains(addr, "info@") {
				emails = append(emails, extractedEmail{
					From: m[1],
				})
			}
		}
	}

	return emails
}

// findFIEmails identifies financial institution emails
func findFIEmails(emails []extractedEmail, domainMap map[string]nbfc.Entity, entities []nbfc.Entity) map[string]emailInfo {
	found := make(map[string]emailInfo)
	seenDomain := make(map[string]bool)

	for _, email := range emails {
		addr := strings.ToLower(strings.TrimSpace(email.From))
		at := strings.Index(addr, "<")
		if at >= 0 {
			addr = addr[at+1 : len(addr)-1]
		}
		at = strings.Index(addr, "(")
		if at >= 0 {
			addr = addr[:at]
		}
		addr = strings.TrimSpace(addr)

		domain := ""
		if at := strings.Index(addr, "@"); at >= 0 {
			domain = strings.ToLower(addr[at+1:])
		}
		if domain == "" || strings.Contains(domain, "gmail") ||
			strings.Contains(domain, "yahoo") || strings.Contains(domain, "outlook") ||
			strings.Contains(domain, "hotmail") || strings.Contains(domain, "rediff") ||
			strings.Contains(domain, "icloud") || strings.Contains(domain, "proton") {
			continue
		}

		if seenDomain[domain] {
			info := found[domain]
			info.count++
			found[domain] = info
			continue
		}
		seenDomain[domain] = true

		// Check domain map
		if entity, ok := domainMap[domain]; ok {
			found[entity.Name] = emailInfo{domain: domain}
		} else {
			// Check if domain contains known FI name
			for _, e := range entities {
				if e.GrievanceEmail != "" {
					eDomain := ""
					if at := strings.Index(e.GrievanceEmail, "@"); at >= 0 {
						eDomain = strings.ToLower(e.GrievanceEmail[at+1:])
					}
					if eDomain != "" && strings.Contains(domain, eDomain) {
						found[e.Name] = emailInfo{domain: domain}
						break
					}
				}
				// Also check for partial matches
				nameLower := strings.ToLower(e.Name)
				nameParts := strings.Fields(nameLower)
				for _, part := range nameParts {
					if len(part) > 3 && strings.Contains(domain, part) {
						found[e.Name] = emailInfo{domain: domain}
						return found
					}
				}
			}
		}
	}

	return found
}

func extractEmailAddr(header string) string {
	// Extract email from "Name <email@domain.com>" format
	header = strings.TrimSpace(header)
	at := strings.Index(header, "<")
	if at >= 0 {
		end := strings.Index(header, ">")
		if end > at {
			return strings.TrimSpace(header[at+1 : end])
		}
	}
	return strings.TrimSpace(header)
}
