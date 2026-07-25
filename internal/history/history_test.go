package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) (*DB, func()) {
	tmp := filepath.Join(os.TempDir(), "finwipe_test_"+time.Now().Format("20060102T150405")+".db")
	db, err := New(tmp)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cleanup := func() {
		db.Close()
		os.Remove(tmp)
	}
	return db, cleanup
}

// ============================================================
// SanitizeNBFCID
// ============================================================

func TestSanitizeNBFCID(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		// SanitizeNBFCID: lowercase, trim, spaces→hyphens, keep hyphens
		{"bajaj-finserv", "bajaj-finserv"},
		{"Bajaj Finserv Ltd", "bajaj-finserv-ltd"},
		{"HDFC Bank", "hdfc-bank"},
		{"SBICard", "sbicard"},
		{"KreditBee (Finnovation)", "kreditbee-(finnovation)"},
		{"CRED ", "cred"},
		{"  spaces  ", "spaces"},
	}
	for _, c := range cases {
		got := SanitizeNBFCID(c.input)
		if got != c.want {
			t.Errorf("SanitizeNBFCID(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ============================================================
// Lifecycle state machine
// ============================================================

func TestStateTransitions(t *testing.T) {
	cases := []struct {
		from, to State
		want    bool
	}{
		// Valid
		{StateInitiated, StateDispatched, true},
		{StateInitiated, StateClosed, true},
		{StateDispatched, StateAckReceived, true},
		{StateDispatched, StateDeliveryFailed, true},
		{StateDispatched, StateEscalated, true},
		{StateDispatched, StateClosed, true},
		{StateDeliveryFailed, StateDispatched, true},
		{StateDeliveryFailed, StateClosed, true},
		{StateAckReceived, StateResponseOK, true},
		{StateAckReceived, StatePendingReview, true},
		{StateAckReceived, StateEscalated, true},
		{StateAckReceived, StateClosed, true},
		{StatePendingReview, StateResponseOK, true},
		{StatePendingReview, StateEscalated, true},
		{StatePendingReview, StateClosed, true},
		{StateResponseOK, StateClosed, true},
		{StateEscalated, StateClosed, true},
		{StateEscalated, StateAckReceived, true},
		{StateEscalated, StateResponseOK, true},
		// Invalid
		{StateClosed, StateInitiated, false},
		{StateClosed, StateDispatched, false},
		{StateClosed, StateAckReceived, false},
		{StateResponseOK, StateAckReceived, false},
		{StateResponseOK, StateDispatched, false},
		{StateEscalated, StateInitiated, false},
		{StateInitiated, StateAckReceived, false},
		{StateInitiated, StateEscalated, false},
	}
	for _, c := range cases {
		got := c.from.IsValid(c.to)
		if got != c.want {
			t.Errorf("%s → %s: got %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// ============================================================
// CreateRequest
// ============================================================

func TestCreateRequest(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, err := db.CreateRequest(
		"bajaj-finserv", "Bajaj Finserv Ltd",
		ChannelEmail, "gro@bajajfinserv.in",
		"user@example.com", "Test User",
	)
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	if req.RequestID == "" {
		t.Error("RequestID should not be empty")
	}
	if req.NBFCID != "bajaj-finserv" {
		t.Errorf("NBFCID = %q, want %q", req.NBFCID, "bajaj-finserv")
	}
	if req.LifecycleState != StateInitiated {
		t.Errorf("LifecycleState = %q, want %q", req.LifecycleState, StateInitiated)
	}
	if req.Channel != ChannelEmail {
		t.Errorf("Channel = %q, want %q", req.Channel, ChannelEmail)
	}
	if req.AckDeadlineAt.IsZero() {
		t.Error("AckDeadlineAt should not be zero")
	}
}

func TestCreateRequest_Uniqueness(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	ids := make(map[string]bool)
	for i := 0; i < 20; i++ {
		req, err := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
		if err != nil {
			t.Fatalf("CreateRequest: %v", err)
		}
		if ids[req.RequestID] {
			t.Errorf("RequestID %q generated twice", req.RequestID)
		}
		ids[req.RequestID] = true
	}
}

// ============================================================
// GetByRequestID
// ============================================================

func TestGetByRequestID(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	created, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	req, err := db.GetByRequestID(created.RequestID)
	if err != nil {
		t.Fatalf("GetByRequestID: %v", err)
	}
	if req.NBFCName != "NBFC One" {
		t.Errorf("NBFCName = %q", req.NBFCName)
	}
}

func TestGetByRequestID_NotFound(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()
	_, err := db.GetByRequestID("DPR-2099-999999")
	if err == nil {
		t.Error("expected error for non-existent request")
	}
}

// ============================================================
// Dispatch
// ============================================================

func TestDispatch(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	err := db.Dispatch(req.RequestID, "", "msg-123", ChannelEmail)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	updated, _ := db.GetByRequestID(req.RequestID)
	if updated.LifecycleState != StateDispatched {
		t.Errorf("LifecycleState = %q, want %q", updated.LifecycleState, StateDispatched)
	}
	if updated.DispatchedAt.IsZero() {
		t.Error("DispatchedAt should not be zero")
	}
}

func TestDispatch_OnlyFromInitiated(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	db.Dispatch(req.RequestID, "", "", ChannelEmail) // move to DISPATCHED

	// Try to dispatch again — should fail
	err := db.Dispatch(req.RequestID, "", "", ChannelEmail)
	if err == nil {
		t.Error("expected error dispatching from DISPATCHED state")
	}
}

// ============================================================
// RecordAck
// ============================================================

func TestRecordAck(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	db.Dispatch(req.RequestID, "", "", ChannelEmail)

	ackTime := time.Now()
	err := db.RecordAck(req.RequestID, "TICKET-123", "CLI_USER", ackTime)
	if err != nil {
		t.Fatalf("RecordAck: %v", err)
	}

	updated, _ := db.GetByRequestID(req.RequestID)
	if updated.LifecycleState != StateAckReceived {
		t.Errorf("LifecycleState = %q, want %q", updated.LifecycleState, StateAckReceived)
	}
	if updated.ExternalRef != "TICKET-123" {
		t.Errorf("ExternalRef = %q", updated.ExternalRef)
	}
	if updated.ResponseDeadlineAt.IsZero() {
		t.Error("ResponseDeadlineAt should not be zero")
	}
}

// ============================================================
// CloseRequest
// ============================================================

func TestCloseRequest(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	db.Dispatch(req.RequestID, "", "", ChannelEmail)

	err := db.CloseRequest(req.RequestID, StateDispatched, OutcomeDeleted, "", "CLI_USER")
	if err != nil {
		t.Fatalf("CloseRequest: %v", err)
	}

	closed, _ := db.GetByRequestID(req.RequestID)
	if closed.LifecycleState != StateClosed {
		t.Errorf("LifecycleState = %q, want %q", closed.LifecycleState, StateClosed)
	}
	if closed.Outcome != OutcomeDeleted {
		t.Errorf("Outcome = %q", closed.Outcome)
	}
}

// ============================================================
// TransitionState idempotency
// ============================================================

func TestTransitionState_Idempotency(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")

	err := db.TransitionState(req.RequestID, StateInitiated, StateDispatched, "CLI_USER", "first")
	if err != nil {
		t.Fatalf("TransitionState 1: %v", err)
	}

	// Same transition again — should be idempotent
	err = db.TransitionState(req.RequestID, StateDispatched, StateDispatched, "CLI_USER", "second")
	if err != nil {
		t.Errorf("Idempotent transition returned error: %v", err)
	}

	// Backward — should fail
	err = db.TransitionState(req.RequestID, StateDispatched, StateInitiated, "CLI_USER", "invalid")
	if err == nil {
		t.Error("expected error for invalid backward transition")
	}
}

// ============================================================
// Escalation
// ============================================================

func TestCreateEscalation(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	esc, err := db.CreateEscalation(req.RequestID, "DPO", "Escalated to DPO")
	if err != nil {
		t.Fatalf("CreateEscalation: %v", err)
	}
	if esc.Channel != "DPO" {
		t.Errorf("Channel = %q", esc.Channel)
	}
}

func TestEscalationLevel_UpgradeAndDowngrade(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")

	err := db.SetEscalationLevel(req.RequestID, EscNone, EscDPDPBoard, "CLI_USER", "escalated")
	if err != nil {
		t.Fatalf("SetEscalationLevel L0→L2: %v", err)
	}

	// Cannot downgrade from L2 to L1
	err = db.SetEscalationLevel(req.RequestID, EscDPDPBoard, EscDPO, "CLI_USER", "downgrade")
	if err == nil {
		t.Error("expected error for downgrading escalation level")
	}

	// Can escalate further from L2 to L3
	err = db.SetEscalationLevel(req.RequestID, EscDPDPBoard, EscRBIOmbu, "CLI_USER", "escalate")
	if err != nil {
		t.Errorf("escalating L2→L3: %v", err)
	}
}

// ============================================================
// Audit trail
// ============================================================

func TestAuditLog(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")
	db.Dispatch(req.RequestID, "", "", ChannelEmail)

	trail, err := db.GetAuditTrail(req.RequestID)
	if err != nil {
		t.Fatalf("GetAuditTrail: %v", err)
	}
	if len(trail) < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", len(trail))
	}
}

// ============================================================
// Followup tracking
// ============================================================

func TestFollowupExists(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	req, _ := db.CreateRequest("nbfc1", "NBFC One", ChannelEmail, "a@b.com", "u@x.com", "User")

	exists := db.FollowupExists(req.RequestID, "FRIENDLY_NUDGE")
	if exists {
		t.Error("FollowupExists should be false before scheduling")
	}

	fuID, err := db.ScheduleFollowup(req.RequestID, "FRIENDLY_NUDGE", "email", time.Now())
	if err != nil {
		t.Fatalf("ScheduleFollowup: %v", err)
	}
	if fuID <= 0 {
		t.Errorf("ScheduleFollowup returned invalid ID: %d", fuID)
	}

	exists = db.FollowupExists(req.RequestID, "FRIENDLY_NUDGE")
	if !exists {
		t.Error("FollowupExists should be true after scheduling")
	}
}

// ============================================================
// Summary
// ============================================================

func TestSummary(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	sum, err := db.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum["total"] != 0 {
		t.Errorf("initial total should be 0, got %d", sum["total"])
	}

	db.CreateRequest("n1", "N1", ChannelEmail, "a@b.com", "u@x.com", "User")
	db.CreateRequest("n2", "N2", ChannelEmail, "a@b.com", "u@x.com", "User")
	db.CreateRequest("n3", "N3", ChannelEmail, "a@b.com", "u@x.com", "User")

	sum, _ = db.Summary()
	if sum["total"] != 3 {
		t.Errorf("total = %d, want 3", sum["total"])
	}
}

// ============================================================
// GetByState
// ============================================================

func TestGetByState(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	_, _ = db.CreateRequest("n1", "N1", ChannelEmail, "a@b.com", "u@x.com", "User")
	r2, _ := db.CreateRequest("n2", "N2", ChannelEmail, "a@b.com", "u@x.com", "User")
	r3, _ := db.CreateRequest("n3", "N3", ChannelEmail, "a@b.com", "u@x.com", "User")

	db.Dispatch(r2.RequestID, "", "", ChannelEmail)
	err := db.CloseRequest(r3.RequestID, StateInitiated, OutcomeDeleted, "", "CLI_USER")
	if err != nil {
		t.Fatalf("CloseRequest r3: %v", err)
	}

	r1State, _ := db.GetByRequestID("DPR-2026-000001")
	r2State, _ := db.GetByRequestID(r2.RequestID)
	r3State, _ := db.GetByRequestID(r3.RequestID)
	if r1State != nil {
		t.Logf("r1 state: %s", r1State.LifecycleState)
	} else {
		t.Logf("r1 not found")
	}
	t.Logf("r2 state: %s", r2State.LifecycleState)
	t.Logf("r3 state: %s", r3State.LifecycleState)

	initiated, _ := db.GetByState(StateInitiated)
	if len(initiated) != 1 {
		t.Errorf("INITIATED count = %d, want 1 (only n1 untouched)", len(initiated))
	}

	dispatched, _ := db.GetByState(StateDispatched)
	if len(dispatched) != 1 {
		t.Errorf("DISPATCHED count = %d, want 1", len(dispatched))
	}

	closed, _ := db.GetByState(StateClosed)
	if len(closed) != 1 {
		t.Errorf("CLOSED count = %d, want 1", len(closed))
	}
}

// ============================================================
// Label helpers
// ============================================================

func TestStateLabel(t *testing.T) {
	for _, s := range []State{StateInitiated, StateDispatched, StateDeliveryFailed,
		StateAckReceived, StatePendingReview, StateResponseOK, StateEscalated, StateClosed} {
		if StateLabel(s) == "" {
			t.Errorf("StateLabel(%q) returned empty", s)
		}
	}
}

func TestEscalationChannelLabel(t *testing.T) {
	for _, c := range []string{"DPO", "DPDP_BOARD", "RBI_OMBUDSMAN", "CONSUMER_FORUM", "LEGAL"} {
		if EscalationChannelLabel(c) == "" {
			t.Errorf("EscalationChannelLabel(%q) returned empty", c)
		}
	}
}

func TestEscalationLevelLabel(t *testing.T) {
	for lvl := 0; lvl <= 5; lvl++ {
		if EscalationLevelLabel(lvl) == "" {
			t.Errorf("EscalationLevelLabel(%d) returned empty", lvl)
		}
	}
}

func TestChannelLabel(t *testing.T) {
	if ChannelLabel("email") == "" {
		t.Error("ChannelLabel(\"email\") returned empty")
	}
}
