package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

// sanitizeHeader strips \r, \n, and null bytes from strings used in SMTP headers.
// This prevents CRLF injection attacks where an attacker could inject
// arbitrary headers (e.g., Bcc:) by including \r\n in user-controlled fields.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

// sanitizeBody strips trailing whitespace and normalized multiple spaces in email body.
// CRLF in body is allowed but we clean it up for readability.
func sanitizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

type Sender struct {
	cfg  *config.SMTP
	from string
	Cfg  *config.SMTP
}

func New(cfg *config.SMTP) *Sender {
	return &Sender{cfg: cfg, from: cfg.From, Cfg: cfg}
}

// dialSMTP connects to the SMTP server.
// For port 465, uses implicit SSL/TLS (tls.Dial).
// For port 587, uses STARTTLS upgrade on plain TCP connection.
func (s *Sender) dialSMTP(addr string) (*smtp.Client, error) {
	tlsCfg := &tls.Config{ServerName: s.cfg.Host}

	if s.cfg.Port == 465 {
		// Port 465: implicit SSL/TLS — wrap connection in TLS from the start
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("SSL connect failed to %s: %w (hint: try port 587 for STARTTLS)", addr, err)
		}
		client, err := smtp.NewClient(conn, s.cfg.Host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("create SMTP client: %w", err)
		}
		return client, nil
	}

	// Port 587 (or other): plain TCP, then upgrade via STARTTLS
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create SMTP client: %w", err)
	}

	// Send EHLO
	if err := client.Hello("localhost"); err != nil {
		client.Close()
		return nil, fmt.Errorf("EHLO: %w", err)
	}

	// Upgrade to TLS if supported
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsCfg); err != nil {
			client.Close()
			return nil, fmt.Errorf("STARTTLS failed: %w (hint: try port 465 for SSL)", err)
		}
	} else {
		client.Close()
		return nil, fmt.Errorf("STARTTLS not supported by %s on port %d", s.cfg.Host, s.cfg.Port)
	}

	return client, nil
}

func (s *Sender) Send(n nbfc.Entity, profile config.Profile, templateBody string) error {
	if s.cfg.Host == "" {
		return fmt.Errorf("SMTP not configured")
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	msg := buildMessage(n, profile, templateBody, s.from)

	client, err := s.dialSMTP(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err = client.Mail(s.from); err != nil {
		return fmt.Errorf("SMTP from: %w", err)
	}
	if err = client.Rcpt(n.GrievanceEmail); err != nil {
		return fmt.Errorf("SMTP to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP data: %w", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("SMTP write: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("SMTP close: %w", err)
	}

	return client.Quit()
}

func (s *Sender) SendBatch(nbfcs []nbfc.Entity, profile config.Profile, templateBody string, rateLimitMs int) (sent int, failed []string) {
	for i, n := range nbfcs {
		if n.GrievanceEmail == "" {
			failed = append(failed, n.ID+": no grievance email")
			continue
		}

		err := s.Send(n, profile, templateBody)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", n.ID, err))
		} else {
			sent++
		}

		if i < len(nbfcs)-1 && rateLimitMs > 0 {
			time.Sleep(time.Duration(rateLimitMs) * time.Millisecond)
		}
	}
	return sent, failed
}

func (s *Sender) SendFollowup(reqID, grievanceEmail string, p config.Profile, body, subject string) (string, error) {
	if s.cfg.Host == "" {
		return "", fmt.Errorf("SMTP not configured")
	}

	// CRITICAL: Sanitize all user-controlled fields to prevent CRLF header injection
	cleanSubject := sanitizeHeader(subject)
	cleanBody := sanitizeBody(body)

	// Sanitize entity name if used in subject (buildFollowupSubject uses req.NBFCName)
	// and in body (nbfcName is passed separately)

	// Verify sanitization worked
	if cleanSubject != subject || cleanBody != body {
		// Log sanitization for debugging (does not log content)
		fmt.Printf("[FinWipe] Sanitized %d bytes from subject and %d bytes from body\n",
			len(subject)-len(cleanSubject), len(body)-len(cleanBody))
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	// Build email with sanitized headers and body
	msg := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"Message-ID: <%s.%s@finwipe>\r\n"+
			"Auto-Submitted: auto-generated\r\n"+
			"\r\n"+
			"%s\r\n",
		sanitizeHeader(s.from),
		sanitizeHeader(grievanceEmail),
		cleanSubject,
		time.Now().Format(time.RFC1123Z),
		sanitizeHeader(reqID),
		time.Now().Format("20060102T150405"),
		cleanBody,
	)

	client, err := s.dialSMTP(addr)
	if err != nil {
		return "", err
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return "", fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err = client.Mail(sanitizeHeader(s.from)); err != nil {
		return "", fmt.Errorf("SMTP from: %w", err)
	}
	if err = client.Rcpt(sanitizeHeader(grievanceEmail)); err != nil {
		return "", fmt.Errorf("SMTP to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("SMTP data: %w", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return "", fmt.Errorf("SMTP write: %w", err)
	}
	w.Close()

	msgID := fmt.Sprintf("<%s.%s@finwipe>", sanitizeHeader(reqID), time.Now().Format("20060102T150405"))
	return msgID, client.Quit()
}

// buildMessage creates an email message with sanitized headers.
func buildMessage(n nbfc.Entity, p config.Profile, body, from string) string {
	// Sanitize user fields that appear in headers
	safeName := sanitizeHeader(p.Name)
	safeFrom := sanitizeHeader(from)

	msg := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: DPDPA Section 8(6) Data Deletion Request — %s\r\n"+
			"Date: %s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"Auto-Submitted: auto-generated\r\n"+
			"\r\n"+
			"%s\r\n",
		safeFrom,
		sanitizeHeader(n.GrievanceEmail),
		safeName,
		time.Now().Format(time.RFC1123Z),
		buildBody(n, p, body),
	)
	return msg
}

// buildBody replaces template placeholders with sanitized profile data.
func buildBody(n nbfc.Entity, p config.Profile, template string) string {
	if template == "" {
		template = DefaultTemplate
	}

	s := template
	s = strings.ReplaceAll(s, "{{.NBFCName}}", sanitizeHeader(n.Name))
	s = strings.ReplaceAll(s, "{{.FullName}}", sanitizeHeader(p.Name))
	s = strings.ReplaceAll(s, "{{.Email}}", sanitizeHeader(p.Email))
	s = strings.ReplaceAll(s, "{{.Phone}}", sanitizeHeader(p.Phone))
	s = strings.ReplaceAll(s, "{{.Address}}", sanitizeHeader(p.Address))
	s = strings.ReplaceAll(s, "{{.Date}}", time.Now().Format("02 January 2006"))
	return s
}

var DefaultTemplate = `Dear Grievance Officer,

I, {{.FullName}}, residing at {{.Address}}, am exercising my right to erasure under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act).

The purpose for which my personal data was collected by {{.NBFCName}} is no longer being served. I hereby request deletion of the following categories of personal data held by {{.NBFCName}}:

□ Marketing and promotional data
□ Third-party shared data
□ Behavioral/usage data
□ Pre-approved loan profiles
□ Call recordings and customer service interaction logs

I request acknowledgment of this request as soon as reasonably practicable, and completion of erasure where applicable, in accordance with Section 8(6) of the DPDP Act, 2023.

This request does not extend to personal data whose retention is required by or under any law for the time being in force, including but not limited to the RBI Act, Companies Act, and Income Tax Act, including KYC documents, transaction records, and active loan account data.

For any communication regarding this request:
Email: {{.Email}}
Phone: {{.Phone}}

Regards,
{{.FullName}}
{{.Address}}
Date: {{.Date}}
`

// GenerateFollowupBody creates a follow-up email body for requests without acknowledgment.
// dayNum is 7 for first reminder, 14 for second, 21 for final notice.
func GenerateFollowupBody(reqID, nbfcName string, profile config.Profile, dayNum int) string {
	urgency := map[int]string{
		7:  "This is a follow-up reminder regarding my data deletion request.",
		14: "This is a second reminder. Please note this matter will be escalated if unresolved.",
		21: "FINAL NOTICE — This request remains unacknowledged and unresolved.",
	}
	preamble := urgency[dayNum]
	if preamble == "" {
		preamble = fmt.Sprintf("This is a follow-up (#%d) regarding my data deletion request.", dayNum/7)
	}

	// Sanitize all user fields
	safeNBFCName := sanitizeHeader(nbfcName)
	safeEmail := sanitizeHeader(profile.Email)
	safePhone := sanitizeHeader(profile.Phone)
	safeName := sanitizeHeader(profile.Name)
	safeAddress := sanitizeHeader(profile.Address)

	body := fmt.Sprintf(`Subject: DPDPA Data Deletion Request — FOLLOW-UP #%d — Ref: %s

Dear Grievance Officer,

%s

I submitted a formal data deletion request to %s on %s (Ref: %s) under Section 8(6) of the Digital Personal Data Protection Act, 2023.

To date, I have received no acknowledgment of this request. This is a violation of Section 8(6) which requires acknowledgment as soon as reasonable.

My original request specifically sought deletion of:
  □ Marketing and promotional data
  □ Third-party shared data
  □ Behavioral/usage data collected through your app/website
  □ Pre-approved loan offer profiles
  □ Call recordings and customer service interaction logs

Please:
  1. Acknowledge this request immediately
  2. Confirm the categories of data that will be deleted
  3. Complete deletion as soon as reasonably practicable

This request does not extend to data whose retention is required by law (KYC, transaction records, active loan accounts).

If no response is received within 7 days, I will escalate this matter to:
  — Reserve Bank of India (Sachet Portal)
  — Data Protection Board of India (DPB)
  — Consumer Forum (for deficiency in service)

Reference: %s
Request ID: %s
Date of original request: %s

For any communication:
Email: %s
Phone: %s

Regards,
%s
%s
`, dayNum/7, sanitizeHeader(reqID), preamble, safeNBFCName,
		time.Now().AddDate(0, 0, -dayNum).Format("02 January 2006"),
		sanitizeHeader(reqID), sanitizeHeader(reqID), sanitizeHeader(reqID),
		time.Now().AddDate(0, 0, -dayNum).Format("02 January 2006"),
		safeEmail, safePhone, safeName, safeAddress)

	return body
}
