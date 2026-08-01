package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
	"github.com/spf13/cobra"
)

// GuidedConsentJourney walks a user through consent withdrawal step by step
func guidedConsentJourney() error {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║       FinWipe Consent Withdrawal Wizard                        ║")
	fmt.Println("║   Guided journey to exercise your Section 8(7) rights         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This wizard will help you withdraw consent from ANY company.")
	fmt.Println("Section 8(7) DPDP Act: They must STOP processing your data IMMEDIATELY.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Step 1: What type of entity?
	fmt.Println("STEP 1: What type of company is this?")
	fmt.Println("────────────────────────────────────────")
	fmt.Println("  1. Bank / NBFC / Fintech / Lending App")
	fmt.Println("  2. Insurance Company")
	fmt.Println("  3. Telecom / Mobile Service")
	fmt.Println("  4. EdTech / Online Course")
	fmt.Println("  5. E-commerce / Shopping App")
	fmt.Println("  6. Social Media / OTT Platform")
	fmt.Println("  7. Other / I don't know")
	fmt.Print("\nEnter number (1-7): ")

	entityType, _ := reader.ReadString('\n')
	entityType = strings.TrimSpace(entityType)

	// Step 2: Do you have an account or was your data collected without account?
	fmt.Println()
	fmt.Println("STEP 2: How was your data collected?")
	fmt.Println("────────────────────────────────────────")
	fmt.Println("  1. I had an account (login/registration)")
	fmt.Println("  2. No account - they collected data when I applied/tried their service")
	fmt.Println("  3. Through an app I downloaded")
	fmt.Println("  4. Through a website/form I filled")
	fmt.Println("  5. They scraped my data from somewhere else")
	fmt.Print("\nEnter number (1-5): ")

	collectionType, _ := reader.ReadString('\n')
	collectionType = strings.TrimSpace(collectionType)

	// Step 3: What data do you think they have?
	fmt.Println()
	fmt.Println("STEP 3: What data do you think they have?")
	fmt.Println("────────────────────────────────────────")
	fmt.Println("Select all that apply (comma-separated numbers, or 'all'):")
	fmt.Println("  1. Basic profile (name, email, phone)")
	fmt.Println("  2. KYC documents (Aadhaar, PAN, passport copies)")
	fmt.Println("  3. Financial data (bank account, transactions, income)")
	fmt.Println("  4. Location / GPS data")
	fmt.Println("  5. Call/SMS logs")
	fmt.Println("  6. App usage / behavioral data")
	fmt.Println("  7. Social connections / contacts")
	fmt.Println("  8. Marketing/ promotional preferences")
	fmt.Println("  9. Third-party shared data (from other apps/brokers)")
	fmt.Println(" 10. All of the above")
	fmt.Print("\nEnter numbers (e.g., 1,2,3 or 'all'): ")

	dataSelection, _ := reader.ReadString('\n')
	dataSelection = strings.TrimSpace(dataSelection)

	// Determine categories
	var categories []letter.DeletionCategory
	allSelected := strings.ToLower(dataSelection) == "all"

	if allSelected || strings.Contains(dataSelection, "1") {
		categories = append(categories, letter.CatMarketing)
	}
	if allSelected || strings.Contains(dataSelection, "2") {
		categories = append(categories, letter.CatKYCSupplements)
	}
	if allSelected || strings.Contains(dataSelection, "3") {
		categories = append(categories, letter.CatCreditProfile)
	}
	if allSelected || strings.Contains(dataSelection, "4") {
		categories = append(categories, letter.CatLocation)
	}
	if allSelected || strings.Contains(dataSelection, "5") {
		categories = append(categories, letter.CatCallRecords, letter.CatSMSLogs)
	}
	if allSelected || strings.Contains(dataSelection, "6") {
		categories = append(categories, letter.CatAppUsage, letter.CatBehavioral)
	}
	if allSelected || strings.Contains(dataSelection, "7") {
		categories = append(categories, letter.CatThirdParty)
	}
	if allSelected || strings.Contains(dataSelection, "8") {
		categories = append(categories, letter.CatMarketingPref)
	}
	if allSelected || strings.Contains(dataSelection, "9") {
		categories = append(categories, letter.CatThirdParty)
	}

	if len(categories) == 0 {
		categories = []letter.DeletionCategory{letter.CatMarketing, letter.CatThirdParty, letter.CatAppUsage, letter.CatMarketingPref}
	}

	// Step 4: Your details
	fmt.Println()
	fmt.Println("STEP 4: Your details for the request")
	fmt.Println("────────────────────────────────────────")

	var name, email, phone string

	// Check for existing profile
	cfgPath := filepath.Join(os.Getenv("HOME"), ".finwipe", "config.yaml")
	if cfg, err := config.Load(cfgPath); err == nil && cfg.Profile.Name != "" {
		name = cfg.Profile.Name
		email = cfg.Profile.Email
		phone = cfg.Profile.Phone
		fmt.Printf("(Using saved profile: %s | %s)\n", name, email)
	}

	if name == "" {
		fmt.Print("Your full legal name: ")
		name, _ = reader.ReadString('\n')
		name = strings.TrimSpace(name)
	}

	if email == "" {
		fmt.Print("Your email (used with this company): ")
		email, _ = reader.ReadString('\n')
		email = strings.TrimSpace(email)
	}

	if phone == "" {
		fmt.Print("Your phone number: ")
		phone, _ = reader.ReadString('\n')
		phone = strings.TrimSpace(phone)
	}

	// Step 5: Company name
	fmt.Println()
	fmt.Println("STEP 5: Company details")
	fmt.Println("────────────────────────────────────────")

	var companyName, companyWebsite string

	fmt.Print("Company/Brand name: ")
	companyName, _ = reader.ReadString('\n')
	companyName = strings.TrimSpace(companyName)

	fmt.Print("Website (optional): ")
	companyWebsite, _ = reader.ReadString('\n')
	companyWebsite = strings.TrimSpace(companyWebsite)

	// Step 6: Account ID if applicable
	var accountID string
	if collectionType == "1" || collectionType == "3" {
		fmt.Println()
		fmt.Print("Account ID / User ID / Phone number used (if any): ")
		accountID, _ = reader.ReadString('\n')
		accountID = strings.TrimSpace(accountID)
	}

	// Step 7: Generate the request
	fmt.Println()
	fmt.Println("STEP 6: Generating your consent withdrawal request...")
	fmt.Println("────────────────────────────────────────")

	profile := config.Profile{
		Name:    name,
		Email:   email,
		Phone:   phone,
		Address: "India",
	}

	// Generate email body
	emailBody := letter.GenerateEmailBody(
		"DPR-WIZARD-"+fmt.Sprintf("%d", time.Now().Unix()%1000000),
		companyName,
		profile,
		categories,
		letter.LegalBasisWithdrawal,
	)

	// Print the email
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  YOUR CONSENT WITHDRAWAL EMAIL - COPY AND SEND THIS          ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Extract just the body (remove headers)
	lines := strings.Split(emailBody, "\n")
	var bodyStart int
	for i, line := range lines {
		if strings.Contains(line, "Dear") || strings.Contains(line, "Grievance") {
			bodyStart = i - 1
			break
		}
	}
	if bodyStart > 0 && bodyStart < len(lines) {
		emailBody = strings.Join(lines[bodyStart:], "\n")
	}

	fmt.Println(emailBody)

	// Step 8: What to do next
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  WHAT TO DO NEXT                                                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("1. COPY the email above (Ctrl+A, Ctrl+C)")
	fmt.Println("2. SEND IT TO:")
	if companyWebsite != "" {
		fmt.Printf("   - Privacy/Grievance officer email on: %s\n", companyWebsite)
		fmt.Println("   - Or search for: [company name] grievance officer email")
	} else {
		fmt.Println("   - Search for: [company name] grievance officer email")
		fmt.Println("   - Or use their contact form")
	}
	fmt.Println()
	fmt.Println("3. EXPECTED RESPONSE:")
	fmt.Println("   - As soon as reasonable: Acknowledgment (Section 8(6), DPDP Act 2023)")
	fmt.Println("   - As soon as reasonable: Erasure where applicable of data deletion")
	fmt.Println()
	fmt.Println("4. IF NO RESPONSE AFTER 30 DAYS:")
	fmt.Println("   - Escalate to Data Protection Board of India (DPBB)")
	fmt.Println("   - File complaint at: https://legalcry.gitbook.io/dpbb/")
	fmt.Println("   - Or approach relevant regulator (RBI/SEBI/IRDAI)")
	fmt.Println()

	// Ask to save
	fmt.Print("Save this email to a file? (y/n): ")
	saveChoice, _ := reader.ReadString('\n')
	saveChoice = strings.TrimSpace(strings.ToLower(saveChoice))

	if saveChoice == "y" {
		home, _ := os.UserHomeDir()
		saveDir := filepath.Join(home, ".finwipe", "letters")
		os.MkdirAll(saveDir, 0700)

		filename := filepath.Join(saveDir, fmt.Sprintf("consent-withdrawal-%s-%s.txt",
			strings.ReplaceAll(companyName, " ", "-"),
			time.Now().Format("2006-01-02")))

		// Add headers to saved file
		fullEmail := fmt.Sprintf("To: %s\nSubject: CONSENT WITHDRAWAL — Section 8(7), DPDP Act 2023\n\n%s",
			getGrievanceEmail(companyName),
			emailBody)

		if err := os.WriteFile(filename, []byte(fullEmail), 0600); err == nil {
			fmt.Printf("\nSaved to: %s\n", filename)
		}
	}

	// Ask if they want to track this
	fmt.Println()
	fmt.Print("Do you want to track this request with FinWipe? (y/n): ")
	trackChoice, _ := reader.ReadString('\n')
	trackChoice = strings.TrimSpace(strings.ToLower(trackChoice))

	if trackChoice == "y" {
		// Check if finwipe is initialized
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			fmt.Println("\nRun 'finwipe init' first to set up tracking.")
			fmt.Println("Then use: finwipe new --name ... --email ... --nbfc [company-id]")
		} else {
			fmt.Println("\nTo track this request:")
			fmt.Printf("  finwipe init  # if not done already\n")
			fmt.Printf("  finwipe new --name %q --email %q --nbfc [company-id]\n", name, email)
			fmt.Println("  finwipe send  # when ready to send")
		}
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════════")
	fmt.Println("  Remember: They must STOP processing your data IMMEDIATELY")
	fmt.Println("  upon receiving your withdrawal of consent.")
	fmt.Println("════════════════════════════════════════════════════════════════════")

	return nil
}

// getGrievanceEmail tries to find the grievance officer email for a company
func getGrievanceEmail(companyName string) string {
	if entities, err := nbfc.Load(""); err == nil {
		lower := strings.ToLower(companyName)
		for _, e := range entities {
			if strings.Contains(strings.ToLower(e.Name), lower) ||
				strings.Contains(lower, strings.ToLower(e.Name)) ||
				strings.Contains(strings.ToLower(e.ShortName), lower) {
				if e.GrievanceEmail != "" {
					return e.GrievanceEmail
				}
			}
		}
	}

	// Default placeholder
	return "[grievance-officer@" + strings.ToLower(strings.ReplaceAll(companyName, " ", "")) + ".com]"
}

var askCmd = &cobra.Command{
	Use:   "ask",
	Short: "Interactive consent withdrawal wizard",
	Long: `Guided journey to withdraw consent from any company.

Works WITHOUT needing finwipe init - just answer questions.

Example:
  finwipe ask

This wizard will:
  1. Ask what type of company and data is involved
  2. Generate a proper Section 8(7) withdrawal email
  3. Show you exactly what to send and to whom
  4. Explain the next steps if no response`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return guidedConsentJourney()
	},
}
