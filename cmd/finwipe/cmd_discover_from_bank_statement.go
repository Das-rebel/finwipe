package main

import (
	"archive/zip"
	
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var bankStatementCmd = &cobra.Command{
	Use:   "discover-from-statement [PDF file or ZIP of statements]",
	Short: "Parse bank statement PDFs to extract all financial institutions",
	Long: `Extract financial institutions from bank statement PDFs (any bank format).

Bank statements reveal far more than CIBIL:
- UPI transactions: shows counterparty VPA names (PhonePe, GPay, etc.)
- IMPS/NEFT credits: sender bank + account number prefix
- EMI deductions: clearly shows lender name (Bajaj Finance EMI, HDFC Bank Loan, etc.)
- Card transactions: merchant + issuing bank
- Standing instructions: shows billers, insurers, mutual funds

Usage:
  finwipe discover-from-statement bank_statement.pdf
  finwipe discover-from-statement statements.zip --bank hdfc
  finwipe discover-from-statement --directory ./statements --dry-run=false

Supports: HDFC, ICICI, SBI, Axis, Kotak, Yes Bank, and most Indian banks.
Output is cross-referenced against FinWipe's 91-entity registry.`,
	RunE: runDiscoverFromStatement,
}

var (
	statementDirectory string
	statementBank      string
	statementDryRun    bool
)

func init() {
	bankStatementCmd.Flags().StringVar(&statementDirectory, "directory", "",
		"Directory containing multiple statement PDFs")
	bankStatementCmd.Flags().StringVar(&statementBank, "bank", "auto",
		"Bank name: hdfc, icici, sbi, axis, kotak, or auto-detect")
	bankStatementCmd.Flags().BoolVar(&statementDryRun, "dry-run", true,
		"Preview only — don't create requests (default: true)")
	rootCmd.AddCommand(bankStatementCmd)
}

func runDiscoverFromStatement(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && statementDirectory == "" {
		return fmt.Errorf("provide a PDF file or --directory")
	}

	var files []string

	if statementDirectory != "" {
		entries, err := os.ReadDir(statementDirectory)
		if err != nil {
			return fmt.Errorf("read directory: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				ext := strings.ToLower(filepath.Ext(e.Name()))
				if ext == ".pdf" || ext == ".zip" {
					files = append(files, filepath.Join(statementDirectory, e.Name()))
				}
			}
		}
	} else {
		files = append(files, args[0])
	}

	if len(files) == 0 {
		return fmt.Errorf("no PDF/ZIP files found")
	}

	profile, err := loadProfile()
	if err != nil {
		return err
	}

	// Load registry
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}
	entityMap := make(map[string]nbfc.Entity)
	entityNames := make(map[string]bool)
	for _, e := range entities {
		entityMap[e.ID] = e
		entityNames[strings.ToLower(e.Name)] = true
	}

	fmt.Println()
	fmt.Println("  🏦 FinWipe — Bank Statement Discovery")
	fmt.Println("  ─────────────────────────────────────────────────────────────")

	// Extract all text from all files
	var allText strings.Builder
	for _, f := range files {
		text, err := extractTextFromStatement(f)
		if err != nil {
			fmt.Printf("  ⚠️  %s: %v\n", filepath.Base(f), err)
			continue
		}
		fmt.Printf("  ✅ %s: %d chars extracted\n", filepath.Base(f), len(text))
		allText.WriteString(text)
		fmt.Println()
	}

	text := allText.String()
	if text == "" {
		return fmt.Errorf("could not extract text from any file")
	}

	// Parse for institutions
	found := parseStatementForInstitutions(text, entityMap)

	// Categorize results
	type result struct {
		Name     string
		Category string
		Evidence string
		Entity   *nbfc.Entity
	}
	var matched []result
	var unmatched []result
	seen := make(map[string]bool)

	for name, evidence := range found {
		if seen[name] {
			continue
		}
		seen[name] = true

		// Check against registry
		lower := strings.ToLower(name)
		var entity *nbfc.Entity
		for _, e := range entities {
			if strings.Contains(lower, strings.ToLower(e.Name)) ||
				strings.Contains(strings.ToLower(e.Name), lower) {
				entity = &e
				break
			}
			if e.ShortName != "" && (strings.Contains(lower, strings.ToLower(e.ShortName))) {
				entity = &e
				break
			}
		}

		if entity != nil {
			matched = append(matched, result{
				Name:     entity.Name,
				Category: string(entity.Category),
				Evidence: evidence,
				Entity:   entity,
			})
		} else {
			unmatched = append(unmatched, result{
				Name:     name,
				Evidence: evidence,
			})
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})
	sort.Slice(unmatched, func(i, j int) bool {
		return unmatched[i].Name < unmatched[j].Name
	})

	// Report
	fmt.Println()
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  ✅ MATCHED IN REGISTRY: %d\n", len(matched))
	fmt.Println("  ──────────────────────────────────────────────────────────")
	for i, m := range matched {
		if i >= 30 {
			fmt.Printf("  ... and %d more\n", len(matched)-30)
			break
		}
		catIcon := "💳"
		if m.Category == "bank" {
			catIcon = "🏛️"
		} else if m.Category == "nbfc" {
			catIcon = "📊"
		} else if m.Category == "hfc" {
			catIcon = "🏠"
		}
		fmt.Printf("  %2d. %s %-28s %s\n", i+1, catIcon, truncate(m.Name, 28), m.Evidence)
	}
	fmt.Println()

	if len(unmatched) > 0 {
		fmt.Printf("  ❓ NOT IN REGISTRY: %d\n", len(unmatched))
		fmt.Println("  ──────────────────────────────────────────────────────────")
		for i, u := range unmatched {
			if i >= 15 {
				fmt.Printf("  ... and %d more\n", len(unmatched)-15)
				break
			}
			fmt.Printf("  %2d. %-30s %s\n", i+1, truncate(u.Name, 30), u.Evidence)
		}
		fmt.Println()
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Total found: %d | Registry match: %d | Unknown: %d\n",
		len(matched)+len(unmatched), len(matched), len(unmatched))
	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Println()

	if statementDryRun {
		fmt.Println("  🔍 DRY RUN — No requests created")
		fmt.Println("  Run with --dry-run=false to create deletion requests")
		return nil
	}

	// Create requests for matched entities
	fmt.Println("  🚀 Creating deletion requests...")
	hist, err := history.New(dbPath())
	if err != nil {
		return err
	}
	defer hist.Close()

	created := 0
	for _, m := range matched {
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
			profile, letter.DefaultDeletionCategories)
		fmt.Printf("  ✅ %-28s %s\n", m.Entity.Name, req.RequestID)
		created++
	}

	fmt.Printf("\n  ✅ Created: %d deletion requests\n", created)
	return nil
}

// extractTextFromStatement extracts text from a PDF or ZIP of PDFs
func extractTextFromStatement(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".zip" {
		return extractFromZip(path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return extractTextFromPDFData(data), nil
}

func extractFromZip(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var all strings.Builder
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
			all.WriteString(extractTextFromPDFData(data))
			all.WriteString("\n")
		}
	}
	return all.String(), nil
}

// extractTextFromPDFData extracts readable text from raw PDF bytes
func extractTextFromPDFData(data []byte) string {
	// Basic PDF text extraction — find text between BT/ET markers
	var result strings.Builder

	// Also look for readable ASCII sequences
	re := regexp.MustCompile(`\(([^\)]+)\)`)
	matches := re.FindAllSubmatch(data, -1)
	for _, m := range matches {
		s := string(m[1])
		// Filter: only keep meaningful text (letters, spaces, common punctuation)
		if len(s) > 3 {
			clean := filterReadable(s)
			if clean != "" && len(clean) > 4 {
				result.WriteString(clean)
				result.WriteString(" ")
			}
		}
	}

	// Also find hex-encoded text
	hexRe := regexp.MustCompile(`<([0-9A-Fa-f\s]+)>`)
	hexMatches := hexRe.FindAllSubmatch(data, -1)
	for _, m := range hexMatches {
		s := string(m[1])
		// Convert hex pairs to chars
		parts := strings.Fields(s)
		var sb strings.Builder
		for _, p := range parts {
			if len(p) == 2 {
				if b, err := hexDecode(p); err == nil && b >= 32 && b < 127 {
					sb.WriteByte(b)
				}
			}
		}
		clean := filterReadable(sb.String())
		if len(clean) > 4 {
			result.WriteString(clean)
			result.WriteString(" ")
		}
	}

	return result.String()
}

func filterReadable(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '.' || r == ',' || r == '\'' || r == '/' || r == '@' || r == '(' || r == ')' {
			result.WriteRune(r)
		} else {
			result.WriteRune(' ')
		}
	}
	return strings.TrimSpace(result.String())
}

func hexDecode(s string) (byte, error) {
	var b byte
	for _, c := range s {
		b <<= 4
		switch {
		case c >= '0' && c <= '9':
			b |= byte(c - '0')
		case c >= 'A' && c <= 'F':
			b |= byte(c - 'A' + 10)
		case c >= 'a' && c <= 'f':
			b |= byte(c - 'a' + 10)
		default:
			return 0, fmt.Errorf("invalid hex")
		}
	}
	return b, nil
}

// parseStatementForInstitutions finds financial institution references in statement text
func parseStatementForInstitutions(text string, entityMap map[string]nbfc.Entity) map[string]string {
	results := make(map[string]string)
	upper := strings.ToUpper(text)

	// Pattern 1: EMI/NACH deductions — "BAJAJ FINANCE EMI", "HDFC BANK LOAN", "TATA CAPITAL", etc.
	emiPatterns := []string{
		`(?i)(BAJAJ FINANCE|BANK OF BARODA|HDFC BANK|ICICI BANK|AXIS BANK|KOTAK MAHINDRA|STATE BANK|SBI|INDUSIND|YES BANK|IDBI BANK|CANARA BANK|UNION BANK|PNB|LAXMI VILLA BANK|CENTRAL BANK)[,\s]+(?:LOAN|EMI|NACH|SI|RD|CD)`,
		`(?i)(EMI|LOAN|NACH)[\s]+(?:TO|FOR|DR|IN FAVOUR OF)[\s]+([A-Z][A-Z\s]{2,20})(?:BANK|NBFC|FINANCE|LENDING)`,
		`(?i)([A-Z][A-Z\s]{2,15})(?:FINANCE|FINSERV|NBFC|BANK|LENDING|HOUSING)[\s]+(?:EMI|NACH|LOAN|DEDUCTION)`,
		`(?i)(BAJAJ|TATA CAPITAL|ADITYA BIRLA|L&T FINANCE|MUTHOOT|CHOLAMANDALAM|MAHINDRA FIN|KISHT|KREDITBEE|STASHFIN|RUPEEK|NAVI|OF BUSINESS|KISHT|RUPEEK)[\s]+(?:EMI|LOAN|FINANCE)`,
		`(?i)(HDB FINANCIAL|CAPSITE|CREAMLINE|FINERV|SATYA MICRO|CAPITAL|BHARAT)[\s]+(?:LOAN|CREDIT)`,
	}

	for _, pat := range emiPatterns {
		re := regexp.MustCompile(pat)
		matches := re.FindAllStringSubmatch(upper, -1)
		for _, m := range matches {
			if len(m) > 1 {
				name := strings.TrimSpace(m[1])
				if len(name) > 3 {
					results[name] = "EMI/NACH deduction"
				}
			}
		}
	}

	// Pattern 2: UPI / VPA transactions — "UPI/phonepe@yesbank", "GPAY/xyz@pytm"
	upiRe := regexp.MustCompile(`(?i)(UPI|VPA)[\s:/]*([a-z0-9]+)@([a-z]+)`)
	for _, m := range upiRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 2 {
			handle := strings.ToLower(m[2])
			// Map UPI handles to known entities
			if entity, ok := mapUPIHandle(handle); ok {
				if _, exists := results[entity.Name]; !exists {
					results[entity.Name] = fmt.Sprintf("UPI handle: %s@%s", m[2], m[3])
				}
			}
		}
	}

	// Pattern 3: Credit card transactions — "AXIS BANK CARD", "HDFC CARD"
	cardRe := regexp.MustCompile(`(?i)(CARD|CC|CR CARD)[\s\#]*[0-9*]+[\s]+(?:TO|AT|ON)[\s]+([A-Z][A-Z\s]{2,20})(?:STORE|MART|SHOP|ONLINE|WEBSITE)`)
	for _, m := range cardRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if len(name) > 3 {
				results[name] = "Card transaction"
			}
		}
	}

	// Pattern 4: Standing instructions / bill payments
	billerRe := regexp.MustCompile(`(?i)(?:SI|BILL PAYMENT|AUTOPAY)[\s]+(?:TO|FOR)[\s]+([A-Z][A-Z\s]{2,25})(?:LTD|LIMITED|PLC|PVT|INC)`)
	for _, m := range billerRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if len(name) > 3 {
				results[name] = "Standing instruction/biller"
			}
		}
	}

	// Pattern 5: Insurance premiums — recognizable insurer names
	insurers := []string{
		"LIC", "HDFC LIFE", "SBI LIFE", "ICICI PRUDENTIAL", "BAJAJ ALLIANZ",
		"TATA AIA", "MAX LIFE", "ADITYA BIRLA SUNLIFE", "NIPPON LIFE",
		"KOTAK LIFE", "AVIVA LIFE", "CGNAT", "STAR HEALTH", "NIVA BUPA",
		"RELIGARE BROKING", "POLICYBAZAAR", "ACKO", "ZUNO", "ETHOS",
	}
	for _, ins := range insurers {
		if strings.Contains(upper, ins) {
			results[ins] = "Insurance premium"
		}
	}

	// Pattern 6: Mutual fund SIPs
	mfRe := regexp.MustCompile(`(?i)(SIP|MUTUAL FUND|NFO)[\s]+(?:TO|FOR)[\s]+([A-Z][A-Z\s]{2,20})(?:MF|MUTUAL FUND|AMC)`)
	for _, m := range mfRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if len(name) > 3 {
				results[name] = "SIP/Mutual fund"
			}
		}
	}

	// Pattern 7: IMPS/NEFT counterparty — "IMPS FROM HDFC BANK" / "NEFT TO ICICI BANK"
	impsRe := regexp.MustCompile(`(?i)(?:IMPS|NEFT|RTGS)[\s]+(?:FROM|TO|DR|CR)[\s]+([A-Z][A-Z\s]{2,25])(?:BANK|NBFC|FINANCE)`)
	for _, m := range impsRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if len(name) > 3 {
				results[name] = "IMPS/NEFT counterparty"
			}
		}
	}

	// Pattern 8: Known Indian FIs in text (generic match against registry)
	knownFIKeywords := []string{
		"BAJAJ FINANCE", "TATA CAPITAL", "ADITYA BIRLA FINANCE", "L&T FINANCE",
		"MUTHOOT FINANCE", "CHOLAMANDALAM", "HDB FINANCIAL", "KISHT",
		"STASHFIN", "RUPEEK", "NAVI", "OF BUSINESS", "KREDITBEE",
		"SLICE", "UNI", "CRED", "PHONEPE", "PAYTM", "RAZORPAY",
		"ZERODHA", "UPSTOX", "GROWW", "ANGLEONE", "DHAN",
		"POLICYBAZAAR", "INSURANCE", "RELIANCE GENERAL", "BAJAJ ALLIANZ",
		"HDFC ERGO", "SBI GENERAL", "CHOLAMANDALAM MS",
		"KOTAK MAHINDRA GENERAL", "GO digit",
	}
	for _, kw := range knownFIKeywords {
		if strings.Contains(upper, kw) {
			// Extract full name
			idx := strings.Index(upper, kw)
			start := idx
			for start > 0 && upper[start-1] >= 'A' && upper[start-1] <= 'Z' {
				start--
			}
			end := idx + len(kw)
			for end < len(upper) && ((upper[end] >= 'A' && upper[end] <= 'Z') || (upper[end] >= 'a' && upper[end] <= 'z')) {
				end++
			}
			name := strings.TrimSpace(text[start:end])
			if len(name) > 3 && len(name) < 50 {
				if _, exists := results[name]; !exists {
					results[name] = "Known FI reference in statement"
				}
			}
		}
	}

	return results
}

// mapUPIHandle maps known UPI handles to entity names
func mapUPIHandle(handle string) (nbfc.Entity, bool) {
	handle = strings.ToLower(handle)
	mappings := map[string]nbfc.Entity{
		"phonepe":    {ID: "phonepe", Name: "PhonePe Pvt Ltd", GrievanceEmail: "grievance.officer@phonepe.com", Category: nbfc.CatFINTECH},
		"gpay":        {ID: "gpay", Name: "Google Payment Corp", GrievanceEmail: "", Category: nbfc.CatFINTECH},
		"paytm":       {ID: "paytm", Name: "Paytm Payments Bank", GrievanceEmail: "support@paytm.com", Category: nbfc.CatFINTECH},
		"amazon":     {ID: "amazon-pay", Name: "Amazon Pay", GrievanceEmail: "", Category: nbfc.CatFINTECH},
		"bhim":        {ID: "bhim", Name: "BHIM UPI / NPCI", GrievanceEmail: "", Category: nbfc.CatFINTECH},
		"mobikwik":    {ID: "mobikwik", Name: "MobiKwik", GrievanceEmail: "", Category: nbfc.CatFINTECH},
		"freecharge":  {ID: "freecharge", Name: "FreeCharge", GrievanceEmail: "", Category: nbfc.CatFINTECH},
		"airtel":      {ID: "airtel-payments-bank", Name: "Airtel Payments Bank", GrievanceEmail: "", Category: nbfc.CatBANK},
		"jio":         {ID: "jio-payments-bank", Name: "Jio Payments Bank", GrievanceEmail: "", Category: nbfc.CatBANK},
		"yesbank":     {ID: "yesbank", Name: "Yes Bank", GrievanceEmail: "contact@yesbank.in", Category: nbfc.CatBANK},
		"icici":       {ID: "icici-bank", Name: "ICICI Bank", GrievanceEmail: "customer.care@icicibank.com", Category: nbfc.CatBANK},
		"hdfcbank":    {ID: "hdfc-bank", Name: "HDFC Bank", GrievanceEmail: "grievancecell@hdfcbank.com", Category: nbfc.CatBANK},
		"sbi":         {ID: "sbi", Name: "State Bank of India", GrievanceEmail: "contact@sbi.co.in", Category: nbfc.CatBANK},
		"axisbank":    {ID: "axis-bank", Name: "Axis Bank", GrievanceEmail: "customer.service@axisbank.com", Category: nbfc.CatBANK},
		"kotak":      {ID: "kotak-bank", Name: "Kotak Mahindra Bank", GrievanceEmail: "customer.service@kotak.com", Category: nbfc.CatBANK},
		"indusind":   {ID: "indusind-bank", Name: "IndusInd Bank", GrievanceEmail: "contactus@indusind.com", Category: nbfc.CatBANK},
		"bob":        {ID: "bank-of-baroda", Name: "Bank of Baroda", GrievanceEmail: "customer.service@bankofbaroda.in", Category: nbfc.CatBANK},
	}
	if e, ok := mappings[handle]; ok {
		return e, true
	}
	return nbfc.Entity{}, false
}

func loadProfile() (config.Profile, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return config.Profile{}, fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" || cfg.Profile.Name == "" {
		return config.Profile{}, fmt.Errorf("profile incomplete (run finwipe init)")
	}
	return cfg.Profile, nil
}
