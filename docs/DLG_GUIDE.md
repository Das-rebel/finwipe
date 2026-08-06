# RBI Digital Lending Guidelines (DLG) — Data Deletion Guide

> **"Stop digital lenders from holding your data."**

This guide specifically covers **data deletion requests under RBI's Digital Lending Guidelines** — for fintechs, NBFCs, and apps that gave you loans or credit through apps.

---

## What's Covered

RBI's Master Direction on Digital Lending (updated August 2025) mandates that digital lenders must delete user data once the loan is repaid and accounts are closed.

**Use this if you:**
- Took a personal loan, BNPL, or credit from a fintech app
- Have repaid the loan and closed the account
- Want to stop the lender from retaining your data
- Don't want to receive pre-approved loan offers

---

## Quick Start

```bash
# 1. Install finwipe
npm install -g finwipe

# 2. Set up your profile
finwipe init
# You'll need: name, email, phone(s), address, SMTP credentials

# 3. Find your lender
finwipe list --search KreditBee

# 4. Create deletion request
finwipe new --nbfc-id kreditbee

# 5. Send the request
finwipe send --dry-run  # preview first
finwipe send            # send emails

# 6. Track responses
finwipe track --all
```

---

## Why RBI DLG?

| Law | What It Says |
|-----|--------------|
| **RBI DLG Para 10.2** | "Data shall be deleted once the purpose is over" |
| **RBI DLG Para 11.1** | "No data to be shared without explicit consent after loan closure" |
| **RBI DLG 2025 Update** | Tightened deletion requirements for digital lenders |

**DPDP Act** is the general data protection law — **RBI DLG** is specific to digital lenders and stronger in practice because:
1. RBI can penalize non-compliant lenders
2. DPDP Board enforcement is still building capacity
3. Lenders fear RBI action more than DPDP Board

---

## What Data Can Be Deleted

```
✓ Pre-approved loan offer profiles
✓ Marketing and promotional data
✓ Behavioral and usage data
✓ App permissions and preferences
✓ Call recordings and service logs
✓ Third-party shared data (with agents, co-lenders)
```

## What Cannot Be Deleted

```
✗ KYC documents (PMLA: must keep 10 years post-closure)
✗ Transaction records (RBI: must keep 5-10 years)
✗ Active loan account data
✗ CIBIL's own records (separate process)
```

---

## Step-by-Step Process

### Step 1: Discover Who Has Your Data

**From CIBIL Report** (recommended):
```bash
finwipe discover-from-cibil --file your_cibil_report.pdf --auto
```

**From Bank Statement**:
```bash
finwipe discover-from-statement --file statement.pdf
```

**From Email**:
```bash
finwipe discover-from-email --file gmail_export.zip --auto
```

### Step 2: Create Requests

```bash
# Single lender
finwipe new --nbfc-id kreditbee

# Multiple lenders
finwipe new --nbfc-id kreditbee
finwipe new --nbfc-id earlysalary
finwipe new --nbfc-id cred
finwipe new --nbfc-id slice
```

### Step 3: Send Requests

```bash
# Preview
finwipe send --dry-run

# Send all
finwipe send

# Send to specific lender
finwipe send --request-id DPR-2026-000001
```

### Step 4: Track and Follow Up

```bash
# Check all requests
finwipe track --all

# Follow up after 3 days
finwipe send --request-id DPR-2026-000001  # sends follow-up

# Escalate after 7 days
finwipe escalate --request-id DPR-2026-000001 --to dpo
```

### Step 5: If No Response — Escalate

```bash
# Step 1: Escalate to DPO
finwipe escalate --request-id DPR-2026-000001 --to dpo

# Step 2: Escalate to RBI
finwipe escalate --request-id DPR-2026-000001 --to rbi_ombudsman

# Step 3: Consumer Forum (as last resort)
finwipe escalate --request-id DPR-2026-000001 --to consumer_forum
```

---

## Template Email (RBI DLG Version)

The standard template cites **RBI DLG** instead of just DPDP Act:

```
Subject: Data Deletion Request — RBI Digital Lending Guidelines — [Entity Name] — [DPR-ID]

Dear Grievance Officer / Nodal Officer,

I am writing to request deletion of my personal data held by [Entity Name] 
under **RBI Master Direction on Digital Lending (updated August 2025)**.

I request your organization to:
1. Confirm receipt of this request
2. Delete all my personal data in your digital lending records
3. Provide written confirmation of data deletion

As per RBI Digital Lending Guidelines, you are required to complete 
deletion within 7 days of receiving this request.

Reference:
- DPR-ID: [DPR-ID]
- Request Date: [Date]
- Entity: [Entity Name]

Contact:
- Email: [your email]
- Mobile: [your phone(s)]

Regards,
[Your Name]
[Your Phone(s)]
```

---

## Common Lenders (90+ in Registry)

**Top Digital Lenders:**
- KreditBee, EarlySalary, Stashfin, MoneyTap, FlexMoney
- CRED, Slice, Uni, PostPe, Simpl, Spenny, ZestMoney
- PhonePe, Paytm, Google Pay, Amazon Pay, Razorpay
- Bajaj Finserv, Tata Capital, Aditya Birla Finance, HDB Financial
- Lendingkart, LoanTap, Indifi, Capital Float, Airtel Payments Bank
- Paysense, Nira, KazzBack, Ivy, MoneyView, Kratos

**Full list:**
```bash
finwipe list --category fintech    # 133 fintechs
finwipe list --category nbfc       # 28 NBFCs
finwipe list --category mfi        # 25 MFIs
finwipe list --category p2p        # 9 P2P platforms
finwipe list --category bnpl       # 6 BNPL providers
finwipe list                        # all 230 entities
```

---

## FAQ

**Q: How long does it take?**
A: Most lenders acknowledge within 3-7 days. Full deletion may take 30 days.

**Q: What if lender claims data already deleted?**
A: Request a certificate of deletion as proof.

**Q: Can I request deletion while loan is active?**
A: Only non-essential data. Active loan data must be retained.

**Q: What if lender ignores the request?**
A: Escalate to DPO → RBI Ombudsman → Consumer Forum.

**Q: Does this work for banks?**
A: Banks are under BFSR/RBI banking guidelines, not DLG specifically. Use DPDP Act for banks.

---

## Contact Your Legislators

If lenders consistently ignore DLG deletion requests:
- File complaint at **RBI CMS portal**: https://cms.rbi.org.in
- Tweet at **@RBI**: RBI monitors digital lending compliance
- Contact **MFIN/DAKSH** (digital lending association)
