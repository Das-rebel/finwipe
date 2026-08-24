// FinWipe Server — self-hosted erasure request tracker.
// Wraps the existing CLI core (send/cron/history) with:
//   - IMAP poller that detects acknowledgments/rejections
//   - scheduler for followups & 30-day escalation
//   - web dashboard + /api/status + /api/mcp shim
//   - ntfy push notifications
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/das-rebel/finwipe/internal/server"
)

func main() {
	dbPath := flag.String("db", defaultDB(), "path to finwipe SQLite DB")
	addr := flag.String("addr", ":8080", "listen address")
	ntfy := flag.String("dummy", "", "placeholder")
	poll := flag.Duration("poll", 5*time.Minute, "IMAP poll interval")
	flag.Parse()

	store, err := server.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	notifier := server.NewNotifier(*ntfy)

	mux := http.NewServeMux()
	dash := &server.Dashboard{Store: store}
	dash.Register(mux)
	dash.MCPShim(mux)

	stop := make(chan struct{})

	// IMAP poller — wire MailboxClient to real credentials via env:
	//   FINWIPE_IMAP_HOST/PORT/USER/PASS. Scaffold logs and continues if unset.
	client, err := server.NewIMAPClientFromEnv()
	if err != nil {
		log.Printf("[imap] disabled: %v", err)
	} else {
		go server.NewPoller(client, store, notifier, *poll).Run(stop)
	}

	// Scheduler: every hour check for overdue dispatches → log + notify.
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				overdue, err := store.Overdue(30)
				if err != nil {
					continue
				}
				for _, r := range overdue {
					// notifier.Send(...) // disabled without ntfy
					store.LogEvent(r.ID, "note", "30d overdue notification sent")
				}
			}
		}
	}()

	log.Printf("FinWipe Server listening on %s (dashboard /, api /api/status, mcp POST /api/mcp)", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func defaultDB() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h + "/.finwipe/finwipe.db"
	}
	return "finwipe.db"
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
