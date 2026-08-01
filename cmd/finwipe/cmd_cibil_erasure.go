package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
)

var cibilErasureCmd = &cobra.Command{
	Use:   "cibl-erasure",
	Short: "Send DPDPA Section 8(6) erasure request to CIBIL",
	Long: `Send a formal DPDPA 2023 Section 8(6) data erasure request to CIBIL.

CIBIL should respond as soon as reasonable (Section 8(6), DPDP Act 2023).

This sends an email to CIBIL's grievance officer and creates a PDF letter.

Requires: finwipe init (for SMTP config)`,
	RunE: runCibilErasure,
}

var (
	cibilErasureEmail string
	cibilErasureID   string
)

func init() {
	rootCmd.AddCommand(cibilErasureCmd)
	cibilErasureCmd.Flags().StringVar(&cibilErasureEmail, "email", "",
		"Your email address")
	cibilErasureCmd.Flags().StringVar(&cibilErasureID, "id", "",
		"Your CIBIL Member ID")
}

func runCibilErasure(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Profile.Name == "" {
		return fmt.Errorf("profile not configured. Run: finwipe init")
	}
	if cfg.SMTP.Host == "" && !dryRun {
		return fmt.Errorf("SMTP not configured. Run: finwipe init")
	}
	if dryRun {
		fmt.Println("⚠️  DRY RUN — no email will be sent")
	}
	email := cibilErasureEmail
	if email == "" {
		email = cfg.Profile.Email
	}
	cibilID := cibilErasureID
	if cibilID == "" {
		fmt.Print("Enter your CIBIL Member ID: ")
		fmt.Scanln(&cibilID)
	}
	if cibilID == "" {
		return fmt.Errorf("CIBIL Member ID required")
	}

	to := "grievance.officer@cibil.com"
	subject := fmt.Sprintf("DPDPA Section 8(6) — Erasure Request — Member ID: %s", cibilID)
	body := buildCibilErasureBody(cibilID, cfg.Profile.Name, email, cfg.Profile.Phone, cfg.Profile.Address)

	fmt.Printf("\n📤 Sending DPDPA erasure request to CIBIL...\n")
	fmt.Printf("   To: %s\n", to)
	fmt.Printf("   Subject: %s\n\n", subject)

	if dryRun {
		fmt.Println("📧 Would send email to:", to)
		fmt.Println("📋 Subject:", subject)
		fmt.Println("📄 Body preview:", body[:200], "...")
	} else {
		err = sendCibilEmail(cfg.SMTP, to, subject, body, cfg.Profile.Name, email)
		if err != nil {
			fmt.Printf("❌ Failed to send: %v\n", err)
			fmt.Println("   Please email grievance.officer@cibil.com manually with the subject above")
		} else {
			fmt.Println("✅ Email sent successfully!")
		}
	}

	// Save letter
	letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
	os.MkdirAll(letterDir, 0700)
	letterPath := filepath.Join(letterDir,
		fmt.Sprintf("CIBIL_Erasure_%s_%s.txt", cibilID, time.Now().Format("20060102")))
	os.WriteFile(letterPath, []byte(body), 0600)
	fmt.Printf("\n📄 Letter saved: %s\n", letterPath)

	fmt.Print(`
📋 Next Steps:
  1. CIBIL should acknowledge "as soon as reasonable" (Section 8(6), DPDP Act 2023)
  2. CIBIL should complete erasure "as soon as reasonable" — no statutory period
  3. Track: https://consumer.cibil.com
  4. No response? Escalate to MeitY: dpdo@meity.gov.in
`)
	return nil
}

func buildCibilErasureBody(id, name, email, phone, address string) string {
	date := time.Now().Format("02 January 2006")
	return fmt.Sprintf(`To,
The Grievance Officer
CIBIL Technologies Pvt Ltd
Tower 3, Empire Complex
414 Senapati Bapat Marg
Lower Parel, Mumbai 400013

Email: grievance.officer@cibil.com

Date: %s

Subject: Request for Erasure of Personal Data under Section 8(6) of the Digital Personal Data Protection Act, 2023

Dear Grievance Officer,

I, %s, exercising my rights under Section 8(6) of the Digital Personal Data Protection Act, 2023 (DPDP Act).

My Details:
- Full Name: %s
- Email: %s
- Phone: %s
- Address: %s
- CIBIL Member ID: %s

Requests:

1. ERASE all personal data no longer necessary for the purpose for which it was collected.

2. CEASE sharing my personal data with member institutions beyond what is strictly necessary for credit information services.

3. CEASE use of my personal data for marketing, analytics, or third-party sharing.

4. PROVIDE written confirmation of all data erased and sharing terminated.

Grounds:

Under Section 8(6) of the DPDP Act, 2023, I have the right to demand erasure of personal data that is no longer necessary. Continued retention and sharing without free, informed, specific consent violates Section 8(6).

Please confirm receipt of this request as soon as reasonably practicable, and complete erasure of personal data that is no longer necessary for its original purpose.

If no satisfactory response, I reserve the right to escalate to the Data Protection Board of India (DPBI), the enforcement authority under the DPDP Act, 2023.

For inaccuracies in credit information, I also invoke my rights under Section 17 of the Credit Information Companies (Regulation) Act, 2005, and the RBI Master Direction on Credit Information.

I reserve all rights under the DPDP Act, 2023, including compensation under Section 32.

Sincerely,
%s
%s
%s
Date: %s

---
DPDPA Section 8(6) Erasure Request
CIBIL Member ID: %s
`, date, name, name, email, phone, address, id, name, address, email, date, id)
}

func sendCibilEmail(smtpCfg config.SMTP, to, subject, body, fromName, fromEmail string) error {
	from := smtpCfg.From
	if from == "" {
		from = smtpCfg.Username
	}

	msg := fmt.Sprintf(
		"From: %s <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
			"\r\n"+
			"%s\r\n",
		fromName, from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", smtpCfg.Host, smtpCfg.Port)

	var auth smtp.Auth
	if smtpCfg.Username != "" {
		auth = smtp.PlainAuth("", smtpCfg.Username, smtpCfg.Password, smtpCfg.Host)
	}

	if smtpCfg.Port == 465 {
		return sendSSL(smtpCfg.Host, smtpCfg.Port, smtpCfg.Username, smtpCfg.Password, from, to, []byte(msg))
	}
	return sendSTARTTLS(addr, auth, from, to, []byte(msg))
}

func sendSSL(host string, port int, username, password, from, to string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	tlsCfg := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("SSL connect: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}
	defer client.Close()

	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	w.Close()
	return client.Quit()
}

func sendSTARTTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	client, err := smtp.NewClient(conn, "")
	if err != nil {
		conn.Close()
		return fmt.Errorf("client: %w", err)
	}
	defer client.Close()

	tlsCfg := &tls.Config{ServerName: strings.Split(addr, ":")[0]}
	if err := client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("STARTTLS: %w", err)
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	w.Close()
	return client.Quit()
}
