// Package web exposes the singbox-helper HTTP API. v1.0-α: JSON only,
// no UI yet — the embed-assets and HTML come in v1.0-β.
package web

import (
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/wolfram0108/sing-box-helper/internal/config"
	"github.com/wolfram0108/sing-box-helper/internal/logbuf"
	"github.com/wolfram0108/sing-box-helper/internal/state"
)

// Server holds the runtime state and configuration for the API.
//
// The "what's currently running" question is answered by reading
// /opt/etc/sing-box/config.json on every /api/status — that's the
// source of truth. state.json on disk carries only metadata we added
// (the original URI string, the friendly label, when it was applied)
// which sing-box itself doesn't track.
//
// Settings are protected by mu because POST /api/settings can mutate them
// concurrently with other requests reading them.
type Server struct {
	mu           sync.RWMutex
	settings     config.Settings
	ConfigPath   string         // path to /opt/etc/sing-box/config.json
	InitScript   string         // path to /opt/etc/init.d/S99sing-box
	BackupDir    string         // optional separate dir for backups; if empty, beside ConfigPath
	StatePath    string         // path to state.json (URI + label + applied_at)
	SettingsPath string         // path to /opt/etc/singbox-helper/config.yaml
	KeepBackups  int            // 0 = keep all
	Logs         *logbuf.Buffer // helper-process log ring; nil → no /api/logs?source=helper
}

// New creates a Server with sensible defaults.
func New(s config.Settings) *Server {
	return &Server{
		settings:     s,
		ConfigPath:   "/opt/etc/sing-box/config.json",
		InitScript:   "/opt/etc/init.d/S99sing-box",
		StatePath:    "/opt/etc/singbox-helper/state.json",
		SettingsPath: "/opt/etc/singbox-helper/config.yaml",
		KeepBackups:  10,
		Logs:         logbuf.New(500),
	}
}

// Settings returns a snapshot copy of current settings. Cheap — Settings is
// a flat struct of primitives.
func (s *Server) Settings() config.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// setSettings replaces current settings under the write lock.
func (s *Server) setSettings(v config.Settings) {
	s.mu.Lock()
	s.settings = v
	s.mu.Unlock()
}

// readStateFromDisk returns the persisted metadata or nil if absent /
// corrupt. Errors are intentionally swallowed — UI only loses extra labels.
func (s *Server) readStateFromDisk() *state.State {
	st, err := state.Load(s.StatePath)
	if err != nil {
		return nil
	}
	return st
}

// saveStateToDisk persists the latest applied URI + label + timestamp.
// Called from /api/apply on success.
func (s *Server) saveStateToDisk(uri, label string, at time.Time) error {
	return state.Save(s.StatePath, &state.State{
		URI:       uri,
		Label:     label,
		AppliedAt: at,
	})
}

// Handler returns the http.Handler that wires API routes and serves the
// embedded UI assets from "/".
//
// Route precedence: ServeMux uses longest-prefix match, so /api/* hits
// the API handlers first and "/" falls through to the static file server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/preview", s.handlePreview)
	mux.HandleFunc("/api/apply", s.handleApply)
	mux.HandleFunc("/api/test", s.handleTest)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/backups", s.handleBackups)
	mux.HandleFunc("/api/backups/restore", s.handleBackupsRestore)

	// Strip the "assets/" prefix so URLs look like /style.css, /app.js.
	sub, err := fs.Sub(assetsFS, "assets")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}
	return mux
}

// ListenAndServe is a thin wrapper around http.Server with sensible
// timeouts for the slow operations (curl probes can take up to ~15s).
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv.ListenAndServe()
}

