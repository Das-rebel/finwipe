package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

// awesomeFintechIndiaEntities maps company names to their category and known grievance email patterns
type fintechEntity struct {
	cat  nbfc.Category
	email string
}

var awesomeFintechIndiaEntities = map[string]fintechEntity{
	// Credit and Lending APIs (most relevant for data deletion)
	"Lendingkart":            {cat: nbfc.CatNBFC, email: "grievance@lendingkart.com"},
	"KreditBee":              {cat: nbfc.CatFINTECH, email: "support@kissht.com"},
	"Capital Float":           {cat: nbfc.CatNBFC, email: "support@capitalfloat.com"},
	"Indifi":                 {cat: nbfc.CatNBFC, email: "care@indifi.com"},
	"Loanzen":                {cat: nbfc.CatNBFC, email: "info@loanzen.com"},
	"InCred":                 {cat: nbfc.CatNBFC, email: "grievance@incred.com"},
	"EarlySalary":            {cat: nbfc.CatFINTECH, email: "support@earlysalary.com"},
	"Credy":                  {cat: nbfc.CatFINTECH, email: "support@credy.in"},
	"StashFin":               {cat: nbfc.CatFINTECH, email: "support@stashfin.com"},
	"MoneyTap":               {cat: nbfc.CatFINTECH, email: "support@moneytap.com"},
	// Insurance APIs
	"PolicyBazaar":           {cat: nbfc.CatFINTECH, email: "support@policybazaar.com"},
	"Tata AIA Life Insurance": {cat: nbfc.CatFINTECH, email: "customer.care@tataaia.com"},
	"HDFC ERGO":             {cat: nbfc.CatFINTECH, email: "support@hdfcergo.com"},
	"Bajaj Allianz":           {cat: nbfc.CatFINTECH, email: "care@bajajallianz.co.in"},
	"Max Life Insurance":      {cat: nbfc.CatFINTECH, email: "customer.care@maxlifeinsurance.com"},
	"SBI Life Insurance":     {cat: nbfc.CatFINTECH, email: "care@sbilife.co.in"},
	"ICICI Prudential":       {cat: nbfc.CatFINTECH, email: "customer.care@iciciprulife.com"},
	"LIC":                    {cat: nbfc.CatFINTECH, email: "webcare@licindia.com"},
	"Reliance Nippon Life":    {cat: nbfc.CatFINTECH, email: "rnccare@reliancelife.com"},
	// Investment APIs
	"Zerodha":                {cat: nbfc.CatFINTECH, email: "zerodha@gmail.com"},
	"Upstox":                 {cat: nbfc.CatFINTECH, email: "support@upstox.com"},
	"5paisa":                 {cat: nbfc.CatFINTECH, email: "care@5paisa.com"},
	"Angel Broking":          {cat: nbfc.CatFINTECH, email: "support@angelone.in"},
	"Edelweiss":              {cat: nbfc.CatFINTECH, email: "customer.care@edelweissfin.com"},
	"HDFC Securities":        {cat: nbfc.CatFINTECH, email: "helpdesk@hdfcSecurities.com"},
	"Kotak Securities":        {cat: nbfc.CatFINTECH, email: "service.grievance@kotak.com"},
	"Motilal Oswal":          {cat: nbfc.CatFINTECH, email: "investor.care@motilaloswal.com"},
	// Payment Processing
	"Paytm":                  {cat: nbfc.CatFINTECH, email: "care@paytm.com"},
	"PhonePe":                 {cat: nbfc.CatFINTECH, email: "grievance@phonepe.com"},
	"Razorpay":                {cat: nbfc.CatFINTECH, email: "support@razorpay.com"},
	"MobiKwik":                {cat: nbfc.CatFINTECH, email: "support@mobikwik.com"},
	"PayU":                    {cat: nbfc.CatFINTECH, email: "care@payu.in"},
	"Instamojo":                {cat: nbfc.CatFINTECH, email: "support@instamojo.com"},
	"Amazon Pay":              {cat: nbfc.CatFINTECH, email: "amazonpay-india@amazon.com"},
	"Google Pay":              {cat: nbfc.CatFINTECH, email: "support@gpay.co.in"},
	// Banking
	"Bank of Baroda":          {cat: nbfc.CatBANK, email: "customercare@bankofbaroda.com"},
	"Canara Bank":             {cat: nbfc.CatBANK, email: "cb垂直电话@canarabank.com"},
	"Union Bank of India":     {cat: nbfc.CatBANK, email: "customercare@unionbankofindia.com"},
	"Bank of India":          {cat: nbfc.CatBANK, email: "customercare@bankofindia.co.in"},
}

var updateRegistryCmd = &cobra.Command{
	Use:   "update-registry",
	Short: "Update NBFC registry from awesome-fintech-india",
	Long: `Fetch the latest list of Indian fintech entities from awesome-fintech-india
and add new entities not yet in the local registry.

This adds companies from:
- Credit and Lending APIs (KreditBee, Lendingkart, etc.)
- Insurance APIs (PolicyBazaar, LIC, etc.)
- Investment APIs (Zerodha, Upstox, etc.)
- Payment Processing (Paytm, PhonePe, Razorpay, etc.)

Entities with known grievance emails are pre-filled.`,
	RunE: runUpdateRegistry,
}

var (
	updateScrape  bool
	updateDryRun bool
)

func init() {
	rootCmd.AddCommand(updateRegistryCmd)
	updateRegistryCmd.Flags().BoolVar(&updateScrape, "scrape",
		false, "Scrape privacy policies for grievance emails (slow)")
	updateRegistryCmd.Flags().BoolVar(&updateDryRun, "dry-run",
		false, "Show what would be added without modifying registry")
}

func runUpdateRegistry(cmd *cobra.Command, args []string) error {
	// Load existing registry
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Build existing ID map
	existing := make(map[string]bool)
	for _, e := range entities {
		existing[e.ID] = true
	}

	// Parse our predefined list
	newEntities := []nbfc.Entity{}

	for name, info := range awesomeFintechIndiaEntities {
		// Create URL-safe ID
		id := nameToID(name)

		if existing[id] {
			continue // Already in registry
		}

		grievanceEmail := info.email
		if grievanceEmail == "" && updateScrape {
			fmt.Printf("🔍 Scraping for: %s...\n", name)
			email := scrapeGrievanceEmail(name)
			if email != "" {
				grievanceEmail = email
				fmt.Printf("   Found: %s\n", grievanceEmail)
			}
		}

		entity := nbfc.Entity{
			ID:             id,
			Name:           name,
			ShortName:      name,
			Category:       info.cat,
			GrievanceEmail: grievanceEmail,
			Active:         true,
		}

		newEntities = append(newEntities, entity)
		existing[id] = true
	}

	if len(newEntities) == 0 {
		fmt.Println("✅ Registry is already up to date!")
		return nil
	}

	// Sort by category then name
	sort.Slice(newEntities, func(i, j int) bool {
		if newEntities[i].Category != newEntities[j].Category {
			return newEntities[i].Category < newEntities[j].Category
		}
		return newEntities[i].Name < newEntities[j].Name
	})

	fmt.Printf("\n📊 Found %d new entities to add:\n\n", len(newEntities))

	// Group by category
	byCategory := make(map[nbfc.Category][]nbfc.Entity)
	for _, e := range newEntities {
		byCategory[e.Category] = append(byCategory[e.Category], e)
	}

	catLabel := map[nbfc.Category]string{
		nbfc.CatBANK:    "🏛️  Banks",
		nbfc.CatNBFC:   "🏦 NBFCs",
		nbfc.CatFINTECH: "💳 Fintechs",
		nbfc.CatHFC:     "🏠 HFCs",
		nbfc.CatLSP:     "📦 LSPs",
		nbfc.CatDSP:     "🔗 DSPs",
	}

	for _, cat := range []nbfc.Category{nbfc.CatBANK, nbfc.CatNBFC, nbfc.CatFINTECH, nbfc.CatHFC, nbfc.CatLSP, nbfc.CatDSP} {
		ents := byCategory[cat]
		if len(ents) == 0 {
			continue
		}
		label := catLabel[cat]
		if label == "" {
			label = string(cat)
		}
		fmt.Printf("%s (%d)\n", label, len(ents))
		for _, e := range ents {
			email := e.GrievanceEmail
			if email == "" {
				email = "(no email)"
			}
			fmt.Printf("  • %-30s %s\n", e.Name, email)
		}
		fmt.Println()
	}

	if updateDryRun {
		fmt.Println("🔍 Dry run — no changes made. Run without --dry-run to add.")
		return nil
	}

	// Merge with existing
	allEntities := append(entities, newEntities...)

	// Save
	if err := saveRegistry(nbfcRegistryPath(), allEntities); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("✅ Added %d new entities to registry\n", len(newEntities))
	fmt.Printf("   Registry now has %d total entities\n", len(allEntities))

	return nil
}

// nameToID converts a company name to URL-safe ID
func nameToID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(id, "")
	id = regexp.MustCompile(`-+`).ReplaceAllString(id, "-")
	return id
}

// scrapeGrievanceEmail tries to find grievance email for a company
func scrapeGrievanceEmail(companyName string) string {
	domains := []string{
		strings.ToLower(companyName),
		strings.ToLower(strings.ReplaceAll(companyName, " ", "")),
		strings.ToLower(strings.ReplaceAll(companyName, " ", "-")),
	}

	for _, domain := range domains {
		// Try common email patterns
		patterns := []string{
			"grievance@%s.com",
			"privacy@%s.com",
			"support@%s.com",
			"care@%s.com",
		}
		for _, pattern := range patterns {
			email := fmt.Sprintf(pattern, domain)
			if checkDomainExists(email) {
				return email
			}
		}
	}
	return ""
}

// checkDomainExists checks if a domain is reachable
func checkDomainExists(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]

	url := "https://" + domain
	resp, err := http.Head(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// saveRegistry saves entities to YAML file
func saveRegistry(path string, entities []nbfc.Entity) error {
	// Build YAML structure
	yamlContent := "# FinWipe NBFC Registry — https://github.com/das-rebel/finwipe\n"
	yamlContent += "# Add new entities by editing this file or using: finwipe update-registry\n\n"
	yamlContent += "nbfcs:\n"

	// Group by category
	byCat := make(map[nbfc.Category][]nbfc.Entity)
	for _, e := range entities {
		byCat[e.Category] = append(byCat[e.Category], e)
	}

	catOrder := []nbfc.Category{
		nbfc.CatBANK, nbfc.CatNBFC, nbfc.CatFINTECH,
		nbfc.CatHFC, nbfc.CatLSP, nbfc.CatDSP,
	}

	for _, cat := range catOrder {
		ents := byCat[cat]
		if len(ents) == 0 {
			continue
		}

		catName := map[nbfc.Category]string{
			nbfc.CatBANK:    "# Banks",
			nbfc.CatNBFC:   "# NBFCs",
			nbfc.CatFINTECH: "# Fintechs",
			nbfc.CatHFC:     "# Housing Finance Companies",
			nbfc.CatLSP:     "# Loan Service Providers",
			nbfc.CatDSP:     "# Data Service Providers",
		}[cat]

		if catName == "" {
			catName = "# " + string(cat)
		}

		yamlContent += "\n" + catName + "\n"
		for _, e := range ents {
			yamlContent += fmt.Sprintf("  - id: %s\n", e.ID)
			yamlContent += fmt.Sprintf("    name: %s\n", e.Name)
			yamlContent += fmt.Sprintf("    short_name: %s\n", e.ShortName)
			yamlContent += fmt.Sprintf("    category: %s\n", e.Category)
			if e.GrievanceEmail != "" {
				yamlContent += fmt.Sprintf("    grievance_email: %s\n", e.GrievanceEmail)
			}
			if e.GrievancePhone != "" {
				yamlContent += fmt.Sprintf("    grievance_phone: %q\n", e.GrievancePhone)
			}
			if e.Address != "" {
				yamlContent += fmt.Sprintf("    address: %q\n", e.Address)
			}
			if e.DLAApp != "" {
				yamlContent += fmt.Sprintf("    dla_app: %s\n", e.DLAApp)
			}
			if e.Website != "" {
				yamlContent += fmt.Sprintf("    website: %s\n", e.Website)
			}
			if e.Notes != "" {
				yamlContent += fmt.Sprintf("    notes: %q\n", e.Notes)
			}
			if !e.Active {
				yamlContent += "    active: false\n"
			}
		}
	}

	return os.WriteFile(path, []byte(yamlContent), 0644)
}

// fetchPrivacyPolicy fetches and extracts grievance email from a URL
func fetchPrivacyPolicy(pageURL string) string {
	resp, err := http.Get(pageURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
	if err != nil {
		return ""
	}

	content := string(body)

	// Extract emails
	re := regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	emails := re.FindAllString(content, -1)

	// Look for grievance-related emails
	for _, email := range emails {
		lower := strings.ToLower(email)
		if strings.Contains(lower, "grievance") ||
		   strings.Contains(lower, "dpo") ||
		   strings.Contains(lower, "dataprotection") {
			return email
		}
	}

	return ""
}
