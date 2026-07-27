package evidence

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EvidenceType categorizes the type of evidence
type EvidenceType string

const (
	TypeEmailSent         EvidenceType = "email_sent"
	TypeEmailReceived    EvidenceType = "email_received"
	TypeEmailBounce      EvidenceType = "email_bounce"
	TypeAcknowledgment   EvidenceType = "acknowledgment"
	TypeNBFCResponse     EvidenceType = "nbfc_response"
	TypeEscalationFiling EvidenceType = "escalation_filing"
	TypeDeliveryReceipt  EvidenceType = "delivery_receipt"
	TypePostalReceipt    EvidenceType = "postal_receipt"
	TypeGenericFile      EvidenceType = "generic_file"
)

// Store manages evidence files attached to DPR requests
type Store struct {
	BasePath string
}

// Evidence represents a single evidence record
type Evidence struct {
	ID          string       `json:"id"`
	RequestID   string       `json:"request_id"`
	Type        EvidenceType `json:"type"`
	Filename    string       `json:"filename"`
	ContentType string       `json:"content_type"`
	SizeBytes   int64        `json:"size_bytes"`
	Notes       string       `json:"notes,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	SHA256      string       `json:"sha256,omitempty"`
}

// New creates a new evidence store at the given base path
func New(basePath string) (*Store, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("mkdir evidence base: %w", err)
	}
	return &Store{BasePath: basePath}, nil
}

// Path returns the directory path for a specific request's evidence
func (s *Store) Path(requestID string) string {
	return filepath.Join(s.BasePath, requestID)
}

// EnsureDir creates the evidence subdirectory for a request if it doesn't exist
func (s *Store) EnsureDir(requestID string) (string, error) {
	dir := s.Path(requestID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// Save stores evidence from a reader and returns metadata
func (s *Store) Save(requestID string, etype EvidenceType, filename string, content io.Reader, notes string) (*Evidence, error) {
	dir, err := s.EnsureDir(requestID)
	if err != nil {
		return nil, err
	}

	// Generate timestamped filename
	timestamp := time.Now().Format("20060102T150405")
	ext := filepath.Ext(filename)
	safeName := sanitizeName(requestID)
	safeFilename := fmt.Sprintf("%s_%s_%s%s", timestamp, etype, safeName, ext)
	filePath := filepath.Join(dir, safeFilename)

	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", filePath, err)
	}

	// Tee reader to compute hash while writing
	h := sha256.New()
	tee := io.TeeReader(content, h)

	_, err = io.Copy(f, tee)
	f.Close()
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("write %s: %w", filePath, err)
	}

	stat, _ := os.Stat(filePath)

	ev := &Evidence{
		ID:          fmt.Sprintf("EV-%s-%d", requestID, time.Now().UnixNano()%1000000),
		RequestID:   requestID,
		Type:        etype,
		Filename:    filename,
		ContentType: mimeType(ext),
		SizeBytes:   stat.Size(),
		Notes:       notes,
		CreatedAt:   time.Now(),
		SHA256:      fmt.Sprintf("%x", h.Sum(nil)),
	}

	// Write metadata JSON alongside file
	metaPath := filePath + ".meta.json"
	metaJSON, _ := json.MarshalIndent(ev, "", "  ")
	os.WriteFile(metaPath, metaJSON, 0600)

	return ev, nil
}

// SaveFile stores evidence from an existing file path
func (s *Store) SaveFile(requestID string, etype EvidenceType, srcPath, notes string) (*Evidence, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("open src file: %w", err)
	}
	defer f.Close()

	filename := filepath.Base(srcPath)
	return s.Save(requestID, etype, filename, f, notes)
}

// SaveEmailBounce stores an email bounce as evidence
func (s *Store) SaveEmailBounce(requestID, toEmail, bounceMessage, smtpOutput string) (*Evidence, error) {
	content := fmt.Sprintf("Bounce Report\nTime: %s\nTo: %s\n\nSMTP Output:\n%s\n\nBounce Message:\n%s",
		time.Now().Format(time.RFC1123Z), toEmail, smtpOutput, bounceMessage)
	return s.Save(requestID, TypeEmailBounce, "bounce.txt",
		io.NopCloser(strings.NewReader(content)), "SMTP bounce notification")
}

// SaveDeliveryReceipt stores an SMTP delivery receipt
func (s *Store) SaveDeliveryReceipt(requestID, msgID, toEmail string) (*Evidence, error) {
	content := fmt.Sprintf("Delivery Receipt\nTime: %s\nMessage-ID: %s\nTo: %s\nStatus: DELIVERED\n",
		time.Now().Format(time.RFC1123Z), msgID, toEmail)
	return s.Save(requestID, TypeDeliveryReceipt, "delivery_receipt.txt",
		io.NopCloser(strings.NewReader(content)),
		fmt.Sprintf("Message-ID: %s", msgID))
}

// List returns all evidence for a request
func (s *Store) List(requestID string) ([]Evidence, error) {
	dir := s.Path(requestID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var evs []Evidence
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta.json") {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var ev Evidence
			if json.Unmarshal(data, &ev) == nil {
				evs = append(evs, ev)
			}
		}
	}
	return evs, nil
}

// GetPath returns the full path to an evidence file
func (s *Store) GetPath(requestID, evidenceID string) (string, error) {
	evs, err := s.List(requestID)
	if err != nil {
		return "", err
	}
	for _, ev := range evs {
		if ev.ID == evidenceID {
			dir := s.Path(requestID)
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".meta.json") {
					data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
					var stored Evidence
					if json.Unmarshal(data, &stored) == nil && stored.ID == evidenceID {
						base := strings.TrimSuffix(e.Name(), ".meta.json")
						return filepath.Join(dir, base), nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("evidence %s not found", evidenceID)
}

// ListByType returns evidence filtered by type
func (s *Store) ListByType(requestID string, etype EvidenceType) ([]Evidence, error) {
	all, err := s.List(requestID)
	if err != nil {
		return nil, err
	}
	var filtered []Evidence
	for _, e := range all {
		if e.Type == etype {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func sanitizeName(s string) string {
	var result []rune
	for _, r := range s {
		if r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		}
	}
	return string(result)
}

func mimeType(ext string) string {
	mimeMap := map[string]string{
		".txt":  "text/plain",
		".html": "text/html",
		".eml":  "message/rfc822",
		".pdf":  "application/pdf",
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".json": "application/json",
		".msg":  "application/vnd.ms-outlook",
	}
	if mt, ok := mimeMap[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}
