# FinWipe — Stop Digital Lenders from Holding Your Data

> **"Your financial data. Your rules."**

FinWipe is an open-source CLI tool that helps Indian citizens exercise their **right to erasure** under:
- **RBI Master Direction on Digital Lending** (updated August 2025) — *for fintechs, NBFCs, and digital lenders*
- **Section 8(6) of the DPDP Act, 2023** — *for all companies*

Every request gets a unique **DPR-ID** (e.g., `DPR-2026-000001`) for full auditability. All data stays on **YOUR machine**.

---

## ⚡ Quick Start

```bash
# 1. Install
npm install -g finwipe

# 2. Set up your profile (name, email, phones, SMTP)
finwipe init

# 3. Find your lender
finwipe list --search KreditBee

# 4. Create deletion request
finwipe new --nbfc-id kreditbee

# 5. Send
finwipe send --dry-run   # preview first
finwipe send             # actually send

# 6. Track
finwipe track --all
```

---

## 🎯 Use Cases

### For Digital Lending (DLG) — Recommended First

If you took a **loan, BNPL, or credit from a fintech app** (KreditBee, CRED, Slice, PhonePe, etc.):

```bash
# Find your lender
finwipe list --category fintech

# Send deletion request
finwipe new --nbfc-id kreditbee
finwipe send

# Follow up after 3 days
finwipe send --request-id DPR-2026-XXXXXX
```

**Legal basis:** RBI Master Direction on Digital Lending (updated August 2025) — Para 10.2, 11.1

### For DPDP Act (General)

For any company holding your data under the general data protection law:

```bash
finwipe new --nbfc-id some-company
finwipe send
```

**Legal basis:** Section 8(6), DPDP Act 2023

### For CIBIL/Bureau Data

Stop credit bureaus from sharing your data:

```bash
finwipe discover-from-cibil --file your_cibil_report.pdf --auto
finwipe send
```

See [CIBIL Guide](docs/CIBIL_GUIDE.md) for full process.

---

## 📋 Core Commands

| Command | Description |
|---------|-------------|
| `finwipe init` | Set up profile (name, email, phones, SMTP) |
| `finwipe list` | Browse 91 registered entities |
| `finwipe new` | Create deletion request → get DPR-ID |
| `finwipe send` | Send emails |
| `finwipe track` | Monitor lifecycle |
| `finwipe ack` | Record acknowledgment |
| `finwipe escalate` | Escalate ignored requests |
| `finwipe report` | Compliance dashboard |

### `finwipe init`

```bash
finwipe init
# Interactive prompts:
#   Full name
#   Email (for sending/receiving)
#   Phone(s) — include both numbers
#   Address
#   SMTP host, port, username, app password
```

### `finwipe list`

```bash
finwipe list                          # all 91
finwipe list --category fintech       # 59 fintechs
finwipe list --category nbfc         # 18 NBFCs
finwipe list --category bank         # 12 banks
finwipe list --search HDFC           # by name
finwipe list --json                  # JSON output
```

### `finwipe new`

```bash
finwipe new --nbfc-id bajaj-finserv
finwipe new --nbfc-id tata-capital --categories marketing,third_party
```

**Deletion categories:** `marketing` · `third_party` · `behavioral` · `app_usage` · `loan_profile` · `all_non_essential`

### `finwipe send`

```bash
finwipe send --dry-run                # preview (no emails sent)
finwipe send                          # send all pending
finwipe send --request-id DPR-2026-000001
finwipe send --rate-limit 2000       # 2s between emails
```

### `finwipe track`

```bash
finwipe track --request-id DPR-2026-000001
finwipe track --all                   # all active
finwipe track --overdue               # past deadline
```

**Lifecycle:**
```
INITIATED → DISPATCHED → ACK_RECEIVED → RESPONSE_OK → CLOSED
                ↓                                   ↓
          AWAITING_ACK                        ESCALATED
                ↓                                   ↓
          DELIVERY_FAILED               RBI OM ~ BPDT ~ Consumer Forum
```

### `finwipe escalate`

```bash
finwipe escalate --request-id DPR-2026-000001 --to dpo            # L1: DPO
finwipe escalate --request-id DPR-2026-000001 --to dpd_board    # L2: DPDP Board
finwipe escalate --request-id DPR-2026-000001 --to rbi_ombudsman # L3: RBI
finwipe escalate --request-id DPR-2026-000001 --to consumer_forum # L4: Consumer Forum
```

---

## 🔍 Discovery — Find Who Has Your Data

### From CIBIL Report
```bash
finwipe discover-from-cibil --file your_cibil_report.pdf --auto
```

### From Bank Statement
```bash
finwipe discover-from-statement --file statement.pdf
finwipe discover-from-statement --directory ./statements/ --auto
```

### From Gmail
```bash
finwipe discover-from-email --file gmail_export.zip --auto
```

### From WhatsApp
```bash
finwipe discover-from-whatsapp --path ./whatsapp_chat.txt
```

---

## 🤖 Automation

### Daily Cron
```bash
finwipe cron --dry-run  # preview
finwipe cron            # runs: follow-ups, deadline checks, auto-escalation
```

**Schedule:** Day 3 → follow-up · Day 7 → DPO escalation · Day 30 → DPDP Board

### Monthly GitHub Actions
Fork the repo, add secrets, and the workflow runs monthly.

---

## 📖 Documentation

| Guide | When to Use |
|-------|-------------|
| **[DLG Guide](docs/DLG_GUIDE.md)** | **START HERE** — Digital lending data deletion |
| **[Gmail Setup](docs/GMAIL_SETUP.md)** | SMTP setup, receiving replies |
| **[Regulatory Framework](docs/REGULATORY_FRAMEWORK.md)** | All laws and sections |
| **[CIBIL Guide](docs/CIBIL_GUIDE.md)** | Credit bureau disputes |

---

## ⚖️ Legal Basis

### For Digital Lenders (Use First)

| Law | Section | What It Says |
|-----|---------|--------------|
| **RBI DLG 2025** | Para 10.2 | "Data shall be deleted once purpose is over" |
| **RBI DLG 2025** | Para 11.1 | "No data sharing without consent after closure" |

### For All Companies

| Law | Section | Right |
|-----|---------|-------|
| **DPDP Act 2023** | §8(6) | Right to Erasure |
| **DPDP Act 2023** | §6(9) | Right to Data Portability |
| **DPDP Rules 2025** | Rule 8(1) | 48-hour acknowledgment |
| **DPDP Rules 2025** | Rule 8(2) | 30-day deletion |

---

## ✅ What Can Be Deleted

```
✓ Marketing and promotional data
✓ Third-party shared data
✓ Behavioral and usage data
✓ Pre-approved loan offer profiles
✓ App activity and preferences
```

## ❌ What Cannot Be Deleted

```
✗ KYC documents (PMLA: 10 years post-closure)
✗ Transaction records (RBI: 5-10 years)
✗ Active loan account data
✗ CIBIL's own records (use separate process)
```

---

## 📊 Registry

| Category | Count |
|----------|------:|
| Fintechs | 59 |
| NBFCs | 18 |
| Banks | 12 |
| HFCs | 2 |
| **Total** | **91** |

---

## 💻 Installation

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
# macOS ARM64 (M1-M3)
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-arm64 -o finwipe

# macOS Intel
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-darwin-amd64 -o finwipe

# Linux
curl -fsSL https://github.com/Das-rebel/finwipe/releases/latest/download/finwipe-linux-amd64 -o finwipe

chmod +x finwipe && sudo mv finwipe /usr/local/bin/
```

### Build from Source
```bash
git clone https://github.com/das-rebel/finwipe
cd finwipe && go build -o finwipe ./cmd/finwipe
```

---

## 📁 Data Location

All data stays local:
```
~/.finwipe/
├── config.yaml      # Profile + SMTP
├── history.db       # SQLite — full audit trail
├── nbfcs.yaml      # Entity registry
├── letters/        # Generated PDFs
└── evidence/       # Screenshots, acknowledgments
```

---

## 🤝 Contributing

PRs welcome for:
- New NBFCs/fintechs in `data/nbfcs.yaml`
- Email/letter templates in `templates/`
- Better discovery methods
- CIC dispute form improvements

---

## License

MIT — Use it. Modify it. Distribute it. Delete your data.

---

**"Your financial data. Your rules."**
