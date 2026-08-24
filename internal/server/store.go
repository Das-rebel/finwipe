package server

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // reuse CLI's driver
)

// Store wraps the existing finwipe SQLite DB (same schema as the CLI's
// history store) and adds server-side queries.

type RequestRow struct {
	ID             string
	EntityName     string
	GrievanceEmail string
	State          string
	SentAt         *time.Time
	AckedAt        *time.Time
}

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS requests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  request_id TEXT NOT NULL UNIQUE,
  nbfc_id TEXT NOT NULL DEFAULT '',
  nbfc_name TEXT NOT NULL DEFAULT '',
  channel TEXT NOT NULL DEFAULT 'email',
  lifecycle_state TEXT NOT NULL DEFAULT 'INITIATED',
  escalation_level INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  dispatched_at DATETIME,
  ack_deadline_at DATETIME,
  ack_received_at DATETIME,
  response_deadline_at DATETIME,
  closed_at DATETIME,
  external_ref TEXT,
  grievance_email TEXT,
  user_email TEXT,
  user_name TEXT,
  outcome TEXT,
  outcome_notes TEXT,
  letter_path TEXT,
  active INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS server_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  dpr_id TEXT NOT NULL,
  ts DATETIME DEFAULT CURRENT_TIMESTAMP,
  kind TEXT NOT NULL,      -- sent | ack | reject | escalate | note
  detail TEXT
);
CREATE INDEX IF NOT EXISTS idx_server_events_dpr ON server_events(dpr_id);
`)
	return err
}

// OpenRequests returns all requests not in a terminal state.
func (s *Store) OpenRequests() ([]RequestRow, error) {
	rows, err := s.db.Query(`SELECT request_id, nbfc_name, grievance_email, lifecycle_state, dispatched_at, ack_received_at
FROM requests
WHERE lifecycle_state NOT IN ('CLOSED','RESPONSE_OK')`)
	if err != nil {
		return nil, err // table may not exist yet on fresh installs
	}
	defer rows.Close()
	var out []RequestRow
	for rows.Next() {
		var r RequestRow
		var sent, ack sql.NullTime
		if err := rows.Scan(&r.ID, &r.EntityName, &r.GrievanceEmail, &r.State, &sent, &ack); err != nil {
			continue
		}
		r.SentAt, r.AckedAt = nilIfZero(sent), nilIfZero(ack)
		out = append(out, r)
	}
	return out, rows.Err()
}

// All returns every request for the dashboard.
func (s *Store) All() ([]RequestRow, error) { return s.queryAll("") }

func (s *Store) queryAll(where string) ([]RequestRow, error) {
	q := `SELECT request_id, nbfc_name, grievance_email, lifecycle_state, dispatched_at, ack_received_at FROM requests ORDER BY request_id`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RequestRow
	for rows.Next() {
		var r RequestRow
		var sent, ack sql.NullTime
		if err := rows.Scan(&r.ID, &r.EntityName, &r.GrievanceEmail, &r.State, &sent, &ack); err != nil {
			continue
		}
		r.SentAt, r.AckedAt = nilIfZero(sent), nilIfZero(ack)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Transition moves a request to a new state and logs the event.
func (s *Store) Transition(dprID, newState, detail string) error {
	col := map[string]string{
		"AckReceived": "ack_at", // handled below via explicit times
	}[newState]
	_ = col

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	switch newState {
	case "ACK_RECEIVED":
		if _, err := tx.Exec(`UPDATE requests SET lifecycle_state=?, ack_received_at=? WHERE request_id=?`, newState, now, dprID); err != nil {
			return err
		}
	default:
		if _, err := tx.Exec(`UPDATE requests SET lifecycle_state=? WHERE request_id=?`, newState, dprID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO server_events (dpr_id, kind, detail) VALUES (?,?,?)`,
		dprID, eventKind(newState), detail); err != nil {
		return err
	}
	return tx.Commit()
}

func eventKind(state string) string {
	switch state {
	case "ACK_RECEIVED":
		return "ack"
	case "PENDING_REVIEW":
		return "reject"
	case "ESCALATED":
		return "escalate"
	default:
		return "note"
	}
}

// Overdue returns dispatched requests with no acknowledgment after days.
func (s *Store) Overdue(days int) ([]RequestRow, error) {
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
	rows, err := s.db.Query(`SELECT request_id, nbfc_name, grievance_email, lifecycle_state, dispatched_at, ack_received_at
FROM requests
WHERE lifecycle_state='DISPATCHED' AND ack_received_at IS NULL AND dispatched_at < ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RequestRow
	for rows.Next() {
		var r RequestRow
		var sent, ack sql.NullTime
		if err := rows.Scan(&r.ID, &r.EntityName, &r.GrievanceEmail, &r.State, &sent, &ack); err != nil {
			continue
		}
		r.SentAt, r.AckedAt = nilIfZero(sent), nilIfZero(ack)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) LogEvent(dprID, kind, detail string) error {
	_, err := s.db.Exec(`INSERT INTO server_events (dpr_id, kind, detail) VALUES (?,?,?)`, dprID, kind, detail)
	return err
}

func nilIfZero(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

var _ = fmt.Sprintf // keep fmt during scaffold phase
