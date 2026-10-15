package server

import (
	"crypto/subtle"
	"github.com/ruanmonteiro19/observa-lite/internal/store"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

const sessionCookie = "vetra_session"

type attempt struct {
	count int
	until time.Time
}

var loginAttempts = struct {
	sync.Mutex
	entries map[string]attempt
}{entries: map[string]attempt{}}

func limitedLogin(key string) bool {
	loginAttempts.Lock()
	defer loginAttempts.Unlock()
	now := time.Now()
	for k, v := range loginAttempts.entries {
		if now.After(v.until) {
			delete(loginAttempts.entries, k)
		}
	}
	a := loginAttempts.entries[key]
	if a.until.IsZero() {
		a.until = now.Add(time.Minute)
	}
	a.count++
	if len(loginAttempts.entries) < 2000 || a.count > 1 {
		loginAttempts.entries[key] = a
	}
	return a.count > 10 || len(loginAttempts.entries) >= 2000
}
func (s *Server) browserSession(r *http.Request) (store.User, string, bool) {
	c, e := r.Cookie(sessionCookie)
	if e != nil {
		return store.User{}, "", false
	}
	return s.Store.Session(c.Value)
}
func validCSRF(r *http.Request, csrf string) bool {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrf)) != 1 {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Host != r.Host {
			return false
		}
	}
	return true
}
func (s *Server) authorizeUser(w http.ResponseWriter, r *http.Request, adminOnly bool) bool {
	if s.checkToken(r, s.AdminToken) {
		return true
	}
	u, csrf, ok := s.browserSession(r)
	if !ok {
		http.Error(w, "unauthorized", 401)
		return false
	}
	if u.MustChangePassword {
		http.Error(w, "altere a senha inicial", 403)
		return false
	}
	if adminOnly && u.Role != "admin" {
		http.Error(w, "administrador necessário", 403)
		return false
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if u.Role != "admin" && !(r.Method == "POST" && r.URL.Path == "/api/v1/vql") {
			http.Error(w, "perfil de leitura", 403)
			return false
		}
		if !validCSRF(r, csrf) {
			http.Error(w, "CSRF inválido", 403)
			return false
		}
	}
	return true
}
func (s *Server) userRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || u.Host != r.Host {
				http.Error(w, "origin inválida", 403)
				return
			}
		}
		if limitedLogin(r.RemoteAddr[:remoteHostEnd(r.RemoteAddr)]) {
			http.Error(w, "aguarde um minuto antes de tentar novamente", 429)
			return
		}
		var p struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decode(w, r, &p) {
			return
		}
		if len(p.Password) > 72 || len(p.Username) > 64 {
			http.Error(w, "credenciais inválidas", 401)
			return
		}
		token, u, csrf, e := s.Store.Login(p.Username, p.Password)
		if e != nil {
			http.Error(w, "usuário ou senha inválidos", 401)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil || os.Getenv("VETRA_COOKIE_SECURE") == "true", SameSite: http.SameSiteStrictMode, MaxAge: 28800})
		write(w, 200, map[string]any{"user": u, "csrf": csrf})
	})
	m.HandleFunc("GET /api/v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		if s.checkToken(r, s.AdminToken) {
			write(w, 200, map[string]any{"user": store.User{Username: "api-token", Role: "admin"}, "csrf": ""})
			return
		}
		u, csrf, ok := s.browserSession(r)
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		write(w, 200, map[string]any{"user": u, "csrf": csrf})
	})
	m.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(sessionCookie)
		if e == nil {
			_, csrf, ok := s.browserSession(r)
			if !ok || !validCSRF(r, csrf) {
				http.Error(w, "CSRF inválido", 403)
				return
			}
			if e = s.Store.Logout(c.Value); e != nil {
				fail(w, 500, e)
				return
			}
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
		write(w, 200, map[string]bool{"logged_out": true})
	})
	m.HandleFunc("POST /api/v1/auth/password", func(w http.ResponseWriter, r *http.Request) {
		u, csrf, ok := s.browserSession(r)
		if !ok || !validCSRF(r, csrf) {
			http.Error(w, "unauthorized", 401)
			return
		}
		var p struct {
			Current string `json:"current_password"`
			Next    string `json:"new_password"`
		}
		if !decode(w, r, &p) {
			return
		}
		if e := s.Store.ChangePassword(u.Username, p.Current, p.Next); e != nil {
			fail(w, 400, e)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
		write(w, 200, map[string]bool{"changed": true})
	})
	m.HandleFunc("GET /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorizeUser(w, r, true) {
			return
		}
		write(w, 200, map[string]any{"users": s.Store.Users()})
	})
	m.HandleFunc("POST /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorizeUser(w, r, true) {
			return
		}
		var p struct {
			Username string `json:"username"`
			Name     string `json:"name"`
			Role     string `json:"role"`
			Disabled bool   `json:"disabled"`
			Password string `json:"password"`
		}
		if !decode(w, r, &p) {
			return
		}
		if e := s.Store.PutUser(store.User{Username: p.Username, Name: p.Name, Role: p.Role, Disabled: p.Disabled}, p.Password); e != nil {
			fail(w, 400, e)
			return
		}
		write(w, 200, map[string]bool{"saved": true})
	})
}
func remoteHostEnd(address string) int {
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] == ':' {
			return i
		}
	}
	return len(address)
}
