package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestComponentDownloadsAuthenticatedAndVerified(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VETRA_DOWNLOAD_DIR", dir)
	body := []byte("controlled binary fixture")
	name := "vetra-agent-linux-amd64"
	os.WriteFile(filepath.Join(dir, name), body, 0700)
	os.WriteFile(filepath.Join(dir, "private.env"), []byte("SECRET"), 0600)
	db, _ := store.New(filepath.Join(t.TempDir(), "state.json"))
	s := New(db, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
	h := s.Handler()
	call := func(path, token string) *httptest.ResponseRecorder {
		q := httptest.NewRequest("GET", path, nil)
		q.Header.Set("X-API-Key", token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, q)
		return w
	}
	if w := call("/api/v1/downloads", ""); w.Code != 401 {
		t.Fatal("anonymous download")
	}
	w := call("/api/v1/downloads", s.AdminToken)
	var out struct {
		Components []componentDownload `json:"components"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(body)
	if len(out.Components) != 1 || out.Components[0].SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal(w.Body.String())
	}
	w = call(out.Components[0].URL, s.AdminToken)
	if w.Code != 200 || string(w.Body.Bytes()) != string(body) {
		t.Fatal("bytes changed")
	}
	if w = call("/api/v1/downloads/private.env", s.AdminToken); w.Code != 404 {
		t.Fatal("served arbitrary file")
	}
}
