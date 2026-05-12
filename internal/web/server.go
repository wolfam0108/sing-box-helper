// Package web exposes the singbox-helper HTTP API. v1.0-α: JSON only,
// no UI yet — the embed-assets and HTML come in v1.0-β.
package web

import (
	"net/http"
	"sync"
	"time"

	"github.com/wolfam0108/sing-box-helper/internal/config"
	"github.com/wolfam0108/sing-box-helper/internal/parser"
)

// Server holds the runtime state and configuration for the API.
type Server struct {
	Settings    config.Settings
	ConfigPath  string // path to /opt/etc/sing-box/config.json
	InitScript  string // path to /opt/etc/init.d/S99sing-box
	BackupDir   string // optional separate dir for backups; if empty, beside ConfigPath
	KeepBackups int    // 0 = keep all

	// lastApplied stores the most recently applied parsed node so /api/test
	// knows what server:port to probe. nil until first /api/apply.
	mu          sync.Mutex
	lastApplied *parser.ParsedNode
}

// New creates a Server with sensible defaults.
func New(s config.Settings) *Server {
	return &Server{
		Settings:    s,
		ConfigPath:  "/opt/etc/sing-box/config.json",
		InitScript:  "/opt/etc/init.d/S99sing-box",
		KeepBackups: 10,
	}
}

// Handler returns the http.Handler that wires all the API routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/preview", s.handlePreview)
	mux.HandleFunc("/api/apply", s.handleApply)
	mux.HandleFunc("/api/test", s.handleTest)
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

func (s *Server) getLastApplied() *parser.ParsedNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastApplied
}

func (s *Server) setLastApplied(n *parser.ParsedNode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastApplied = n
}
