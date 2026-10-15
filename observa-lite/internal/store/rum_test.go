package store

import (
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testRUMApp() model.RUMApplication {
	return model.RUMApplication{ID: "shop", Name: "Shop", Origins: []string{"https://shop.test"}, Enabled: true, RequireConsent: true, ReplayEnabled: true, SampleRate: 1, ReplaySampleRate: 1, Masking: "all", ApdexT: 1000}
}
func testRUMEvent(id string) model.RUMEvent {
	return model.RUMEvent{ID: id, SessionID: "session", PageID: "page", At: time.Now().UTC(), Kind: "page_view", Name: "Someone@example.test", URL: "https://user:password@shop.test/orders?token=secret#private", Duration: 800}
}
func TestRUMPrivacyPersistenceUpsertAndPurge(t *testing.T) {
	file := filepath.Join(t.TempDir(), "db.json")
	s, _ := New(file)
	app := testRUMApp()
	app.IdentifyUsers = true
	if err := s.PutRUMApplication(app); err != nil {
		t.Fatal(err)
	}
	e := testRUMEvent("event")
	e.UserID = strings.Repeat("a", 64)
	if err := s.IngestRUM(app.ID, []model.RUMEvent{e}, nil); err != nil {
		t.Fatal(err)
	}
	v := e
	v.Kind = "vital"
	v.Name = "LCP"
	v.ID = "metric1"
	v.Value = 4000
	_ = s.IngestRUM(app.ID, []model.RUMEvent{v}, nil)
	v.ID = "metric2"
	v.Value = 2500
	_ = s.IngestRUM(app.ID, []model.RUMEvent{v}, nil)
	s, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	events := s.RUMEvents(app.ID, time.Now().Add(-time.Hour))
	if len(events) != 2 {
		t.Fatalf("vital duplicated: %d", len(events))
	}
	for _, v := range events {
		if v.URL != "https://shop.test/" || v.UserID == e.UserID || len(v.UserID) != 24 {
			t.Fatalf("privacy leak: %+v", v)
		}
		if v.Kind == "vital" && v.Value != 2500 {
			t.Fatal("final value not retained")
		}
	}
	if err := s.RUMPurgeSession(app.ID, "session"); err != nil {
		t.Fatal(err)
	}
	if len(s.RUMEvents(app.ID, time.Time{})) != 0 {
		t.Fatal("purge failed")
	}
}
func TestReplaySanitizationStripsActiveContentAndPrivateData(t *testing.T) {
	app := testRUMApp()
	raw := `[{"type":2,"timestamp":1,"data":{"node":{"type":0,"id":1,"childNodes":[{"type":2,"id":2,"tagName":"input","attributes":{"value":"password-secret","onfocus":"alert(1)","src":"https://evil.test","style":"width:200px;background:url(https://secret.test);color:red"},"childNodes":[]},{"type":2,"id":3,"tagName":"script","attributes":{},"childNodes":[{"type":3,"id":4,"textContent":"private-script"}]},{"type":3,"id":5,"textContent":"private-email@example.test"}]}}},{"type":3,"timestamp":2,"data":{"source":5,"id":2,"text":"card-number-secret","isChecked":true}}]`
	var events []map[string]any
	_ = json.Unmarshal([]byte(raw), &events)
	safe, err := SanitizeReplay(events, app)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(safe)
	s := string(b)
	for _, secret := range []string{"password-secret", "alert(1)", "evil.test", "secret.test", "private-script", "private-email", "card-number-secret"} {
		if strings.Contains(s, secret) {
			t.Fatalf("leaked %s: %s", secret, s)
		}
	}
	if !strings.Contains(s, "width:200px") || !strings.Contains(s, "***") {
		t.Fatal("safe layout or masking missing")
	}
}
func TestRUMFailureDoesNotCommitAndInvalidInputsAreRejected(t *testing.T) {
	s, _ := New(filepath.Join(t.TempDir(), "db.json"))
	a := testRUMApp()
	_ = s.PutRUMApplication(a)
	e := testRUMEvent("test")
	e.Kind = "made-up"
	if s.IngestRUM(a.ID, []model.RUMEvent{e}, nil) == nil {
		t.Fatal("unknown event accepted")
	}
	e = testRUMEvent("test")
	e.At = time.Now().Add(-48 * time.Hour)
	if s.IngestRUM(a.ID, []model.RUMEvent{e}, nil) == nil {
		t.Fatal("expired event accepted")
	}
	e = testRUMEvent("test")
	s.file = "/dev/null/db.json"
	if s.IngestRUM(a.ID, []model.RUMEvent{e}, nil) == nil {
		t.Fatal("expected persistence failure")
	}
	if len(s.RUMEvents(a.ID, time.Time{})) != 0 {
		t.Fatal("failed write changed state")
	}
}
