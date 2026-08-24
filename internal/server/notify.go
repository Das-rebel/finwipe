package server

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// Notifier pushes events to an ntfy topic (self-host or ntfy.sh).
// Zero-config: set FINWIPE_NTFY_URL (default https://ntfy.sh/finwipe-<random>).

type Notifier struct {
	url    string
	client *http.Client
}

func NewNotifier(url string) *Notifier {
	if url == "" {
		url = "" // disabled unless configured
	}
	return &Notifier{url: url, client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *Notifier) Send(msg string) {
	if n == nil || n.url == "" {
		return
	}
	go func() {
		req, err := http.NewRequest("POST", n.url, bytes.NewReader([]byte(msg)))
		if err != nil {
			return
		}
		req.Header.Set("Title", "FinWipe")
		resp, err := n.client.Do(req)
		if err != nil {
			fmt.Printf("[ntfy] %v\n", err)
			return
		}
		resp.Body.Close()
	}()
}
