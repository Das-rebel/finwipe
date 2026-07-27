# FinWipe — DIY Financial Data Deletion for India

> **"Your financial data. Your rules."**

FinWipe is an open-source CLI tool that helps Indian citizens exercise their **right to erasure** under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act) and Rule 8 of the DPDP Rules, 2025.

Every request gets a unique **DPR-ID** (e.g., `DPR-2026-000001`) for full auditability. All data stays on **YOUR machine**.

---

## The Problem

Indian citizens have no easy way to:
- Know which NBFCs, fintechs, and lenders hold their data
- Send legally-grounded deletion requests at scale
- Track which entities complied vs. ignored requests
- Escalate non-compliant entities to the Data Protection Board

**FinWipe solves all of this.**

---

## The Solution

```
┌─────────────────────────────────────────────────────────────────┐
│  YOUR MACHINE                                                │
│                                                              │
│  finwipe new --nbfc-id bajaj-finserv                        │
│  finwipe send                                               │
│  finwipe track --request-id DPR-2026-000001                │
│  finwipe dpd-board --request-id DPR-2026-000001            │
│                                                              │
│  All data: ~/.finwipe/                                     │
│  • history.db        — SQLite with full audit trail         │
│  • letters/          — PDF deletion letters                │
│  • config.yaml       — Your profile + SMTP                  │
│  • evidence/        — Screenshots, acknowledgments         │
└─────────────────────────────────────────────────────────────────┘
```

---

## Quick Start

```bash
# 1. Install
git clone https://github.com/das-rebel/finwipe
cd finwipe
go build -o finwipe ./cmd/finwipe

# 2. Setup (one-time)
./finwipe init

# 3. Preview what would happen
./finwipe send --dry-run

# 4. Send deletion emails
./finwipe send

# 5. Track responses
./finwipe track --all
```

---

## Core Commands

### `finwipe init`
Initialize your profile with name, email, address, and SMTP credentials.

```bash
./finwipe init
# Interactive — asks for:
#   • Full name
#   • Email address
#   • Address (for registered post)
#   • SMTP host (e.g., smtp.gmail.com)
#   • SMTP port (e.g., 587)
#   • SMTP username (your email)
#   • SMTP password (app password)
```

Config saved to `~/.finwipe/config.yaml`

---

### `finwipe list`
Browse the 91 registered NBFCs, banks, fintechs, and HFCs.

```bash
./finwipe list
./finwipe list --category fintech
./finwipe list --category bank
./finwipe list --search HDFC
./finwipe list --json
```

**Categories:** `bank` (12) · `nbfc` (18) · `fintech` (59) · `hfc` (2)

---

### `finwipe new`
Create a new deletion request. Returns a **DPR-ID** for tracking.

```bash
# Single NBFC
./finwipe new --nbfc-id bajaj-finserv

# Batch by category
./finwipe new --batch fintech --count 10

# With specific categories
./finwipe new --nbfc-id bajaj-finserv \
  --categories marketing,third_party,app_usage
```

**DPDPA Deletion Categories:**
- `marketing` — Promotional and marketing communications
- `third_party` — Data shared with third parties
- `behavioral` — Behavioral and usage data
- `app_usage` — App activity and preferences
- `call_records` — Call recordings and service logs
- `loan_profile` — Pre-approved loan offers and credit profile
- `all_non_essential` — Everything except legally required data

---

### `finwipe send`
Dispatch deletion requests via email or registered post.

```bash
# Send all pending requests
./finwipe send

# Dry run (preview)
./finwipe send --dry-run

# Specific request
./finwipe send --request-id DPR-2026-000001

# Rate limit (ms between emails)
./finwipe send --rate-limit 2000

# Via registered post (generates letter PDFs)
./finwipe send --channel post

# Via CIC (in-person filing)
./finwipe send --channel cic
```

---

### `finwipe track`
Track deletion request lifecycle and audit trail.

```bash
# Track specific request
./finwipe track --request-id DPR-2026-000001

# All active requests
./finwipe track --all

# Requests overdue (past acknowledgment deadline)
./finwipe track --overdue

# Requests awaiting acknowledgment
./finwipe track --awaiting-ack

# Escalated requests
./finwipe track --escalated
```

**Request Lifecycle:**
```
INITIATED → DISPATCHED → ACK_RECEIVED → RESPONSE_OK → CLOSED
                 ↓
           AWAITING_ACK
                 ↓
           DELIVERY_FAILED (retry)
                 ↓
           ESCALATED
                 ↓
           DPDP_BOARD / RBI_OMBUDSMAN / CONSUMER_FORUM
                 ↓
              CLOSED
```

---

### `finwipe ack`
Record when an NBFC acknowledges your deletion request.

```bash
./finwipe ack --request-id DPR-2026-000001
./finwipe ack --request-id DPR-2026-000001 --reference ABC123XYZ
```

---

### `finwipe followup`
Send follow-up emails after 48 hours of no acknowledgment.

```bash
# Follow up on all awaiting-ack requests
./finwipe followup

# Specific request
./finwipe followup --request-id DPR-2026-000001
```

---

### `finwipe escalate`
Escalate ignored requests to higher authorities.

```bash
# Escalate to NBFC's DPO (Data Protection Officer)
./finwipe escalate --request-id DPR-2026-000001 --to dpo

# Escalate to DPDP Board (Section 27(3))
./finwipe escalate --request-id DPR-2026-000001 --to dpd_board

# Escalate to RBI Ombudsman
./finwipe escalate --request-id DPR-2026-000001 --to rbi_ombudsman

# Escalate to Consumer Forum
./finwipe escalate --request-id DPR-2026-000001 --to consumer_forum
```

**Escalation Path:**
```
L0 → L1: DPO (7 days no ack)
L1 → L2: DPDP Board (30 days no response)
L2 → L3: RBI Ombudsman / Consumer Forum
```

---

### `finwipe close`
Close a request with outcome.

```bash
./finwipe close --request-id DPR-2026-000001
./finwipe close --request-id DPR-2026-000001 --outcome deleted
./finwipe close --request-id DPR-2026-000001 \
  --outcome rejected --notes "NBFC claimed data already deleted"
```

**Outcomes:** `deleted` · `acknowledged_not_deleted` · `partial` · `rejected` · `exemption_claimed` · `no_response` · `escalated`

---

### `finwipe report`
Dashboard showing compliance metrics.

```bash
./finwipe report
./finwipe report --days 30
./finwipe report --format json
./finwipe report --format csv
```

---

### `finwipe compliance`
Track NBFC compliance rates (anonymized community data).

```bash
./finwipe compliance
./finwipe compliance --shame
```

**Shame List:** Ranks NBFCs by worst acknowledgment rates.

---

## Discovery Commands

Find out which entities hold your data before sending deletion requests.

### `finwipe discover-from-cibil`
Parse a CIBIL credit report PDF and auto-generate deletion requests for every institution that queried your report.

```bash
./finwipe discover-from-cibil --file your_cibil_report.pdf
./finwipe discover-from-cibil --file report.pdf --auto
```

**How it works:**
1. CIBIL shows every institution that queried your credit report
2. These are FIs that have done due diligence on you
3. Cross-references against 91-entity registry
4. Creates deletion requests for matched entities

---

### `finwipe discover-from-bureau`
Parse credit bureau reports from all 4 bureaus: CIBIL, Experian, Equifax, CRIF HighMark.

```bash
./finwipe discover-from-bureau --file Experian_Report.pdf
./finwipe discover-from-bureau --file CRIF_HighMark.pdf --auto
```

---

### `finwipe discover-from-bank-statement`
Extract financial institution references from bank statement PDFs.

```bash
./finwipe discover-from-bank-statement --file statement.pdf
./finwipe discover-from-bank-statement --directory ./statements/
./finwipe discover-from-bank-statement --bank hdfc --auto
```

**Parses:**
- EMI deductions (identifies lender)
- NACH/NECS references
- Transaction descriptions with FI names
- Standing instruction mandates

---

### `finwipe discover-from-email`
Parse Gmail Takeout exports to find financial institutions.

```bash
./finwipe discover-from-email --file gmail_export.zip
./finwipe discover-from-email --format mbox --auto
```

**Supported formats:** ZIP (Takeout), MBOX, CSV, plain text

---

### `finwipe discover-from-whatsapp`
Extract FI contacts from WhatsApp Business chat exports.

```bash
./finwipe discover-from-whatsapp --path ./whatsapp_chat.txt
./finwipe discover-from-whatsapp --auto
```

**How to export WhatsApp chats:**
1. iPhone: WhatsApp → Chat → Export Chat
2. Android: GB WhatsApp → Chat → Export

---

### `finwipe discover-from-aa`
Discover FIs via Account Aggregator apps (NADL, CAMS, SAafe, Finvu).

```bash
./finwipe discover-from-aa --provider nadl
./finwipe discover-from-aa --provider cams --auto
```

**What it does:**
1. Opens AA app login page in browser
2. You authenticate with phone + OTP
3. AA shows all linked financial accounts (FIPs)
4. These are entities that have your financial data

**AA Providers:** NADL · CAMS · SAafe · Finvu

---

## Enforcement Commands

### `finwipe dpd-board`
Generate a pre-filled complaint to the **Data Protection Board of India** (DPBB).

```bash
./finwipe dpd-board --request-id DPR-2026-000001
./finwipe dpd-board --nbfc-id bajaj-finserv \
  --name "John Doe" --email john@example.com
```

**What it generates:**
- Pre-filled Form III complaint
- Timeline of your deletion requests
- Legal basis: Section 8(6), DPDP Act 2023
- Relief sought: deletion + penalty

**How to file:**
1. Online: https://dpdpboard.gov.in → File Complaint → Form III
2. Email: complaints@dpdpboard.gov.in

**DPBB Powers:**
- Order data deletion: Section 27(3)(i)
- Impose penalty up to ₹250 crore: Section 33
- Investigate systemic non-compliance: Section 27(4)

---

### `finwipe portability`
Request **all data** an entity holds about you (Section 6(9), DPDP Act).

```bash
./finwipe portability --nbfc-id bajaj-finserv
./finwipe portability --nbfc-id tata-capital --send
```

**Why it matters:**
- Deletion: "Delete my data" (company may say done, not prove it)
- Portability: "Give me all data you have" (you verify what they actually hold)
- Use both: portability first, then deletion

**Company must respond within 72 hours** (Section 6(9))

---

### `finwipe verify`
Verify if an NBFC actually deleted your data.

```bash
./finwipe verify --request-id DPR-2026-000001
./finwipe verify --request-id DPR-2026-000001 --method certificate
```

**Verification Methods:**
- `email` — Send verification email asking to confirm deletion
- `certificate` — Request official deletion certificate
- `login` — Try to access service (account should fail if deleted)

---

### `finwipe mass-request`
Send deletion requests to ALL entities in a category with one command.

```bash
# All fintechs
./finwipe mass-request --category fintech

# All banks except HDFC and ICICI
./finwipe mass-request --category bank \
  --exclude hdfc-bank,icici-bank

# First 10 entities randomly
./finwipe mass-request --category all --count 10

# Dry run preview
./finwipe mass-request --category fintech --dry-run
```

⚠️ **Warning:** You will receive ~90 acknowledgment emails!

---

## Automation Commands

### `finwipe cron`
Daily automation: follow-ups, deadline checks, auto-escalation.

```bash
# Dry run
./finwipe cron --dry-run

# Full automation
./finwipe cron

# Follow-up only
./finwipe cron --followup

# Check escalation only
./finwipe cron --escalate
```

**Cron schedule:**
- Every morning: Check for overdue acknowledgments (48h+ no ack)
- Day 3: Send follow-up emails
- Day 7: Escalate to DPO
- Day 30: Escalate to DPDP Board

---

### `finwipe setup-forward`
Get your FinWipe cloud inbox for **passive** FI discovery (CRED/Fold model).

```bash
./finwipe setup-forward
```

**How it works:**
```
Gmail Filter (you set up once)
  ↓ FORWARDS all financial emails
Mailgun (free: 5K/month)
  ↓ RECEIVES emails, extracts sender domain ONLY
Cloudflare Worker
  ↓ MATCHES domains → known FIs
finwipe sync
  ↓ PULLS discoveries
finwipe sync --auto
  ↓ CREATES deletion requests
```

**Privacy Guarantee:**
- Email content NEVER received or stored
- Only sender DOMAIN extracted (not full email)
- User ID is one-way hash (no PII)
- Data lives in your KV namespace only

---

### `finwipe sync`
Sync discoveries from FinWipe cloud.

```bash
# Show discoveries
./finwipe sync

# Auto-create deletion requests
./finwipe sync --auto

# Use local emails (offline)
./finwipe sync --import ~/.finwipe/forwarded/
```

---

### `finwipe check-inbox`
Parse locally forwarded emails (no cloud required).

```bash
./finwipe check-inbox
./finwipe check-inbox --import /path/to/emails/
```

---

### `finwipe cloud-status`
Check FinWipe cloud connectivity.

```bash
./finwipe cloud-status
```

---

## Supporting Commands

### `finwipe letter`
Generate professional PDF deletion letters.

```bash
./finwipe letter --nbfc-id bajaj-finserv
./finwipe letter --request-id DPR-2026-000001
./finwipe letter --batch fintech
```

---

### `finwipe evidence`
Attach and manage evidence for deletion requests.

```bash
# Attach screenshot
./finwipe evidence attach DPR-2026-000001 \
  --type email_sent --file screenshot.png

# Attach acknowledgment
./finwipe evidence attach DPR-2026-000001 \
  --type email_received --file acknowledgment.eml

# List evidence
./finwipe evidence list DPR-2026-000001

# Get evidence
./finwipe evidence get <evidence-id>
```

**Evidence Types:**
- `email_sent` — Original deletion request email
- `email_received` — NBFC's acknowledgment
- `email_bounce` — Email bounce/failure notification
- `letter_pdf` — Registered post acknowledgment
- `dpd_board_filing` — DPBB complaint confirmation
- `rbi_ombudsman_filing` — RBI complaint confirmation
- `cic_receipt` — CIC in-person filing receipt
- `screenshot` — Screenshots of interactions

---

### `finwipe cic`
Generate pre-filled CIC (CIBIL/Experian/Equifax/CRIF) dispute forms.

```bash
./finwipe cic --bureau CIBIL
./finwipe cic --bureau Experian --nbfc-id bajaj-finserv
./finwipe cic --batch
```

---

### `finwipe parse`
Parse a CIBIL report PDF to extract NBFC names.

```bash
./finwipe parse --file your_cibil_report.pdf
./finwipe parse --file report.pdf --format text
```

---

### `finwipe ask`
Interactive consent withdrawal wizard — **works without `finwipe init`**.

```bash
./finwipe ask
```

Answers questions, generates Section 8(7) withdrawal email, shows where to send it.

---

### `finwipe wizard`
Interactive guided deletion request flow.

```bash
./finwipe wizard
```

Step-by-step walkthrough for first-time users.

---

### `finwipe compliance --shame`
NBFC shame list — ranks NBFCs by worst compliance rates.

```bash
./finwipe compliance --shame
./finwipe compliance --shame --export csv
```

Shows:
- Acknowledgment rate
- Average response time
- Number of complaints
- Recommended action

---

## Legal Basis

FinWipe invokes:

| Law | Section | Right |
|-----|---------|-------|
| DPDP Act 2023 | Section 8(6) | Right to Erasure |
| DPDP Act 2023 | Section 6(9) | Right to Data Portability |
| DPDP Act 2023 | Section 27(3) | Complaint to DPBB |
| DPDP Rules 2025 | Rule 8(1) | 48-hour acknowledgment |
| DPDP Rules 2025 | Rule 8(2) | Deletion within 30 days |
| RBI DLG 2022 | Para 10.2, 11.1, 11.2 | Data deletion in lending |

Read [docs/REGULATORY_FRAMEWORK.md](docs/REGULATORY_FRAMEWORK.md) for full details.

---

## What CAN Be Deleted

```
✓ Marketing and promotional data
✓ Third-party shared data
✓ Behavioral and usage data
✓ Pre-approved loan offer profiles
✓ Call recordings and service logs
✓ App activity and preferences
```

## What CANNOT Be Deleted

```
✗ KYC documents (PMLA: 10 years post-closure)
✗ Transaction records (RBI: 5-10 years)
✗ Active loan account data
✗ CIBIL's own records (separate fiduciary duty)
```

---

## Architecture

```
~/.finwipe/
├── config.yaml          # Profile + SMTP credentials
├── history.db           # SQLite WAL — full audit trail
├── inbox               # FinWipe cloud inbox address
├── cloud_api_key       # Cloud API key
├── letters/            # Generated PDF letters
│   ├── Deletion_[nbfc]_[date].pdf
│   ├── DPBB_complaint_[nbfc]_[date].pdf
│   └── Portability_[nbfc]_[date].pdf
├── evidence/           # Screenshots, acknowledgments
└── forwarded/         # Local forwarded emails
```

**Database Schema:**
```sql
requests: DPR-ID, NBFC, channel, state, timeline
escalations: DPR-ID, level, channel, details
evidence: DPR-ID, type, file, notes, timestamp
```

---

## Registry

The NBFC registry is at `data/nbfcs.yaml`. Add new entities:

```yaml
- id: my-company
  name: My Finance Company Ltd
  short_name: MyFin
  category: nbfc          # bank | nbfc | fintech | hfc
  grievance_email: grievance@myfin.com
  grievance_phone: "18001234567"
  address: "MyFin, Mumbai, MH"
  active: true
```

---

## Cloud Deployment

Deploy FinWipe Cloud for passive email forwarding:

```bash
cd apps/finwipe-cloud
./deploy.sh
```

**Free tier:**
- Cloudflare Workers: 100K requests/day
- Cloudflare KV: 1M reads/day
- Mailgun: 5K emails/month

---

## GitHub Actions Automation

Fork and add secrets for monthly automation:

```bash
# Secrets needed:
SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD
```

Workflow runs `./finwipe send --rate-limit 2000` monthly.

---

## The Honest Limitation

FinWipe cannot guarantee NBFCs will respond within 30 days.

It CAN guarantee:
- ✅ Every request is timestamped
- ✅ You have evidence of what was sent
- ✅ You have legal documentation
- ✅ You can escalate: DPO → DPDP Board → RBI Ombudsman

---

## Tech Stack

- **Go 1.21+** — single binary, no runtime deps
- **Cobra** — CLI framework
- **SQLite (WAL mode)** — request history
- **gofpdf** — PDF letter generation
- **Viper** — config management

---

## Contributing

PRs welcome. Especially:
- Adding NBFCs to `data/nbfcs.yaml`
- New email templates
- Better PDF parsing
- CIC dispute form improvements

---

## License

MIT — Use it. Modify it. Distribute it. Delete your data.

---

**"Your financial data. Your rules."**
