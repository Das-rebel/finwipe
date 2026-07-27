package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/das-rebel/finwipe/internal/history"
)

var trackCmd = &cobra.Command{
	Use:     "track",
	Aliases: []string{"status"},
	Short:   "Track deletion request lifecycle and audit trail",
	RunE:  runTrack,
}

var (
	trackRequestID string
	trackNBFCID   string
	trackAll      bool
	trackDeadlines int
	trackEscalated bool
	trackJSON     bool
)

func init() {
	trackCmd.Flags().StringVar(&trackRequestID, "request-id", "", "Track specific request by DPR-ID")
	trackCmd.Flags().StringVar(&trackNBFCID, "nbfc", "", "Track all requests for an NBFC")
	trackCmd.Flags().BoolVar(&trackAll, "all", false, "Show all active requests")
	trackCmd.Flags().IntVar(&trackDeadlines, "deadline", 0, "Show requests with deadline within N days")
	trackCmd.Flags().BoolVar(&trackEscalated, "escalated", false, "Show only escalated requests")
	trackCmd.Flags().BoolVar(&trackJSON, "json", false, "Output as JSON")
}

func runTrack(cmd *cobra.Command, args []string) error {
	dbPath := dbPath()
	hist, err := history.New(dbPath)
	if err != nil {
		return fmt.Errorf("open history: %w", err)
	}
	defer hist.Close()

	switch {
	case trackRequestID != "":
		return showRequest(hist, trackRequestID, trackJSON)
	case trackNBFCID != "":
		return showByNBFC(hist, trackNBFCID, trackJSON)
	case trackAll:
		return showAll(hist, trackJSON)
	case trackDeadlines > 0:
		return showDeadlines(hist, trackDeadlines, trackJSON)
	case trackEscalated:
		return showEscalated(hist, trackJSON)
	default:
		return showAll(hist, trackJSON)
	}
}

func showRequest(hist *history.DB, reqID string, json bool) error {
	req, err := hist.GetByRequestID(reqID)
	if err != nil {
		return fmt.Errorf("request not found: %s", reqID)
	}

	audit, _ := hist.GetAuditTrail(reqID)
	escs, _ := hist.GetEscalationsByRequest(reqID)

	if json {
		fmt.Printf(`{"request_id":"%s","nbfc":"%s","channel":"%s","state":"%s","escalation_level":%d`,
			req.RequestID, req.NBFCName, req.Channel, req.LifecycleState, req.EscalationLevel)
		fmt.Printf(`,"deadline_status":"%s","days_to_deadline":%d`, req.DeadlineStatus, req.DaysToDeadline)
		if !req.AckDeadlineAt.IsZero() {
			fmt.Printf(`,"ack_deadline":"%s"`, req.AckDeadlineAt.Format(time.RFC3339))
		}
		if !req.ResponseDeadlineAt.IsZero() {
			fmt.Printf(`,"response_deadline":"%s"`, req.ResponseDeadlineAt.Format(time.RFC3339))
		}
		if !req.AckReceivedAt.IsZero() {
			fmt.Printf(`,"ack_received":"%s"`, req.AckReceivedAt.Format(time.RFC3339))
		}
		if req.ExternalRef != "" {
			fmt.Printf(`,"external_ref":"%s"`, req.ExternalRef)
		}
		if req.Outcome != "" {
			fmt.Printf(`,"outcome":"%s"`, req.Outcome)
		}
		fmt.Printf(`,"audit_count":%d,"escalation_count":%d}`, len(audit), len(escs))
		return nil
	}

	fmt.Printf("\n")
	fmt.Printf("  %s  %s / %s\n", req.RequestID, req.NBFCName, history.ChannelLabel(req.Channel))
	fmt.Println(strings.Repeat("─", 65))

	statusColor := map[string]string{
		"ok":          "✅",
		"awaiting_ack": "⏳",
		"ack_overdue": "⚠️ ",
		"warning":     "🔶",
		"critical":    "🔴",
		"expired":     "🚨",
		"closed":      "✔️ ",
	}[req.DeadlineStatus]

	fmt.Printf("  State:    %s  |  Deadline: %s  |  Escalation: L%d\n",
		history.StateLabel(req.LifecycleState),
		statusColor+" "+strings.ToUpper(req.DeadlineStatus),
		req.EscalationLevel)

	if req.ExternalRef != "" {
		fmt.Printf("  Ref:      %s\n", req.ExternalRef)
	}
	if !req.DispatchedAt.IsZero() {
		fmt.Printf("  Sent:     %s", req.DispatchedAt.Format("02 Jan 2006"))
		if !req.AckDeadlineAt.IsZero() {
			fmt.Printf(" (ack deadline: %s)", req.AckDeadlineAt.Format("02 Jan 15:04 MST"))
		}
		fmt.Println()
	}
	if !req.AckReceivedAt.IsZero() {
		fmt.Printf("  Acked:    %s", req.AckReceivedAt.Format("02 Jan 2006"))
		if !req.ResponseDeadlineAt.IsZero() {
			fmt.Printf(" (response deadline: %s)", req.ResponseDeadlineAt.Format("02 Jan 2006"))
		}
		fmt.Println()
	}
	if req.Outcome != "" {
		fmt.Printf("  Outcome:  %s\n", strings.ToUpper(req.Outcome))
	}
	fmt.Println()

	if len(audit) > 0 {
		fmt.Println("  Timeline:")
		for _, e := range audit {
			actor := ""
			if e.Actor == "AUTOMATION" {
				actor = " 🤖"
			}
			ref := ""
			if e.RefNumber != "" {
				ref = fmt.Sprintf(" [ref:%s]", e.RefNumber)
			}
			fmt.Printf("    [%s] %-15s %s%s %s\n",
				e.CreatedAt.Format("02 Jan"),
				e.Action,
				e.Detail,
				ref,
				actor)
		}
		fmt.Println()
	}

	if len(escs) > 0 {
		fmt.Println("  Escalations:")
		for _, e := range escs {
			status := e.Status
			if e.ComplaintRef != "" {
				status += " | Ref: " + e.ComplaintRef
			}
			fmt.Printf("    • %-20s %s\n",
				history.EscalationChannelLabel(e.Channel),
				status)
		}
		fmt.Println()
	}

	return nil
}

func showByNBFC(hist *history.DB, nbfcID string, json bool) error {
	sanitized := history.SanitizeNBFCID(nbfcID)
	reqs, err := hist.GetByNBFCID(sanitized)
	if err != nil {
		return err
	}
	if len(reqs) == 0 {
		fmt.Printf("No requests found for NBFC: %s\n", nbfcID)
		return nil
	}
	return printRequests(hist, reqs, json)
}

func showAll(hist *history.DB, json bool) error {
	reqs, err := hist.GetActive()
	if err != nil {
		return err
	}
	return printRequests(hist, reqs, json)
}

func showDeadlines(hist *history.DB, withinDays int, json bool) error {
	reqs, err := hist.GetOverdue()
	if err != nil {
		return err
	}

	deadlineTime := time.Now().Add(time.Duration(withinDays) * 24 * time.Hour)
	var approaching []history.Request

	if withinDays > 0 {
		db := hist.GetDb()
		rows, err := db.Query(`
			SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
				escalation_level, created_at, dispatched_at, ack_deadline_at,
				ack_received_at, response_deadline_at, closed_at,
				external_ref, grievance_email, user_email, user_name,
				outcome, outcome_notes, letter_path, active
			FROM requests
			WHERE active = 1 AND lifecycle_state IN ('DISPATCHED','ACK_RECEIVED')
				AND response_deadline_at BETWEEN ? AND ?
			ORDER BY response_deadline_at ASC
		`, time.Now(), deadlineTime)
		if err != nil {
			return err
		}
		approaching, _ = scanRequests(hist, rows)
		rows.Close()
	}

	fmt.Printf("\n📋 FinWipe Requests — Deadline Report\n")
	fmt.Printf("─────────────────────────────────────────\n")

	if len(reqs) > 0 {
		fmt.Printf("\n🚨 OVERDONE (%d requests)\n", len(reqs))
		for _, r := range reqs {
			r.ComputeDeadline()
			fmt.Printf("  %s  %-30s  %s\n", r.RequestID, r.NBFCName, history.StateLabel(r.LifecycleState))
		}
	}

	if len(approaching) > 0 {
		fmt.Printf("\n⏰ Within %d days (%d requests)\n", withinDays, len(approaching))
		for _, r := range approaching {
			r.ComputeDeadline()
			fmt.Printf("  %s  %-30s  %s  %ddays\n",
				r.RequestID, r.NBFCName, r.DeadlineStatus, r.DaysToDeadline)
		}
	}

	fmt.Println()
	return nil
}

func showEscalated(hist *history.DB, json bool) error {
	reqs, err := hist.GetEscalated()
	if err != nil {
		return err
	}
	return printRequests(hist, reqs, json)
}

func printRequests(hist *history.DB, reqs []history.Request, json bool) error {
	if len(reqs) == 0 {
		fmt.Println("No requests found.")
		return nil
	}

	if json {
		fmt.Println("[")
		for i, r := range reqs {
			sep := ","
			if i == len(reqs)-1 {
				sep = ""
			}
			r.ComputeDeadline()
			fmt.Printf(`  {"request_id":"%s","nbfc":"%s","channel":"%s","state":"%s","escalation_level":%d,"deadline":"%s","days":%d}%s`,
				r.RequestID, r.NBFCName, r.Channel, r.LifecycleState,
				r.EscalationLevel, r.DeadlineStatus, r.DaysToDeadline, sep)
			fmt.Println()
		}
		fmt.Println("]")
		return nil
	}

	fmt.Printf("\n📋 FinWipe Active Requests (%d)\n\n", len(reqs))

	byState := make(map[history.State][]history.Request)
	for _, r := range reqs {
		r.ComputeDeadline()
		byState[r.LifecycleState] = append(byState[r.LifecycleState], r)
	}

	stateOrder := []history.State{
		history.StateInitiated, history.StateDispatched,
		history.StateAckReceived, history.StateEscalated,
	}

	for _, state := range stateOrder {
		if items, ok := byState[state]; ok && len(items) > 0 {
			fmt.Printf("%s (%d)\n", history.StateLabel(state), len(items))
			fmt.Println(strings.Repeat("─", 60))
			for _, r := range items {
				deadline := r.DeadlineStatus
				if r.DaysToDeadline != 0 {
					deadline = fmt.Sprintf("%s (%dd)", r.DeadlineStatus, r.DaysToDeadline)
				}
				esc := ""
				if r.EscalationLevel > 0 {
					esc = fmt.Sprintf(" L%d", r.EscalationLevel)
				}
				ref := ""
				if r.ExternalRef != "" {
					ref = " | " + r.ExternalRef
				}
				fmt.Printf("  %s  %-28s %-15s%s%s\n",
					r.RequestID,
					truncate(r.NBFCName, 28),
					deadline,
					esc,
					ref)
			}
			fmt.Println()
		}
	}

	sum, _ := hist.Summary()
	fmt.Printf("Total: %d | Awaiting ack: %d | Escalated: %d\n\n",
		sum["total"],
		sum["DISPATCHED"],
		sum["escalated"])

	return nil
}


// scanRequests scans requests from a *sql.Rows into []history.Request
func scanRequests(hist *history.DB, rows *sql.Rows) ([]history.Request, error) {
	var result []history.Request
	for rows.Next() {
		var r history.Request
		var ackReceived, respDeadline, dispatched, closed sql.NullTime
		var outcome, externalRef, letterPath, userName sql.NullString

		err := rows.Scan(
			&r.ID, &r.RequestID, &r.NBFCID, &r.NBFCName, &r.Channel,
			&r.LifecycleState, &r.EscalationLevel, &r.CreatedAt,
			&dispatched, &r.AckDeadlineAt, &ackReceived, &respDeadline, &closed,
			&externalRef, &r.GrievanceEmail, &r.UserEmail, &userName,
			&outcome, &r.OutcomeNotes, &letterPath, &r.Active,
		)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if ackReceived.Valid {
			r.AckReceivedAt = ackReceived.Time
		}
		if respDeadline.Valid {
			r.ResponseDeadlineAt = respDeadline.Time
		}
		if dispatched.Valid {
			r.DispatchedAt = dispatched.Time
		}
		if closed.Valid {
			r.ClosedAt = closed.Time
		}
		if externalRef.Valid {
			r.ExternalRef = externalRef.String
		}
		if userName.Valid {
			r.UserName = userName.String
		}
		if outcome.Valid {
			r.Outcome = outcome.String
		}
		if letterPath.Valid {
			r.LetterPath = letterPath.String
		}
		r.ComputeDeadline()
		result = append(result, r)
	}
	return result, rows.Err()
}
