# Gmail Setup for FinWipe

FinWipe needs Gmail in two directions:

| Direction | Purpose | Method |
|-----------|---------|--------|
| **Send** | Dispatch deletion requests to NBFCs | SMTP (port 465 SSL or 587 STARTTLS) |
| **Receive** | Track NBFC replies & acknowledgments | Gmail forwarding rules |

This guide covers both, plus alternatives if you don't use Gmail.

---

## Part 1: Sending — SMTP Setup

Run `finwipe init` and enter your Gmail credentials:

```bash
finwipe init
```

### What to enter:

```
Profile name: Your Name
Email: your-email@gmail.com
Phone: +91 XXXXXXXXXX
Address: Your full address (for registered letters)

SMTP Host: smtp.gmail.com
SMTP Port: 465  ← recommended (SSL)
Username: your-email@gmail.com
App Password: xxxxxxxxxxxx  ← Gmail app password
```

> **Port 465 (SSL)** is recommended. Port 587 (STARTTLS) also works.
> 
> **App Password**: Gmail requires an [App Password](https://myaccount.google.com/apppasswords), not your regular password. Generate one at:
> `Google Account → Security → App passwords → Mail → FinWipe`

### If sending fails with TLS error:

- Port **465** (SSL): `tls.Dial` — implicit TLS from connection start
- Port **587** (STARTTLS): plain TCP → `STARTTLS` upgrade

Both are supported. If 465 fails, try 587.

---

## Part 2: Receiving — Gmail Forwarding (Recommended)

### Step 1: Get your FinWipe inbox address

```bash
finwipe setup-forward
```

This gives you a unique address like:
```
usd752c089ed9@inbox.finwipe.in
```

**All NBFC replies forwarded here are automatically tracked.**

### Step 2: Create a Gmail forwarding rule

Gmail can't natively forward to custom domains, so we use **filters**:

1. Go to [Gmail Settings → Filters](https://mail.google.com/mail/settings/filters)
2. Click **Create a new filter**
3. Add these filter conditions (any that apply):
   - **From**: `grievance@`, `nodalofficer@`, `grievance.officer@`, `care@`, `support@`
   - **Has the words**: `DPR-`, `deletion request`, `DPDP`, `data deletion`, `personal data`
   - **Subject**: `deletion`, `DPDP`, `acknowledgment`
4. Click **Create filter**
5. Check **Skip the Inbox (Archive it)** + **Apply the label: FinWipe** + **Forward it to**: `[your FinWipe inbox address]`

#### Alternative: Forward specific domains only

To avoid forwarding everything, filter by NBFC domains you contacted:

```
From: (@hdfcbank.com OR @icicibank.com OR @sbicards.com OR @bajaj.com OR @tatacapital.com ...)
```

### Step 3: Sync replies to FinWipe

```bash
# Sync from FinWipe cloud
finwipe sync

# Or import from local email export
finwipe check-inbox --import ~/Downloads/emails/
```

---

## Part 3: Gmail API (Alternative — No App Password)

If you don't want to use SMTP/app passwords, use Gmail API:

### Option A: Gmail API with OAuth (Most Secure)

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create project → Enable Gmail API
3. Create OAuth 2.0 credentials (Desktop app)
4. Download `credentials.json`
5. Place in `~/.finwipe/credentials.json`
6. FinWipe will prompt for authorization on first run

> This method doesn't store your password — it uses Google's OAuth flow.

### Option B: IMAP (Read-only, for receiving)

```bash
# Configure IMAP in Gmail
# Settings → See all settings → Forwarding and POP/IMAP → Enable IMAP

# Use any email client (Thunderbird, Apple Mail, etc.)
# to forward NBFC replies to your FinWipe inbox address
```

---

## Part 4: Non-Gmail Alternatives

### Outlook / Hotmail

```yaml
smtp:
  host: smtp-mail.outlook.com
  port: 587  # STARTTLS
  username: your-email@outlook.com
  password: your-app-password
```

### Yahoo Mail

```yaml
smtp:
  host: smtp.mail.yahoo.com
  port: 587  # STARTTLS
  username: your-email@yahoo.com
  password: your-app-password
```

### ProtonMail

ProtonMail blocks SMTP to external domains on free plans. Options:

1. **ProtonMail Bridge** (paid) — exposes IMAP/SMTP
2. **Custom domain** with ProtonMail
3. Use a forwarding service (like `forwardemail.net`)

```yaml
smtp:
  host: 127.0.0.1   # If using ProtonMail Bridge
  port: 1025         # Local IMAP port
```

### iCloud Mail

```yaml
smtp:
  host: smtp.mail.me.com
  port: 587  # STARTTLS
  username: your-email@icloud.com
  password: your-app-password  # iCloud app password
```

---

## Part 5: Troubleshooting

### "TLS connection failed" on port 587

- Port 587 uses **STARTTLS** (not raw TLS)
- Make sure your network allows outbound port 587
- Try port **465** (SSL) instead

### "Authentication failed" 

- Gmail requires an **App Password**, not your regular password
- Generate at: `myaccount.google.com → Security → App passwords`
- If you have 2FA, app password is mandatory

### "535 Authentication credentials invalid"

- Username must be the full Gmail address (`user@gmail.com`)
- App password should have no spaces
- Double-check the app password is correct

### Gmail blocking "less secure apps"

Gmail no longer has "Less Secure Apps." If SMTP fails:
1. Go to [Google Account → Security](https://myaccount.google.com/security)
2. Ensure **2-Step Verification** is ON
3. Generate a new **App Password** at `myaccount.google.com/apppasswords`

### Forwarding not working

1. Verify your FinWipe inbox address is correct: `finwipe setup-forward`
2. Check Gmail filter doesn't have typos
3. Test by sending an email to your FinWipe inbox address
4. Run `finwipe sync` to pull new emails

---

## Quick Reference

| Task | Command |
|------|---------|
| Set up Gmail sending | `finwipe init` |
| Get inbox address | `finwipe setup-forward` |
| Pull NBFC replies | `finwipe sync` |
| Check status | `finwipe track --all` |
| Test SMTP | `finwipe send --dry-run` |

---

## Architecture Overview

```
                    ┌─────────────────────────────────────────────┐
                    │              Your Gmail Account               │
                    │                                              │
NBFC ──reply──►  Filter Rule ──forward──►  FinWipe Cloud         │
                         │                  (inbox.finwipe.in)      │
                         │                        │                │
                    ┌────┴────┐           ┌──────┴──────┐         │
                    │ FinWipe │           │   finwipe     │         │
                    │  SMTP   │           │   sync        │         │
                    │ (send)  │           │  (receive)    │         │
                    └────┬────┘           └──────┬───────┘         │
                         │                       │                   │
                    smtp.gmail.com          inbox.finwipe.in        │
                         │                                          │
                    DPR Email ──────►  NBFC Grievance Officer       │
                    (sdas22@gmail.com → gro@nbfc.com)              │
                    └─────────────────────────────────────────────┘
```
