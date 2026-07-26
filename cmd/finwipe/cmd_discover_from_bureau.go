package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var bureauCmd = &cobra.Command{
	Use:   "discover-from-bureau [report file]",
	Short: "Parse credit bureau reports from all 4 bureaus",
	Long: `Parse credit bureau reports from India's 4 RBI-licensed bureaus:
  1. CIBIL (TransUnion CIBIL)
  2. Experian
  3. Equifax
  4. CRIF High Mark

Each bureau shows "Inquiries" = all institutions that queried your credit report.
This is THE most comprehensive source for who has your financial data.

Download free annual reports from:
  - CIBIL: cibil.com/free-credit-score
  - Experian: experian.in/free-credit-score
  - Equifax: equifax.co.in/free-credit-report
  - CRIF High Mark: crifhighmark.com/free-credit-score

Usage:
  finwipe discover-from-bureau --file CIBIL_Report.pdf
  finwipe discover-from-bureau --file Experian_Report.pdf --dry-run=false`,
	RunE: runDiscoverFromBureau,
}

var (
	bureauFile  string
)

func init() {
	bureauCmd.Flags().StringVar(&bureauFile, "file", "", "Path to bureau report (PDF or text)")
	bureauCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(bureauCmd)
}

func runDiscoverFromBureau(cmd *cobra.Command, args []string) error {
	profile, err := loadProfile()
	if err != nil {
		return err
	}

	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load NBFC registry: %w", err)
	}

	data, err := os.ReadFile(bureauFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	textStr := string(data)

	bureau := detectBureau(textStr)
	fmt.Println()
	fmt.Println("  🏦 FinWipe — Bureau Report Discovery")
	fmt.Println("  ─────────────────────────────────────────────────────────────")
	fmt.Printf("  📄 File: %s\n", filepath.Base(bureauFile))
	fmt.Printf("  🔬 Detected: %s\n", bureau)

	institutions := parseBureauReport(textStr, bureau)
	fmt.Printf("  ✅ Found %d institutions in report\n\n", len(institutions))

	type match struct {
		Name     string
		Evidence string
		Entity   *nbfc.Entity
	}
	var matched []match
	var unknown []string
	seen := make(map[string]bool)

	for _, inst := range institutions {
		if seen[inst.Name] {
			continue
		}
		seen[inst.Name] = true

		lower := strings.ToLower(inst.Name)
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
				Evidence: inst.Evidence,
				Entity:   entity,
			})
		} else {
			unknown = append(unknown, inst.Name+" ("+inst.Evidence+")")
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Name < matched[j].Name
	})

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  ✅ IN REGISTRY: %d\n", len(matched))
	fmt.Println("  ──────────────────────────────────────────────────────────")
	for i, m := range matched {
		if i >= 40 {
			fmt.Printf("  ... and %d more\n", len(matched)-40)
			break
		}
		fmt.Printf("  %2d. %-30s %s [%s]\n", i+1,
			truncate(m.Name, 30), m.Entity.GrievanceEmail, m.Evidence)
	}
	fmt.Println()

	if len(unknown) > 0 {
		fmt.Printf("  ❓ NOT IN REGISTRY: %d\n", len(unknown))
		fmt.Println("  ──────────────────────────────────────────────────────────")
		for i, u := range unknown {
			if i >= 15 {
				fmt.Printf("  ... and %d more\n", len(unknown)-15)
				break
			}
			fmt.Printf("  • %s\n", truncate(u, 65))
		}
		fmt.Println()
	}

	fmt.Println("  ═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Total: %d | Registry: %d | Unknown: %d\n",
		len(institutions), len(matched), len(unknown))
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

type bureauInstitution struct {
	Name     string
	Evidence string
}

func detectBureau(text string) string {
	upper := strings.ToUpper(text)
	if strings.Contains(upper, "CIBIL") || strings.Contains(upper, "TRANSUNION") {
		return "CIBIL (TransUnion CIBIL)"
	}
	if strings.Contains(upper, "EXPERIAN") {
		return "Experian Credit Bureau"
	}
	if strings.Contains(upper, "EQUIFAX") {
		return "Equifax Credit Information Services"
	}
	if strings.Contains(upper, "CRIF") || strings.Contains(upper, "HIGH MARK") {
		return "CRIF High Mark"
	}
	return "Unknown Bureau"
}

func parseBureauReport(text string, bureau string) []bureauInstitution {
	var results []bureauInstitution
	seen := make(map[string]bool)
	upper := strings.ToUpper(text)

	// Known institutions to search for
	knownInstitutions := []string{
		// Banks
		"HDFC BANK", "ICICI BANK", "AXIS BANK", "KOTAK MAHINDRA BANK",
		"STATE BANK OF INDIA", "INDUSIND BANK", "YES BANK", "IDBI BANK",
		"BANK OF BARODA", "PUNJAB NATIONAL BANK", "CANARA BANK",
		"UNION BANK OF INDIA", "BANK OF INDIA", "CENTRAL BANK OF INDIA",
		"INDIAN BANK", "INDIAN OVERSEAS BANK", "UCO BANK",
		"RBL BANK", "FEDERAL BANK", "BANDHAN BANK", "JAMMU AND KASHMIR BANK",
		"SOUTH INDIAN BANK", "KARUR VYSYA BANK", "CITY UNION BANK",
		"DHANLAXMI BANK", "TAMILNAD MERCANTILE BANK",
		"COOPERATIVE BANK", "REGIONAL RURAL BANK",
		// Large NBFCs
		"BAJAJ FINANCE", "TATA CAPITAL", "ADITYA BIRLA FINANCE",
		"L&T FINANCE", "MUTHOOT FINANCE", "CHOLAMANDALAM INVESTMENT",
		"HDB FINANCIAL SERVICES", "KISHT CONSUMER FINANCE", "STASHFIN",
		"RUPEEK", "NAVI FINSERV", "OF BUSINESS", "KREDITBEE",
		"SLICE", "UNI", "MONEYVIEW", "MONEYWIDTH", "EARLF",
		"RUBICON", "BERYL", "KASHIV", "ANANTA", "LAZEE",
		// Fintech
		"CRED", "PHONEPE", "PAYTM", "RAZORPAY", "PAISABAZAAR",
		"BANKBAZAAR", "INDMONEY", "ZOPPO", "CARDekho", "PROSTAGE",
		"UPSTOX", "ZERODHA", "GROWW", "ANGLE ONE", "DHAN",
		// Insurance
		"LIC", "HDFC LIFE", "SBI LIFE", "ICICI PRUDENTIAL LIFE",
		"BAJAJ ALLIANZ LIFE", "TATA AIA LIFE", "MAX LIFE INSURANCE",
		"ADITYA BIRLA SUNLIFE", "NIPPON LIFE", "KOTAK LIFE INSURANCE",
		"AVIVA LIFE", "RELIGARE LIFE", "STAR HEALTH", "NIVA BUPA",
		"BAJAJ ALLIANZ GENERAL", "HDFC ERGO", "SBI GENERAL",
		"RELIANCE GENERAL", "TATA AIG", "GO DIGIT",
		// HFCs
		"AADHAR HOUSING FINANCE", "PNB HOUSING FINANCE", "REPUBLIC HOUSING",
		"CAPITAL FIRST", "DEWAN HOUSING FINANCE", "Gruh Finance",
		// NBFCs
		"INDIFI", "AYE FINANCE", "UGRO CAPITAL",
		"SHRIRAM TRANSPORT", "MAHINDRA & MAHINDRA FINANCIAL",
		"SUNDARAM FINANCE", "SAGAR JAIN", "SUKAN",
		"IIFL", "INDIA BULLS", "EDELWEISS", "PRAKASH",
	}

	for _, name := range knownInstitutions {
		if strings.Contains(upper, name) {
			idx := strings.Index(upper, name)
			start := idx - 80
			if start < 0 {
				start = 0
			}
			end := idx + len(name) + 80
			if end > len(text) {
				end = len(text)
			}
			context := strings.ToUpper(text[start:end])

			evidence := "Listed in bureau report"
			if strings.Contains(context, "INQUIRY") || strings.Contains(context, "ENQUIRY") {
				evidence = "Hard inquiry"
			} else if strings.Contains(context, "ACCOUNT") && strings.Contains(context, "LOAN") {
				evidence = "Loan account"
			} else if strings.Contains(context, "CREDIT CARD") {
				evidence = "Credit card"
			} else if strings.Contains(context, "CLOSED") {
				evidence = "Closed account"
			} else if strings.Contains(context, "OVERDUE") || strings.Contains(context, "DPD") {
				evidence = "Active (may have overdue)"
			} else if strings.Contains(context, "WRITE OFF") || strings.Contains(context, "WRITTEN OFF") {
				evidence = "Written off account"
			}

			if !seen[name] {
				seen[name] = true
				results = append(results, bureauInstitution{
					Name:     strings.TrimSpace(name),
					Evidence: evidence,
				})
			}
		}
	}

	// Also parse "Member:" patterns
	memberRe := regexp.MustCompile(`(?i)(?:Member|Institution|Creditor|Lender)\s*[:\-]\s*([A-Z][A-Z\s\L]{3,40}(?:Ltd|Limited|Pvt|Housing|Finance|NBFC)?)`)
	for _, m := range memberRe.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			name := strings.TrimSpace(m[1])
			if len(name) > 4 && len(name) < 50 && !seen[name] {
				skip := map[string]bool{
					"THE": true, "AND": true, "FOR": true, "WITH": true,
					"YOUR": true, "ACCOUNT": true, "STATEMENT": true,
					"LOAN": true, "CREDIT": true, "FINANCE": true, "BANK": true,
				}
				upperName := strings.ToUpper(name)
				firstWord := strings.Fields(upperName)[0]
				if !skip[firstWord] {
					seen[name] = true
					results = append(results, bureauInstitution{
						Name:     name,
						Evidence: "Listed in bureau report",
					})
				}
			}
		}
	}

	// Deduplicate
	deduped := make([]bureauInstitution, 0, len(results))
	seenName := make(map[string]bool)
	for _, r := range results {
		if !seenName[r.Name] {
			seenName[r.Name] = true
			deduped = append(deduped, r)
		}
	}
	return deduped
}
