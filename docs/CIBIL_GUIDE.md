# CIBIL Data & Credit Report Control Guide

> How to stop CIBIL from sharing your credit data with lenders, freeze your report, and exercise your rights under DPDPA 2023 and the Credit Information Companies Act.

---

## What is CIBIL?

**CIBIL** (Credit Information Bureau India Limited) is India's oldest and largest credit bureau. It:
- Collects credit history from **member institutions** (banks, NBFCs, HFCs)
- Generates your **CIBIL Score** and **Credit Report**
- Shares your data with **member lenders** when you apply for credit

---

## The Problem: CIBIL Shares Your Data with Members

When you take a loan or credit card, your lender:
1. Shares your payment history with CIBIL
2. CIBIL bundles it into your credit profile
3. **Other lenders query CIBIL** to assess your creditworthiness

This is CIBIL's core business model — selling access to your financial data.

---

## Your Rights

### Under DPDPA 2023 (Section 8)

| Right | What it means |
|-------|---------------|
| **Right to Erasure** (Sec 8(6)) | You can request deletion of your personal data |
| **Right to Access** (Sec 6) | You can ask CIBIL what data they hold on you |
| **Right to Correction** (Sec 7) | You can correct inaccurate data |

### Under Credit Information Companies Act, 2005

- **Section 22**: You can dispute inaccurate credit information
- **RBI CIC Guidelines**: You can seek correction/deletion of incorrect data
- **Member Access Audit**: You can see who has accessed your report

---

## CIBIL Watch — See Who Accesses Your Report

**CIBIL Watch** is CIBIL's consumer portal feature that lets you:

1. **View all member accesses** — See which lenders have accessed your CIBIL report (last 24 months)
2. **Freeze/Unfreeze** — Prevent new loans from being opened in your name
3. **Raise Disputes** — Flag unauthorized access
4. **Opt out of marketing** — Prevent CIBIL from sharing your data for marketing

### Portal URL

**https://consumer.cibil.com**

### How to Access

1. Go to [https://consumer.cibil.com](https://consumer.cibil.com)
2. Login with your CIBIL ID and password
3. Navigate to **CIBIL Watch** section

---

## How to Stop CIBIL from Sharing Your Data

### Option 1: Freeze Your CIBIL Report (Recommended)

A **CIBIL freeze** prevents any new credit from being opened in your name:

```
1. Login to https://consumer.cibil.com
2. Go to "CIBIL Watch" → "Freeze Report"
3. Confirm freeze

To unfreeze: Same path → "Unfreeze Report"
```

**What this does:**
- ✅ Lenders cannot access your CIBIL report
- ✅ New loan/credit card applications will be rejected
- ✅ You can temporarily unfreeze when applying for credit
- ⚠️ Existing loans continue normally

### Option 2: Opt Out of Marketing Use

CIBIL may share your data for marketing. To opt out:

```
1. Login to https://consumer.cibil.com
2. Go to "Consent Settings" or "Privacy"
3. Toggle off "Marketing Use" or "Third-party sharing"
```

### Option 3: File DPDPA Erasure Request with CIBIL

Under Section 8(6) of DPDPA, you can request CIBIL to delete your data:

**Email:** `grievance.officer@cibil.com`  
**Address:**
```
Grievance Officer
CIBIL Technologies Pvt Ltd
Tower 3, Empire Complex,
414 Senapati Bapat Marg,
Lower Parel, Mumbai 400013
```

**Subject:** DPDPA Section 8(6) — Request for Data Erasure

**Template:**
```
Subject: DPDPA Section 8(6) — Request for Erasure of Personal Data

Dear Grievance Officer,

I, [Your Full Name], am exercising my right to erasure under Section 8(6) 
of the Digital Personal Data Protection Act, 2023.

My Details:
- CIBIL Member ID: [Your CIBIL ID]
- Email: [Your Email]
- Phone: [Your Phone]

I request that CIBIL:
1. Delete all personal data held in my credit information file
2. Cease sharing my data with all member institutions
3. Confirm deletion in writing

This request is made because continued retention and sharing of my data 
without my free, informed, and specific consent violates Section 8(6) 
of the DPDPA Act, 2023.

Please confirm receipt and action within 48 hours as per Rule 8 of 
the DPDP Rules, 2025.

Sincerely,
[Your Full Name]
[Date]
```

---

## What CIBIL Cannot Do

| Cannot Delete | Why |
|--------------|-----|
| **Active loan data** | Required by RBI regulations for ongoing credit |
| **Historical defaults** | Must maintain credit history for 5-7 years |
| **Data shared with lenders** | Once sent to a member, CIBIL cannot retrieve it |

### What CAN Be Deleted

- ✅ Marketing use data
- ✅ Soft inquiries (lenders who checked but you didn't apply)
- ✅ Incorrect/outdated information
- ✅ Data no longer necessary for the purpose it was collected

---

## How to Check Your CIBIL Report for Free

**Official (1 free report per year):** [https://www.cibil.com/freecibilscore](https://www.cibil.com/freecibilscore)

**No bank account linking required.**

---

## FinWipe — Automated DPDPA Deletion Letters for NBFCs

While CIBIL controls your credit bureau data, **FinWipe** helps you exercise DPDPA rights against **lenders and fintechs** who hold your financial data:

```bash
# Install
npm install -g finwipe

# Send deletion request to any NBFC in FinWipe's registry
finwipe new --nbfc bajaj-finserv
finwipe send

# Track lifecycle: send → acknowledge → follow-up → escalate → close
finwipe track --all
finwipe report
```

---

## Related Commands in FinWipe

| Command | Purpose |
|---------|---------|
| `finwipe cic` | Generate pre-filled CIC (CIBIL/Experian/Equifax/CRIF) dispute forms |
| `finwipe discover-from-bureau` | Parse your credit bureau report to find all FIs |
| `finwipe new --nbfc` | Create DPDPA deletion request for any NBFC |
| `finwipe letter` | Generate PDF deletion letter for registered post |

---

## Key Contacts

| Entity | Contact |
|--------|---------|
| **CIBIL Grievance** | grievance.officer@cibil.com |
| **CIBIL Support** | 1800-258-6363 |
| **CIBIL Portal** | https://consumer.cibil.com |
| **RBI CIC Division** | cic@rbi.org.in |
| **MeitY DPDPA** | dpdo@meity.gov.in |
