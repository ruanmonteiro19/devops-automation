package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"regexp"
	"strings"
	"time"
)

type User struct {
	Username           string `json:"username"`
	Name               string `json:"name"`
	Role               string `json:"role"`
	Disabled           bool   `json:"disabled"`
	MustChangePassword bool   `json:"must_change_password"`
	PasswordHash       string `json:"-"`
}
type storedUser struct {
	User
	Hash string `json:"password_hash"`
}
type LoginSession struct {
	User    string    `json:"user"`
	CSRF    string    `json:"csrf"`
	Expires time.Time `json:"expires"`
}

var userName = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,64}$`)

func secret() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s *Store) BootstrapUsers() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Users) > 0 {
		return nil
	}
	hash, e := bcrypt.GenerateFromPassword([]byte("12345"), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	s.data.Users = map[string]storedUser{"admin": {User: User{Username: "admin", Name: "Administrador", Role: "admin", MustChangePassword: true}, Hash: string(hash)}}
	if e = s.persist(); e != nil {
		s.data.Users = nil
	}
	return e
}
func (s *Store) Users() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []User{}
	for _, u := range s.data.Users {
		out = append(out, u.User)
	}
	return out
}
func (s *Store) PutUser(u User, password string) error {
	u.Username = strings.ToLower(strings.TrimSpace(u.Username))
	if !userName.MatchString(u.Username) || len(u.Name) > 128 || (u.Role != "admin" && u.Role != "viewer") {
		return fmt.Errorf("usuário ou perfil inválido")
	}
	var hash string
	if password != "" {
		if len(password) < 8 || len(password) > 72 {
			return fmt.Errorf("senha deve ter 8..72 bytes")
		}
		h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if e != nil {
			return e
		}
		hash = string(h)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.data.Users[u.Username]
	if !exists && len(s.data.Users) >= 200 {
		return fmt.Errorf("limite de 200 usuários")
	}
	if !exists && hash == "" {
		return fmt.Errorf("informe a senha inicial")
	}
	if exists && old.Role == "admin" && !old.Disabled && (u.Role != "admin" || u.Disabled) {
		n := 0
		for name, x := range s.data.Users {
			if name != u.Username && x.Role == "admin" && !x.Disabled {
				n++
			}
		}
		if n == 0 {
			return fmt.Errorf("mantenha um administrador ativo")
		}
	}
	if hash == "" {
		hash = old.Hash
		u.MustChangePassword = old.MustChangePassword
	} else {
		u.MustChangePassword = true
	}
	if s.data.Users == nil {
		s.data.Users = map[string]storedUser{}
	}
	s.data.Users[u.Username] = storedUser{User: u, Hash: hash}
	if e := s.persist(); e != nil {
		if exists {
			s.data.Users[u.Username] = old
		} else {
			delete(s.data.Users, u.Username)
		}
		return e
	}
	return nil
}
func (s *Store) Login(username, password string) (string, User, string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	s.mu.RLock()
	u, ok := s.data.Users[username]
	s.mu.RUnlock()
	// Always perform bcrypt, including unknown accounts.
	hash := u.Hash
	if !ok {
		hash = "$2a$10$7EqJtq98hPqEX7fNZaFWoO5uLVl9KgC5GmQWqj8XLoMZ0HnI8fH8W"
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || !ok || u.Disabled {
		return "", User{}, "", fmt.Errorf("usuário ou senha inválidos")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.data.Users[username]
	if !ok || current.Disabled || current.Hash != u.Hash {
		return "", User{}, "", fmt.Errorf("conta alterada; tente novamente")
	}
	old := s.data.Sessions
	next := map[string]LoginSession{}
	for k, v := range old {
		if time.Now().Before(v.Expires) {
			next[k] = v
		}
	}
	if len(next) >= 1000 {
		return "", User{}, "", fmt.Errorf("limite de sessões atingido")
	}
	token, csrf := secret(), secret()
	next[sessionKey(token)] = LoginSession{User: username, CSRF: csrf, Expires: time.Now().Add(8 * time.Hour)}
	s.data.Sessions = next
	if e := s.persist(); e != nil {
		s.data.Sessions = old
		return "", User{}, "", e
	}
	return token, current.User, csrf, nil
}
func (s *Store) Session(token string) (User, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.data.Sessions[sessionKey(token)]
	if !ok || !time.Now().Before(session.Expires) {
		return User{}, "", false
	}
	u, ok := s.data.Users[session.User]
	return u.User, session.CSRF, ok && !u.Disabled
}
func (s *Store) Logout(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionKey(token)
	old, ok := s.data.Sessions[key]
	delete(s.data.Sessions, key)
	if e := s.persist(); e != nil {
		if ok {
			s.data.Sessions[key] = old
		}
		return e
	}
	return nil
}
func (s *Store) ChangePassword(username, current, next string) error {
	if len(next) < 8 || len(next) > 72 {
		return fmt.Errorf("nova senha deve ter 8..72 bytes")
	}
	s.mu.RLock()
	u, ok := s.data.Users[username]
	s.mu.RUnlock()
	if !ok || bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(current)) != nil {
		return fmt.Errorf("senha atual incorreta")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.data.Users[username]
	if old.Hash != u.Hash {
		return fmt.Errorf("conta alterada")
	}
	sessions := s.data.Sessions
	remaining := map[string]LoginSession{}
	for k, v := range sessions {
		if v.User != username {
			remaining[k] = v
		}
	}
	u.Hash = string(h)
	u.MustChangePassword = false
	s.data.Users[username] = u
	s.data.Sessions = remaining
	if e = s.persist(); e != nil {
		s.data.Users[username] = old
		s.data.Sessions = sessions
	}
	return e
}
