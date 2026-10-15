package model

import "time"

type RUMApplication struct {
	ConsentBanner       bool     `json:"consent_banner"`
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Origins             []string `json:"origins"`
	Enabled             bool     `json:"enabled"`
	RequireConsent      bool     `json:"require_consent"`
	ReplayEnabled       bool     `json:"replay_enabled"`
	SampleRate          float64  `json:"sample_rate"`
	ReplaySampleRate    float64  `json:"replay_sample_rate"`
	Masking             string   `json:"masking"`
	PublicTextSelectors []string `json:"public_text_selectors"`
	BlockSelectors      []string `json:"block_selectors"`
	ExcludePaths        []string `json:"exclude_paths"`
	CapturePath         bool     `json:"capture_path"`
	IdentifyUsers       bool     `json:"identify_users"`
	ApdexT              float64  `json:"apdex_t_ms"`
}

type RUMEvent struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	SessionID string    `json:"session_id"`
	PageID    string    `json:"page_id"`
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Duration  float64   `json:"duration_ms"`
	Value     float64   `json:"value"`
	Status    int       `json:"status"`
	Error     bool      `json:"error"`
	Browser   string    `json:"browser"`
	OS        string    `json:"os"`
	Device    string    `json:"device"`
	UserID    string    `json:"user_id,omitempty"`
}

// Replay contains already-masked rrweb events, with active content removed.
type RUMReplayChunk struct {
	URL       string           `json:"url"`
	ID        string           `json:"id"`
	AppID     string           `json:"app_id"`
	SessionID string           `json:"session_id"`
	PageID    string           `json:"page_id"`
	At        time.Time        `json:"at"`
	Events    []map[string]any `json:"events"`
}
