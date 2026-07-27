package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/letter"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var whatsappDiscoverCmd = &cobra.Command{
	Use:   "discover-from-whatsapp",
	Short: "Discover FIs from WhatsApp Business chat exports",
	Long: `Extract financial institution contacts from WhatsApp Business chat exports.

WhatsApp Business apps store chat databases locally.
By parsing the WhatsApp database or export, you can find
which companies have messaged you.

What you can find:
  - Bank messages (EMI statements, alerts)
  - Fintech messages (loan offers, payments)
  - Insurance renewals
  - Credit card updates

How to get the data:
  1. iPhone: WhatsApp → Chat → Export Chat
  2. Android: Local database (needs root)
  3. GB WhatsApp: Easier database access

Usage:
  finwipe discover-from-whatsapp --path ./whatsapp_chat.txt
  finwipe discover-from-whatsapp --path ./chat_export --auto`,
	RunE:  runWhatsappDiscover,
}

var (
	whatsappPath  string
	whatsappAuto  bool
)

func init() {
	rootCmd.AddCommand(whatsappDiscoverCmd)
	whatsappDiscoverCmd.Flags().StringVar(&whatsappPath, "path", "",
		"Path to WhatsApp chat export (.txt, .json, or directory)")

}

func runWhatsappDiscover(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w (run finwipe init)", err)
	}
	if cfg.Profile.Email == "" {
		return fmt.Errorf("run finwipe init first")
	}

	if whatsappPath == "" {
		// Try default locations
		defaults := []string{
			filepath.Join(os.Getenv("HOME"), "whatsapp_chat.txt"),
			filepath.Join(os.Getenv("HOME"), "Downloads", "whatsapp_chat.txt"),
			filepath.Join(os.Getenv("HOME"), "WhatsApp", "Chat", "chat_export.txt"),
		}
		for _, p := range defaults {
			if _, err := os.Stat(p); err == nil {
				whatsappPath = p
				break
			}
		}
		if whatsappPath == "" {
			fmt.Println("  📱 WhatsApp Business Discovery")
			fmt.Println("  ─────────────────────────────────────────────────────────────────")
			fmt.Println()
			fmt.Println("  ⚠️  No chat export found.")
			fmt.Println()
			fmt.Println("  📋 HOW TO EXPORT YOUR WHATSAPP CHATS:")
			fmt.Println()
			fmt.Println("  iPhone:")
			fmt.Println("     1. Open WhatsApp → tap the business chat")
			fmt.Println("     2. Go to Chat Info → Export Chat")
			fmt.Println("     3. Save as .txt file")
			fmt.Println()
			fmt.Println("  Android (GB WhatsApp):")
			fmt.Println("     1. GB WhatsApp → Chat → Export")
			fmt.Println("     2. Select business chats to export")
			fmt.Println()
			fmt.Println("  Then run:")
			fmt.Printf("     finwipe discover-from-whatsapp --path /path/to/chat.txt\n")
			fmt.Println()
			fmt.Println("  💡 WHAT FINWIPE LOOKS FOR:")
			fmt.Println("     • Bank names in message sender")
			fmt.Println("     • Keywords: EMI, loan, insurance, credit, statement")
			fmt.Println("     • Company names in message content")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("  ╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  WhatsApp Business Discovery                              ║")
	fmt.Println("  ╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📂 Parsing: %s\n", whatsappPath)
	fmt.Println()

	// Read the chat file
	data, err := os.ReadFile(whatsappPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	content := strings.ToUpper(string(data))

	// Load entities
	entities, err := nbfc.Load(nbfcRegistryPath())
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	// Known FI keywords to search for in WhatsApp chats
	fiKeywords := []string{
		// Banks
		"HDFC BANK", "ICICI BANK", "AXIS BANK", "KOTAK MAHINDRA",
		"STATE BANK", "INDUSIND BANK", "YES BANK", "IDBI BANK",
		"BANK OF BARODA", "PNB", "CANARA BANK", "FEDERAL BANK",
		"RBL BANK", "BANDHAN BANK", "UNION BANK",

		// NBFCs
		"BAJAJ FINSERV", "BAJAJ FINANCE", "TATA CAPITAL",
		"ADITYA BIRLA FINANCE", "L&T FINANCE", "MUTHOOT",
		"CHOLAMANDALAM", "HDB FINANCIAL", "KISHT",
		"STASHFIN", "RUPEEK", "KREDITBEE", "NAVI FINSERV",
		"OFBUSINESS", "EARLYSALARY", "SLICE", "UNI", "MONEYVIEW",

		// Fintech
		"PHONEPE", "PAYTM", "RAZORPAY", "CRED",
		"PAISABAZAAR", "BANKBAZAAR", "INDMONEY", "GROWW",
		"ZERODHA", "UPSTOX", "ANGEL ONE", "POLICYBAZAAR",

		// Insurance
		"LIC", "HDFC LIFE", "SBI LIFE", "ICICI PRUDENTIAL",
		"BAJAJ ALLIANZ", "TATA AIA", "MAX LIFE", "STAR HEALTH",
		"NIVA BUPA", "DIGIT INSURANCE",

		// Credit cards
		"HDFC CREDIT CARD", "ICICI CREDIT CARD", "AMEX",
		"STANDARD CHARTERED", "CITI BANK", "HSBC",
	}

	type waMatch struct {
		Name     string
		Entity   *nbfc.Entity
		Messages int
	}

	seen := make(map[string]*waMatch)

	for _, keyword := range fiKeywords {
		if strings.Contains(content, keyword) {
			// Try to find matching entity
			lower := strings.ToLower(keyword)
			var entity *nbfc.Entity
			for i := range entities {
				e := &entities[i]
				if strings.Contains(strings.ToLower(e.Name), lower) ||
					strings.Contains(lower, strings.ToLower(e.Name)) {
					entity = e
					break
				}
			}

			if entity != nil {
				if _, exists := seen[entity.ID]; !exists {
					seen[entity.ID] = &waMatch{
						Name:   entity.Name,
						Entity: entity,
					}
				}
				seen[entity.ID].Messages++
			} else {
				// Unknown FI
				name := strings.Title(strings.ToLower(keyword))
				if _, exists := seen[name]; !exists {
					seen[name] = &waMatch{Name: name}
				}
				seen[name].Messages++
			}
		}
	}

	var matched []waMatch
	var unknown []waMatch
	for _, m := range seen {
		if m.Entity != nil {
			matched = append(matched, *m)
		} else {
			unknown = append(unknown, *m)
		}
	}

	if len(matched) == 0 && len(unknown) == 0 {
		fmt.Println("  📭 No financial institutions found in WhatsApp chats.")
		fmt.Println()
		fmt.Println("  💡 Tips:")
		fmt.Println("     • Make sure you exported business chats")
		fmt.Println("     • Check if keywords are spelled differently")
		return nil
	}

	fmt.Printf("  ✅ Found %d financial institutions\n\n", len(matched)+len(unknown))

	// Show matched
	if len(matched) > 0 {
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  ✅ MATCHED IN REGISTRY")
		fmt.Println("  ─────────────────────────────────────────────────────")
		for i, m := range matched {
			if i >= 20 {
				fmt.Printf("  ... and %d more\n", len(matched)-i)
				break
			}
			icon := "💳"
			if m.Entity.Category == nbfc.CatBANK {
				icon = "🏛️"
			} else if m.Entity.Category == nbfc.CatHFC {
				icon = "🏠"
			}
			grievance := m.Entity.GrievanceEmail
			if grievance == "" {
				grievance = "—"
			}
			fmt.Printf("  %2d. %s %-28s [%d msgs]\n", i+1, icon,
				truncate(m.Entity.Name, 28), m.Messages)
		}
		fmt.Println()
	}

	// Show unknown
	if len(unknown) > 0 {
		fmt.Println("  ═══════════════════════════════════════════════════════════════")
		fmt.Println("  ❓ NOT IN REGISTRY")
		fmt.Println("  ─────────────────────────────────────────────────────")
		for i, m := range unknown {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(unknown)-i)
				break
			}
			fmt.Printf("  %2d. %-28s [%d msgs]\n", i+1, truncate(m.Name, 28), m.Messages)
		}
		fmt.Println()
	}

	fmt.Println("  ─────────────────────────────────────────────────────────────────")

	if !dryRun && len(matched) > 0 {
		fmt.Println()
		fmt.Println("  🚀 Creating deletion requests...")

		hist, err := history.New(dbPath())
		if err != nil {
			return err
		}
		defer hist.Close()

		letterDir := filepath.Join(os.Getenv("HOME"), ".finwipe", "letters")
		gen := letter.New(letterDir)

		created := 0
		for _, m := range matched {
			if m.Entity.GrievanceEmail == "" {
				continue
			}

			existing, _ := hist.GetByNBFCID(m.Entity.ID)
			isDup := false
			for _, e := range existing {
				if e.LifecycleState != history.StateClosed &&
					e.LifecycleState != history.StateDeliveryFailed {
					isDup = true
					break
				}
			}
			if isDup {
				continue
			}

			req, err := hist.CreateRequest(
				m.Entity.ID, m.Entity.Name,
				history.ChannelEmail,
				m.Entity.GrievanceEmail,
				cfg.Profile.Email, cfg.Profile.Name)
			if err != nil {
				continue
			}

			gen.Generate(req.RequestID, m.Entity.Name, m.Entity.GrievanceEmail,
				cfg.Profile, letter.DefaultDeletionCategories, letter.LegalBasisBoth)

			fmt.Printf("  ✅ %-28s %s\n", m.Entity.Name, req.RequestID)
			created++
		}

		fmt.Printf("\n  ✅ Created: %d deletion requests\n", created)
	} else if dryRun {
		fmt.Println()
		fmt.Println("  🔍 DRY RUN — No requests created")
		fmt.Println("  Run with --dry-run=false to create requests")
	}

	return nil
}
