package history

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ============================================================
// Request lifecycle states
// ============================================================
type State string

const (
	StateInitiated      State = "INITIATED"
	StateDispatched     State = "DISPATCHED"
	StateDeliveryFailed State = "DELIVERY_FAILED"
	StateAckReceived    State = "ACK_RECEIVED"
	StatePendingReview  State = "PENDING_REVIEW" // NBFC response needs user review
	StateResponseOK     State = "RESPONSE_OK"
	StateEscalated      State = "ESCALATED"
	StateClosed         State = "CLOSED"
)

const (
	OutcomeDeleted         = "deleted"
	OutcomePartial        = "partial"
	OutcomeExemptionClaimed = "exemption_claimed"
	OutcomeNoResponse      = "no_response"
	OutcomeRejected       = "rejected"
	OutcomeEscalated      = "escalated"
	OutcomeWithdrawn      = "withdrawn"
)

const (
	EscNone       = 0
	EscDPO        = 1 // Internal DPO/NBFC Grievance Officer (L0→L1)
	EscDPDPBoard  = 2 // Primary regulatory: Data Protection Board of India (L1)
	EscRBIOmbu    = 3 // For RBI-regulated entities: RBI Ombudsman (L2)
	EscConsumer   = 4 // Consumer Forum: CPA 2019 deficiency in service (L3)
	EscLegal      = 5 // Civil litigation / High Court (L4)
)

const (
	ChannelEmail = "email"
	ChannelPost  = "post"
	ChannelCIC   = "cic"
)

// ValidTransitions maps current state -> allowed next states
var ValidTransitions = map[State][]State{
	StateInitiated:      {StateDispatched, StateClosed},
	StateDispatched:     {StateAckReceived, StateDeliveryFailed, StateEscalated, StateClosed},
	StateDeliveryFailed: {StateDispatched, StateClosed}, // Can retry dispatch
	StateAckReceived:    {StateResponseOK, StatePendingReview, StateEscalated, StateClosed},
	StatePendingReview:  {StateResponseOK, StateEscalated, StateClosed}, // User reviews NBFC response
	StateResponseOK:     {StateClosed},
	StateEscalated:      {StateClosed, StateAckReceived, StateResponseOK}, // Can de-escalate if resolved
	StateClosed:         {},
}

// IsValidTransition checks if a state transition is allowed
func (s State) IsValid(next State) bool {
	for _, a := range ValidTransitions[s] {
		if a == next {
			return true
		}
	}
	return false
}

// ============================================================
// Core data types
// ============================================================

type Request struct {
	ID          int64  `db:"id"`
	RequestID   string `db:"request_id"` // DPR-YYYY-NNNNNN
	NBFCID      string `db:"nbfc_id"`
	NBFCName    string `db:"nbfc_name"`
	Channel     string `db:"channel"` // email | post | cic

	// State machine
	LifecycleState   State `db:"lifecycle_state"`
	EscalationLevel int   `db:"escalation_level"` // 0-5

	// Timestamps
	CreatedAt          time.Time `db:"created_at"`
	DispatchedAt       time.Time `db:"dispatched_at"`
	AckDeadlineAt      time.Time `db:"ack_deadline_at"`
	AckReceivedAt      time.Time `db:"ack_received_at"`
	ResponseDeadlineAt time.Time `db:"response_deadline_at"`
	ClosedAt           time.Time `db:"closed_at"`

	// References
	ExternalRef    string `db:"external_ref"`
	GrievanceEmail string `db:"grievance_email"`
	UserEmail      string `db:"user_email"`
	UserName       string `db:"user_name"`

	// Outcome
	Outcome      string `db:"outcome"`
	OutcomeNotes string `db:"outcome_notes"`

	// Letter
	LetterPath string `db:"letter_path"`

	// Active flag (soft delete)
	Active bool `db:"active"`

	// Computed
	DaysSinceDispatch int  `db:"-"`
	DaysToDeadline   int  `db:"-"`
	DeadlineStatus   string `db:"-"`
}

type AuditEntry struct {
	ID             int64     `db:"id"`
	RequestID      string    `db:"request_id"`
	Action         string    `db:"action"`
	PrevState     *string   `db:"prev_state"`
	NewState      *string   `db:"new_state"`
	PrevEscLevel  *int      `db:"prev_escalation"`
	NewEscLevel   *int      `db:"new_escalation"`
	Actor         string    `db:"actor"` // CLI_USER | AUTOMATION | SYSTEM
	Detail        string    `db:"detail"`
	RefNumber     string    `db:"ref_number"`
	CreatedAt     time.Time `db:"created_at"`
}

type Escalation struct {
	ID             int64     `db:"id"`
	RequestID      string    `db:"request_id"`
	Channel        string    `db:"channel"` // FOLLOWUP_EMAIL | RBI_SACHET | DPDP_BOARD | CONSUMER_FORUM | RBI_OMBUDSMAN
	Status         string    `db:"status"`  // pending | filed | acknowledged | resolved | rejected | escalated_further
	ComplaintRef   string    `db:"complaint_ref"`
	FiledAt        time.Time `db:"filed_at"`
	AckDeadlineAt  time.Time `db:"ack_deadline_at"`
	ResolvedAt     time.Time `db:"resolved_at"`
	Resolution     string    `db:"resolution"`
	Summary        string    `db:"summary"`
	AuditID        int64     `db:"audit_id"`
	CreatedAt      time.Time `db:"created_at"`
}

type Followup struct {
	ID             int64     `db:"id"`
	RequestID      string    `db:"request_id"`
	Type           string    `db:"type"` // ACK_DEMAND | FRIENDLY_NUDGE | FORMAL_REMINDER | DEADLINE_WARNING | ESCALATION_NOTICE
	ScheduledAt    time.Time `db:"scheduled_at"`
	SentAt         time.Time `db:"sent_at"`
	Channel        string    `db:"channel"` // email | post | whatsapp
	DeliveryStatus string    `db:"delivery_status"` // pending | sent | delivered | bounced | failed
	Subject        string    `db:"subject"`
	MessageID      string    `db:"message_id"`
	RefAttachment  string    `db:"ref_attachment"`
	CreatedAt      time.Time `db:"created_at"`
}

type DB struct {
	db *sql.DB
}

// ============================================================
// New creates/opens the database and runs migrations
// ============================================================
func New(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal=WAL&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	h := &DB{db: db}
	if err := h.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return h, nil
}

func (h *DB) Close() error { return h.db.Close() }

// ============================================================
// Migrations
// ============================================================
func (h *DB) migrate() error {
	// Check schema version — if old schema exists, drop it for clean migration
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='requests'`).Scan(&count)
	if count > 0 {
		// Check if it's the new schema by looking for lifecycle_state column
		var colCount int
		h.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('requests') WHERE name='lifecycle_state'`).Scan(&colCount)
		if colCount == 0 {
			// Old schema — drop and recreate
			h.db.Exec(`DROP TABLE IF EXISTS requests`)
			h.db.Exec(`DROP TABLE IF EXISTS audit_log`)
			h.db.Exec(`DROP TABLE IF EXISTS escalations`)
			h.db.Exec(`DROP TABLE IF EXISTS followups`)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS requests (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id      TEXT    NOT NULL UNIQUE,
		nbfc_id         TEXT    NOT NULL,
		nbfc_name       TEXT    NOT NULL,
		channel         TEXT    NOT NULL DEFAULT 'email',
		lifecycle_state TEXT    NOT NULL DEFAULT 'INITIATED',
		escalation_level INTEGER NOT NULL DEFAULT 0,

		created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		dispatched_at        DATETIME,
		ack_deadline_at      DATETIME,
		ack_received_at      DATETIME,
		response_deadline_at DATETIME,
		closed_at            DATETIME,

		external_ref    TEXT,
		grievance_email TEXT,
		user_email      TEXT,
		user_name       TEXT,

		outcome       TEXT,
		outcome_notes TEXT,
		letter_path   TEXT,
		active       INTEGER NOT NULL DEFAULT 1,

		CONSTRAINT chk_channel CHECK (channel IN ('email','post','cic')),
		CONSTRAINT chk_state CHECK (lifecycle_state IN (
			'INITIATED','DISPATCHED','ACK_RECEIVED','RESPONSE_OK','ESCALATED','CLOSED'))
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id      TEXT    NOT NULL,
		action          TEXT    NOT NULL,
		prev_state      TEXT,
		new_state       TEXT,
		prev_escalation INTEGER,
		new_escalation  INTEGER,
		actor           TEXT    NOT NULL,
		detail          TEXT,
		ref_number      TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (request_id) REFERENCES requests(request_id)
	);

	CREATE TABLE IF NOT EXISTS escalations (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id      TEXT    NOT NULL,
		channel         TEXT    NOT NULL,
		status          TEXT    NOT NULL DEFAULT 'pending',
		complaint_ref   TEXT,
		filed_at        DATETIME,
		ack_deadline_at DATETIME,
		resolved_at     DATETIME,
		resolution      TEXT,
		summary         TEXT,
		audit_id        INTEGER,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (request_id) REFERENCES requests(request_id)
	);

	CREATE TABLE IF NOT EXISTS followups (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id      TEXT    NOT NULL,
		type            TEXT    NOT NULL,
		scheduled_at    DATETIME NOT NULL,
		sent_at         DATETIME,
		channel         TEXT    NOT NULL DEFAULT 'email',
		delivery_status TEXT    NOT NULL DEFAULT 'pending',
		subject         TEXT,
		message_id      TEXT,
		ref_attachment  TEXT,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (request_id) REFERENCES requests(request_id)
	);

	CREATE INDEX IF NOT EXISTS idx_req_nbfc    ON requests(nbfc_id);
	CREATE INDEX IF NOT EXISTS idx_req_state   ON requests(lifecycle_state);
	CREATE INDEX IF NOT EXISTS idx_req_esc     ON requests(escalation_level);
	CREATE INDEX IF NOT EXISTS idx_req_active  ON requests(active);
	CREATE INDEX IF NOT EXISTS idx_audit_req   ON audit_log(request_id);
	CREATE INDEX IF NOT EXISTS idx_esc_req     ON escalations(request_id);
	CREATE INDEX IF NOT EXISTS idx_esc_status  ON escalations(status);
	CREATE INDEX IF NOT EXISTS idx_fu_req      ON followups(request_id);
	CREATE INDEX IF NOT EXISTS idx_fu_sched    ON followups(scheduled_at);
	`

	_, err := h.db.Exec(schema)
	return err
}

// ============================================================
// Request ID generation: DPR-YYYY-NNNNNN
// ============================================================
func (h *DB) nextRequestID() (string, error) {
	year := time.Now().Year()
	var maxSeq sql.NullInt64
	query := `SELECT MAX(CAST(SUBSTR(request_id, 13) AS INTEGER)) FROM requests WHERE request_id LIKE ?`
	pattern := fmt.Sprintf("DPR-%d-%%", year)
	row := h.db.QueryRow(query, pattern)
	if err := row.Scan(&maxSeq); err != nil && err != sql.ErrNoRows {
		return "", err
	}
	seq := 1
	if maxSeq.Valid {
		seq = int(maxSeq.Int64) + 1
	}
	return fmt.Sprintf("DPR-%d-%06d", year, seq), nil
}

// ============================================================
// Request CRUD
// ============================================================

// CreateRequest creates a new deletion request in INITIATED state
func (h *DB) CreateRequest(nbfcID, nbfcName, channel, grievanceEmail, userEmail, userName string) (*Request, error) {
	reqID, err := h.nextRequestID()
	if err != nil {
		return nil, fmt.Errorf("request_id: %w", err)
	}

	now := time.Now()
	ackDeadline := now.Add(48 * time.Hour)

	query := `
	INSERT INTO requests (request_id, nbfc_id, nbfc_name, channel,
		lifecycle_state, escalation_level, created_at,
		ack_deadline_at, grievance_email, user_email, user_name, active)
	VALUES (?, ?, ?, ?, 'INITIATED', 0, ?, ?, ?, ?, ?, 1)
	`
	res, err := h.db.Exec(query, reqID, nbfcID, nbfcName, channel,
		now, ackDeadline, grievanceEmail, userEmail, userName)
	if err != nil {
		return nil, fmt.Errorf("insert request: %w", err)
	}

	id, _ := res.LastInsertId()
	req := &Request{
		ID:           id,
		RequestID:    reqID,
		NBFCID:       nbfcID,
		NBFCName:     nbfcName,
		Channel:      channel,
		LifecycleState: StateInitiated,
		CreatedAt:    now,
		AckDeadlineAt: ackDeadline,
		GrievanceEmail: grievanceEmail,
		UserEmail:    userEmail,
		UserName:     userName,
		Active:       true,
	}

	// Audit the creation
	h.audit(reqID, "INITIATED", nil, strPtr(string(StateInitiated)),
		nil, nil, "CLI_USER", fmt.Sprintf("created for %s via %s", nbfcName, channel), "")

	return req, nil
}

// TransitionState safely transitions a request between states
// Uses idempotency token if provided to prevent double-processing on retry
func (h *DB) TransitionState(reqID string, from, to State, actor, detail string) error {
	if !from.IsValid(to) {
		return fmt.Errorf("invalid transition %s → %s", from, to)
	}

	// Idempotency check: if already in target state, skip
	var current string
	err := h.db.QueryRow(`SELECT lifecycle_state FROM requests WHERE request_id = ? AND active = 1`, reqID).Scan(&current)
	if err == sql.ErrNoRows {
		return fmt.Errorf("request %s not found", reqID)
	}
	if err != nil {
		return err
	}
	if State(current) == to {
		return nil // Already in target state — idempotent success
	}

	if State(current) != from {
		return fmt.Errorf("request %s is in state %s, expected %s", reqID, current, from)
	}

	tx, err := h.db.Begin()
	if err != nil {
		return err
	}

	query := `UPDATE requests SET lifecycle_state = ? WHERE request_id = ? AND lifecycle_state = ?`
	res, err := tx.Exec(query, string(to), reqID, string(from))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("update state: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		tx.Rollback()
		return fmt.Errorf("concurrent modification on %s", reqID)
	}

	// Audit in same transaction
	_, err = tx.Exec(`
		INSERT INTO audit_log (request_id, action, prev_state, new_state, actor, detail, created_at)
		VALUES (?, 'STATE_CHANGE', ?, ?, ?, ?, ?)
	`, reqID, string(from), string(to), actor, detail, time.Now())
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("audit log: %w", err)
	}

	return tx.Commit()
}

// Dispatch marks a request as DISPATCHED (email sent or letter generated)
func (h *DB) Dispatch(reqID, letterPath, messageID, actor string) error {
	now := time.Now()
	respDeadline := now.Add(30 * 24 * time.Hour) // 30 days from dispatch

	tx, err := h.db.Begin()
	if err != nil {
		return err
	}

	query := `
		UPDATE requests
		SET lifecycle_state = 'DISPATCHED', dispatched_at = ?,
			response_deadline_at = ?, letter_path = COALESCE(?, letter_path)
		WHERE request_id = ? AND lifecycle_state = 'INITIATED'
	`
	res, err := tx.Exec(query, now, respDeadline, letterPath, reqID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("dispatch: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		tx.Rollback()
		return fmt.Errorf("request %s not in INITIATED state or not found", reqID)
	}

	detail := fmt.Sprintf("dispatched via %s", actor)
	if messageID != "" {
		detail += fmt.Sprintf(" message_id=%s", messageID)
	}
	if letterPath != "" {
		detail += fmt.Sprintf(" letter=%s", filepath.Base(letterPath))
	}

	_, err = tx.Exec(`
		INSERT INTO audit_log (request_id, action, prev_state, new_state, actor, detail, created_at)
		VALUES (?, 'DISPATCHED', 'INITIATED', 'DISPATCHED', ?, ?, ?)
	`, reqID, actor, detail, now)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("audit: %w", err)
	}

	return tx.Commit()
}

// RecordAck records an acknowledgment from an NBFC
// Idempotent: if already acknowledged with same ref, returns nil
func (h *DB) RecordAck(reqID, externalRef, actor string, ackDate time.Time) error {
	if ackDate.IsZero() {
		ackDate = time.Now()
	}

	tx, err := h.db.Begin()
	if err != nil {
		return err
	}

	// Idempotency check
	var existingRef string
	err = tx.QueryRow(`SELECT external_ref FROM requests WHERE request_id = ?`, reqID).Scan(&existingRef)
	if err == sql.ErrNoRows {
		tx.Rollback()
		return fmt.Errorf("request %s not found", reqID)
	}
	if err != nil {
		tx.Rollback()
		return err
	}
	// Same acknowledgment already recorded — idempotent success
	if externalRef != "" && existingRef == externalRef {
		tx.Rollback()
		return nil
	}

	// Calculate new response deadline from ack date
	respDeadline := ackDate.Add(30 * 24 * time.Hour)

	query := `
		UPDATE requests
		SET lifecycle_state = 'ACK_RECEIVED', external_ref = COALESCE(?, external_ref),
			ack_received_at = ?, response_deadline_at = ?
		WHERE request_id = ? AND lifecycle_state = 'DISPATCHED'
	`
	res, err := tx.Exec(query, externalRef, ackDate, respDeadline, reqID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("record ack: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		tx.Rollback()
		return fmt.Errorf("request %s not in DISPATCHED state or not found", reqID)
	}

	detail := fmt.Sprintf("acknowledged by NBFC ref=%s", externalRef)
	_, err = tx.Exec(`
		INSERT INTO audit_log (request_id, action, prev_state, new_state, actor, detail, ref_number, created_at)
		VALUES (?, 'ACK_RECEIVED', 'DISPATCHED', 'ACK_RECEIVED', ?, ?, ?, ?)
	`, reqID, actor, detail, externalRef, ackDate)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("audit: %w", err)
	}

	return tx.Commit()
}

// SetEscalationLevel atomically sets the escalation level
func (h *DB) SetEscalationLevel(reqID string, fromLevel, toLevel int, actor, detail string) error {
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}

	res, err := tx.Exec(`
		UPDATE requests SET escalation_level = ?
		WHERE request_id = ? AND escalation_level = ?
	`, toLevel, reqID, fromLevel)
	if err != nil {
		tx.Rollback()
		return err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		tx.Rollback()
		return fmt.Errorf("request %s escalation_level is not %d (concurrent update?)", reqID, fromLevel)
	}

	_, err = tx.Exec(`
		INSERT INTO audit_log (request_id, action, prev_escalation, new_escalation, actor, detail, created_at)
		VALUES (?, 'ESCALATION_CHANGE', ?, ?, ?, ?, ?)
	`, reqID, fromLevel, toLevel, actor, detail, time.Now())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// CloseRequest closes a request with an outcome
func (h *DB) CloseRequest(reqID string, fromState State, outcome, outcomeNotes, actor string) error {
	now := time.Now()

	tx, err := h.db.Begin()
	if err != nil {
		return err
	}

	res, err := tx.Exec(`
		UPDATE requests
		SET lifecycle_state = 'CLOSED', outcome = ?, outcome_notes = ?,
			closed_at = ?, escalation_level = 0
		WHERE request_id = ? AND lifecycle_state = ?
	`, outcome, outcomeNotes, now, reqID, string(fromState))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("close: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		tx.Rollback()
		return fmt.Errorf("request %s not in %s state", reqID, fromState)
	}

	_, err = tx.Exec(`
		INSERT INTO audit_log (request_id, action, prev_state, new_state, actor, detail, created_at)
		VALUES (?, 'CLOSED', ?, 'CLOSED', ?, ?, ?)
	`, reqID, string(fromState), actor, fmt.Sprintf("outcome=%s notes=%s", outcome, outcomeNotes), now)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// ============================================================
// Queries
// ============================================================

func (h *DB) GetByRequestID(reqID string) (*Request, error) {
	var r Request
	var ackReceived, respDeadline, dispatched, closed sql.NullTime
	var externalRef, grievanceEmail, userEmail, userName sql.NullString
	var outcome, outcomeNotes, letterPath sql.NullString

	err := h.db.QueryRow(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), grievance_email, COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests WHERE request_id = ? AND active = 1
	`, reqID).Scan(
		&r.ID, &r.RequestID, &r.NBFCID, &r.NBFCName, &r.Channel,
		&r.LifecycleState, &r.EscalationLevel, &r.CreatedAt,
		&dispatched, &r.AckDeadlineAt, &ackReceived, &respDeadline, &closed,
		&externalRef, &grievanceEmail, &userEmail, &userName,
		&outcome, &outcomeNotes, &letterPath, &r.Active,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("request not found: %s", reqID)
	}
	if err != nil {
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
	if grievanceEmail.Valid {
		r.GrievanceEmail = grievanceEmail.String
	}
	if userEmail.Valid {
		r.UserEmail = userEmail.String
	}
	if userName.Valid {
		r.UserName = userName.String
	}
	if outcome.Valid {
		r.Outcome = outcome.String
	}
	if outcomeNotes.Valid {
		r.OutcomeNotes = outcomeNotes.String
	}
	if letterPath.Valid {
		r.LetterPath = letterPath.String
	}
	r.ComputeDeadline()
	return &r, nil
}

func (h *DB) GetByNBFCID(nbfcID string) ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests WHERE nbfc_id = ? AND active = 1
		ORDER BY created_at DESC
	`, nbfcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

func (h *DB) GetActive() ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests WHERE active = 1 AND lifecycle_state != 'CLOSED'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

// GetAll returns all requests (active + closed) for reporting
func (h *DB) GetAll() ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests WHERE active = 1
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

func (h *DB) GetPendingAck() ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests
		WHERE active = 1 AND lifecycle_state = 'DISPATCHED' AND ack_received_at IS NULL
		ORDER BY ack_deadline_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

func (h *DB) GetOverdue() ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests
		WHERE active = 1 AND lifecycle_state IN ('DISPATCHED','ACK_RECEIVED')
			AND response_deadline_at < ?
		ORDER BY response_deadline_at ASC
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

func (h *DB) GetEscalated() ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests
		WHERE active = 1 AND escalation_level > 0
		ORDER BY escalation_level DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

func (h *DB) GetByState(state State) ([]Request, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, nbfc_id, nbfc_name, channel, lifecycle_state,
			escalation_level, created_at, dispatched_at, ack_deadline_at,
			ack_received_at, response_deadline_at, closed_at,
			COALESCE(external_ref, ''), COALESCE(grievance_email, ''), COALESCE(user_email, ''), COALESCE(user_name, ''),
			COALESCE(outcome, ''), COALESCE(outcome_notes, ''), COALESCE(letter_path, ''), active
		FROM requests WHERE active = 1 AND lifecycle_state = ?
		ORDER BY created_at DESC
	`, string(state))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return h.scanRequests(rows)
}

func (h *DB) scanRequests(rows *sql.Rows) ([]Request, error) {
	var result []Request
	for rows.Next() {
		var r Request
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

// ============================================================
// Audit trail
// ============================================================
func (h *DB) audit(reqID, action string, prevState, newState *string, prevEsc, newEsc *int, actor, detail, ref string) {
	h.db.Exec(`
		INSERT INTO audit_log (request_id, action, prev_state, new_state, prev_escalation, new_escalation, actor, detail, ref_number, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, reqID, action, prevState, newState, prevEsc, newEsc, actor, detail, ref, time.Now())
}

func (h *DB) GetAuditTrail(reqID string) ([]AuditEntry, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, action, prev_state, new_state, prev_escalation, new_escalation, actor, detail, ref_number, created_at
		FROM audit_log WHERE request_id = ? ORDER BY created_at ASC
	`, reqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var prevS, newS *string
		var prevE, newE *int
		err := rows.Scan(&e.ID, &e.RequestID, &e.Action, &prevS, &newS, &prevE, &newE, &e.Actor, &e.Detail, &e.RefNumber, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		if prevS != nil {
			e.PrevState = prevS
		}
		if newS != nil {
			e.NewState = newS
		}
		if prevE != nil {
			e.PrevEscLevel = prevE
		}
		if newE != nil {
			e.NewEscLevel = newE
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ============================================================
// Escalations
// ============================================================

func (h *DB) CreateEscalation(reqID, channel, summary string) (*Escalation, error) {
	now := time.Now()
	ackDeadline := now.Add(30 * 24 * time.Hour) // RBI/DPDP: 30-day acknowledgment

	res, err := h.db.Exec(`
		INSERT INTO escalations (request_id, channel, status, summary, filed_at, ack_deadline_at, created_at)
		VALUES (?, ?, 'filed', ?, ?, ?, ?)
	`, reqID, channel, summary, now, ackDeadline, now)
	if err != nil {
		return nil, fmt.Errorf("create escalation: %w", err)
	}

	id, _ := res.LastInsertId()
	return &Escalation{
		ID:            id,
		RequestID:     reqID,
		Channel:       channel,
		Status:        "filed",
		FiledAt:       now,
		AckDeadlineAt: ackDeadline,
		Summary:       summary,
		CreatedAt:     now,
	}, nil
}

func (h *DB) UpdateEscalation(escID int64, status, complaintRef string) error {
	query := `UPDATE escalations SET status = ?, complaint_ref = ? WHERE id = ?`
	_, err := h.db.Exec(query, status, complaintRef, escID)
	return err
}

func (h *DB) GetEscalationsByRequest(reqID string) ([]Escalation, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, channel, status, complaint_ref, filed_at,
			ack_deadline_at, resolved_at, resolution, summary, audit_id, created_at
		FROM escalations WHERE request_id = ? ORDER BY created_at ASC
	`, reqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var escs []Escalation
	for rows.Next() {
		var e Escalation
		var filed, ackDL, resolved *time.Time
		err := rows.Scan(&e.ID, &e.RequestID, &e.Channel, &e.Status, &e.ComplaintRef,
			&filed, &ackDL, &resolved, &e.Resolution, &e.Summary, &e.AuditID, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		if filed != nil {
			e.FiledAt = *filed
		}
		if ackDL != nil {
			e.AckDeadlineAt = *ackDL
		}
		if resolved != nil {
			e.ResolvedAt = *resolved
		}
		escs = append(escs, e)
	}
	return escs, rows.Err()
}

// ============================================================
// Followups
// ============================================================

func (h *DB) ScheduleFollowup(reqID, fuType, channel string, scheduledAt time.Time) (int64, error) {
	res, err := h.db.Exec(`
		INSERT INTO followups (request_id, type, scheduled_at, channel, delivery_status, created_at)
		VALUES (?, ?, ?, ?, 'pending', ?)
	`, reqID, fuType, scheduledAt, channel, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (h *DB) MarkFollowupSent(fuID int64, messageID string) error {
	_, err := h.db.Exec(`
		UPDATE followups SET sent_at = ?, delivery_status = 'sent', message_id = ?
		WHERE id = ?
	`, time.Now(), messageID, fuID)
	return err
}

func (h *DB) MarkFollowupFailed(fuID int64) error {
	_, err := h.db.Exec(`
		UPDATE followups SET delivery_status = 'failed'
		WHERE id = ?
	`, fuID)
	return err
}

// GetDb exposes the underlying database for cross-package queries (use sparingly)
func (h *DB) GetDb() *sql.DB {
	return h.db
}

func (h *DB) FollowupExists(reqID, fuType string) bool {
	var count int
	h.db.QueryRow(`
		SELECT COUNT(*) FROM followups WHERE request_id = ? AND type = ?
	`, reqID, fuType).Scan(&count)
	return count > 0
}

func (h *DB) GetDueFollowups(from, to time.Time) ([]Followup, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, type, scheduled_at, sent_at, channel, delivery_status, subject, message_id, ref_attachment, created_at
		FROM followups
		WHERE delivery_status = 'pending' AND scheduled_at BETWEEN ? AND ?
		ORDER BY scheduled_at ASC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fus []Followup
	for rows.Next() {
		var f Followup
		var sent, sched *time.Time
		err := rows.Scan(&f.ID, &f.RequestID, &f.Type, &sched, &sent, &f.Channel, &f.DeliveryStatus, &f.Subject, &f.MessageID, &f.RefAttachment, &f.CreatedAt)
		if err != nil {
			return nil, err
		}
		if sent != nil {
			f.SentAt = *sent
		}
		if sched != nil {
			f.ScheduledAt = *sched
		}
		fus = append(fus, f)
	}
	return fus, rows.Err()
}

func (h *DB) GetPendingFollowupsByType(reqID, fuType string) ([]Followup, error) {
	rows, err := h.db.Query(`
		SELECT id, request_id, type, scheduled_at, sent_at, channel, delivery_status, subject, message_id, ref_attachment, created_at
		FROM followups WHERE request_id = ? AND type = ? AND delivery_status = 'pending'
	`, reqID, fuType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fus []Followup
	for rows.Next() {
		var f Followup
		var sent, sched *time.Time
		err := rows.Scan(&f.ID, &f.RequestID, &f.Type, &sched, &sent, &f.Channel, &f.DeliveryStatus, &f.Subject, &f.MessageID, &f.RefAttachment, &f.CreatedAt)
		if err != nil {
			return nil, err
		}
		if sent != nil {
			f.SentAt = *sent
		}
		if sched != nil {
			f.ScheduledAt = *sched
		}
		fus = append(fus, f)
	}
	return fus, rows.Err()
}

// ============================================================
// Summary (for status command compatibility)
// ============================================================
func (h *DB) Summary() (map[string]int, error) {
	rows, err := h.db.Query(`
		SELECT lifecycle_state, COUNT(*) FROM requests WHERE active = 1 GROUP BY lifecycle_state
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := map[string]int{"total": 0}
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, err
		}
		m[state] = count
		m["total"] += count
	}

	// Escalation counts
	var escCount int
	h.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE active = 1 AND escalation_level > 0`).Scan(&escCount)
	m["escalated"] = escCount

	// Outcome counts
	outcomes := []string{"deleted", "partial", "exemption_claimed", "no_response", "rejected", "escalated", "withdrawn"}
	for _, o := range outcomes {
		var count int
		h.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE active = 1 AND outcome = ?`, o).Scan(&count)
		m[o] = count
	}

	return m, nil
}

// ============================================================
// Helpers
// ============================================================
func (r *Request) ComputeDeadline() {
	if !r.DispatchedAt.IsZero() {
		r.DaysSinceDispatch = int(time.Since(r.DispatchedAt).Hours() / 24)
	}
	if !r.ResponseDeadlineAt.IsZero() {
		r.DaysToDeadline = int(time.Until(r.ResponseDeadlineAt).Hours() / 24)
	}

	if r.LifecycleState == StateClosed {
		r.DeadlineStatus = "closed"
		return
	}
	if !r.AckReceivedAt.IsZero() {
		switch {
		case r.DaysToDeadline < 0:
			r.DeadlineStatus = "expired"
		case r.DaysToDeadline <= 3:
			r.DeadlineStatus = "critical"
		case r.DaysToDeadline <= 10:
			r.DeadlineStatus = "warning"
		default:
			r.DeadlineStatus = "ok"
		}
	} else {
		if time.Since(r.AckDeadlineAt) > 0 {
			r.DeadlineStatus = "ack_overdue"
		} else {
			r.DeadlineStatus = "awaiting_ack"
		}
	}
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// ChannelLabel maps channel code to display label
func ChannelLabel(c string) string {
	return map[string]string{
		"email": "✉️  email",
		"post":  "📮 registered post",
		"cic":   "🏛️  CIC dispute",
	}[c]
}

// EscalationChannelLabel maps channel to display label
// Order reflects regulatory hierarchy: DPDP Board is primary data protection authority
func EscalationChannelLabel(c string) string {
	return map[string]string{
		"FOLLOWUP_EMAIL":  "Follow-up Email",
		"DPO":             "NBFC Grievance Officer",
		"DPDP_BOARD":      "Data Protection Board of India",
		"RBI_OMBUDSMAN":   "RBI Integrated Ombudsman",
		"RBI_SACHET":      "RBI Sachet Portal",
		"CONSUMER_FORUM":  "Consumer Forum (CPA 2019)",
		"LEGAL":           "Civil Litigation",
	}[c]
}

// EscalationLevelLabel maps level number to display label
func EscalationLevelLabel(level int) string {
	return map[int]string{
		0: "L0 — No escalation",
		1: "L1 — DPO / Internal Grievance Officer",
		2: "L2 — DPDP Board of India (primary)",
		3: "L3 — RBI Ombudsman / Consumer Forum",
		4: "L4 — Legal / High Court",
	}[level]
}

// FollowupTypeLabel maps followup type to display label
func FollowupTypeLabel(t string) string {
	return map[string]string{
		"ACK_DEMAND":         "⚠️  ACK Demand",
		"FRIENDLY_NUDGE":     "📧 Friendly Nudge",
		"FORMAL_REMINDER":    "📋 Formal Reminder",
		"DEADLINE_WARNING":   "🔴 Deadline Warning",
		"ESCALATION_NOTICE":  "📮 Escalation Notice",
	}[t]
}

// StateLabel maps state to emoji label
func StateLabel(s State) string {
	return map[State]string{
		StateInitiated:      "🆕 INITIATED",
		StateDispatched:     "📨 DISPATCHED",
		StateDeliveryFailed: "❌ DELIVERY_FAILED",
		StateAckReceived:    "✅ ACK_RECEIVED",
		StatePendingReview:  "👁️  PENDING_REVIEW",
		StateResponseOK:     "✔️  RESPONSE_OK",
		StateEscalated:      "🔺 ESCALATED",
		StateClosed:         "✔️  CLOSED",
	}[s]
}

// Exists checks if a request with the given ID exists
func (h *DB) Exists(reqID string) bool {
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE request_id = ? AND active = 1`, reqID).Scan(&count)
	return count > 0
}

// NBFCRequestCount returns count of active requests for an NBFC
func (h *DB) NBFCRequestCount(nbfcID string) int {
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE nbfc_id = ? AND active = 1`, nbfcID).Scan(&count)
	return count
}

// SanitizeNBFCID cleans and lowercases NBFC IDs
func SanitizeNBFCID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, " ", "-")
	return id
}
