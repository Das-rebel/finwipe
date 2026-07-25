# Regulatory Framework — FinWipe

This document explains the legal basis for FinWipe's deletion requests and the obligations of NBFCs under Indian law.

---

## 1. Right to Erasure — DPDP Act 2023

### Section 8(6): Right to Erasure

> *"A data principal shall have the right to erasure of personal data processed by a data fiduciary where such personal data is no longer necessary for the purpose for which it was collected."*

**What this means:**
- When the purpose of collecting personal data (e.g., granting a loan) is complete, the data principal can demand erasure
- The NBFC must delete data that is no longer necessary
- This is not unlimited — regulatory requirements carve out exceptions

### Rule 8, DPDP Rules 2025 — Operational Requirements

| Requirement | Detail |
|------------|--------|
| **Acknowledgment** | 48 hours from receipt of request |
| **Completion** | Without undue delay, typically 30 days |
| **Propagated deletion** | Must delete from ALL processors (cloud, SaaS, backups) |
| **Logs** | Retain deletion logs for 1 year minimum |
| **Notice before deletion** | Must notify data principal 48 hours before deletion |

### What CAN be requested for deletion

```
✓ Marketing and promotional data
✓ Third-party shared data (sold to data brokers)
✓ Behavioral/usage data from apps/websites
✓ Pre-approved loan offer profiles
✓ Call recordings and service interaction logs
✓ Device fingerprint and metadata
✓ KYC data beyond regulatory requirement period (10 years post-closure)
```

### What CANNOT be deleted

```
✗ KYC documents (PMLA: retain 10 years post account closure)
✗ Transaction records (RBI: 5-10 years depending on product)
✗ Active loan account data
✗ Data required for regulatory reporting
✗ Tax records (Income Tax Act)
✗ Records for dispute resolution
```

---

## 2. RBI Digital Lending Guidelines (DLG) 2022

RBI issued the Digital Lending Guidelines on September 2, 2022, applicable to all digital lenders including NBFCs.

### Key provisions relevant to deletion:

**Para 10.1 — Data Collection**
> "Any collection of data by DLAs shall be need-based and with prior and explicit consent of the borrower having audit trail."

→ Consent must be purpose-specific and revocable.

**Para 10.2 — Borrower's Rights**
> "The borrower shall be provided with an option to give or deny consent for use of specific data, restrict disclosure to third parties, data retention, revoke consent already granted to collect personal data and, if required, make the app delete/forget the data."

→ This is the strongest direct mandate. Borrowers can demand the app **delete/forget** their data.

**Para 11.1 — Storage Limitations**
> "LSPs/DLAs shall not store personal information of borrowers except some basic minimal data..."

→ NBFCs must not store more data than necessary.

**Para 11.2 — Data Destruction Protocol**
> "RE shall ensure that clear policy guidelines regarding storage of customer data including... data destruction protocol... are put in place."

→ NBFCs must have documented data deletion policies.

**Para 5.3 — Key Fact Statement**
> Borrowers must receive a standardized Key Fact Statement before loan execution, including details of grievance redressal.

---

## 3. NBFC-CIC Data Reporting Directions 2025

The RBI's November 2025 Directions mandate that all NBFCs:

1. **Must be members of all 4 CICs**: CIBIL, Experian, Equifax, CRIF High Mark
2. **Report fortnightly**: Every 15 days, within 7 calendar days
3. **Grievance redress within 30 days**: ₹100/day compensation for delays
4. **Consumer can access credit information**: Right to view all data held

**Implication for deletion**: NBFCs must still report to CICs even after deletion requests — credit reporting is a regulatory obligation that overrides deletion requests for transaction data.

---

## 4. The 4 Credit Bureaus

| Bureau | Website | Dispute Method |
|--------|---------|---------------|
| **TransUnion CIBIL** | cibil.com | Online portal → Dispute Center |
| **Experian** | exiperian.in | Online dispute form |
| **Equifax** | equifax.co.in | Online dispute |
| **CRIF High Mark** | crifhighmark.com | Online dispute |

**Note**: CIBIL disputes can only be filed by the individual whose data is in question. You must log in with your own credentials.

---

## 5. Escalation Path

If an NBFC does not comply with deletion requests:

```
Step 1: Follow up via email (30 days)
        ↓
Step 2: RBI Sachet Portal
        https://sachet.rbi.org.in
        → Consumer grievance → NBFC complaint
        ↓
Step 3: DPDP Board complaint
        https://dpb.gov.in
        → Data principal complaint
        ↓
Step 4: Consumer Forum (CPA 2019)
        → State/National consumer forum
        ↓
Step 5: Civil Court
```

---

## 6. Template Legal Text

The deletion email template cites:

```
Subject: DPDPA Section 8(6) Data Deletion Request — [Name]

"I am exercising my right to erasure under Section 8(6) of the Digital
Personal Data Protection Act, 2023 (DPDP Act) and Rule 8 of the DPDP
Rules, 2025.

The purpose for which my personal data was collected is no longer being
served. I hereby request deletion of:

□ Marketing and promotional data
□ Third-party shared data
□ Behavioral/usage data
□ Pre-approved loan profiles
□ Call recordings

I request acknowledgment within 48 hours (Rule 8(3)) and completion
within 30 days.

This request does not extend to data required by law (KYC, transactions,
tax records)."
```

---

## 7. References

- DPDP Act 2023: https://www.meity.gov.in/writereaddata/files/digital_personal_data_protection_act_2023.pdf
- DPDP Rules 2025: https://www.meity.gov.in/writereaddata/files/Digital-Personal-Data-Protection-Rules-2025.pdf
- RBI Digital Lending Guidelines: https://www.rbi.org.in/scripts/NotificationUser.aspx?Id=12382
- RBI NBFC Credit Information Reporting Directions 2025: https://taxguru.in/rbi/rbi-non-banking-financial-companies-credit-information-reporting-directions-2025.html
- CIBIL Dispute: https://www.cibil.com/consumer-dispute-resolution
