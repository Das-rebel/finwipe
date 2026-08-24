# FinWipe Server — Self‑Hosted Erasure‑Request Tracker

A lightweight, zero‑dependency service that watches a Gmail/IMAP inbox for
acknowledgements from lenders and automatically advances the lifecycle of
Data‑Protection‑Request (DPR) records stored in an SQLite database.

* **No external services** – runs on your own machine or in a Docker container.  
* **Multi‑arch image** – `amd64` & `arm64` supported.  
* **JSON APIs** – `/api/status` (current DPR state) and `/api/mcp` (MCP shim).  
* **Web dashboard** – dark‑theme UI at `/` that lists all open DPRs.  

---

## Quick start (local)

```bash
# 1️⃣ Clone and enter the repo
git clone https://github.com/Das-rebel/finwipe.git
cd finwipe

# 2️⃣ Build the binary (uses Go 1.22+)
go build ./cmd/server

# 3️⃣ Create a tiny SQLite DB (the CLI will do this automatically on first run)
FINWIPE_DB=/tmp/fw-test.db ./cmd/server -db /tmp/fw-test.db -addr :18099
```

Now open a browser at `http://localhost:18099`:

* `/` → Dashboard (dark theme)  
* `/api/status` → JSON payload with all open DPRs  
* `POST /api/mcp` → MCP shim (e.g. `{"method":"list_requests"}`)

---

## Running via Docker (recommended for production)

### 1️⃣ Build & push (automated on tag)

The repo ships a dedicated CI workflow that builds a **multi‑arch** image and
pushes it to **GitHub Container Registry (GHCR)** whenever a tag that matches
`v*` is pushed.

```bash
# Tag a new version (example)
git tag v0.3.0
git push origin v0.3.0   # triggers GitHub Actions → image pushed to ghcr.io/das-rebel/finwipe-server
```

### 2️⃣ Run the container

```bash
docker run -d \
  --name finwipe \
  -p 8080:8080 \
  -e FINWIPE_IMAP_HOST=imap.gmail.com \
  -e FINWIPE_IMAP_USER=you@gmail.com \
  -e FINWIPE_IMAP_PASS=YOUR_APP_PASSWORD \
  -e FINWIPE_DB=/data/finwipe.db \
  ghcr.io/das-rebel/finwipe-server:latest
```

The container:

* Listens on **port 8080** (exposed as `http://localhost:8080`).  
* Stores its SQLite DB in a **named volume** (`/data`).  
* Requires **app‑password** env vars for Gmail/IMAP access (see below).

### 3️⃣ Environment variables

| Variable | Required? | Description |
|----------|-----------|-------------|
| `FINWIPE_IMAP_HOST` | ✅ | IMAP server hostname (e.g. `imap.gmail.com`). |
| `FINWIPE_IMAP_PORT` | ❌ | Default `993`. |
| `FINWIPE_IMAP_USER` | ✅ | Email address used for the account. |
| `FINWIPE_IMAP_PASS` | ✅ | **App password** (Google, Outlook, etc.). |
| `FINWIPE_DB` | ✅ | Path to the SQLite file (mounted as a volume). |
| `FINWIPE_NTFY_URL` | ❌ *(removed in v0.2)* | No longer needed – push notifications were removed. |

### 4️⃣ Data persistence

Create a persistent volume so the DB survives container restarts:

```bash
docker volume create --name finwipe-data
docker run -d \
  --name finwipe \
  -p 8080:8080 \
  -e FINWIPE_IMAP_HOST=imap.gmail.com \
  -e FINWIPE_IMAP_USER=you@gmail.com \
  -e FINWIPE_IMAP_PASS=YOUR_APP_PASSWORD \
  -v finwipe-data:/data \
  ghcr.io/das-rebel/finwipe-server:latest
```

The DB will live at `/data/finwipe.db` inside the container.

---

## API reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | `GET` | HTML dashboard (dark UI). |
| `/api/status` | `GET` | JSON array of open DPR requests. Each object contains:<br>`id`, `entity_name`, `grievance_email`, `state`, `sent_at`, `acked_at`. |
| `/api/mcp` | `POST` | Minimal JSON‑RPC shim for external agents.<br>Supported methods:<br>`list_requests` → returns the same payload as `/api/status`.<br>`escalate` → transitions a request to `ESCALATED` state; body: `{ `"dpr_id":"DPR‑xxxx"` }`. |
| `POST /api/mcp` (internal) | `POST` | Used by the **OmniClaw MCP** integration to query or control the server from other agents. |

### Example `list_requests` call

```bash
curl -s -X POST http://localhost:8080/api/mcp \
  -H "Content-Type: application/json" \
  -d '{"method":"list_requests"}'
```

Response:

```json
{
  "requests": [
    {
      "ID":"DPR-2026-000001",
      "EntityName":"Bajaj Finserv Ltd",
      "GrievanceEmail":"gro@bajajfinserv.in",
      "State":"DISPATCHED",
      "SentAt":"2026-07-15T17:38:48Z",
      "AckedAt":null
    }
  ]
}
```

### Example `escalate` call

```bash
curl -s -X POST http://localhost:8080/api/mcp \
  -H "Content-Type: application/json" \
  -d '{"method":"escalate","params":{"dpr_id":"DPR-2026-000001"}}'
```

Response:

```json
{"ok":true}
```

After escalation, a subsequent `list_requests` call will show `State":"ESCALATED"`.

---

## IMAP authentication

The server uses **app passwords** (recommended) rather than OAuth because they are
simple to generate and revoke.

* **Google** – Settings → Security → App passwords → Generate → use the 16‑character token as `FINWIPE_IMAP_PASS`.  
* **Outlook/Office365** – Security → App passwords → Create a new password.  
* **Yahoo** – Account Info → Account Security → Generate app password.

The IMAP client only needs read‑write access to the `INBOX` folder; no other
permissions are required.

---

## Development checklist

| ✅ | Item |
|---|------|
| **Build** | `go build ./...` (no external CGO deps) |
| **Vet** | `go vet ./...` (cleans warnings) |
| **Test** | Run unit‑tests (`go test ./...`) – currently only sanity checks. |
| **Linter** | `golangci-lint run` (optional). |
| **Docker** | `docker compose up --build` (spins up the service locally). |
| **CI** | Tag push (`vX.Y.Z`) triggers GitHub Actions → multi‑arch image pushed to GHCR. |
| **Documentation** | `docs/server.md` (this file) + `README.md` at repo root (see below). |

---

## Repository layout (relevant files)

```
finwipe/
├─ cmd/
│  └─ server/                # entry point (main.go)
├─ internal/
│  └─ server/
│     ├─ store.go            # SQLite wrapper
│     ├─ poller.go           # IMAP fetch + state machine
│     ├─ imap.go             # Real IMAP client (go‑imap/v2)
│     ├─ http.go             # dashboard + APIs + MCP shim
│     └─ dashboard.html      # embedded UI
├─ Dockerfile.server         # multi‑stage image build
├─ docker-compose.yml        # dev‑run example
├─ .github/
│  └─ workflows/
│     └─ docker.yml           # CI → GHCR publishing
├─ VERSION                   # current semver (e.g. 0.2.0)
└─ docs/
   └─ server.md              # this documentation
```

---

## Frequently asked questions

**Q: Do I need a Google Cloud project?**  
No. A regular Gmail account with an **app password** works fine.

**Q: Can I use a different IMAP provider?**  
Yes – just set `FINWIPE_IMAP_HOST` (and optionally `FINWIPE_IMAP_PORT`) to the
desired server. The client uses `imapclient.DialTLS`, which works with any
standard IMAP server that speaks TLS.

**Q: What if I don’t want to expose port 8080?**  
Run the container with `-p 0:8080` and connect via `localhost` port mapping, or
use Docker networking modes (`--network host`, etc.) to keep the port internal.

**Q: Is the server safe to run on a cheap VPS?**  
Absolutely. It runs entirely in-process, uses only SQLite, and has no external
databases or secrets stores. Only the IMAP credentials are required, and they
should be stored in environment variables with restricted file permissions.

**Q: How do I upgrade?**  
```bash
docker pull ghcr.io/das-rebel/finwipe-server:latest
docker restart finwipe
```

---

## License

MIT © Das‑Rebel and contributors.

