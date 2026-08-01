package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
)

var cibilOptoutCmd = &cobra.Command{
	Use:   "cibl-optout",
	Short: "Guide to opt out of CIBIL data sharing and marketing use",
	Long: `Guide you through CIBIL's consent settings to:
1. Opt out of marketing use of your data
2. Stop third-party sharing
3. Freeze your credit report

This opens the CIBIL portal and provides step-by-step guidance.

Requires: browser access to https://consumer.cibil.com`,
	RunE: runCibilOptout,
}

func init() {
	rootCmd.AddCommand(cibilOptoutCmd)
}

func runCibilOptout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Print(`
🏦 CIBIL Opt-Out Guide

This command guides you through CIBIL's consent settings to:
  ① Opt out of marketing use of your credit data
 ② Stop third-party sharing
  ③ Freeze your credit report (prevent new loans)

─────────────────────────────────────────
STEP-BY-STEP GUIDE
─────────────────────────────────────────

STEP 1: Opt Out of Marketing & Third-Party Sharing
─────────────────────────────────────────
1. Go to: https://consumer.cibil.com
2. Login with your CIBIL Member ID and password

3. Navigate to: "Privacy" or "Consent Settings"
   (Usually found in profile/settings menu)

4. Look for these toggles:
   [ ] "Use my data for marketing purposes"        ← TURN OFF
   [ ] "Share with third-party partners"           ← TURN OFF
   [ ] "Analytics and profiling"                  ← TURN OFF

5. Save changes

─────────────────────────────────────────
STEP 2: Freeze Your Credit Report (Recommended)
─────────────────────────────────────────
A freeze prevents ANY new credit from being opened in your name.

1. Go to: https://consumer.cibil.com
2. Navigate to: "CIBIL Watch" → "Freeze Report"

3. Click: "Freeze My Report"

4. Save your freeze PIN securely

⚠️  IMPORTANT: Keep your freeze PIN safe!
   You'll need it to unfreeze when applying for credit.

─────────────────────────────────────────
STEP 3: Verify Opt-Out
─────────────────────────────────────────
After opting out, verify:
1. Log out and log back in
2. Check "Consent Settings" — toggles should be OFF
3. Try to check your CIBIL score — should still work

─────────────────────────────────────────
WHAT THIS DOES:
─────────────────────────────────────────
✅ Lenders can still see your credit history (for loan decisions)
✅ Your existing loans continue normally
✅ You can temporarily unfreeze to apply for credit
❌ CIBIL cannot share your data for marketing
❌ Third parties cannot access your profile for pre-approved offers
❌ No new loans can be opened while frozen

─────────────────────────────────────────
UNFREEZE WHEN YOU NEED CREDIT:
─────────────────────────────────────────
1. Go to: https://consumer.cibil.com → CIBIL Watch
2. Click: "Unfreeze Report"
3. Enter your freeze PIN
4. Apply for credit within your chosen window (usually 7-30 days)
5. Refreeze after credit is approved

─────────────────────────────────────────
TROUBLESHOOTING
─────────────────────────────────────────
Can't find consent settings?
  → Look in: Profile → Settings → Privacy
  → Or search: "consent" in the CIBIL search bar

Toggle won't save?
  → Ensure all required fields are filled
  → Try a different browser

Need help?
  → CIBIL: 1800-258-6363 (toll-free)
  → Email: grievance.officer@cibil.com
`)

	if cfg.Profile.Name != "" {
		fmt.Printf("\n👤 Your Name: %s\n", cfg.Profile.Name)
	}
	fmt.Println("🌐 Portal: https://consumer.cibil.com")

	// Check if there's a consent URL we can provide
	fmt.Print(`
─────────────────────────────────────────
DIRECT LINKS (CIBIL)
─────────────────────────────────────────
CIBIL Watch (Access History):  https://consumer.cibil.com/cibil-watch
Freeze Report:               https://consumer.cibil.com/cibil-watch/freeze
Consent Settings:             https://consumer.cibil.com/privacy
Raise Dispute:                https://consumer.cibil.com/dispute
`)
	return nil
}
