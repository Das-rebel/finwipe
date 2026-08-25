# FinWipe – Delete Your Data from Indian Fintechs

**Stop lenders from selling your data.** Use your right to erasure under the DPDP Act 2023.

FinWipe automates deletion requests to 230+ NBFCs, fintechs, and banks in India. Send requests in minutes, track responses, escalate to the RBI if they ignore you.

[![Latest Release](https://img.shields.io/github/v/tag/Das-rebel/finwipe?include_prereleases&label=download)](https://github.com/Das-rebel/finwipe/releases)
[![License](https://img.shields.io/github/license/Das-rebel/finwipe)](LICENSE)
[![Go 1.21+](https://img.shields.io/badge/go-1.21+-blue)](https://golang.org)

---

## ⚡ 5-Minute Setup

### Install & Run

**macOS / Linux:**
```bash
# Download the latest binary
curl -L https://github.com/Das-rebel/finwipe/releases/download/v0.2.24/finwipe-$(uname -s)-$(uname -m) -o finwipe
chmod +x finwipe
./finwipe init
```

**Windows:**
```bash
# Download .exe from releases page
# https://github.com/Das-rebel/finwipe/releases
finwipe.exe init
```

**Or via npm:**
```bash
npm install -g finwipe
finwipe init
```

### First Deletion Request (7 steps)

```bash
# 1. Set up your profile (name, email, phone, SMTP password)
finwipe init

# 2. Download the list of lenders
finwipe update-registry

# 3. Find a lender (e.g., Bajaj Finserv)
finwipe list --search bajaj

# 4. Create a deletion request (you'll get a unique DPR-ID)
finwipe new --nbfc bajaj-finserv --reason "unused account"

# 5. Review the email before sending
finwipe send --dry-run

# 6. Send it
finwipe send

# 7. Track their response
finwipe track
```

That's it. You're done. The tool will monitor your inbox and alert you when they respond.

---

## 🎯 What Can You Delete From?

230+ entities including:

- **Major banks** – SBI, HDFC, ICICI, Axis
- **Fintechs** – PayU, Instacred, Cashe, LazyPay
- **NBFCs** – Bajaj Finserv, Shriram Finance, HDB Financial
- **Credit bureaus** – CIBIL, CRIF, Experian

```bash
finwipe list                    # see all lenders
finwipe list --search payu      # search for a specific one
```

---

## 📧 Email Setup

You need to give FinWipe permission to:
1. Send emails from your account
2. Read your inbox (to track their responses)

**Gmail (easiest):**
1. Go to [myaccount.google.com/security](https://myaccount.google.com/security)
2. Turn on **2-Step Verification** (if not already on)
3. Go to **App Passwords** (near the bottom)
4. Select "Mail" and "Windows Computer"
5. Google will give you a 16-character password → copy it
6. When `finwipe init` asks for your SMTP password, paste it there

**Outlook:**
1. Go to [account.microsoft.com/security](https://account.microsoft.com/security)
2. Click **Advanced security options**
3. Under **App passwords**, create one for Mail
4. Use that password in `finwipe init`

**Yahoo:**
1. Go to [account.yahoo.com](https://account.yahoo.com)
2. **Account → Security → Generate app password**
3. Use that in `finwipe init`

---

## 📋 How It Works

### 1. **Create a Request**
FinWipe generates a formal deletion letter with your details and a unique request ID (e.g., `DPR-2026-000001`).

```bash
finwipe new --nbfc bajaj-finserv
```

### 2. **Send It**
Email is sent to the lender's legal/grievance email with a PDF attachment.

```bash
finwipe send
```

### 3. **Track Responses**
FinWipe monitors your inbox. When the lender replies, it automatically records their response.

```bash
finwipe track              # check once
finwipe cron               # auto-check every day
```

### 4. **They Ignore You? Escalate.**
If they don't respond in 30 days, file a complaint with the RBI.

```bash
finwipe escalate --request-id DPR-2026-000001
```

### 5. **Keep Proof**
Export a PDF report of all your requests + their responses for your records.

```bash
finwipe compliance --request-id DPR-2026-000001 --format pdf
```

---

## 🚀 Handy Commands

```bash
# See all your requests
finwipe status

# Check a specific request
finwipe status --request-id DPR-2026-000001

# Manually record their response (if tracking fails)
finwipe ack --request-id DPR-2026-000001

# Close a request when they confirm deletion
finwipe close --request-id DPR-2026-000001 --outcome deleted

# Send to multiple lenders at once
finwipe mass --entities bajaj,hdfc,icici --reason "unused"

# Get an interactive guide
finwipe wizard

# Need help?
finwipe --help
```

---

## 🤖 Auto-Track Mode (Optional)

**One-time setup for hands-off tracking:**

```bash
# Start a background service that checks your inbox every 5 minutes
# and auto-updates your request status
docker run -d \
  --name finwipe-server \
  -p 8080:8080 \
  -e FINWIPE_IMAP_HOST=imap.gmail.com \
  -e FINWIPE_IMAP_USER=you@gmail.com \
  -e FINWIPE_IMAP_PASS=YOUR_APP_PASSWORD \
  -v finwipe-data:/data \
  ghcr.io/das-rebel/finwipe-server:latest

# Open http://localhost:8080 to see a live dashboard
```

This is completely optional. The CLI tool works fine without it.

---

## 📁 Where's My Data?

Everything is stored locally on your machine:

```
~/.finwipe/
├── config.yaml          # Your profile (name, email, SMTP password)
└── finwipe.db           # All your requests & responses
```

**It's encrypted.** Your SMTP password is protected. No data leaves your computer unless you tell it to.

---

## ⚖️ Is This Legal?

**Yes.** You have the right to erasure under:

- **DPDP Act 2023** (Section 8) – Indian data protection law
- **RBI Digital Lending Guidelines 2025** – Lenders must delete data on request
- **CIBIL Regulations** – You can request your credit record be deleted

See [`docs/REGULATORY_FRAMEWORK.md`](docs/REGULATORY_FRAMEWORK.md) for legal details.

---

## ❓ FAQ

**Q: Will this really work?**  
A: Lenders are legally required to respond. If they don't, you can escalate to the RBI. FinWipe helps you track who's ignoring you.

**Q: Is my Gmail password safe?**  
A: FinWipe never stores your Gmail password—only an "app password" that you can revoke anytime. It's stored locally, encrypted.

**Q: Can I use a different email provider?**  
A: Yes. During `finwipe init`, enter your email provider's IMAP server. Works with Outlook, Yahoo, ProtonMail, etc.

**Q: What if a lender says no?**  
A: Document their response. Escalate to the RBI. They can't legally refuse unless the data is required for a live loan.

**Q: Can I batch-send to many lenders?**  
A: Yes. `finwipe mass --entities bajaj,hdfc,icici --reason "unused"`

**Q: Do I need Docker for the auto-tracker?**  
A: No, it's optional. The CLI tool works 100% standalone. Docker is only for hands-off auto-tracking.

**Q: Is there a web UI?**  
A: Yes, but optional. `finwipe-server` includes a dashboard. Use it if you want real-time status updates. The CLI tool is fully functional on its own.

**Q: What if I lose my laptop?**  
A: Your requests are stored in `~/.finwipe/finwipe.db`. Back it up or use the server mode with a persistent volume.

---

## 🛠️ Troubleshooting

**"Authentication failed" when sending emails?**
- Verify your app password (not your account password)
- For Gmail, confirm 2-Step Verification is on
- Try resetting the app password and re-running `finwipe init`

**"Entity not found" when searching?**
- Run `finwipe update-registry` to refresh the lender list
- Search by partial name: `finwipe list --search pay` (instead of "PayU")

**Can't find your inbox responses?**
- Check the SPAM folder (add finwipe@yourmail to contacts)
- Run `finwipe track --verbose` to see what it's looking for

**Stuck? Need help?**
- Open an issue: [github.com/Das-rebel/finwipe/issues](https://github.com/Das-rebel/finwipe/issues)
- Read docs: [`docs/server.md`](docs/server.md)

---

## 🔒 Privacy & Security

- ✅ Everything runs locally (no cloud, no external servers)
- ✅ Your email password never leaves your machine
- ✅ App password can be revoked anytime
- ✅ Open source – inspect the code yourself
- ✅ No analytics, no tracking, no ads

---

## 📚 Learn More

- [DPDP Act compliance guide](docs/REGULATORY_FRAMEWORK.md)
- [Gmail app password setup (step-by-step)](docs/GMAIL_SETUP.md)
- [Credit bureau deletion guide](docs/CIBIL_GUIDE.md)
- [RBI Digital Lending Guidelines](docs/DLG_GUIDE.md)
- [Full API reference](docs/server.md)

---

## 🤝 Want to Help?

- Found a bug? [File an issue](https://github.com/Das-rebel/finwipe/issues)
- Missing a lender? [Add it to the registry](https://github.com/Das-rebel/finwipe/blob/main/internal/nbfc/nbfcs.yaml)
- Know other people who need this? Share it 💙

---

## 📝 License

MIT © Das-Rebel and contributors

---

**Your data. Your choice. Your right.**

*Stop waiting. Send your first deletion request today.*

```bash
finwipe init && finwipe wizard
```

---

## 🔧 More from Das-rebel

- **[A3M Router](https://github.com/Das-rebel/a3m-router)** — Open-source LLM routing gateway. Routes AI queries across 80+ providers with biology-inspired algorithms (EXP3, swarm intelligence). 92% cost savings, self-hosted. `npm install a3m-router` · `pip install a3m-router`
