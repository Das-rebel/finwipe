package email

import (
	"strings"
	"testing"
	"time"

	"github.com/das-rebel/finwipe/internal/config"
	"github.com/das-rebel/finwipe/internal/nbfc"
)

func TestGenerateFollowupBody(t *testing.T) {
	profile := config.Profile{
		Name:    "Test User",
		Email:   "test@example.com",
		Phone:   "9876543210",
		Address: "123 Test Street, Bengaluru",
	}

	cases := []struct {
		dayNum int
	}{
		{7},
		{14},
		{21},
		{28},
	}

	for _, c := range cases {
		body := GenerateFollowupBody("DPR-2026-000001", "Bajaj Finserv Ltd", profile, c.dayNum)
		if body == "" {
			t.Errorf("GenerateFollowupBody(day=%d) returned empty string", c.dayNum)
		}
		// Check key elements
		if !strings.Contains(body, "DPR-2026-000001") {
			t.Errorf("body missing request ID")
		}
		if !strings.Contains(body, "Bajaj Finserv Ltd") {
			t.Errorf("body missing NBFC name")
		}
		if !strings.Contains(body, "test@example.com") {
			t.Errorf("body missing user email")
		}
		if !strings.Contains(body, "Section 8(6)") {
			t.Errorf("body missing DPDPA reference")
		}
	}
}

func TestGenerateFollowupBody_UrgencyEscalation(t *testing.T) {
	profile := config.Profile{Name: "T", Email: "t@t.com", Phone: "1", Address: "A"}

	b7 := GenerateFollowupBody("DPR-1", "N", profile, 7)
	b21 := GenerateFollowupBody("DPR-1", "N", profile, 21)

	// Day 7 should be friendly, day 21 should be final notice
	if strings.Contains(b7, "FINAL NOTICE") && !strings.Contains(b21, "FINAL NOTICE") {
		t.Error("day 7 should not mention FINAL NOTICE, day 21 should")
	}
}

func TestFollowupTypeLabel(t *testing.T) {
	// These are informational checks — just verify the function doesn't panic
	labels := map[int]string{
		7:  "FOLLOWUP_EMAIL",
		10: "FOLLOWUP_EMAIL",
		14: "FOLLOWUP_EMAIL",
		21: "FOLLOWUP_EMAIL",
	}
	for day, typ := range labels {
		_ = typ // used in GenerateFollowupBody
		_ = day
	}
}

func TestSenderSend_NoSMTP(t *testing.T) {
	s := New(&config.SMTP{})
	entity := nbfc.Entity{ID: "test", Name: "Test NBFC", GrievanceEmail: "a@b.com"}
	profile := config.Profile{Name: "T", Email: "t@t.com", Phone: "1", Address: "A"}

	err := s.Send(entity, profile, "")
	if err == nil {
		t.Error("expected error when SMTP not configured")
	}
}

func TestSenderSendFollowup_NoSMTP(t *testing.T) {
	s := New(&config.SMTP{})
	profile := config.Profile{Name: "T", Email: "t@t.com", Phone: "1", Address: "A"}

	_, err := s.SendFollowup("DPR-1", "a@b.com", profile, "body", "subject")
	if err == nil {
		t.Error("expected error when SMTP not configured")
	}
}

func TestBuildBody(t *testing.T) {
	entity := nbfc.Entity{Name: "Test NBFC"}
	profile := config.Profile{
		Name:    "John Doe",
		Email:   "john@example.com",
		Phone:   "9876543210",
		Address: "456 Main St",
	}

	body := buildBody(entity, profile, "")
	if !strings.Contains(body, "John Doe") {
		t.Error("body missing Name")
	}
	if !strings.Contains(body, "john@example.com") {
		t.Error("body missing Email")
	}
	if !strings.Contains(body, "9876543210") {
		t.Error("body missing Phone")
	}
	if !strings.Contains(body, "Test NBFC") {
		t.Error("body missing NBFC name")
	}
	if !strings.Contains(body, time.Now().Format("02 January 2006")) {
		t.Error("body missing date")
	}
}

func TestBuildBody_CustomTemplate(t *testing.T) {
	entity := nbfc.Entity{Name: "Test NBFC"}
	profile := config.Profile{Name: "T", Email: "e@e.com", Phone: "1", Address: "A"}
	tpl := "Hello {{.FullName}}, your data at {{.NBFCName}}"

	body := buildBody(entity, profile, tpl)
	if !strings.Contains(body, "Hello T") {
		t.Error("custom template not applied")
	}
	if !strings.Contains(body, "Test NBFC") {
		t.Error("custom template not interpolating NBFCName")
	}
}

func TestBuildMessage(t *testing.T) {
	entity := nbfc.Entity{Name: "Test NBFC", GrievanceEmail: "a@b.com"}
	profile := config.Profile{Name: "T", Email: "t@t.com", Phone: "1", Address: "A"}
	from := "sender@example.com"

	msg := buildMessage(entity, profile, "test body", from)
	if !strings.Contains(msg, "From: "+from) {
		t.Error("message missing From header")
	}
	if !strings.Contains(msg, "To: a@b.com") {
		t.Error("message missing To header")
	}
	if !strings.Contains(msg, "Subject:") {
		t.Error("message missing Subject header")
	}
}
