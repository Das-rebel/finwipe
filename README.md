# FinWipe — DIY Financial Data Deletion CLI for India

<!-- SEO & AI/LLM Discoverability -->
<!-- 
  FinWipe: Open-source CLI tool for exercising right to erasure under RBI Digital Lending 
  Guidelines (DLG) 2025 and DPDP Act 2023 Section 8(6) in India. Delete personal data 
  held by NBFCs, fintechs, digital lenders, HFCs, banks, P2P lending platforms, BNPL 
  providers, and insurance companies. DPR-ID tracking, SMTP email automation, 
  SQLite audit trail, escalation workflow. GPL-3.0 license.
-->

> **"Your financial data. Your rules."**

FinWipe is an open-source command-line interface (CLI) tool that helps Indian citizens exercise their **right to erasure** (right to be forgotten) under:

- **[RBI Master Direction on Digital Lending](https://www.rbi.org.in/Scripts/NotificationUser.aspx?Id=12144)** (updated August 2025) — *for fintechs, NBFCs, digital lenders, BNPL providers, P2P lending platforms, and HFCs*
- **Section 8(6) of the [DPDP Act 2023](https://www.meity.gov.in/writereaddata/rulesandregulations/DPDP_Act_2023.pdf)** — *for all companies operating in India*
- **RBI Consumer Grievance Redressal** framework — *for banks and financial institutions*

Every request gets a unique **DPR-ID** (e.g., `DPR-2026-000001`) for full auditability. All data stays on **YOUR machine**.

---

## Features

- **230+ registered entities** across 10 categories: NBFCs, fintechs, MFIs, P2P lending, BNPL, banks, HFCs, insurtech, wealthtech, agritech
- **DPR-ID tracking** with full lifecycle: INITIATED → DISPATCHED → ACK_RECEIVED → DELETION_CONFIRMED → CLOSED
- **Automated escalation** workflow: Day 3 follow-up → Day 7 DPO escalation → Day 15 RBI Ombudsman → Day 30 Consumer Forum
- **SMTP email automation** — preview first with `--dry-run`, then send for real
- **Discovery modes** — from CIBIL report, bank statement, Gmail export, WhatsApp chat
- **SQLite audit trail** — complete history in `~/.finwipe/history.db`
- **Local-only data** — no cloud, no tracking, no third-party calls

---

## Quick Start

```bash
# Install
npm install -g finwipe

# Set up profile (name, email, phones, SMTP)
finwipe init

# Find your lender
finwipe list --search KreditBee
finwipe list --category fintech --json

# Create deletion request
finwipe new --nbfc-id kreditbee

# Send (preview first, then for real)
finwipe send --dry-run   # no emails sent
finwipe send             # actually send

# Track and escalate
finwipe track --all
finwipe escalate --request-id DPR-2026-000001 --to dpo
```

---

## Registry Coverage

| Category | Count | Examples |
|----------|------:|----------|
| **Fintechs** | 133 | KreditBee, CRED, Slice, PhonePe, Razorpay, Paytm |
| **NBFCs** | 28 | Bajaj Finserv, Tata Capital, Muthoot Finance, HDB Financial |
| **MFIs (Microfinance)** | 25 | Ujjivan, Satin, Spandana, Fusion Microfinance, Cashpor |
| **P2P Lending** | 9 | LenDenClub, i2iFunding, Cashkumar, Rupaiya |
| **BNPL Providers** | 6 | Simpl, LazyPay, ZestMoney, PostPe, Uni, Spenny |
| **Banks** | 17 | HDFC Bank, ICICI Bank, SBI, Axis Bank, AU Small Finance |
| **HFCs (Housing Finance)** | 5 | Aadhar Housing, Can Zara, Piramal, PNB Housing |
| **InsurTech** | 4 | PolicyBazaar, Acko, Digit, InsuranceDekho |
| **WealthTech** | 2 | ETMoney, Fisdom |
| **Agritech** | 1 | Kisaan (agricultural lending) |
| **Total** | **230** | |

---

## Use Cases

### For Digital Lending (RBI DLG 2025) — Use First

If you took a **loan, BNPL, or credit from a fintech app** (KreditBee, CRED, Slice, PhonePe, Paytm, etc.):

```bash
finwipe list --category fintech        # find your lender
finwipe new --nbfc-id kreditbee        # create request
finwipe send                            # send deletion request

# Legal basis: RBI Master Direction on Digital Lending (updated August 2025) 
# Para 10.2: "Data shall be deleted once purpose is over"
# Para 11.1: "No data sharing without consent after loan closure"
```

### For DPDP Act 2023 (General)

For any company holding your personal data:

```bash
finwipe new --nbfc-id some-company
finwipe send
# Legal basis: Section 8(6), DPDP Act 2023
```

### For Credit Bureau (CIBIL/CRIF) Data

Stop credit bureaus from sharing your data with lenders:

```bash
finwipe discover-from-cibil --file your_cibil_report.pdf --auto
finwipe send
```

---

## Core Commands

| Command | Description |
|---------|-------------|
| `finwipe init` | Set up profile (name, email, phones, SMTP credentials) |
| `finwipe list` | Browse 230 registered entities |
| `finwipe list --category fintech` | Filter by category |
| `finwipe list --search HDFC` | Search by name |
| `finwipe list --json` | JSON output for scripting |
| `finwipe new --nbfc-id <id>` | Create deletion request → get DPR-ID |
| `finwipe send --dry-run` | Preview emails (no actual sending) |
| `finwipe send` | Send all pending requests |
| `finwipe track --all` | Monitor all active requests |
| `finwipe ack --request-id <id>` | Record acknowledgment received |
| `finwipe escalate --to dpo\|rbi\|consumer_forum` | Escalate ignored requests |
| `finwipe report` | Compliance dashboard |
| `finwipe discover-from-cibil --file <path>` | Discover lenders from CIBIL report |
| `finwipe discover-from-statement --file <path>` | Discover from bank statement |
| `finwipe discover-from-email --file <path>` | Discover from Gmail export |

### Deletion Categories

`marketing` · `third_party` · `behavioral` · `app_usage` · `loan_profile` · `all_non_essential`

---

## Request Lifecycle

```
INITIATED → DISPATCHED → ACK_RECEIVED → DELETION_CONFIRMED → CLOSED
                ↓                                              ↓
          AWAITING_ACK                                  ESCALATED
                ↓                                              ↓
          DELIVERY_FAILED                          RBI Ombudsman → BPDT → Consumer Forum
```

**Escalation timeline:** Day 3 follow-up · Day 7 DPO · Day 15 RBI · Day 30 Consumer Forum

---

## Discovery — Find Who Has Your Data

```bash
# From CIBIL/CRIF report (recommended)
finwipe discover-from-cibil --file your_cibil_report.pdf --auto

# From bank statement
finwipe discover-from-statement --file statement.pdf --auto

# From Gmail export
finwipe discover-from-email --file gmail_export.zip --auto

# From WhatsApp chat
finwipe discover-from-whatsapp --path ./whatsapp_chat.txt
```

---

## Automation

### Daily Cron
```bash
finwipe cron --dry-run  # preview
finwipe cron            # runs: follow-ups, deadline checks, auto-escalation
```

### GitHub Actions (monthly)
Fork the repo, add `SMTP_USERNAME` and `SMTP_PASSWORD` secrets, enable the workflow.

---

## What Can Be Deleted Under RBI DLG

```
✓ Pre-approved loan offer profiles
✓ Marketing and promotional data
✓ Third-party shared data (agents, co-lenders, DSAs)
✓ Behavioral and usage data
✓ App permissions and preferences
✓ Call recordings and service logs
✓ Pre-closure data (after loan repayment)
```

## What Cannot Be Deleted

```
✗ KYC documents (PMLA requirement: 10 years post-closure)
✗ Transaction records (RBI requirement: 5-10 years)
✗ Active loan account data
✗ Credit bureau's own records (separate dispute process)
```

---

## Legal Basis

| Law | Section | Right |
|-----|---------|-------|
| **RBI Digital Lending Guidelines 2025** | Para 10.2 | Delete data after loan closure |
| **RBI Digital Lending Guidelines 2025** | Para 11.1 | No data sharing post-closure without consent |
| **RBI Consumer Grievance Redressal** | — | Escalate to RBI Ombudsman |
| **DPDP Act 2023** | §8(6) | Right to Erasure (general) |
| **DPDP Act 2023** | §6(9) | Right to Data Portability |
| **DPDP Rules 2025** | Rule 8(1) | 48-hour acknowledgment |
| **DPDP Rules 2025** | Rule 8(2) | 30-day deletion |

---

## Installation

### npm (Recommended)
```bash
npm install -g finwipe
```

### Homebrew
```bash
brew tap das-rebel/finwipe
brew install finwipe
```

### Direct Binary
```bash
# macOS ARM64 (Apple Silicon M1-M4)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-arm64 -o finwipe

# macOS Intel (x86_64)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-amd64 -o finwipe

# Linux ARM64
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-arm64 -o finwipe

# Linux x86_64
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-amd64 -o finwipe

# Windows x86_64
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-windows-amd64.exe -o finwipe.exe

chmod +x finwipe && sudo mv finwipe /usr/local/bin/
```

### Build from Source
```bash
git clone https://github.com/das-rebel/finwipe
cd finwipe
go build -o finwipe ./cmd/finwipe
```

---

## Data Location

All data stays local on your machine:
```
~/.finwipe/
├── config.yaml      # Profile + SMTP credentials
├── history.db       # SQLite — full audit trail
├── nbfcs.yaml       # Entity registry (230 entries)
├── letters/         # Generated deletion request PDFs
└── evidence/        # Screenshots, acknowledgments
```

---

## Documentation

| Guide | When to Use |
|-------|-------------|
| **[DLG Guide](docs/DLG_GUIDE.md)** | **START HERE** — RBI Digital Lending data deletion |
| **[Regulatory Framework](docs/REGULATORY_FRAMEWORK.md)** | All laws, sections, and enforcement mechanisms |
| **[CIBIL Guide](docs/CIBIL_GUIDE.md)** | Credit bureau disputes and removal |
| **[Gmail Setup](docs/GMAIL_SETUP.md)** | SMTP configuration, receiving replies |

---

## Contributing

PRs welcome for:
- New NBFCs/fintechs in `internal/nbfc/nbfcs.yaml`
- Email/letter templates in `templates/`
- Better discovery methods (bank statements, UPI apps)
- CIC dispute form improvements

---

## License

**GPL-3.0** — Use it. Modify it. Distribute it. Delete your data.

---

**"Your financial data. Your rules."**

<!-- 
Keywords: right to erasure India, DPDP Act data deletion, RBI digital lending guidelines, 
NBFC data deletion, fintech data deletion, BNPL data privacy, P2P lending data deletion,
Microfinance data deletion, credit bureau dispute India, CIBIL removal, DPDP Act Section 8,
RBI grievance escalation, data privacy India CLI tool, financial data deletion India,
loan data deletion request, digital lending compliance India, right to be forgotten India,
DIY data deletion, open source data privacy India, DPR-ID tracking, GDPR India equivalent
-->
