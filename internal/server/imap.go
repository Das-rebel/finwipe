package server

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// IMAPClient implements MailboxClient against a real IMAP server.
// Configure via env:
//
//	FINWIPE_IMAP_HOST=imap.gmail.com   (default)
//	FINWIPE_IMAP_PORT=993              (default)
//	FINWIPE_IMAP_USER=you@gmail.com
//	FINWIPE_IMAP_PASS=<app password>   (NOT your login password)
//
// Gmail: create an app password at https://myaccount.google.com/apppasswords
// (2FA required). Yahoo/Outlook: similar app-password flows.

type IMAPClient struct {
	host string
	port string
	user string
	pass string
}

func NewIMAPClientFromEnv() (*IMAPClient, error) {
	user := os.Getenv("FINWIPE_IMAP_USER")
	pass := os.Getenv("FINWIPE_IMAP_PASS")
	if user == "" || pass == "" {
		return nil, fmt.Errorf("FINWIPE_IMAP_USER/PASS not set — poller disabled")
	}
	host := envOr("FINWIPE_IMAP_HOST", "imap.gmail.com")
	port := envOr("FINWIPE_IMAP_PORT", "993")
	return &IMAPClient{host: host, port: port, user: user, pass: pass}, nil
}

// FetchUnseen connects, searches for unseen messages from the last 30 days,
// fetches envelope + first 1KB of text (peeked, so flags are untouched here),
// then marks them \Seen. Returns parsed inbound messages.
func (c *IMAPClient) FetchUnseen() ([]InboundMessage, error) {
	addr := fmt.Sprintf("%s:%s", c.host, c.port)
	conn, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	if err := conn.Login(c.user, c.pass).Wait(); err != nil {
		return nil, fmt.Errorf("login %s: %w", c.user, err)
	}

	selectCmd := conn.Select("INBOX", &imap.SelectOptions{ReadOnly: false})
	data, err := selectCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("select INBOX: %w", err)
	}
	if data.NumMessages == 0 {
		return nil, nil
	}

	criteria := &imap.SearchCriteria{
		Since:   time.Now().AddDate(0, 0, -30),
		NotFlag: []imap.Flag{imap.FlagSeen},
	}
	searchRes, err := conn.Search(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search unseen: %w", err)
	}
	seqSet, ok := searchRes.All.(imap.SeqSet)
	if !ok || len(seqSet) == 0 {
		return nil, nil
	}

	fetchOpts := &imap.FetchOptions{
		Envelope: true,
		BodySection: []*imap.FetchItemBodySection{{
			Peek:    true,
			Partial: &imap.SectionPartial{Offset: 0, Size: 1024},
		}},
	}
	bufs, err := conn.Fetch(seqSet, fetchOpts).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}

	var out []InboundMessage
	var seqNums []uint32
	for _, buf := range bufs {
		msg := InboundMessage{}
		if env := buf.Envelope; env != nil {
			msg.Subject = env.Subject
			for _, a := range env.From {
				if addr := a.Addr(); addr != "" {
					msg.From = strings.ToLower(addr)
					break
				}
			}
		}
		for _, sec := range buf.BodySection {
			b := strings.TrimSpace(string(sec.Bytes))
			// strip headers if the section included them
			if i := strings.Index(b, "\r\n\r\n"); i >= 0 && i < len(b)-4 {
				b = b[i+4:]
			}
			if len(b) > 500 {
				b = b[:500]
			}
			msg.Excerpt = b
			break
		}
		out = append(out, msg)
		seqNums = append(seqNums, buf.SeqNum)
	}

	// Mark processed messages as seen (read-write session).
	for _, seq := range seqNums {
		storeCmd := conn.Store(imap.SeqSetNum(seq), &imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{imap.FlagSeen},
		}, nil)
		if err := storeCmd.Close(); err != nil {
			return out, fmt.Errorf("mark seen seq %d: %w", seq, err)
		}
	}
	return out, nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
