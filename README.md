# FinWipe Server

Self‑hosted tracker for data‑erasure (DPR) requests.  
Monitors an IMAP inbox for lender acknowledgements, updates SQLite state,
and exposes a dark‑theme dashboard + JSON API.

[![Docker Image](https://img.shields.io/badge/docker%20image-12MB-success)](https://hub.docker.com/r/das-rebel/finwipe-server) 
[![GitHub Tag](https://img.shields.io/github/v/tag/Das-rebel/finwipe?include_prereleases&label=latest)](https://github.com/Das-rebel/finwipe) 
[![License](https://img.shields.io/github/license/Das-rebel/finwipe)](https://github.com/Das-rebel/finwipe/blob/main/LICENSE)

---  

- **Dashboard** – `http://localhost:8080`  
- **API** – `/api/status`, `/api/mcp`  
- **Docker** – multi‑arch (`amd64`/`arm64`) image on GHCR  

For full docs see `docs/server.md`.

---  

## Quick Start (Docker)

```bash
docker run -d \
  --name finwipe \
  -p 8080:8080 \
  -e FINWIPE_IMAP_HOST=imap.gmail.com \
  -e FINWIPE_IMAP_USER=you@gmail.com \
  -e FINWIPE_IMAP_PASS=YOUR_APP_PASSWORD \
  -v finwipe-data:/data \
  ghcr.io/das-rebel/finwipe-server:latest
```

Open `http://localhost:8080` to view the dashboard.  

For CI/CD details see `.github/workflows/docker.yml`.
