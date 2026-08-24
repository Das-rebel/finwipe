package server

import (
	"fmt"
	"os"
)

// IMAPClient implements MailboxClient against a real IMAP server.
// Wire-up uses github.com/emersion/go-imap/v2 (add via: go get github.com/emersion/go-imap/v2).
//
// This scaffold compiles without the dependency by keeping the real
// fetch behind build-time wiring; NewIMAPClientFromEnv returns an error
// when env credentials are absent so the server runs dashboard-only.

type IMAPClient struct {
	host string
	port string
	user string
	pass string
}

func NewIMAPClientFromEnv() (*IMAPClient, error) {
	host := os.Getenv("FINWIPE_IMAP_HOST")
	user := os.Getenv("FINWIPE_IMAP_USER")
	pass := os.Getenv("FINWIPE_IMAP_PASS")
	if host == "" || user == "" || pass == "" {
		return nil, fmt.Errorf("FINWIPE_IMAP_HOST/USER/PASS not set — poller disabled")
	}
	port := os.Getenv("FINWIPE_IMAP_PORT")
	if port == "" {
		port = "993"
	}
	return &IMAPClient{host: host, port: port, user: user, pass: pass}, nil
}

// FetchUnseen will connect, EXAMINE INBOX, and return unseen messages.
// TODO(v0.2): implement with go-imap v2:
//
//	c := client.NewTLS(dialAddr, nil)
//	if err := c.Login(user, pass); err != nil { ... }
//	_, err := c.Select("INBOX", &client.SelectOptions{ReadOnly: true})
//	search := imap.SearchCriteria{Unseen: true}
//	data, _ := c.Search(search, nil)
//	... fetch envelopes + body snippets, mark \Seen ...
func (c *IMAPClient) FetchUnseen() ([]InboundMessage, error) {
	return nil, fmt.Errorf("imap fetch: not yet wired (v0.2) — set FINWIPE_IMAP_* and add go-imap/v2")
}
