package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/email"
	"github.com/das-rebel/finwipe/internal/history"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Daily automation: follow-ups, deadline checks, auto-escalation",
	Long: `Run daily via systemd timer or GitHub Actions.
Automatically:
- Schedules follow-ups at the right lifecycle stage
- Flags overdue requests
- Auto-escalates after 32 days of no response
`,
	RunE: runCron,
}

var (
	cronFollowupDays  string
	cronEscalateAfter int
	cronDryRun        bool
	cronMode          string
)

func init() {
	cronCmd.Flags().BoolVar(&cronDryRun, "dry-run", false, "Preview actions without sending")
	cronCmd.Flags().StringVar(&cronMode, "mode", "full",
		"Mode: full (all), followup (follow-ups only), escalate (escalations only)")
	cronCmd.Flags().StringVar(&cronFollowupDays, "followup-days", "3,10,15,25,28,32",
		"Comma-separated day-numbers after acknowledgment to send follow-ups")
	cronCmd.Flags().IntVar(&cronEscalateAfter, "escalate-after", 32,
		"Auto-escalate after N days with no acknowledgment")
}

func runCron(cmd *cobra.Command, args []string) error {
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	nbfcEntities, err := loadNBFCs()
	if err != nil {
		return fmt.Errorf("load NBFCs: %w", err)
	}
	nbfcMap := make(map[string]nbfc.Entity)
	for _, e := range nbfcEntities {
		nbfcMap[e.ID] = e
	}

	sender := email.New(&cfg.SMTP)
	now := time.Now()

	fmt.Printf("\n🤖 FinWipe Cron — %s\n", now.Format("02 Jan 2006 15:04 MST"))
	fmt.Printf("Mode: %s | Dry-run: %v\n", cronMode, cronDryRun)
	fmt.Println(strings.Repeat("─", 60))

	// ── 1. Handle overdue acknowledgments ──────────────────────────
	if cronMode == "full" || cronMode == "followup" {
		pendingAck, _ := hist.GetPendingAck()
		for _, req := range pendingAck {
			if req.AckDeadlineAt.IsZero() {
				continue
			}
			if now.After(req.AckDeadlineAt) {
				if !hist.FollowupExists(req.RequestID, "ACK_DEMAND") {
					fuID, err := hist.ScheduleFollowup(req.RequestID, "ACK_DEMAND", "email", now)
					if err != nil {
						fmt.Printf("  ❌ Schedule ACK_DEMAND for %s: %v\n", req.RequestID, err)
						continue
					}
					fmt.Printf("  📧 Scheduled: ACK_DEMAND for %s (deadline missed)\n", req.RequestID)

					if !cronDryRun {
						msgID, err := sendFollowup(hist, sender, fuID, "ACK_DEMAND", req, cfg.Profile)
						if err != nil {
							fmt.Printf("  ❌ Send ACK_DEMAND for %s: %v\n", req.RequestID, err)
						} else if msgID != "" {
							fmt.Printf("  ✅ Sent: ACK_DEMAND → %s (msg_id=%s)\n", req.GrievanceEmail, msgID)
						}
					}
				}
			}
		}
	}

	// ── 2. Follow-up scheduling for acknowledged requests ─────────────
	if cronMode == "full" || cronMode == "followup" {
		fuDays := parseCommaInts(cronFollowupDays)
		activeReqs, _ := hist.GetActive()

		for _, req := range activeReqs {
			if req.AckReceivedAt.IsZero() || req.LifecycleState == history.StateClosed {
				continue
			}

			daysSinceAck := int(time.Since(req.AckReceivedAt).Hours() / 24)

			for _, day := range fuDays {
				if daysSinceAck == day {
					fuType := followupTypeForDay(day)
					if !hist.FollowupExists(req.RequestID, fuType) {
						fuID, err := hist.ScheduleFollowup(req.RequestID, fuType, "email", now)
						if err != nil {
							fmt.Printf("  ❌ Schedule %s for %s: %v\n", fuType, req.RequestID, err)
							continue
						}
						fmt.Printf("  📧 Scheduled: %s for %s (day %d)\n", fuType, req.RequestID, day)

						if !cronDryRun {
							msgID, err := sendFollowup(hist, sender, fuID, fuType, req, cfg.Profile)
							if err != nil {
								fmt.Printf("  ❌ Send %s for %s: %v\n", fuType, req.RequestID, err)
							} else if msgID != "" {
								fmt.Printf("  ✅ Sent: %s → %s\n", fuType, req.GrievanceEmail)
							}
						}
					}
				}
			}

			// Deadline warnings: 3 days before and on deadline
			if !req.ResponseDeadlineAt.IsZero() {
				daysToDeadline := int(time.Until(req.ResponseDeadlineAt).Hours() / 24)
				if (daysToDeadline == 3 || daysToDeadline == 0 || daysToDeadline < 0) {
					fuType := "DEADLINE_WARNING"
					if !hist.FollowupExists(req.RequestID, fuType) {
						fuID, _ := hist.ScheduleFollowup(req.RequestID, fuType, "email", now)
						fmt.Printf("  📧 Scheduled: DEADLINE_WARNING for %s (%d days)\n",
							req.RequestID, daysToDeadline)

						if !cronDryRun && fuID > 0 {
							msgID, _ := sendFollowup(hist, sender, fuID, fuType, req, cfg.Profile)
							if msgID != "" {
								fmt.Printf("  ✅ Sent: DEADLINE_WARNING → %s\n", req.GrievanceEmail)
							}
						}
					}
				}
			}
		}
	}

	// ── 3. Auto-escalation ──────────────────────────────────────────
	if cronMode == "full" || cronMode == "escalate" {
		pendingAck, _ := hist.GetPendingAck()
		for _, req := range pendingAck {
			daysSinceDispatch := int(time.Since(req.DispatchedAt).Hours() / 24)
			if daysSinceDispatch >= cronEscalateAfter && req.EscalationLevel == history.EscNone {
				fmt.Printf("  🔺 AUTO-ESCALATE: %s — no acknowledgment in %d days\n",
					req.RequestID, daysSinceDispatch)

				if !cronDryRun {
					if req.LifecycleState != history.StateEscalated {
						err := hist.TransitionState(req.RequestID, history.StateDispatched,
							history.StateEscalated, "AUTOMATION",
							fmt.Sprintf("auto-escalated: no ack in %d days", daysSinceDispatch))
						if err != nil {
							fmt.Printf("  ❌ Auto-escalate %s: %v\n", req.RequestID, err)
							continue
						}
					}

					hist.SetEscalationLevel(req.RequestID, 0, history.EscRBIOmbu,
						"AUTOMATION",
						fmt.Sprintf("auto-escalated: no ack in %d days", daysSinceDispatch))

					esc, err := hist.CreateEscalation(req.RequestID, "RBI_SACHET",
						fmt.Sprintf("Auto-escalated: no acknowledgment received within %d days", daysSinceDispatch))
					if err != nil {
						fmt.Printf("  ❌ Create escalation record for %s: %v\n", req.RequestID, err)
					} else {
						fmt.Printf("  ✅ Escalation record created: %s (id=%d)\n", esc.Channel, esc.ID)
					}
				}
			}
		}
	}

	fmt.Println()
	sum, _ := hist.Summary()
	fmt.Printf("📊 Summary — Active: %d | Initiated: %d | Dispatched: %d | Acked: %d | Escalated: %d\n",
		sum["total"], sum["INITIATED"], sum["DISPATCHED"], sum["ACK_RECEIVED"], sum["escalated"])

	return nil
}

func sendFollowup(hist *history.DB, sender *email.Sender, fuID int64, fuType string,
	req history.Request, profile config.Profile) (string, error) {

	body := buildFollowupBody(req, fuType, profile)
	subject := buildFollowupSubject(req, fuType)

	if sender == nil || sender.Cfg.Host == "" {
		return "", nil
	}

	msgID, err := sender.SendFollowup(req.RequestID, req.GrievanceEmail, profile, body, subject)
	if err != nil {
		hist.MarkFollowupFailed(fuID)
		return "", err
	}

	hist.MarkFollowupSent(fuID, msgID)
	return msgID, nil
}

func buildFollowupSubject(req history.Request, fuType string) string {
	switch fuType {
	case "ACK_DEMAND":
		return fmt.Sprintf("REMINDER: DPDPA Acknowledgment Due Within 24 Hours — %s", req.RequestID)
	case "FRIENDLY_NUDGE":
		return fmt.Sprintf("Following Up: DPDPA Data Deletion Request — %s", req.RequestID)
	case "FORMAL_REMINDER":
		return fmt.Sprintf("FORMAL NOTICE: DPDPA Data Deletion — %s — Response Required", req.RequestID)
	case "DEADLINE_WARNING":
		return fmt.Sprintf("FINAL NOTICE: Data Deletion Deadline — %s — %s", req.RequestID, req.NBFCName)
	case "ESCALATION_NOTICE":
		return fmt.Sprintf("NOTICE OF ESCALATION: Filing RBI/DPDP Board Complaint — %s", req.RequestID)
	default:
		return fmt.Sprintf("DPDPA Data Deletion Request — %s — Follow-up", req.RequestID)
	}
}

func buildFollowupBody(req history.Request, fuType string, profile config.Profile) string {
	var tone, actionReq string
	switch fuType {
	case "ACK_DEMAND":
		tone = "URGENT — Your acknowledgment is overdue."
		actionReq = "Please acknowledge receipt of this deletion request immediately."
	case "FRIENDLY_NUDGE":
		tone = "We are following up on our data deletion request sent recently."
		actionReq = "Please confirm receipt and initiate the deletion process."
	case "FORMAL_REMINDER":
		tone = "This is a formal reminder that our data deletion request remains pending."
		actionReq = "Please process the deletion request within the regulatory timeframe."
	case "DEADLINE_WARNING":
		tone = "FINAL NOTICE — Our 30-day response deadline is imminent or exceeded."
		actionReq = "Please act immediately to avoid escalation to regulatory authorities."
	case "ESCALATION_NOTICE":
		tone = "NOTICE OF ESCALATION — We are filing a formal complaint with RBI/DPDP Board."
		actionReq = "This is our final communication before regulatory escalation."
	default:
		tone = "We are following up on our pending data deletion request."
		actionReq = "Please confirm receipt and process the deletion."
	}

	return fmt.Sprintf(`Dear Grievance Officer,

%s

Request Reference: %s
Data Principal: %s
Organization: %s

%s

This follow-up is sent automatically under FinWipe (github.com/das-rebel/finwipe)
in accordance with Section 8(6) of the DPDP Act 2023 and Rule 8 of the DPDP Rules 2025.

Regards,
%s
%s | %s | %s
`, tone, req.RequestID, profile.Name, req.NBFCName,
		actionReq,
		profile.Name, profile.Email, profile.Phone, profile.Address)
}

func loadNBFCs() ([]nbfc.Entity, error) {
	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".finwipe", "data", "nbfcs.yaml"),
		"./data/nbfcs.yaml",
		"../data/nbfcs.yaml",
	}
	for _, p := range paths {
		entities, err := nbfc.Load(p)
		if err == nil {
			return entities, nil
		}
	}
	return nil, fmt.Errorf("could not find nbfcs.yaml")
}

func parseCommaInts(s string) []int {
	var result []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		fmt.Sscanf(part, "%d", &n)
		result = append(result, n)
	}
	return result
}

func followupTypeForDay(day int) string {
	switch {
	case day <= 3:
		return "FRIENDLY_NUDGE"
	case day <= 15:
		return "FORMAL_REMINDER"
	default:
		return "DEADLINE_WARNING"
	}
}
