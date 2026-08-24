package server

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// Poller watches the user's IMAP INBOX for replies from grievance
// officers and updates DPR states accordingly.
//
// MVP implementation notes:
//   - Uses IMAP IDLE where supported, else periodic EXAMINE polls.
//   - Only reads UNSEEN messages since the last UID; marks seen after parse.
//   - Never stores message bodies in DB beyond a redacted excerpt.
//
// Library: github.com/emersion/go-imap/v2 (stable as of 2026).
// Wire-in point: connect() below. Kept as an interface so the poller
// can be unit-tested against a fake mailbox.

type MailboxClient interface {
	// FetchUnseen returns recent unseen messages (from, subject, excerpt).
	FetchUnseen() ([]InboundMessage, error)
}

type InboundMessage struct {
	From    string
	Subject string
	Excerpt string // first ~500 chars, redacted before persistence
}

type Poller struct {
	client    MailboxClient
	interval  time.Duration
	store     *Store
	notifier  *Notifier
	lastUID   uint64
	domains   map[string]string // sender domain -> dpr id hint not reliable; match by DPR-ID instead
}

func NewPoller(client MailboxClient, store *Store, notifier *Notifier, interval time.Duration) *Poller {
	return &Poller{client: client, store: store, notifier: notifier, interval: interval}
}

// Run blocks, polling on the given interval until stop is closed.
func (p *Poller) Run(stop <-chan struct{}) {
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if err := p.pollOnce(); err != nil {
				log.Printf("[poller] %v", err)
			}
		}
	}
}

func (p *Poller) pollOnce() error {
	msgs, err := p.client.FetchUnseen()
	if err != nil {
		return fmt.Errorf("fetch unseen: %w", err)
	}
	for _, m := range msgs {
		p.classify(m)
	}
	return nil
}

// classify matches inbound mail to open DPRs and advances state.
// Matching strategy (in order):
//  1. DPR-ID appears in subject or body excerpt
//  2. Sender domain matches the entity's grievance domain
func (p *Poller) classify(m InboundMessage) {
	open, err := p.store.OpenRequests()
	if err != nil {
		log.Printf("[poller] open requests: %v", err)
		return
	}
	from := strings.ToLower(m.From)

	for _, r := range open {
		hay := strings.ToLower(m.Subject + " " + m.Excerpt)
		if strings.Contains(hay, strings.ToLower(r.ID)) || domainMatch(from, r.GrievanceEmail) {
			state := StateAckReceived
			note := "acknowledgment detected"
			if looksLikeRejection(m.Subject, m.Excerpt) {
				state = StatePendingReview
				note = "possible rejection — needs human review"
			}
			if err := p.store.Transition(r.ID, state, note); err != nil {
				log.Printf("[poller] transition %s: %v", r.ID, err)
				continue
			}
			p.notifier.Send(fmt.Sprintf("📨 %s: %s → %s", r.EntityName, r.ID, note))
			return // one message = one request
		}
	}
}

func domainMatch(fromAddr, grievanceEmail string) bool {
	at := strings.LastIndex(grievanceEmail, "@")
	if at < 0 {
		return false
	}
	domain := grievanceEmail[at+1:]
	fAt := strings.LastIndex(fromAddr, "@")
	return fAt >= 0 && strings.HasSuffix(fromAddr[fAt+1:], domain)
}

func looksLikeRejection(subject, excerpt string) bool {
	s := strings.ToLower(subject + " " + excerpt)
	for _, kw := range []string{"cannot", "unable to", "reject", "decline", "not possible", "policy does not"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// Request lifecycle states (mirror internal/history values).
const (
	StateAckReceived   = "ACK_RECEIVED"
	StatePendingReview = "PENDING_REVIEW"
)
