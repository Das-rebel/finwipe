package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

type Sender struct {
	cfg  *config.SMTP
	from string
	Cfg  *config.SMTP
}

func New(cfg *config.SMTP) *Sender {
	return &Sender{cfg: cfg, from: cfg.From, Cfg: cfg}
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

	var conn *tls.Conn
	var err error

	if s.cfg.UseTLS {
		tlsCfg := &tls.Config{ServerName: s.cfg.Host}
		conn, err = tls.Dial("tcp", addr, tlsCfg)
	} else {
		conn, err = tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	}
	if err != nil {
		err = smtp.SendMail(addr, auth, s.from, []string{n.GrievanceEmail}, []byte(msg))
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
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

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	msg := fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"Message-ID: <%s.%s@finwipe>\r\n"+
			"\r\n"+
			"%s\r\n",
		s.from, grievanceEmail, subject,
		time.Now().Format(time.RFC1123Z),
		reqID, time.Now().Format("20060102T150405"),
		body,
	)

	tlsCfg := &tls.Config{ServerName: s.cfg.Host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		err = smtp.SendMail(addr, auth, s.from, []string{grievanceEmail}, []byte(msg))
		return "", err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return "", err
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return "", fmt.Errorf("SMTP auth: %w", err)
		}
	}
	if err = client.Mail(s.from); err != nil {
		return "", err
	}
	if err = client.Rcpt(grievanceEmail); err != nil {
		return "", err
	}
	w, err := client.Data()
	if err != nil {
		return "", err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return "", err
	}
	w.Close()

	msgID := fmt.Sprintf("<%s.%s@finwipe>", reqID, time.Now().Format("20060102T150405"))
	return msgID, client.Quit()
}

func buildMessage(n nbfc.Entity, p config.Profile, body, from string) string {
	return fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: DPDPA Section 8(6) Data Deletion Request — %s\r\n"+
			"Date: %s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"%s\r\n",
		from,
		n.GrievanceEmail,
		p.Name,
		time.Now().Format(time.RFC1123Z),
		buildBody(n, p, body),
	)
}

func buildBody(n nbfc.Entity, p config.Profile, template string) string {
	if template == "" {
		template = DefaultTemplate
	}

	s := template
	s = strings.ReplaceAll(s, "{{.NBFCName}}", n.Name)
	s = strings.ReplaceAll(s, "{{.FullName}}", p.Name)
	s = strings.ReplaceAll(s, "{{.Email}}", p.Email)
	s = strings.ReplaceAll(s, "{{.Phone}}", p.Phone)
	s = strings.ReplaceAll(s, "{{.Address}}", p.Address)
	s = strings.ReplaceAll(s, "{{.Date}}", time.Now().Format("02 January 2006"))
	return s
}

var DefaultTemplate = `Dear Grievance Officer,

I, {{.FullName}}, residing at {{.Address}}, am exercising my right to erasure under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act) and Rule 8 of the DPDP Rules, 2025.

The purpose for which my personal data was collected by {{.NBFCName}} is no longer being served. I hereby request deletion of the following categories of personal data held by {{.NBFCName}}:

□ Marketing and promotional data
□ Third-party shared data
□ Behavioral/usage data
□ Pre-approved loan profiles
□ Call recordings and customer service interaction logs

I request acknowledgment of this request within 48 hours as mandated by Rule 8(3) of the DPDP Rules, 2025, and completion of deletion within 30 days.

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

	return fmt.Sprintf(`Subject: DPDPA Data Deletion Request — FOLLOW-UP #%d — Ref: %s

Dear Grievance Officer,

%s

I submitted a formal data deletion request to %s on %s (Ref: %s) under Section 8(6) of the Digital Personal Data Protection Act, 2023 and Rule 8 of the DPDP Rules, 2025.

To date, I have received no acknowledgment of this request. This is a violation of Rule 8(3) which mandates acknowledgment within 48 hours.

My original request specifically sought deletion of:
  □ Marketing and promotional data
  □ Third-party shared data
  □ Behavioral/usage data collected through your app/website
  □ Pre-approved loan offer profiles
  □ Call recordings and customer service interaction logs

Please:
  1. Acknowledge this request immediately
  2. Confirm the categories of data that will be deleted
  3. Complete deletion within the remaining time available under the 30-day window

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
`, dayNum/7, reqID, preamble, nbfcName, time.Now().AddDate(0, 0, -dayNum).Format("02 January 2006"), reqID, reqID, reqID, time.Now().AddDate(0, 0, -dayNum).Format("02 January 2006"), profile.Email, profile.Phone, profile.Name, profile.Address)
}
