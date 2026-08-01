# FinWipe — DIY Financial Data Deletion for India

> **"Your financial data. Your rules."**

FinWipe is an open-source CLI tool that helps Indian citizens exercise their **right to erasure** under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act) and Rule 8 of the DPDP Rules, 2025.

Every request gets a unique **DPR-ID** (e.g., `DPR-2026-000001`) for full auditability. All data stays on **YOUR machine**.

---

## 🚀 Install

**Option 1 — npm (recommended, cross-platform):**
```bash
npm install -g finwipe
```

**Option 2 — Homebrew (macOS/Linux):**
```bash
brew tap das-rebel/finwipe
brew install finwipe
```

**Option 3 — Direct binary (no package manager):**
```bash
# macOS (Apple Silicon / M1-M3)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-arm64 -o finwipe

# macOS (Intel)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-amd64 -o finwipe

# Linux (AMD64)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-amd64 -o finwipe

# Linux (ARM64 / Raspberry Pi)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-arm64 -o finwipe

# Windows
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-windows-amd64.exe -o finwipe.exe

chmod +x finwipe && sudo mv finwipe /usr/local/bin/

# Windows: save as finwipe.exe and run from Command Prompt or PowerShell
```

**Option 4 — Build from source:**
```bash
git clone https://github.com/das-rebel/finwipe
cd finwipe && go build -o finwipe ./cmd/finwipe
```

Verify it works:
```bash
finwipe --help
```

---

## ⚡ Quick Start (5 Steps)

```bash
# 1. Set up your profile (name, email, SMTP)
finwipe init

# 2. Browse the 91-entity registry
finwipe list --category fintech

# 3. Create a deletion request
finwipe new --nbfc-id bajaj-finserv

# 4. Preview, then send
finwipe send --dry-run    # preview first
finwipe send              # actually send emails

# 5. Track responses
finwipe track --all
```

---

## 📋 Core Commands

### `finwipe init`
One-time setup. Saves to `~/.finwipe/config.yaml`.
```bash
finwipe init
# Interactive prompts for:
#   Full name, email, phone, address
#   SMTP host (e.g. smtp.gmail.com), port (587), username, app password
```

### `finwipe list`
Browse 91 registered entities.
```bash
finwipe list                          # all 91
finwipe list --category fintech       # 59 fintechs
finwipe list --category bank          # 12 banks
finwipe list --category nbfc          # 18 NBFCs
finwipe list --search HDFC            # search by name
finwipe list --json                   # JSON output
```

### `finwipe new`
Create a deletion request → returns a **DPR-ID**.
```bash
finwipe new --nbfc-id bajaj-finserv
finwipe new --nbfc-id tata-capital --categories marketing,third_party
```

**Deletion categories:** `marketing` · `third_party` · `behavioral` · `app_usage` · `call_records` · `loan_profile` · `all_non_essential`

### `finwipe send`
Dispatch requests via email or registered post.
```bash
finwipe send --dry-run                # preview (no emails sent)
finwipe send                          # send all pending
finwipe send --request-id DPR-2026-000001
finwipe send --rate-limit 2000        # 2s between emails
finwipe send --channel post           # generate PDF letters
```

### `finwipe track`
Monitor the full lifecycle.
```bash
finwipe track --request-id DPR-2026-000001
finwipe track --all                   # all active
finwipe track --overdue               # past deadline
finwipe track --awaiting-ack          # no response yet
```

**Lifecycle:**
```
INITIATED → DISPATCHED → ACK_RECEIVED → RESPONSE_OK → CLOSED
                ↓                                   ↓
          AWAITING_ACK                        ESCALATED
                ↓                                   ↓
          DELIVERY_FAILED               DPDP_BOARD / RBI_OMBUDSMAN
```

### `finwipe ack`
Record an NBFC's acknowledgment.
```bash
finwipe ack --request-id DPR-2026-000001
finwipe ack --request-id DPR-2026-000001 --reference ABC123XYZ
```

### `finwipe escalate`
Escalate ignored requests up the chain.
```bash
finwipe escalate --request-id DPR-2026-000001 --to dpo           # L1: Data Protection Officer
finwipe escalate --request-id DPR-2026-000001 --to dpd_board     # L2: DPDP Board §27(3)
finwipe escalate --request-id DPR-2026-000001 --to rbi_ombudsman # L3: RBI
finwipe escalate --request-id DPR-2026-000001 --to consumer_forum # L3: Consumer Forum
```

### `finwipe close`
Close a request with an outcome.
```bash
finwipe close --request-id DPR-2026-000001 --outcome deleted
finwipe close --request-id DPR-2026-000001 --outcome rejected --notes "claimed already deleted"
```

**Outcomes:** `deleted` · `acknowledged_not_deleted` · `partial` · `rejected` · `exemption_claimed` · `no_response`

### `finwipe report`
Dashboard with compliance metrics.
```bash
finwipe report
finwipe report --days 30 --format json
```

---

## 🔍 Discovery Commands

Find out **who holds your data** before sending requests.

### `finwipe discover-from-cibil`
Parse a CIBIL report PDF → auto-create requests for every institution that queried your credit.
```bash
finwipe discover-from-cibil --file your_cibil_report.pdf --auto
```

### `finwipe discover-from-bureau`
All 4 bureaus: CIBIL, Experian, Equifax, CRIF HighMark.
```bash
finwipe discover-from-bureau --file Experian_Report.pdf --auto
```

### `finwipe discover-from-bank-statement`
Extract FI references from bank statement PDFs (EMI deductions, NACH mandates).
```bash
finwipe discover-from-statement --file statement.pdf
finwipe discover-from-statement --directory ./statements/ --auto
```

### `finwipe discover-from-email`
Parse Gmail Takeout exports.
```bash
finwipe discover-from-email --file gmail_export.zip --auto
```

### `finwipe discover-from-whatsapp`
Extract FI contacts from WhatsApp Business chat exports.
```bash
finwipe discover-from-whatsapp --path ./whatsapp_chat.txt
```

### `finwipe discover-from-aa`
Discover FIs via Account Aggregator apps (NADL, CAMS, SAafe, Finvu).
```bash
finwipe discover-from-aa --provider nadl
```

---

## ⚖️ Enforcement Commands

### `finwipe portability`
Request **all data** an entity holds about you (§6(9), DPDP Act). Company must respond in 72 hours.
```bash
finwipe portability --nbfc-id bajaj-finserv
finwipe portability --nbfc-id tata-capital --send
```

### `finwipe verify`
Confirm an NBFC actually deleted your data.
```bash
finwipe verify --request-id DPR-2026-000001 --method certificate
```
**Methods:** `email` · `certificate` · `login`

### `finwipe mass-request`
Send to ALL entities in a category at once.
```bash
finwipe mass-request --category fintech --dry-run
finwipe mass-request --category bank --exclude hdfc-bank,icici-bank
```
⚠️ You'll receive ~90 acknowledgment emails.

### `finwipe compliance`
Community compliance rates + shame list.
```bash
finwipe compliance
finwipe compliance --shame              # worst offenders
```

---

## 🤖 Automation

### `finwipe cron`
Daily automation: follow-ups, deadline checks, auto-escalation.
```bash
finwipe cron --dry-run                 # preview
finwipe cron                           # full automation
```
**Schedule:** Day 3 → follow-up · Day 7 → DPO escalation · Day 30 → DPDP Board

### `finwipe wizard`
Interactive guided flow for first-time users.
```bash
finwipe wizard
```

### `finwipe ask`
Quick consent-withdrawal wizard (no `finwipe init` needed).
```bash
finwipe ask
```

### Monthly GitHub Actions
Fork the repo, add secrets (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `FINWIPE_FULL_NAME`, `FINWIPE_EMAIL`, `FINWIPE_PHONE`, `FINWIPE_ADDRESS`), and the workflow runs `finwipe send` monthly on the 1st.

---

## 📄 Supporting Commands

| Command | Description |
|---------|-------------|
| `finwipe letter` | Generate PDF deletion letters |
| `finwipe evidence attach` | Attach screenshots/acknowledgments to a DPR-ID |
| `finwipe cic` | Generate CIC (CIBIL/Experian/Equifax/CRIF) dispute forms |
| `finwipe parse` | Parse CIBIL report → extract NBFC names |
| `finwipe update-registry` | Update NBFC registry from awesome-fintech-india |
| `finwipe setup-forward` | Set up FinWipe cloud inbox for passive discovery |
| `finwipe sync` | Sync discoveries from FinWipe cloud |
| `finwipe cloud-status` | Check cloud connectivity |

---

## 📊 By the Numbers

| Metric | Value |
|--------|------:|
| Version | 0.1.4 |
| Registry | 91 entities (12 banks · 18 NBFCs · 59 fintechs · 2 HFCs) |
| Legal basis | DPDP Act 2023 + RBI DLG |
| npm | [finwipe](https://www.npmjs.com/package/finwipe) |
| Homebrew | [das-rebel/finwipe](https://github.com/Das-rebel/homebrew-finwipe) |

---

## ⚖️ Legal Basis

| Law | Section | Right |
|-----|---------|-------|
| DPDP Act 2023 | §8(6) | Right to Erasure |
| DPDP Act 2023 | §6(9) | Right to Data Portability |
| DPDP Act 2023 | §27(3) | Complaint to DPDP Board |
| DPDP Rules 2025 | Rule 8(1) | 48-hour acknowledgment |
| DPDP Rules 2025 | Rule 8(2) | Deletion within 30 days |
| RBI DLG 2022 | Para 10.2, 11.1 | Data deletion in lending |

---

## ✅ What Can Be Deleted

```
✓ Marketing and promotional data
✓ Third-party shared data
✓ Behavioral and usage data
✓ Pre-approved loan offer profiles
✓ Call recordings and service logs
✓ App activity and preferences
```

## ❌ What Cannot Be Deleted

```
✗ KYC documents (PMLA: 10 years post-closure)
✗ Transaction records (RBI: 5-10 years)
✗ Active loan account data
✗ CIBIL's own records (separate fiduciary duty)
```

---

## 📁 Data Location

All data stays local at `~/.finwipe/`:
```
~/.finwipe/
├── config.yaml          # Profile + SMTP
├── history.db           # SQLite (WAL) — full audit trail
├── nbfcs.yaml           # Entity registry
├── letters/             # Generated PDFs
└── evidence/            # Screenshots, acknowledgments
```

---

## 🛠️ Tech Stack

- **Go 1.21+** — single binary, no runtime deps
- **Cobra** — CLI framework
- **SQLite (WAL)** — request history
- **gofpdf** — PDF generation
- **Viper** — configuration

---

## 🤝 Contributing

PRs welcome — especially:
- New NBFCs/fintechs in `data/nbfcs.yaml`
- Email/letter templates in `templates/`
- Better PDF parsing
- CIC dispute form improvements

---

## 📦 Installation Links

| Platform | Command / Link |
|----------|---------------|
| **npm** | `npm install -g finwipe` |
| **Homebrew** | `brew tap das-rebel/finwipe && brew install finwipe` |
| **macOS ARM64** | [finwipe-darwin-arm64](https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-arm64) |
| **macOS AMD64** | [finwipe-darwin-amd64](https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-amd64) |
| **Linux AMD64** | [finwipe-linux-amd64](https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-amd64) |
| **Linux ARM64** | [finwipe-linux-arm64](https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-arm64) |
| **Windows** | [finwipe-windows-amd64.exe](https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-windows-amd64.exe) |
| **Source** | `git clone https://github.com/das-rebel/finwipe` |

---

## 📖 Documentation

| Guide | Description |
|-------|-------------|
| **[Gmail Setup Guide](docs/GMAIL_SETUP.md)** | SMTP sending (Gmail), receiving replies via forwarding rules, alternatives (Outlook, iCloud, ProtonMail) |
| **[Regulatory Framework](docs/REGULATORY_FRAMEWORK.md)** | DPDPA 2023, RBI Guidelines, SEBI Circulars, CIC Dispute forms |
| **[CONTRIBUTING.md](CONTRIBUTING.md)** | How to contribute, add new NBFCs, submit pull requests |

---

## License

MIT — Use it. Modify it. Distribute it. Delete your data.

---

**"Your financial data. Your rules."**
