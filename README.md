# FinWipe — DIY Financial Data Deletion for India

> *"Send data deletion requests to every NBFC, fintech, and data broker that has your financial data — under India's DPDPA 2023 and RBI Digital Lending Guidelines."*

**Eraser, but for India.** FinWipe is an open-source CLI tool that helps Indian citizens exercise their right to erasure under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act) and Rule 8 of the DPDP Rules, 2025.

All deletion requests are sent from **YOUR email**. All data stays on **YOUR machine**. No platform lock-in.

---

## What it does

```
✉️  Send legally-grounded deletion emails to 247+ NBFCs, fintechs, and lenders
📄  Generate professional PDF deletion letters for registered post
🏛️  Generate pre-filled CIBIL/Experian/Equifax dispute forms
📊  Track every request with full audit history
🔄  Run monthly via GitHub Actions — set and forget
```

---

## No External Dependencies

- **No CIBIL credentials stored**
- **No MSG91 or SMS API**
- **No platform account required**
- **No data leaves your server**

You provide your own Gmail app password. All emails come from you.

---

## Quick Start

```bash
# Install
git clone https://github.com/das-rebel/finwipe
cd finwipe
go build -o finwipe ./cmd/finwipe

# Setup (interactive — asks for your name, email, address, SMTP)
./finwipe init

# Preview what would happen
./finwipe send --dry-run

# Send deletion emails
./finwipe send

# Generate PDF letters for registered post
./finwipe letter

# Check status
./finwipe status

# Generate CIBIL dispute forms
./finwipe cic --bureau CIBIL
```

---

## Commands

| Command | What it does |
|---------|-------------|
| `./finwipe init` | Setup your profile + SMTP |
| `./finwipe list` | Browse 247+ NBFCs in registry |
| `./finwipe list --category fintech` | Filter by category |
| `./finwipe send --dry-run` | Preview emails without sending |
| `./finwipe send` | Send deletion emails to all NBFCs |
| `./finwipe send --include bajaj-finserv,tata-capital` | Send to specific NBFCs |
| `./finwipe send --exclude-category bank` | Exclude banks |
| `./finwipe status` | Full status report |
| `./finwipe letter` | Generate PDF deletion letters |
| `./finwipe letter --nbfcs bajaj-finserv` | Letter for specific NBFC |
| `./finwipe cic --bureau CIBIL` | Generate CIBIL dispute form |
| `./finwipe parse report.pdf` | Extract NBFCs from CIBIL PDF |

---

## How it works

```
┌─────────────────────────────────────────────────┐
│  YOU                                           │
│  ┌──────────────────────────────────────────┐  │
│  │  FinWipe CLI (your machine)               │  │
│  │                                          │  │
│  │  nbfcs.yaml → your Gmail SMTP           │  │
│  │  → Emails sent FROM your address         │  │
│  │  → History stored in ~/.finwipe/       │  │
│  │  → PDF letters in ~/.finwipe/letters/   │  │
│  └──────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

---

## Legal Basis

FinWipe invokes:

- **Section 8(6) DPDP Act 2023** — Right to Erasure
- **Rule 8 DPDP Rules 2025** — Purpose-based retention, 48-hour acknowledgment
- **RBI Digital Lending Guidelines (DLG) 2022** — Para 10.2, 11.1, 11.2

Read the [REGULATORY_FRAMEWORK.md](docs/REGULATORY_FRAMEWORK.md) for full details.

---

## What CAN be deleted

```
✓ Marketing and promotional data
✓ Third-party shared data
✓ Behavioral/app usage data
✓ Pre-approved loan offer profiles
✓ Call recordings and service logs
```

## What CANNOT be deleted

```
✗ KYC documents (PMLA: 10 years post-closure)
✗ Transaction records (RBI: 5-10 years)
✗ Active loan account data
✗ CIBIL's own records (separate fiduciary)
```

---

## Data Directory

The registry lives in `data/nbfcs.yaml`. Add new NBFCs via GitHub PR or manually:

```yaml
- id: new-nbfc
  name: New Finance Company
  short_name: NewFin
  category: nbfc
  grievance_email: grievance@newfin.com
  address: "NewFin, Mumbai"
  active: true
```

---

## Monthly Automation (GitHub Actions)

Fork and add secrets:

```bash
# .github/workflows/finwipe.yml
- name: Send deletion emails
  run: ./finwipe send --rate-limit 2000
  env:
    SMTP_HOST: ${{ secrets.SMTP_HOST }}
    SMTP_PASSWORD: ${{ secrets.SMTP_PASSWORD }}
```

---

## The Honest Limitation

```
FinWipe cannot guarantee NBFCs will respond within 30 days.
It CAN guarantee:
✓ Every request is timestamped
✓ You have evidence of what was sent
✓ You have legal documentation
✓ You can escalate: RBI Sachet, DPDP Board
```

---

## Tech Stack

- **Go 1.21+** — single binary, no runtime deps
- **Cobra** — CLI framework
- **SQLite (WAL mode)** — request history
- **gofpdf** — PDF letter generation
- **Viper** — config management

---

## License

MIT — Use it. Modify it. Distribute it. Delete your data.

---

## Contributing

PRs welcome. Especially:
- Adding NBFCs to `data/nbfcs.yaml`
- New email templates
- Better CIBIL PDF parsing
- CIC dispute form improvements

---

*"Your financial data. Your rules."*
