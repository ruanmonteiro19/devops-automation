package server

import (
	"bytes"
	"encoding/json"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserLoginPasswordCSRFReadOnlyAndPersistence(t *testing.T) {
	db, e := store.NewSQL(filepath.Join(t.TempDir(), "state.json"), "sqlite", "")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.BootstrapUsers(); e != nil {
		t.Fatal(e)
	}
	s := New(db, "admin-token-12345678901234567890", "agent-token-12345678901234567890", false)
	handler := s.Handler()
	call := func(method, path string, p any, cookie *http.Cookie, csrf, token string) *httptest.ResponseRecorder {
		b, _ := json.Marshal(p)
		q := httptest.NewRequest(method, path, bytes.NewReader(b))
		q.RemoteAddr = "192.0.2.51:1000"
		q.Header.Set("X-CSRF-Token", csrf)
		q.Header.Set("X-API-Key", token)
		if cookie != nil {
			q.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, q)
		return w
	}
	login := func(user, pass string) (*http.Cookie, string, bool) {
		w := call("POST", "/api/v1/auth/login", map[string]string{"username": user, "password": pass}, nil, "", "")
		if w.Code != 200 {
			t.Fatalf("login: %d %s", w.Code, w.Body.String())
		}
		var out struct {
			User store.User `json:"user"`
			CSRF string     `json:"csrf"`
		}
		json.Unmarshal(w.Body.Bytes(), &out)
		cookie := w.Result().Cookies()[0]
		if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatal("session cookie settings")
		}
		return cookie, out.CSRF, out.User.MustChangePassword
	}
	cookie, csrf, must := login("admin", "12345")
	if !must {
		t.Fatal("bootstrap password wasn't temporary")
	}
	if w := call("GET", "/api/v1/snapshot", nil, cookie, "", ""); w.Code != 403 {
		t.Fatal("bootstrap user accessed data")
	}
	if w := call("POST", "/api/v1/auth/password", map[string]string{"current_password": "12345", "new_password": "New-password-123"}, cookie, "", ""); w.Code != 401 {
		t.Fatal("missing CSRF accepted")
	}
	w := call("POST", "/api/v1/auth/password", map[string]string{"current_password": "12345", "new_password": "New-password-123"}, cookie, csrf, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, _, ok := db.Session(cookie.Value); ok {
		t.Fatal("password change didn't revoke old session")
	}
	cookie, csrf, must = login("admin", "New-password-123")
	if must {
		t.Fatal("password change not persisted")
	}
	w = call("POST", "/api/v1/users", map[string]any{"username": "reader", "name": "Reader", "role": "viewer", "password": "Reader-pass-123"}, cookie, csrf, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("GET", "/api/v1/users", nil, cookie, "", "")
	if strings.Contains(w.Body.String(), "password_hash") || strings.Contains(w.Body.String(), "$2") {
		t.Fatal("hash leaked")
	}
	reader, rcsrf, _ := login("reader", "Reader-pass-123")
	w = call("POST", "/api/v1/auth/password", map[string]string{"current_password": "Reader-pass-123", "new_password": "Reader-personal-123"}, reader, rcsrf, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	reader, rcsrf, _ = login("reader", "Reader-personal-123")
	if w = call("GET", "/api/v1/snapshot", nil, reader, "", ""); w.Code != 200 {
		t.Fatal("reader cannot read", w.Code)
	}
	if w = call("POST", "/api/v1/workspace", nil, reader, rcsrf, ""); w.Code != 403 {
		t.Fatal("reader can write")
	}
	if w = call("POST", "/api/v1/vql", map[string]string{"query": "fetch hosts"}, reader, rcsrf, ""); w.Code != 200 {
		t.Fatal("reader cannot execute read-only query", w.Code, w.Body.String())
	}
	if w = call("GET", "/api/v1/users", nil, reader, "", ""); w.Code != 403 {
		t.Fatal("reader can list accounts")
	}
	if w = call("GET", "/api/v1/snapshot", nil, nil, "", s.AdminToken); w.Code != 200 {
		t.Fatal("legacy admin token failed")
	}
}
