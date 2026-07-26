package letter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

// DeletionCategory represents a category of data that can be requested for deletion
type DeletionCategory string

const (
	CatMarketing         DeletionCategory = "marketing_data"
	CatThirdParty       DeletionCategory = "third_party_shared"
	CatBehavioral       DeletionCategory = "behavioral_analytics"
	CatAppUsage         DeletionCategory = "app_usage_metadata"
	CatCallRecords      DeletionCategory = "call_interaction_logs"
	CatSMSLogs          DeletionCategory = "sms_communication_logs"
	CatLocation         DeletionCategory = "location_cell_metadata"
	CatEmployment       DeletionCategory = "employment_proof_data"
	CatMedical          DeletionCategory = "medical_health_records"
	CatNominee          DeletionCategory = "nominee_personal_data"
	CatCreditProfile    DeletionCategory = "credit_profile_shared"
	CatKYCSupplements   DeletionCategory = "kyc_supplementary_data"
	CatMarketingPref   DeletionCategory = "promotional_preferences"
	CatAllNonEssential  DeletionCategory = "all_non_essential_data"
)

// DeletionCategoryLabel returns a human-readable label
func (c DeletionCategory) Label() string {
	return map[DeletionCategory]string{
		CatMarketing:        "Marketing & Promotional Data",
		CatThirdParty:       "Third-Party Shared Data",
		CatBehavioral:       "Behavioral/Analytics Data",
		CatAppUsage:         "App Usage & Metadata",
		CatCallRecords:      "Call Records & Interaction Logs",
		CatSMSLogs:          "SMS Communication Logs",
		CatLocation:         "Location & Cell Tower Metadata",
		CatEmployment:       "Employment & Income Proof Data",
		CatMedical:          "Medical & Health Records",
		CatNominee:          "Nominee & Beneficiary Data",
		CatCreditProfile:    "Credit Profile Shared with Third Parties",
		CatKYCSupplements:   "KYC Supplementary Documents",
		CatMarketingPref:    "Marketing & Communication Preferences",
		CatAllNonEssential:  "All Non-Essential Personal Data",
	}[c]
}

// DeletionCategories is the default set of categories for financial entities
var DefaultDeletionCategories = []DeletionCategory{
	CatMarketing,
	CatThirdParty,
	CatBehavioral,
	CatAppUsage,
	CatMarketingPref,
}

// InsuranceDeletionCategories for insurance companies
var InsuranceDeletionCategories = []DeletionCategory{
	CatMedical,
	CatNominee,
	CatEmployment,
	CatThirdParty,
	CatBehavioral,
	CatMarketingPref,
}

// TelecomDeletionCategories for telecom operators
var TelecomDeletionCategories = []DeletionCategory{
	CatCallRecords,
	CatSMSLogs,
	CatLocation,
	CatBehavioral,
	CatAppUsage,
	CatMarketing,
}

// Generator creates professional PDF deletion letters
type Generator struct {
	outputDir string
}

// New creates a new letter generator
func New(outputDir string) *Generator {
	os.MkdirAll(outputDir, 0755)
	return &Generator{outputDir: outputDir}
}

// Generate creates a professional PDF deletion letter
func (g *Generator) Generate(reqID, entityName, grievanceEmail string, profile config.Profile, categories []DeletionCategory) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, "PRIVACY DATA DELETION REQUEST", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "Under Section 8(6), Digital Personal Data Protection Act, 2023", "", 1, "C", false, 0, "")

	pdf.Ln(5)

	// Request Reference
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("Request Reference: %s", reqID), "", 1, "R", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, fmt.Sprintf("Date: %s", time.Now().Format("02 January 2006")), "", 1, "R", false, 0, "")
	pdf.Ln(3)

	// Divider
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(5)

	// To
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "To:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, entityName, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 5, fmt.Sprintf("Grievance Officer: %s", grievanceEmail), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// From
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "From:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, profile.Name+"\nEmail: "+profile.Email+"\nPhone: "+profile.Phone+"\n"+profile.Address, "", "L", false)
	pdf.Ln(3)

	// Subject
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "Subject: Request for Erasure of Personal Data under Section 8(6), DPDP Act, 2023", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Body
	body := `I, the undersigned, am exercising my right to erasure of personal data under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act) and Rule 8(5) of the DPDP Rules, 2025.

I request deletion of the following categories of personal data held by your organization in connection with my relationship/service with you:`

	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, body, "", "L", false)
	pdf.Ln(3)

	// Checkboxes for categories
	for _, cat := range categories {
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(5, 5, "☐", "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 5, cat.Label(), "", 1, "L", false, 0, "")
	}

	pdf.Ln(3)

	// Exclusions
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Data Excluded from This Request:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	exclusions := `I understand that the following data may be retained as permitted under Section 9 of the DPDP Act:
• KYC documents and identification records as required by law
• Transaction records required for tax/compliance purposes
• Data required to be maintained under any other law for the time being in force`

	pdf.MultiCell(0, 5, exclusions, "", "L", false)
	pdf.Ln(3)

	// Verification and Acknowledgment
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Acknowledgment and Timeline:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	ack := `As required under Rule 8(3) of the DPDP Rules, 2025, please acknowledge receipt of this request within 48 hours.

As required under Rule 8(5), please complete the erasure of personal data within 30 days of this request.

Please confirm in writing (email preferred) once the deletion is complete, specifying the scope of data deleted.`
	pdf.MultiCell(0, 5, ack, "", "L", false)
	pdf.Ln(3)

	// Non-Compliance Consequence
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Note on Non-Compliance:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	note := `Failure to comply with this request within the stipulated timeline constitutes a violation of the DPDP Act, 2023. I reserve the right to escalate this matter to:
• The Data Protection Board of India (once operational), or
• The relevant sectoral regulator (RBI/IRDAI/SEBI/TRAI), or
• A consumer forum having jurisdiction.`
	pdf.MultiCell(0, 5, note, "", "L", false)
	pdf.Ln(3)

	// Footer
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(3)
	pdf.SetFont("Arial", "I", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated by FinWipe | %s | This is an automatically generated request.", reqID), "", 1, "C", false, 0, "")

	// Save
	filename := fmt.Sprintf("DPDPA_Deletion_%s_%s.pdf",
		strings.ReplaceAll(reqID, "/", "_"),
		strings.ReplaceAll(entityName, " ", "_"))
	path := filepath.Join(g.outputDir, filename)
	err := pdf.OutputFileAndClose(path)
	return path, err
}

// GenerateBatch creates multiple letters
func (g *Generator) GenerateBatch(entities []nbfc.Entity, profile config.Profile, categories []DeletionCategory) (generated int, failed []string) {
	for _, e := range entities {
		if e.GrievanceEmail == "" {
			failed = append(failed, e.Name+" (no grievance email)")
			continue
		}
		reqID := fmt.Sprintf("DPR-%d-XXXXXX", time.Now().Year())
		_, err := g.Generate(reqID, e.Name, e.GrievanceEmail, profile, categories)
		if err != nil {
			failed = append(failed, e.Name+": "+err.Error())
		} else {
			generated++
		}
	}
	return
}

// GenerateEmailBody creates a plain-text email body for the deletion request
func GenerateEmailBody(reqID, entityName string, profile config.Profile, categories []DeletionCategory) string {
	var catList strings.Builder
	for _, cat := range categories {
		catList.WriteString("  ☐ " + cat.Label() + "\n")
	}

	return fmt.Sprintf(`Subject: DPDPA Section 8(6) — Request for Erasure of Personal Data [%s]

To,
%s
Grievance Officer

Request Reference: %s
Date: %s

Dear Grievance Officer,

I, %s, exercising my right to erasure under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act) and Rule 8(5) of the DPDP Rules, 2025, hereby request deletion of the following categories of personal data held by your organization:

%s
I understand that the following data may be retained as permitted under Section 9 of the DPDP Act:
  ☐ KYC documents required by law
  ☐ Transaction records for tax/compliance purposes
  ☐ Data required under any other law

As required under Rule 8(3), please acknowledge receipt within 48 hours.
As required under Rule 8(5), please complete deletion within 30 days.

Failure to comply will result in escalation to the Data Protection Board of India and/or relevant sectoral regulator.

Contact: %s | %s | %s

Regards,
%s

---
Generated by FinWipe | %s`,
		reqID,
		entityName,
		reqID,
		time.Now().Format("02 January 2006"),
		profile.Name,
		catList.String(),
		profile.Email,
		profile.Phone,
		profile.Address,
		profile.Name,
		reqID)
}

// GenerateFollowupBody creates a follow-up email for requests not acknowledged
func GenerateFollowupBody(reqID, entityName string, profile config.Profile, dayNum int) string {
	return fmt.Sprintf(`Subject: FOLLOW-UP [%d] — DPDPA Deletion Request Not Acknowledged [%s]

Dear Grievance Officer,

This is a follow-up (day %d) to my earlier request dated [ORIGINAL DATE] under Section 8(6), DPDP Act 2023.

Request Reference: %s

I have not received acknowledgment of my deletion request within the 48-hour timeline mandated under Rule 8(3) of the DPDP Rules, 2025.

I hereby reiterate my request for:
  ☐ Marketing & Promotional Data
  ☐ Third-Party Shared Data
  ☐ Behavioral/Analytics Data
  ☐ App Usage Metadata

Please treat this as priority and confirm receipt and expected deletion timeline immediately.

If no response is received within 7 days, I will escalate to the Data Protection Board of India.

Regards,
%s | %s

---
Generated by FinWipe | %s`, dayNum, reqID, dayNum, reqID, profile.Name, profile.Email, reqID)
}

// GenerateRBIComplaint generates a RBI Ombudsman complaint letter
func GenerateRBIComplaint(reqID string, entity *nbfc.Entity, profile config.Profile) (string, string, error) {
	var body bytes.Buffer
	body.WriteString(fmt.Sprintf(`To,
The Appellate Officer / Principal Nodal Officer
RBI Ombudsman

Complainant: %s
Email: %s
Phone: %s
Address: %s

Date: %s

Subject: Complaint under Clause __(_) of the [Banking|Financial Services|Ombudsman] Scheme, 202_ — Non-compliance with DPDPA 2023 Deletion Request

Reference Request: %s
Entity Complained Against: %s (%s)

Dear Sir/Madam,

I submitted a Data Erasure Request under Section 8(6), DPDP Act 2023 to %s on [DATE] (Ref: %s).

The entity has failed to:
1. Acknowledge the request within 48 hours (Rule 8(3))
2. Complete deletion within 30 days (Rule 8(5))
3. Provide any substantive response to repeated follow-ups

I therefore file this complaint requesting your intervention to ensure compliance with the DPDP Act, 2023.

Supporting documents attached:
  1. Original deletion request [%s]
  2. Follow-up communications (if any)
  3. Evidence of no response / unsatisfactory response

I request that you direct %s to comply with my deletion request and submit a compliance report.

Regards,
%s`, profile.Name, profile.Email, profile.Phone, profile.Address,
		time.Now().Format("02 January 2006"),
		reqID,
		entity.Name, entity.GrievanceEmail,
		entity.Name, reqID,
		reqID,
		entity.Name,
		profile.Name))

	// Generate PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, "RBI OMBUDSMAN / APPELLATE AUTHORITY COMPLAINT", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, body.String(), "", "L", false)

	filename := fmt.Sprintf("RBI_Complaint_%s_%s.pdf",
		strings.ReplaceAll(reqID, "/", "_"),
		strings.ReplaceAll(entity.Name, " ", "_"))
	path := filepath.Join(os.TempDir(), filename)
	err := pdf.OutputFileAndClose(path)
	return path, body.String(), err
}

// GenerateInsuranceLetter creates a letter specific to insurance companies
func (g *Generator) GenerateInsuranceLetter(reqID string, entity *nbfc.Entity, profile config.Profile, policyNumber string) (string, error) {
	return g.Generate(reqID, entity.Name, entity.GrievanceEmail, profile, InsuranceDeletionCategories)
}

// GenerateTelecomLetter creates a letter specific to telecom operators
func (g *Generator) GenerateTelecomLetter(reqID string, entity *nbfc.Entity, profile config.Profile, msisdn string) (string, error) {
	// Telecom letters include call/SMS metadata
	categories := TelecomDeletionCategories
	return g.Generate(reqID, entity.Name, entity.GrievanceEmail, profile, categories)
}

// AllDeletionCategories returns all available deletion categories
func AllDeletionCategories() []DeletionCategory {
	return []DeletionCategory{
		CatMarketing,
		CatThirdParty,
		CatBehavioral,
		CatAppUsage,
		CatCallRecords,
		CatSMSLogs,
		CatLocation,
		CatEmployment,
		CatMedical,
		CatNominee,
		CatCreditProfile,
		CatKYCSupplements,
		CatMarketingPref,
		CatAllNonEssential,
	}
}
