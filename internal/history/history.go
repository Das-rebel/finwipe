package history

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

type Request struct {
	ID             int64     `db:"id"`
	NBFCID         string    `db:"nbfc_id"`
	NBFCName       string    `db:"nbfc_name"`
	Channel        string    `db:"channel"` // email | post | cic
	Status         string    `db:"status"`  // pending | sent | acknowledged | completed | failed | manual_required
	SentAt         time.Time `db:"sent_at"`
	AcknowledgedAt time.Time `db:"acknowledged_at"`
	CompletedAt    time.Time `db:"completed_at"`
	ExternalRef    string    `db:"external_ref"` // tracking number, ticket ID
	FailureReason  string    `db:"failure_reason"`
	Notes          string    `db:"notes"`
}

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

func (h *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nbfc_id TEXT NOT NULL,
		nbfc_name TEXT NOT NULL,
		channel TEXT NOT NULL DEFAULT 'email',
		status TEXT NOT NULL DEFAULT 'pending',
		sent_at DATETIME,
		acknowledged_at DATETIME,
		completed_at DATETIME,
		external_ref TEXT,
		failure_reason TEXT,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(nbfc_id, channel)
	);
	CREATE INDEX IF NOT EXISTS idx_status ON requests(status);
	CREATE INDEX IF NOT EXISTS idx_nbfc ON requests(nbfc_id);
	`
	_, err := h.db.Exec(schema)
	return err
}

func (h *DB) RecordRequest(nbfcID, nbfcName, channel, status string) error {
	if status == "" {
		status = "pending"
	}
	now := time.Now()
	query := `
	INSERT OR REPLACE INTO requests (nbfc_id, nbfc_name, channel, status, sent_at)
	VALUES (?, ?, ?, ?, ?)
	`
	_, err := h.db.Exec(query, nbfcID, nbfcName, channel, status, now)
	return err
}

func (h *DB) UpdateStatus(nbfcID, channel, status, reason string) error {
	query := `UPDATE requests SET status = ?, failure_reason = ? WHERE nbfc_id = ? AND channel = ?`
	_, err := h.db.Exec(query, status, reason, nbfcID, channel)
	return err
}

func (h *DB) MarkSent(nbfcID, channel string) error {
	query := `UPDATE requests SET status = 'sent', sent_at = ? WHERE nbfc_id = ? AND channel = ?`
	_, err := h.db.Exec(query, time.Now(), nbfcID, channel)
	return err
}

func (h *DB) MarkAcknowledged(nbfcID, channel, ref string) error {
	query := `UPDATE requests SET status = 'acknowledged', acknowledged_at = ?, external_ref = ? WHERE nbfc_id = ? AND channel = ?`
	_, err := h.db.Exec(query, time.Now(), ref, nbfcID, channel)
	return err
}

func (h *DB) MarkCompleted(nbfcID, channel string) error {
	query := `UPDATE requests SET status = 'completed', completed_at = ? WHERE nbfc_id = ? AND channel = ?`
	_, err := h.db.Exec(query, time.Now(), nbfcID, channel)
	return err
}

func (h *DB) MarkManualRequired(nbfcID, channel, notes string) error {
	query := `UPDATE requests SET status = 'manual_required', notes = ? WHERE nbfc_id = ? AND channel = ?`
	_, err := h.db.Exec(query, notes, nbfcID, channel)
	return err
}

func (h *DB) GetByStatus(status string) ([]Request, error) {
	query := `SELECT id, nbfc_id, nbfc_name, channel, status, sent_at, acknowledged_at, completed_at, external_ref, failure_reason, notes FROM requests WHERE status = ?`
	rows, err := h.db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []Request
	for rows.Next() {
		var r Request
		var sentAt, ackAt, compAt sql.NullTime
		var ref, reason, notes sql.NullString
		if err := rows.Scan(&r.ID, &r.NBFCID, &r.NBFCName, &r.Channel, &r.Status,
			&sentAt, &ackAt, &compAt, &ref, &reason, &notes); err != nil {
			return nil, err
		}
		if sentAt.Valid {
			r.SentAt = sentAt.Time
		}
		if ackAt.Valid {
			r.AcknowledgedAt = ackAt.Time
		}
		if compAt.Valid {
			r.CompletedAt = compAt.Time
		}
		r.ExternalRef = ref.String
		r.FailureReason = reason.String
		r.Notes = notes.String
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func (h *DB) GetAll() ([]Request, error) {
	query := `SELECT id, nbfc_id, nbfc_name, channel, status, sent_at, acknowledged_at, completed_at, external_ref, failure_reason, notes FROM requests ORDER BY created_at DESC`
	rows, err := h.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []Request
	for rows.Next() {
		var r Request
		var sentAt, ackAt, compAt sql.NullTime
		var ref, reason, notes sql.NullString
		if err := rows.Scan(&r.ID, &r.NBFCID, &r.NBFCName, &r.Channel, &r.Status,
			&sentAt, &ackAt, &compAt, &ref, &reason, &notes); err != nil {
			return nil, err
		}
		if sentAt.Valid {
			r.SentAt = sentAt.Time
		}
		if ackAt.Valid {
			r.AcknowledgedAt = ackAt.Time
		}
		if compAt.Valid {
			r.CompletedAt = compAt.Time
		}
		r.ExternalRef = ref.String
		r.FailureReason = reason.String
		r.Notes = notes.String
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func (h *DB) Summary() (total, pending, sent, acknowledged, completed, failed, manual int) {
	rows, err := h.db.Query(`SELECT status, COUNT(*) FROM requests GROUP BY status`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var c int
		if rows.Scan(&s, &c) == nil {
			total += c
			switch s {
			case "pending":
				pending = c
			case "sent":
				sent = c
			case "acknowledged":
				acknowledged = c
			case "completed":
				completed = c
			case "failed":
				failed = c
			case "manual_required":
				manual = c
			}
		}
	}
	return
}

func (h *DB) Close() error {
	return h.db.Close()
}
