package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var ErrRUMNotFound = errors.New("frontend não encontrado")
var rumID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,96}$`)
var rumHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

const rumMaxEvents = 15000
const rumReplayBytes = 25 << 20

func ValidateRUMApplication(a model.RUMApplication) error {
	if !rumID.MatchString(a.ID) || strings.TrimSpace(a.Name) == "" || len(a.Name) > 128 || len(a.Origins) < 1 || len(a.Origins) > 20 {
		return fmt.Errorf("nome, ID e origens válidas são obrigatórios")
	}
	if a.SampleRate < 0 || a.SampleRate > 1 || a.ReplaySampleRate < 0 || a.ReplaySampleRate > 1 || a.ApdexT < 100 || a.ApdexT > 60000 {
		return fmt.Errorf("amostragem deve estar entre 0 e 1; Apdex T entre 100 e 60000 ms")
	}
	if a.Masking != "all" && a.Masking != "allowlist" {
		return fmt.Errorf("mascaramento inválido")
	}
	for _, s := range a.Origins {
		u, e := url.Parse(s)
		if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return fmt.Errorf("use origens exatas, sem caminho ou wildcard")
		}
	}
	for _, list := range [][]string{a.PublicTextSelectors, a.BlockSelectors, a.ExcludePaths} {
		if len(list) > 20 {
			return fmt.Errorf("máximo de 20 regras por categoria")
		}
		for _, s := range list {
			if len(s) > 256 || strings.TrimSpace(s) == "" {
				return fmt.Errorf("regra inválida")
			}
		}
	}
	for _, s := range a.ExcludePaths {
		if !strings.HasPrefix(s, "/") {
			return fmt.Errorf("exclusões devem começar com /")
		}
	}
	return nil
}
func (s *Store) RUMApplications() []model.RUMApplication {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a := make([]model.RUMApplication, 0, len(s.data.RUMApplications))
	for _, v := range s.data.RUMApplications {
		a = append(a, cloneRUMApp(v))
	}
	sort.Slice(a, func(i, j int) bool { return a[i].Name < a[j].Name })
	return a
}
func cloneRUMApp(a model.RUMApplication) model.RUMApplication {
	a.Origins = append([]string{}, a.Origins...)
	a.PublicTextSelectors = append([]string{}, a.PublicTextSelectors...)
	a.BlockSelectors = append([]string{}, a.BlockSelectors...)
	a.ExcludePaths = append([]string{}, a.ExcludePaths...)
	return a
}
func (s *Store) RUMApplication(id string) (model.RUMApplication, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.data.RUMApplications[id]
	return cloneRUMApp(a), ok
}
func (s *Store) PutRUMApplication(a model.RUMApplication) error {
	if e := ValidateRUMApplication(a); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.RUMApplications) >= 50 {
		if _, ok := s.data.RUMApplications[a.ID]; !ok {
			return fmt.Errorf("limite de 50 frontends")
		}
	}
	old, exists := s.data.RUMApplications[a.ID]
	if s.data.RUMApplications == nil {
		s.data.RUMApplications = map[string]model.RUMApplication{}
	}
	s.data.RUMApplications[a.ID] = cloneRUMApp(a)
	oldKey := s.data.RUMIdentityKey
	if oldKey == "" {
		b := make([]byte, 32)
		if _, e := rand.Read(b); e != nil {
			return e
		}
		s.data.RUMIdentityKey = hex.EncodeToString(b)
	}
	if err := s.persist(); err != nil {
		if exists {
			s.data.RUMApplications[a.ID] = old
		} else {
			delete(s.data.RUMApplications, a.ID)
		}
		s.data.RUMIdentityKey = oldKey
		return err
	}
	return nil
}
func (s *Store) DeleteRUMApplication(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data.RUMApplications[id]
	if !ok {
		return ErrRUMNotFound
	}
	oldE, oldR := s.data.RUMEvents, s.data.RUMReplay
	delete(s.data.RUMApplications, id)
	s.data.RUMEvents = []model.RUMEvent{}
	s.data.RUMReplay = []model.RUMReplayChunk{}
	for _, v := range oldE {
		if v.AppID != id {
			s.data.RUMEvents = append(s.data.RUMEvents, v)
		}
	}
	for _, v := range oldR {
		if v.AppID != id {
			s.data.RUMReplay = append(s.data.RUMReplay, v)
		}
	}
	if err := s.persist(); err != nil {
		s.data.RUMApplications[id] = a
		s.data.RUMEvents = oldE
		s.data.RUMReplay = oldR
		return err
	}
	return nil
}
func RUMSafeURL(raw string, capturePath bool) string {
	u, e := url.Parse(raw)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	u.ForceQuery = false
	u.RawPath = ""
	if !capturePath {
		u.Path = "/"
	}
	return u.String()
}
func rumExcluded(a model.RUMApplication, raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return true
	}
	for _, p := range a.ExcludePaths {
		if strings.HasPrefix(u.Path, p) {
			return true
		}
	}
	return false
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 3600000 }
func (s *Store) IngestRUM(id string, events []model.RUMEvent, chunks []model.RUMReplayChunk) error {
	if len(events) > 100 || len(chunks) > 10 || len(events)+len(chunks) == 0 {
		return fmt.Errorf("lote RUM inválido")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data.RUMApplications[id]
	if !ok || !a.Enabled {
		return ErrRUMNotFound
	}
	now := time.Now().UTC()
	for i := range events {
		e := &events[i]
		if !rumID.MatchString(e.ID) || !rumID.MatchString(e.SessionID) || !rumID.MatchString(e.PageID) || e.At.Before(now.Add(-24*time.Hour)) || e.At.After(now.Add(5*time.Minute)) || !finite(e.Duration) || !finite(e.Value) || e.Status < 0 || e.Status > 599 {
			return fmt.Errorf("evento RUM inválido")
		}
		if e.Kind != "navigation" && e.Kind != "page_view" && e.Kind != "request" && e.Kind != "error" && e.Kind != "action" && e.Kind != "vital" {
			return fmt.Errorf("tipo de evento inválido")
		}
		if e.Kind == "vital" && e.Name != "LCP" && e.Name != "INP" && e.Name != "CLS" && e.Name != "FCP" && e.Name != "TTFB" {
			return fmt.Errorf("vital inválido")
		}
		if len(e.Name) > 80 || len(e.URL) > 2048 || len(e.Browser) > 30 || len(e.OS) > 30 || len(e.Device) > 20 {
			return fmt.Errorf("evento excessivo")
		}
		if rumExcluded(a, e.URL) {
			return fmt.Errorf("página excluída")
		}
		e.AppID = id
		e.URL = RUMSafeURL(e.URL, a.CapturePath)
		if e.URL == "" {
			return fmt.Errorf("URL inválida")
		}
		if e.Kind != "vital" && e.Kind != "error" {
			e.Name = e.Kind
		}
		if e.Kind == "error" {
			switch e.Name {
			case "TypeError", "RangeError", "ReferenceError", "SyntaxError", "URIError", "NetworkError", "ResourceError":
			default:
				e.Name = "Error"
			}
		}
		if !a.IdentifyUsers || !rumHash.MatchString(e.UserID) {
			e.UserID = ""
		} else {
			mac := hmac.New(sha256.New, []byte(s.data.RUMIdentityKey))
			mac.Write([]byte(id + ":" + e.UserID))
			e.UserID = hex.EncodeToString(mac.Sum(nil))[:24]
		}
	}
	if len(chunks) > 0 && !a.ReplayEnabled {
		return fmt.Errorf("replay desativado")
	}
	for i := range chunks {
		c := &chunks[i]
		if !rumID.MatchString(c.ID) || !rumID.MatchString(c.SessionID) || !rumID.MatchString(c.PageID) || len(c.Events) == 0 || len(c.Events) > 200 || c.At.Before(now.Add(-24*time.Hour)) || c.At.After(now.Add(5*time.Minute)) {
			return fmt.Errorf("chunk inválido")
		}
		if rumExcluded(a, c.URL) {
			return fmt.Errorf("página excluída")
		}
		c.URL = RUMSafeURL(c.URL, a.CapturePath)
		if c.URL == "" {
			return fmt.Errorf("URL inválida")
		}
		c.AppID = id
		safe, err := SanitizeReplay(c.Events, a)
		if err != nil {
			return err
		}
		c.Events = safe
	}
	oldE, oldR := s.data.RUMEvents, s.data.RUMReplay
	// Copy-on-write keeps failed persistence and readers isolated. Event IDs upsert final vitals.
	em := map[string]model.RUMEvent{}
	for _, e := range oldE {
		if e.At.After(now.Add(-24 * time.Hour)) {
			em[rumEventKey(e)] = e
		}
	}
	for _, e := range events {
		em[rumEventKey(e)] = e
	}
	next := make([]model.RUMEvent, 0, len(em))
	for _, e := range em {
		next = append(next, e)
	}
	sort.Slice(next, func(i, j int) bool { return next[i].At.Before(next[j].At) })
	if len(next) > rumMaxEvents {
		next = next[len(next)-rumMaxEvents:]
	}
	rm := map[string]model.RUMReplayChunk{}
	for _, c := range oldR {
		if c.At.After(now.Add(-24 * time.Hour)) {
			rm[c.AppID+":"+c.ID] = c
		}
	}
	for _, c := range chunks {
		rm[id+":"+c.ID] = c
	}
	replays := make([]model.RUMReplayChunk, 0, len(rm))
	for _, c := range rm {
		replays = append(replays, c)
	}
	sort.Slice(replays, func(i, j int) bool { return replays[i].At.Before(replays[j].At) })
	size := 0
	start := len(replays)
	for start > 0 {
		b, _ := json.Marshal(replays[start-1])
		if size+len(b) > rumReplayBytes {
			break
		}
		size += len(b)
		start--
	}
	replays = replays[start:]
	if len(replays) > 2000 {
		replays = replays[len(replays)-2000:]
	}
	s.data.RUMEvents = next
	s.data.RUMReplay = replays
	if err := s.persist(); err != nil {
		s.data.RUMEvents = oldE
		s.data.RUMReplay = oldR
		return err
	}
	return nil
}
func (s *Store) RUMEvents(id string, since time.Time) []model.RUMEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.RUMEvent{}
	for _, e := range s.data.RUMEvents {
		if (id == "" || e.AppID == id) && !e.At.Before(since) {
			a := s.data.RUMApplications[e.AppID]
			e.URL = RUMSafeURL(e.URL, a.CapturePath)
			if !a.IdentifyUsers {
				e.UserID = ""
			}
			out = append(out, e)
		}
	}
	return out
}
func (s *Store) RUMReplay(id, session string) []model.RUMReplayChunk {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []model.RUMReplayChunk{}
	for _, c := range s.data.RUMReplay {
		if c.AppID == id && c.SessionID == session && c.At.After(time.Now().Add(-24*time.Hour)) {
			b, _ := json.Marshal(c)
			var v model.RUMReplayChunk
			_ = json.Unmarshal(b, &v)
			if a, ok := s.data.RUMApplications[id]; ok && a.Masking == "all" {
				if safe, err := SanitizeReplay(v.Events, a); err == nil {
					v.Events = safe
				} else {
					continue
				}
			}
			out = append(out, v)
		}
	}
	return out
}
func (s *Store) RUMPurgeSession(id, session string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	oe, or := s.data.RUMEvents, s.data.RUMReplay
	s.data.RUMEvents = []model.RUMEvent{}
	s.data.RUMReplay = []model.RUMReplayChunk{}
	for _, e := range oe {
		if e.AppID != id || e.SessionID != session {
			s.data.RUMEvents = append(s.data.RUMEvents, e)
		}
	}
	for _, c := range or {
		if c.AppID != id || c.SessionID != session {
			s.data.RUMReplay = append(s.data.RUMReplay, c)
		}
	}
	if err := s.persist(); err != nil {
		s.data.RUMEvents = oe
		s.data.RUMReplay = or
		return err
	}
	return nil
}

func rumEventKey(e model.RUMEvent) string {
	if e.Kind == "vital" {
		return e.AppID + ":" + e.SessionID + ":" + e.PageID + ":" + e.Name
	}
	return e.AppID + ":" + e.ID
}
