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

// LegalBasis represents the legal basis for the deletion request
type LegalBasis string

const (
	// LegalBasisWithdrawal uses Section 8(7) DPDPA - consent withdrawal (broadest applicability)
	LegalBasisWithdrawal LegalBasis = "dpdp_87"
	// LegalBasisErasure uses Section 8(6) DPDPA - erasure of publicly available data via fraud
	LegalBasisErasure LegalBasis = "dpdp_86"
	// LegalBasisRBI uses RBI Digital Lending Guidelines 2022 - for NBFCs/banks specifically
	LegalBasisRBI LegalBasis = "rbi_dlg"
	// LegalBasisBoth uses both DPDPA 2023 and RBI DLG 2022
	LegalBasisBoth LegalBasis = "both"
	// LegalBasisAccess uses Section 6 - right to access (for data inventory requests)
	LegalBasisAccess LegalBasis = "dpdp_6"
)

// LegalBasisLabel returns human-readable label
func (l LegalBasis) Label() string {
	return map[LegalBasis]string{
		LegalBasisWithdrawal: "DPDP Act 2023 — Sections 8(7) withdrawal + 8(6) erasure",
		LegalBasisErasure: "DPDP Act 2023 — Section 8(6) erasure",
		LegalBasisAccess: "DPDP Act 2023 — Section 6 right to access",
		LegalBasisRBI:  "RBI Digital Lending Guidelines 2022",
		LegalBasisBoth: "DPDP Act 2023 + RBI Digital Lending Guidelines 2022",
	}[l]
}

// LegalBasisDescription returns the legal citation
func (l LegalBasis) Description() string {
	return map[LegalBasis]string{
		LegalBasisWithdrawal: "Section 8(7): Cease processing immediately upon consent withdrawal\nSection 8(6): Erasure of data made publicly via fraud/misrepresentation",
		LegalBasisErasure: "Section 8(6): Erasure of publicly available data obtained via fraud/misrepresentation",
		LegalBasisAccess: "Section 6: Right to access — complete inventory of personal data",
		LegalBasisRBI:  "RBI Digital Lending Guidelines 2022 — Para 10.2, 11.1, 11.2",
		LegalBasisBoth: "Section 8(7): Consent withdrawal (DPDP Act)\nRBI DLG 2022: Paras 10.2, 11.1, 11.2 (data sharing obligations)",
	}[l]
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
func (g *Generator) Generate(reqID, entityName, grievanceEmail string, profile config.Profile, categories []DeletionCategory, legalBasis LegalBasis) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Header - varies by legal basis
	pdf.SetFont("Arial", "B", 16)
	if legalBasis == LegalBasisRBI {
		pdf.CellFormat(0, 8, "DATA DELETION REQUEST — RBI DIGITAL LENDING", "", 1, "C", false, 0, "")
	} else {
		pdf.CellFormat(0, 8, "PRIVACY DATA DELETION REQUEST", "", 1, "C", false, 0, "")
	}
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, legalBasis.Description(), "", 1, "C", false, 0, "")

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

	// Subject - varies by legal basis
	pdf.SetFont("Arial", "B", 11)
	if legalBasis == LegalBasisRBI {
		pdf.CellFormat(0, 6, "Subject: Request for Deletion of Personal Data — RBI Digital Lending Guidelines 2022 (Para 10.2, 11.1, 11.2)", "", 1, "L", false, 0, "")
	} else {
		pdf.CellFormat(0, 6, "Subject: Request for Erasure of Personal Data under Section 8(6), DPDP Act, 2023", "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)

	// Body - varies by legal basis
	var body string
	if legalBasis == LegalBasisRBI {
		body = `I, the undersigned, am exercising my right to request deletion of personal data under the RBI Digital Lending Guidelines, 2022.

Specifically:
• Para 10.2: "The DSP shall not retain data, whether directly or through any third party, beyond the period necessary for the purpose for which it was collected."
• Para 11.1: "Data shared with the DSP shall be used only for the purpose for which it was obtained."
• Para 11.2: "Data collected shall be deleted after the purpose is served."

I request deletion of the following categories of personal data held by your organization in excess of what is necessary for regulatory compliance:`
	} else if legalBasis == LegalBasisBoth {
		body = `I, the undersigned, am exercising my rights under:
(A) Sections 8(6) & 8(7) of the Digital Personal Data Protection Act, 2023 (DPDP Act) — Right to Erasure, and
(B) RBI Digital Lending Guidelines, 2022 — Para 10.2, 11.1, 11.2 — Data minimization and deletion

I request deletion of the following categories of personal data held by your organization in connection with my relationship/service with you:`
	} else {
		body = `I, the undersigned, am exercising my right to erasure of personal data under Sections 8(6) & 8(7) of the Digital Personal Data Protection Act, 2023 (DPDP Act) 

I request deletion of the following categories of personal data held by your organization in connection with my relationship/service with you:`
	}

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

	// Exclusions - varies by legal basis
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Data Excluded from This Request:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	var exclusions string
	if legalBasis == LegalBasisRBI {
		exclusions = `The following may be retained as permitted under applicable law:
• KYC documents as required by PMLA/AML regulations
• Transaction records required for tax/compliance purposes
• Data required under any other law for the time being in force`
	} else {
		exclusions = `I understand that the following data may be retained as permitted under Section 9 of the DPDP Act:
• KYC documents and identification records as required by law
• Transaction records required for tax/compliance purposes
• Data required to be maintained under any other law for the time being in force`
	}
	pdf.MultiCell(0, 5, exclusions, "", "L", false)
	pdf.Ln(3)

	// Verification and Acknowledgment - varies by legal basis
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Acknowledgment and Timeline:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	var ack string
	if legalBasis == LegalBasisRBI {
		ack = `As per RBI Digital Lending Guidelines, please acknowledge receipt as soon as reasonably practicable.

Please complete the deletion of unnecessary personal data as soon as reasonably practicable.

Please confirm in writing (email preferred) once the deletion is complete, specifying the scope of data deleted.`
	} else {
		ack = `Under Section 8(7) of the DPDP Act 2023, you must cease processing upon receipt of this withdrawal. Please acknowledge as soon as reasonably practicable and confirm erasure completion as soon as reasonably practicable.

As required under Section 8(7) DPDP Act 2023, erasure must be completed as soon as reasonably practicable of this request.

Please confirm in writing (email preferred) once the deletion is complete, specifying the scope of data deleted.`
	}
	pdf.MultiCell(0, 5, ack, "", "L", false)
	pdf.Ln(3)

	// Non-Compliance Consequence
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "Note on Non-Compliance:", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	var note string
	if legalBasis == LegalBasisBoth {
		note = `Failure to comply with this request constitutes a violation of both:
(A) DPDP Act, 2023 — Section 8(6), and
(B) RBI Digital Lending Guidelines, 2022 — Para 10.2, 11.1, 11.2

I reserve the right to escalate this matter to:
• The Data Protection Board of India (Section 27(3), DPDP Act), or
• The RBI Ombudsman / relevant sectoral regulator, or
• A consumer forum having jurisdiction.`
	} else {
		note = `Failure to comply with this request within the stipulated timeline constitutes a violation of the ` + legalBasis.Label() + `. I reserve the right to escalate this matter to:
• The Data Protection Board of India (Section 27(3), DPDP Act), or
• The relevant sectoral regulator (RBI/IRDAI/SEBI/TRAI), or
• A consumer forum having jurisdiction.`
	}
	pdf.MultiCell(0, 5, note, "", "L", false)
	pdf.Ln(3)

	// Footer
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(3)
	pdf.SetFont("Arial", "I", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated by FinWipe | %s | Legal Basis: %s", reqID, legalBasis.Label()), "", 1, "C", false, 0, "")

	// Save
	prefix := "DPDPA_Deletion"
	if legalBasis == LegalBasisRBI {
		prefix = "RBI_Deletion"
	} else if legalBasis == LegalBasisBoth {
		prefix = "Dual_Deletion"
	}
	filename := fmt.Sprintf("%s_%s_%s.pdf",
		prefix,
		strings.ReplaceAll(reqID, "/", "_"),
		strings.ReplaceAll(entityName, " ", "_"))
	path := filepath.Join(g.outputDir, filename)
	err := pdf.OutputFileAndClose(path)
	return path, err
}

// GenerateBatch creates multiple letters
func (g *Generator) GenerateBatch(entities []nbfc.Entity, profile config.Profile, categories []DeletionCategory, legalBasis LegalBasis) (generated int, failed []string) {
	for _, e := range entities {
		if e.GrievanceEmail == "" {
			failed = append(failed, e.Name+" (no grievance email)")
			continue
		}
		reqID := fmt.Sprintf("DPR-%d-XXXXXX", time.Now().Year())
		_, err := g.Generate(reqID, e.Name, e.GrievanceEmail, profile, categories, legalBasis)
		if err != nil {
			failed = append(failed, e.Name+": "+err.Error())
		} else {
			generated++
		}
	}
	return
}

// GenerateEmailBody creates a plain-text email body for the deletion request
func GenerateEmailBody(reqID, entityName string, profile config.Profile, categories []DeletionCategory, legalBasis LegalBasis) string {
	var catList strings.Builder
	for _, cat := range categories {
		catList.WriteString("  ☐ " + cat.Label() + "\n")
	}

	// Subject and body vary by legal basis
	var subject, body string
	if legalBasis == LegalBasisRBI {
		subject = fmt.Sprintf("DATA DELETION REQUEST — RBI Digital Lending Guidelines 2022 [%s]", reqID)
		body = fmt.Sprintf(`I, %s, exercising my rights under the RBI Digital Lending Guidelines, 2022 (Para 10.2, 11.1, 11.2), hereby request deletion of the following categories of personal data held by your organization:

%s

Under RBI DLG Para 10.2, data shall not be retained beyond the period necessary for the purpose for which it was collected.
Under RBI DLG Para 11.1, data shall be used only for the purpose for which it was obtained.
Under RBI DLG Para 11.2, data shall be deleted after the purpose is served.

Please acknowledge as soon as reasonably practicable and complete deletion as soon as reasonably practicable.

Contact: %s | %s | %s

Regards,
%s

---
Generated by FinWipe | %s | Legal Basis: RBI Digital Lending Guidelines 2022`,
			profile.Name,
			catList.String(),
			profile.Email,
			profile.Phone,
			profile.Address,
			profile.Name,
			reqID)
	} else if legalBasis == LegalBasisBoth {
		subject = fmt.Sprintf("DUAL LEGAL BASIS — DPDPA + RBI DLG — Deletion Request [%s]", reqID)
		body = fmt.Sprintf(`I, %s, exercising my rights under:
(A) Sections 8(6) & 8(7), Digital Personal Data Protection Act, 2023, and
(B) RBI Digital Lending Guidelines, 2022 (Para 10.2, 11.1, 11.2)

hereby request deletion of the following categories of personal data held by your organization:

%s

As required under Section 8(6), please acknowledge within as soon as reasonably practicable.
Under Section 8(7) DPDP Act: complete erasure as soon as reasonably practicable.

Non-compliance constitutes violation of both DPDP Act 2023 and RBI DLG 2022.

Contact: %s | %s | %s

Regards,
%s

---
Generated by FinWipe | %s | Legal Basis: DPDPA 2023 + RBI DLG 2022`,
			profile.Name,
			catList.String(),
			profile.Email,
			profile.Phone,
			profile.Address,
			profile.Name,
			reqID)
	} else if legalBasis == LegalBasisAccess {
		subject = fmt.Sprintf("DATA ACCESS REQUEST — Section 6, DPDP Act 2023 [%s]", reqID)
		body = fmt.Sprintf(`I, %s, exercising my right to access my personal data under Section 6 of the Digital Personal Data Protection Act, 2023 (DPDP Act), hereby request:

1. A COMPLETE INVENTORY of all personal data your organization holds about me
2. THE PURPOSES for which each category of data is processed
3. THE RECIPIENTS with whom my data has been shared
4. THE RETENTION PERIOD for each data category
5. THE SOURCE of my personal data (where collected from)

I understand this request is free and must be fulfilled as soon as reasonably practicable under Section 6(3) DPDP Act.

Contact: %s | %s | %s

Regards,
%s

Legal Basis: Section 6, DPDP Act 2023
Generated by FinWipe | %s`,
			profile.Name,
			profile.Email,
			profile.Phone,
			profile.Name,
			legalBasis.Label(),
			reqID)
	} else {
		// LegalBasisWithdrawal - Section 8(7) as primary, 8(6) as secondary
		subject = fmt.Sprintf("CONSENT WITHDRAWAL + ERASURE — Section 8(7) DPDP Act 2023 [%s]", reqID)
		body = fmt.Sprintf(`I, %s, exercising my rights under the Digital Personal Data Protection Act, 2023 (DPDP Act):

PRIMARY RIGHT — Section 8(7): WITHDRAWAL OF CONSENT
I hereby withdraw all consent granted to %s for processing my personal data.
You must cease ALL PROCESSING immediately upon receipt. 
You must CEASE ALL PROCESSING immediately upon receipt of this notice.

SECONDARY RIGHT — Section 8(6): ERASURE
I also request erasure of personal data made publicly available through misrepresentation or fraud.

SCOPE OF REQUEST — Delete the following categories of data held about me:

%s

PERMITTED RETENTION (Section 9) — you may retain ONLY:
  ☐ KYC documents required by law (e.g., PAN, Aadhaar for RBI compliance)
  ☐ Transaction records required for tax/tribunal proceedings  
  ☐ Data retained under any court order

MANDATORY TIMELINES:
  • Acknowledge: Within as soon as reasonably practicable of receipt
  • Cease processing: IMMEDIATELY upon consent withdrawal
  • Complete erasure: As soon as reasonable (Section 8(6), DPDP Act 2023)

NON-COMPLIANCE CONSEQUENCE:
Failure to comply will result in complaint to the Data Protection Board of India (DPBB) 
under Section 27, and to the relevant sectoral regulator (RBI/SEBI/IRDAI as applicable).

Contact: %s | %s | %s

Regards,
%s

Legal Basis: Section 8(7) primary + Section 8(6), DPDP Act 2023
Generated by FinWipe | %s`,
			profile.Name,
			entityName,
			catList.String(),
			profile.Email,
			profile.Phone,
			profile.Name,
			legalBasis.Label(),
			reqID)
	}

	return fmt.Sprintf("Subject: %s\n\nTo,\n%s\nGrievance Officer\n\nRequest Reference: %s\nDate: %s\n\nDear Grievance Officer,\n\n%s\n\nRegards,\n%s\n\n---\nGenerated by FinWipe",
		subject,
		entityName,
		reqID,
		time.Now().Format("02 January 2006"),
		body,
		profile.Name)
}


// GenerateFollowupBody creates a follow-up email for requests not acknowledged
func GenerateFollowupBody(reqID, entityName string, profile config.Profile, dayNum int, legalBasis LegalBasis) string {
	var subject, body string
	if legalBasis == LegalBasisRBI {
		subject = fmt.Sprintf("FOLLOW-UP [%d] — RBI DLG Deletion Request — NOT ACKNOWLEDGED [%s]", dayNum, reqID)
		body = fmt.Sprintf(`This is a follow-up (day %d) to my earlier request dated [ORIGINAL DATE] under RBI Digital Lending Guidelines 2022.

Request Reference: %s

I have not received acknowledgment of my deletion request within as soon as reasonably practicable.

I hereby reiterate my request for deletion of personal data under RBI DLG Para 10.2, 11.1, 11.2.

Please treat this as priority and confirm receipt and expected deletion timeline immediately.

If no response is received within 7 days, I will escalate to the RBI Ombudsman.`, dayNum, reqID)
	} else if legalBasis == LegalBasisBoth {
		subject = fmt.Sprintf("URGENT FOLLOW-UP [%d] — DPDPA + RBI DLG Deletion — NOT ACKNOWLEDGED [%s]", dayNum, reqID)
		body = fmt.Sprintf(`This is a follow-up (day %d) to my earlier request dated [ORIGINAL DATE] under both:
(A) Sections 8(6) & 8(7), DPDP Act 2023, and
(B) RBI Digital Lending Guidelines 2022

Request Reference: %s

I have not received acknowledgment within the reasonable acknowledgment period.

I hereby reiterate my request for deletion of personal data.

If no response is received within 7 days, I will escalate to both the Data Protection Board of India and the RBI Ombudsman.`, dayNum, reqID)
	} else {
		subject = fmt.Sprintf("FOLLOW-UP [%d] — DPDPA Deletion Request Not Acknowledged [%s]", dayNum, reqID)
		body = fmt.Sprintf(`This is a follow-up (day %d) to my earlier request dated [ORIGINAL DATE] under Sections 8(6) & 8(7), DPDP Act 2023.

Request Reference: %s

I have not received acknowledgment of my deletion request within the reasonable acknowledgment period under Section 8(6) of the DPDP Act 2023.

I hereby reiterate my request for:
  ☐ Marketing & Promotional Data
  ☐ Third-Party Shared Data
  ☐ Behavioral/Analytics Data
  ☐ App Usage Metadata

Please treat this as priority and confirm receipt and expected deletion timeline immediately.

If no response is received within 7 days, I will escalate to the Data Protection Board of India.`, dayNum, reqID)
	}

	return fmt.Sprintf("Subject: %s\n\nDear Grievance Officer,\n\n%s\n\nRegards,\n%s | %s\n\n---\nGenerated by FinWipe | %s | Legal Basis: %s",
		subject, body, profile.Name, profile.Email, reqID, legalBasis.Label())
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

I submitted a Data Erasure Request under Sections 8(6) & 8(7), DPDP Act 2023 to %s on [DATE] (Ref: %s).

The entity has failed to:
1. Acknowledge the request within as soon as reasonably practicable (Section 8(6))
2. Complete deletion as soon as reasonably practicable (Section 8(6))
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
	return g.Generate(reqID, entity.Name, entity.GrievanceEmail, profile, InsuranceDeletionCategories, LegalBasisWithdrawal)
}

// GenerateTelecomLetter creates a letter specific to telecom operators
func (g *Generator) GenerateTelecomLetter(reqID string, entity *nbfc.Entity, profile config.Profile, msisdn string) (string, error) {
	// Telecom letters include call/SMS metadata
	categories := TelecomDeletionCategories
	return g.Generate(reqID, entity.Name, entity.GrievanceEmail, profile, categories, LegalBasisWithdrawal)
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

// GenerateDPBB creates a complaint letter for the Data Protection Board of India
func (g *Generator) GenerateDPBB(entityName string, entity *nbfc.Entity, profile config.Profile, requestID string, timeline []string, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, "DATA PROTECTION BOARD OF INDIA", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "Government of India | MeitY", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Title
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(0, 8, "COMPLAINT UNDER SECTION 27(3), DPDP ACT 2023", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "Right to Erasure — Section 8(6)", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Date
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("Date: %s", time.Now().Format("02 January 2006")), "", 1, "", false, 0, "")
	pdf.Ln(3)

	// To
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "To:", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "The Secretary, Data Protection Board of India", "", 1, "", false, 0, "")
	pdf.CellFormat(0, 5, "Electronics Niketan, 6, CGO Complex, Lodhi Road", "", 1, "", false, 0, "")
	pdf.CellFormat(0, 5, "New Delhi - 110003", "", 1, "", false, 0, "")
	pdf.Ln(3)

	// Subject
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "Subject: Complaint against "+entityName+" for non-compliance with Sections 8(6) & 8(7), DPDP Act 2023", "", 1, "", false, 0, "")
	pdf.Ln(5)

	// Section 1
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "1. PARTICULARS OF COMPLAINANT", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, fmt.Sprintf("Name: %s\nEmail: %s\nPhone: %s\nAddress: %s",
		profile.Name, profile.Email,
		func() string { if profile.Phone != "" { return profile.Phone }; return "Not provided" }(),
		func() string { if profile.Address != "" { return profile.Address }; return "Not provided" }()), "", "", false)
	pdf.Ln(3)

	// Section 2
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "2. PARTICULARS OF RESPONDENT", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, fmt.Sprintf("Name: %s\nCategory: %s\nGrievance Email: %s",
		entityName,
		func() string { if entity != nil { return string(entity.Category) }; return "Not specified" }(),
		func() string { if entity != nil && entity.GrievanceEmail != "" { return entity.GrievanceEmail }; return "Not available" }()), "", "", false)
	pdf.Ln(3)

	// Section 3 - Facts
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "3. FACTS OF THE COMPLAINT", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5,
		fmt.Sprintf("I hereby file this complaint against %s under Section 27(3) of the Digital Personal Data Protection Act, 2023.\n\n"+"I submitted a data erasure request under Section 8(6) on "+time.Now().AddDate(0,0,-30).Format("02 January 2006")+
			" (Ref: %s). Despite the statutory as soon as reasonable deadline having elapsed, the Respondent has failed to erase my personal data and has not provided any acknowledgment or response.\n\n"+"This constitutes a violation of Section 8(6) of the DPDP Act, 2023.", entityName, requestID), "", "", false)
	pdf.Ln(3)

	// Section 4 - Timeline
	if len(timeline) > 0 {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(0, 6, "4. TIMELINE", "", 1, "", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		for _, t := range timeline {
			if len(t) > 0 {
				pdf.CellFormat(0, 5, "• "+t, "", 1, "", false, 0, "")
			}
		}
		pdf.Ln(3)
	}

	// Section 5 - Relief
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "5. RELIEF SOUGHT", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5,
		"(a) Direct the Respondent to immediately erase all my personal data;\n"+
			"(b) Confirm the erasure of my data in writing;\n"+
			"(c) Impose penalty as provided under Section 33 of the DPDP Act;\n"+
			"(d) Pass such other orders as the Board deems fit.", "", "", false)
	pdf.Ln(3)

	// Verification
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "6. VERIFICATION", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5,
		fmt.Sprintf("I, %s, verify that the above contents are true and correct.\n\nVerified at %s on %s.",
			profile.Name,
			time.Now().Format("02 January 2006"),
			time.Now().Format("02 January 2006")), "", "", false)
	pdf.Ln(8)

	// Signature
	pdf.CellFormat(0, 5, "Signature: _______________", "", 1, "", false, 0, "")
	pdf.Ln(3)
	pdf.CellFormat(0, 5, profile.Name, "", 1, "", false, 0, "")
	pdf.CellFormat(0, 5, profile.Email, "", 1, "", false, 0, "")

	// Footer
	pdf.Ln(5)
	pdf.SetFont("Arial", "I", 8)
	pdf.CellFormat(0, 5, "Generated via FinWipe | DPDP Act 2023 Compliant", "", 1, "C", false, 0, "")

	return pdf.OutputFileAndClose(outputPath)
}

// GeneratePortability creates a data portability request under Section 6(9)
func (g *Generator) GeneratePortability(entity *nbfc.Entity, profile config.Profile, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, "DATA PORTABILITY REQUEST", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "Under Section 6(9), Digital Personal Data Protection Act, 2023", "", 1, "C", false, 0, "")

	pdf.Ln(5)
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(5)

	// Date
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 6, fmt.Sprintf("Date: %s", time.Now().Format("02 January 2006")), "", 1, "", false, 0, "")
	pdf.Ln(3)

	// To
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "To:", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, entity.Name, "", 1, "", false, 0, "")
	if entity.GrievanceEmail != "" {
		pdf.CellFormat(0, 5, entity.GrievanceEmail, "", 1, "", false, 0, "")
	}
	pdf.Ln(5)

	// Subject
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 6, "Subject: Data Portability Request under Section 6(9), DPDP Act 2023", "", 1, "", false, 0, "")
	pdf.Ln(5)

	// Body
	pdf.SetFont("Arial", "", 11)
	pdf.MultiCell(0, 6,
		fmt.Sprintf("Dear %s,\n\nI am a digital resident of India and my personal data is processed by your organization. In accordance with Section 6(9) of the Digital Personal Data Protection Act, 2023 (DPDP Act), I hereby request that you provide me with all personal data you hold about me.\n\nSpecifically, I request:\n\n1. All personal data collected about me, including but not limited to:\n   - Identity information (name, address, phone, email, ID documents)\n   - Financial information (transactions, account history, KYC data)\n   - Usage data (app activity, login history, preferences)\n   - Any data shared with third parties\n\n2. The data should be provided in a commonly used, machine-readable format (JSON, CSV, or XML).\n\n3. This request must be fulfilled within 72 hours as mandated by the DPDP Act.\n\nPlease confirm receipt of this request and provide the timeline for fulfillment.\n\nIf you have collected data about me that you no longer retain or that has been deleted, please confirm this in writing.\n\nYours sincerely,\n\n%s\n%s\n%s", entity.Name, profile.Name, profile.Email,
			func() string { if profile.Phone != "" { return profile.Phone }; return "" }()), "", "", false)

	pdf.Ln(5)
	pdf.SetFont("Arial", "I", 9)
	pdf.CellFormat(0, 5, "Generated via FinWipe | DPDP Act 2023 Compliant", "", 1, "C", false, 0, "")

	return pdf.OutputFileAndClose(outputPath)
}

// GenerateVerification creates a deletion verification request
func (g *Generator) GenerateVerification(requestID, entityName string, entity *nbfc.Entity, profile config.Profile, method, outputPath string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 8, "DATA DELETION VERIFICATION REQUEST", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, "Section 8(7): Cease processing immediately upon consent withdrawal\nSection 8(6): Erasure of publicly available data obtained via fraud/misrepresentation", "", 1, "C", false, 0, "")

	pdf.Ln(5)

	// Reference
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Ref: %s | Date: %s", requestID, time.Now().Format("02 January 2006")), "", 1, "R", false, 0, "")
	pdf.Ln(5)

	// To
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 5, "To:", "", 1, "", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, entityName, "", 1, "", false, 0, "")
	if entity != nil && entity.GrievanceEmail != "" {
		pdf.CellFormat(0, 5, entity.GrievanceEmail, "", 1, "", false, 0, "")
	}
	pdf.Ln(5)

	// Subject
	pdf.SetFont("Arial", "B", 11)
	if method == "certificate" {
		pdf.CellFormat(0, 6, "Subject: Request for Data Deletion Certificate", "", 1, "", false, 0, "")
	} else {
		pdf.CellFormat(0, 6, "Subject: Confirmation of Data Deletion", "", 1, "", false, 0, "")
	}
	pdf.Ln(5)

	// Body
	pdf.SetFont("Arial", "", 11)
	var body string
	if method == "certificate" {
		body = fmt.Sprintf(`Dear %s,

I had previously submitted a data erasure request under Sections 8(6) & 8(7), DPDP Act 2023 (Ref: %s) on %s.

I had previously requested erasure under Sections 8(6) & 8(7), DPDP Act 2023. As per the DPDP Act 2023 (no prescribed statutory period), I kindly request an update on the status of my erasure request.

I hereby request that you provide me with a written Data Deletion Certificate confirming:
1. That all my personal data has been erased from your systems
2. The date on which the deletion was completed
3. That all third parties with whom my data was shared have also deleted it

This certificate is necessary for my records and may be required for future disputes or regulatory purposes.

Please provide this certificate within 72 hours.

Yours sincerely,
%s
%s`, entityName, requestID,
			time.Now().AddDate(0, 0, -35).Format("02 January 2006"),
			time.Now().AddDate(0, 0, -5).Format("02 January 2006"),
			profile.Name, profile.Email)
	} else {
		body = fmt.Sprintf(`Dear %s,

I submitted a data erasure request (Ref: %s) and I wish to verify whether my personal data has been completely deleted from your systems.

I request that you:
1. Confirm in writing that all my personal data has been erased
2. Confirm the date of deletion
3. Confirm that all third parties have also deleted my data

If deletion has not been completed, please provide:
1. The current status of deletion
2. The expected completion date
3. Reason for delay

Please respond within 72 hours.

Yours sincerely,
%s
%s`, entityName, requestID, profile.Name, profile.Email)
	}

	pdf.MultiCell(0, 6, body, "", "", false)
	pdf.Ln(5)

	pdf.SetFont("Arial", "I", 9)
	pdf.CellFormat(0, 5, "Generated via FinWipe | DPDP Act 2023 Compliant", "", 1, "C", false, 0, "")

	return pdf.OutputFileAndClose(outputPath)
}
