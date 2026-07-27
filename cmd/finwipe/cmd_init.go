package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize FinWipe configuration",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	dir := config.DefaultDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	cfg := &config.Config{}

	qs := []*survey.Question{
		{
			Name: "name",
			Prompt: &survey.Input{
				Message: "Your full name:",
				Help:    "Used in all deletion letters and emails",
			},
			Validate: survey.Required,
		},
		{
			Name: "email",
			Prompt: &survey.Input{
				Message: "Your email address:",
				Help:    "Deletion requests will be sent from this email. Use a dedicated Gmail account.",
			},
			Validate: survey.Required,
		},
		{
			Name: "phone",
			Prompt: &survey.Input{
				Message: "Your phone number:",
				Help:    "10-digit mobile number",
			},
			Validate: survey.Required,
		},
		{
			Name: "address",
			Prompt: &survey.Input{
				Message: "Your address:",
				Help:    "Full mailing address for registered post letters",
			},
			Validate: survey.Required,
		},
		{
			Name: "smtp_username",
			Prompt: &survey.Input{
				Message: "SMTP Username:",
				Default: "",
				Help:    "Usually same as your email. For Gmail, use your full email address.",
			},
		},
		{
			Name: "smtp_password",
			Prompt: &survey.Password{
				Message: "SMTP Password / App Password:",
				Help:    "For Gmail: Generate an App Password at myaccount.google.com/apppasswords. DO NOT use your regular password.",
			},
		},
		{
			Name: "template",
			Prompt: &survey.Select{
				Message: "Email template:",
				Options: []string{"dpdpa", "dlg", "simple"},
				Default: "dpdpa",
				Help:    "dpdpa: Full DPDP Act citations | dlg: RBI Digital Lending Guidelines | simple: Plain language",
			},
		},
	}

	if err := survey.Ask(qs, cfg); err != nil {
		return fmt.Errorf("survey: %w", err)
	}

	cfg.Profile.Name = qs[0].Name
	cfg.SMTP.Host = "smtp.gmail.com"
	cfg.SMTP.Port = 465
	cfg.SMTP.UseTLS = true
	if cfg.SMTP.Username != "" {
		cfg.SMTP.From = cfg.SMTP.Username
	}
	cfg.Letter.Template = cfg.Letter.Template

	cfgPath := config.DefaultPath()
	if err := saveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\n✅ Configuration saved to %s\n", cfgPath)
	fmt.Printf("\n⚠️  IMPORTANT: If using Gmail, you must generate an App Password:\n")
	fmt.Printf("   1. Go to myaccount.google.com/apppasswords\n")
	fmt.Printf("   2. Select Mail → your device\n")
	fmt.Printf("   3. Copy the 16-character password\n")
	fmt.Printf("   4. Edit %s and replace smtp.password with the app password\n\n", cfgPath)

	return nil
}

func saveConfig(cfg *config.Config, path string) error {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0700)

	content := fmt.Sprintf(`profile:
  name: "%s"
  email: "%s"
  phone: "%s"
  address: "%s"

smtp:
  host: "%s"
  port: %d
  username: "%s"
  password: "%s"
  from: "%s"
  use_tls: true

letter:
  template: "%s"
`,
		cfg.Profile.Name,
		cfg.Profile.Email,
		cfg.Profile.Phone,
		cfg.Profile.Address,
		"smtp.gmail.com",
		465,
		cfg.SMTP.Username,
		cfg.SMTP.Password,
		cfg.SMTP.From,
		"dpdpa",
	)

	return os.WriteFile(path, []byte(content), 0400)
}
