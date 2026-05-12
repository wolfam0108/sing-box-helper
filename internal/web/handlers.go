package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wolfram0108/sing-box-helper/internal/backup"
	"github.com/wolfram0108/sing-box-helper/internal/config"
	"github.com/wolfram0108/sing-box-helper/internal/parser"
	"github.com/wolfram0108/sing-box-helper/internal/probe"
)

const (
	reachTimeout  = 3 * time.Second
	tunnelTimeout = 8 * time.Second
	directTimeout = 5 * time.Second
)

// errorResp writes {"error": msg} with the given HTTP status code.
func errorResp(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// =====================================================================
// GET /api/status
// =====================================================================

type statusResponse struct {
	probe.Status
	CurrentNode *currentNodeInfo `json:"current_node,omitempty"`
}

// currentNodeInfo carries everything the UI needs to identify the running
// node. Managed=false means "sing-box is running with some other config
// that this utility didn't apply" — UI should treat the metadata as absent.
type currentNodeInfo struct {
	Managed   bool       `json:"managed"`
	Label     string     `json:"label,omitempty"`
	URI       string     `json:"uri,omitempty"`
	AppliedAt *time.Time `json:"applied_at,omitempty"`
	Protocol  string     `json:"protocol,omitempty"`
	Server    string     `json:"server,omitempty"`
	Port      int        `json:"port,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResp(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	resp := statusResponse{
		Status: probe.CollectStatus(s.Settings().TunInterfaceName),
	}

	// Source of truth: read the actual sing-box config.json.
	current := readCurrentOutbound(s.ConfigPath)
	if current == nil {
		// No config (or it's broken). Nothing to report.
		writeJSON(w, http.StatusOK, resp)
		return
	}

	info := &currentNodeInfo{
		Protocol: protocolLabel(current.Type),
		Server:   current.Server,
		Port:     current.ServerPort,
	}

	// Optional metadata layer: state.json contains the original URI / label /
	// time. We treat it as authoritative only if it matches what's actually
	// in config.json (same server+port) — otherwise the state is stale
	// (someone edited the config by hand) and we honestly say managed=false.
	if st := s.readStateFromDisk(); st != nil {
		if pn, err := parser.Parse(st.URI); err == nil &&
			pn.Outbound.Server == current.Server &&
			pn.Outbound.ServerPort == current.ServerPort {
			info.Managed = true
			info.Label = st.Label
			info.URI = st.URI
			at := st.AppliedAt
			info.AppliedAt = &at
		}
	}

	resp.CurrentNode = info
	writeJSON(w, http.StatusOK, resp)
}

// =====================================================================
// POST /api/preview     body: {"uri": "..."}
// Returns parsed display + the rendered config.json as a string, WITHOUT
// touching the filesystem or sing-box.
// =====================================================================

type uriRequest struct {
	URI string `json:"uri"`
}

type previewResponse struct {
	Display parser.Display `json:"display"`
	Label   string         `json:"label,omitempty"`
	Config  string         `json:"config"`
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResp(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req uriRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResp(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	pn, err := parser.Parse(req.URI)
	if err != nil {
		errorResp(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	raw, err := config.Render(pn, s.Settings())
	if err != nil {
		errorResp(w, http.StatusInternalServerError, "render: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, previewResponse{
		Display: pn.Display,
		Label:   pn.Label,
		Config:  string(raw),
	})
}

// =====================================================================
// POST /api/apply       body: {"uri": "..."}
// Pre-checks reachability of the node, renders the config, backs up the
// previous file, writes the new one, runs the init script restart.
// Returns the parsed display + reach result + a flag whether sing-box
// was actually restarted.
// =====================================================================

type applyResponse struct {
	Display    parser.Display `json:"display"`
	Label      string         `json:"label,omitempty"`
	BackupPath string         `json:"backup_path,omitempty"`
	ConfigSize int            `json:"config_size"`
	Reach      *reachInfo     `json:"reach,omitempty"`
	Restarted  bool           `json:"restarted"`
	RestartErr string         `json:"restart_error,omitempty"`
}

type reachInfo struct {
	Network string `json:"network"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResp(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req uriRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResp(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	pn, err := parser.Parse(req.URI)
	if err != nil {
		errorResp(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Pre-apply reachability check. This is best-effort; failure surfaces
	// in the response but doesn't block the apply (the user may know what
	// they're doing — e.g. server reachable but DPI blocks our probe).
	network := probe.ProtoNetwork(pn.Outbound.Type)
	reach := &reachInfo{Network: network}
	if err := probe.Reach(network, pn.Outbound.Server, pn.Outbound.ServerPort, reachTimeout); err != nil {
		reach.OK = false
		reach.Error = err.Error()
	} else {
		reach.OK = true
	}

	rendered, err := config.Render(pn, s.Settings())
	if err != nil {
		errorResp(w, http.StatusInternalServerError, "render: "+err.Error())
		return
	}

	bm := backup.New(s.ConfigPath)
	bak, err := bm.Create()
	if err != nil {
		errorResp(w, http.StatusInternalServerError, "backup: "+err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.ConfigPath), 0o755); err != nil {
		errorResp(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}
	if err := os.WriteFile(s.ConfigPath, rendered, 0o644); err != nil {
		errorResp(w, http.StatusInternalServerError, "write: "+err.Error())
		return
	}

	resp := applyResponse{
		Display:    pn.Display,
		Label:      pn.Label,
		BackupPath: bak,
		ConfigSize: len(rendered),
		Reach:      reach,
	}

	if err := restartSingBox(s.InitScript); err != nil {
		resp.Restarted = false
		resp.RestartErr = err.Error()
	} else {
		resp.Restarted = true
	}

	// Persist metadata so /api/status shows label / URI / time on the next
	// poll. The on-disk config.json itself is the source of truth for
	// "what's running"; state.json adds info sing-box doesn't track.
	// Failure to persist must NOT roll back the (already-applied + restarted)
	// config — we just surface it as a soft error.
	if err := s.saveStateToDisk(req.URI, pn.Label, time.Now().UTC()); err != nil {
		resp.RestartErr = strings.TrimSpace(resp.RestartErr + " | state save: " + err.Error())
	}

	// Trim old backups so the directory doesn't accumulate forever.
	// Soft failure: report but don't block.
	if s.KeepBackups > 0 {
		if _, terr := bm.Trim(s.KeepBackups); terr != nil {
			resp.RestartErr = strings.TrimSpace(resp.RestartErr + " | trim: " + terr.Error())
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// =====================================================================
// GET /api/test
// Runs the seven diagnostic checks from ТЗ section 7.1 against the
// currently-applied node. Returns the result list even on partial failure.
// =====================================================================

type testStep struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "fail" | "skip"
	Detail string `json:"detail,omitempty"`
}

type testResponse struct {
	Steps []testStep `json:"steps"`
}

func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResp(w, http.StatusMethodNotAllowed, "GET only")
		return
	}

	steps := []testStep{}
	add := func(name, status, detail string) {
		steps = append(steps, testStep{Name: name, Status: status, Detail: detail})
	}

	// 1. Reachability of the current node — read from the actual config.json
	// rather than a memory cache, so the probe always matches what sing-box
	// is really talking to.
	if current := readCurrentOutbound(s.ConfigPath); current != nil {
		network := probe.ProtoNetwork(current.Type)
		t0 := time.Now()
		err := probe.Reach(network, current.Server, current.ServerPort, reachTimeout)
		switch {
		case err != nil:
			add("Доступность узла", "fail", err.Error())
		default:
			add("Доступность узла", "ok",
				fmt.Sprintf("%s %s:%d, %d ms",
					strings.ToUpper(network), current.Server, current.ServerPort,
					time.Since(t0).Milliseconds()))
		}
	} else {
		add("Доступность узла", "skip", "В config.json нет proxy-outbound")
	}

	// 2. sing-box процесс
	settings := s.Settings()
	st := probe.CollectStatus(settings.TunInterfaceName)
	if st.SingBoxRunning {
		add("sing-box процесс", "ok", fmt.Sprintf("PID %d %s", st.SingBoxPID, st.SingBoxVersion))
	} else {
		add("sing-box процесс", "fail", "процесс не найден")
	}

	// 3. TUN-интерфейс
	if st.TunUp {
		add("Интерфейс "+st.TunName, "ok", "UP")
	} else {
		add("Интерфейс "+st.TunName, "fail", "не UP или отсутствует")
	}

	// 4. Прямой IP (через WAN)
	directIP, err := probe.DirectIP(directTimeout)
	if err != nil {
		add("Прямой IP", "fail", err.Error())
	} else {
		add("Прямой IP", "ok", directIP)
	}

	// 5. IP через TUN
	tunIP, err := probe.TunnelIP(settings.TunInterfaceName, tunnelTimeout)
	switch {
	case err != nil:
		add("IP через TUN", "fail", err.Error())
	case tunIP == directIP:
		add("IP через TUN", "fail", "совпадает с прямым ("+tunIP+") — туннель не работает")
	default:
		add("IP через TUN", "ok", tunIP)
	}

	writeJSON(w, http.StatusOK, testResponse{Steps: steps})
}

// =====================================================================
// GET  /api/settings
// POST /api/settings    body: full Settings JSON
//
// GET returns current settings plus the resolved effective MixedListen
// so the UI can show "auto (192.168.10.1)" without overwriting the
// "auto" preference. POST validates basic invariants, writes YAML to
// SettingsPath, swaps in-memory settings, and if a node is currently
// applied (state.json present + matches config.json) — re-renders
// config.json and restarts sing-box so the new settings take effect.
// =====================================================================

type settingsResponse struct {
	Settings              config.Settings `json:"settings"`
	MixedListenEffective  string          `json:"mixed_listen_effective"`
	MixedListenAuto       bool            `json:"mixed_listen_auto"`
	ReRendered            bool            `json:"re_rendered,omitempty"`
	Restarted             bool            `json:"restarted,omitempty"`
	RestartErr            string          `json:"restart_error,omitempty"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSettingsGet(w, r)
	case http.MethodPost:
		s.handleSettingsPost(w, r)
	default:
		errorResp(w, http.StatusMethodNotAllowed, "GET or POST only")
	}
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, _ *http.Request) {
	cur := s.Settings()
	eff, auto := probe.ResolveMixedListen(cur.MixedListen, config.LANInterface)
	writeJSON(w, http.StatusOK, settingsResponse{
		Settings:             cur,
		MixedListenEffective: eff,
		MixedListenAuto:      auto,
	})
}

func (s *Server) handleSettingsPost(w http.ResponseWriter, r *http.Request) {
	var newS config.Settings
	if err := json.NewDecoder(r.Body).Decode(&newS); err != nil {
		errorResp(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateSettings(newS); err != nil {
		errorResp(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if s.SettingsPath != "" {
		if err := config.SaveSettings(s.SettingsPath, newS); err != nil {
			errorResp(w, http.StatusInternalServerError, "save settings: "+err.Error())
			return
		}
	}
	s.setSettings(newS)

	resp := settingsResponse{Settings: newS}
	eff, auto := probe.ResolveMixedListen(newS.MixedListen, config.LANInterface)
	resp.MixedListenEffective = eff
	resp.MixedListenAuto = auto

	// If there is a managed node, re-render config.json with the new settings
	// and restart sing-box so the changes take effect immediately. If no
	// managed node — just persist YAML and return (next /api/apply will use
	// the updated settings).
	if st := s.readStateFromDisk(); st != nil {
		if pn, err := parser.Parse(st.URI); err == nil {
			rendered, rerr := config.Render(pn, newS)
			if rerr == nil {
				if _, bakErr := backup.New(s.ConfigPath).Create(); bakErr == nil {
					if werr := os.WriteFile(s.ConfigPath, rendered, 0o644); werr == nil {
						resp.ReRendered = true
						if rsErr := restartSingBox(s.InitScript); rsErr != nil {
							resp.RestartErr = rsErr.Error()
						} else {
							resp.Restarted = true
						}
					} else {
						resp.RestartErr = "write config: " + werr.Error()
					}
				} else {
					resp.RestartErr = "backup: " + bakErr.Error()
				}
			} else {
				resp.RestartErr = "render: " + rerr.Error()
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// validateSettings rejects values that would render an unusable config.json.
// We deliberately keep this minimal — the renderer doesn't crash on weird
// values, sing-box itself will reject them with a clear message on next
// start. We only catch the cases that would silently break the utility.
func validateSettings(s config.Settings) error {
	if s.MixedListenPort < 1 || s.MixedListenPort > 65535 {
		return fmt.Errorf("mixed_listen_port out of range: %d", s.MixedListenPort)
	}
	if s.TunMTU < 576 || s.TunMTU > 9000 {
		return fmt.Errorf("tun_mtu out of range: %d (expect 576..9000)", s.TunMTU)
	}
	if s.TunInterfaceName == "" {
		return fmt.Errorf("tun_interface_name must not be empty")
	}
	if s.TunAddress == "" {
		return fmt.Errorf("tun_address must not be empty (expected CIDR like 198.18.0.1/30)")
	}
	if s.UpstreamDNS == "" {
		return fmt.Errorf("upstream_dns must not be empty")
	}
	return nil
}

// =====================================================================
// GET /api/logs?source=<singbox|helper>&lines=<N>
//
// "singbox" — exec `ndmc -c "show log once"` and keep lines mentioning
// sing-box (logread is not present on Entware-on-Keenetic — see memory
// reference_keenetic_entware_install / general Keenetic ndmc usage).
// "helper"  — return the in-process ring buffer (the helper's own
// stderr, captured via logbuf.Buffer in main.go).
// =====================================================================

type logsResponse struct {
	Source string   `json:"source"`
	Lines  []string `json:"lines"`
	Note   string   `json:"note,omitempty"`
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResp(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	src := r.URL.Query().Get("source")
	if src == "" {
		src = "singbox"
	}
	n := 200
	if v := r.URL.Query().Get("lines"); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x > 0 && x <= 2000 {
			n = x
		}
	}

	switch src {
	case "singbox":
		lines, note, err := readSingBoxLog(n)
		if err != nil {
			writeJSON(w, http.StatusOK, logsResponse{Source: src, Note: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, logsResponse{Source: src, Lines: lines, Note: note})
	case "helper":
		if s.Logs == nil {
			writeJSON(w, http.StatusOK, logsResponse{Source: src, Note: "ring buffer not initialised"})
			return
		}
		entries := s.Logs.Tail(n)
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.When.Format("15:04:05") + " " + e.Text
		}
		writeJSON(w, http.StatusOK, logsResponse{Source: src, Lines: out})
	default:
		errorResp(w, http.StatusBadRequest, "unknown source (expect: singbox, helper)")
	}
}

// readSingBoxLog asks ndmc for the system log and keeps the last n lines
// that mention sing-box (case-insensitive). On routers without ndmc
// (development boxes) this returns a note explaining the situation; the
// HTTP response is still 200 — the UI shows the note rather than an error.
func readSingBoxLog(n int) ([]string, string, error) {
	if _, err := exec.LookPath("ndmc"); err != nil {
		return nil, "ndmc не найден в PATH (это не Keenetic). На дев-машине эта вкладка недоступна.", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ndmc", "-c", "show log once")
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("ndmc show log: %w", err)
	}
	var matched []string
	for _, line := range strings.Split(string(out), "\n") {
		ll := strings.ToLower(line)
		if strings.Contains(ll, "sing-box") || strings.Contains(ll, "singbox") {
			matched = append(matched, line)
		}
	}
	if len(matched) > n {
		matched = matched[len(matched)-n:]
	}
	return matched, "", nil
}

// =====================================================================
// /api/backups
//   GET                    -> list
//   POST   /restore        -> body {"file":"..."} -> restore that backup
//   DELETE ?file=...       -> delete that backup
// =====================================================================

type backupsResponse struct {
	Backups []backup.Entry `json:"backups"`
	Keep    int            `json:"keep"`
}

type restoreRequest struct {
	File string `json:"file"`
}

type restoreResponse struct {
	Restored          string `json:"restored"`
	BackupOfPrevious  string `json:"backup_of_previous,omitempty"`
	Restarted         bool   `json:"restarted"`
	RestartErr        string `json:"restart_error,omitempty"`
}

// handleBackups multiplexes GET / DELETE on /api/backups.
func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	bm := backup.New(s.ConfigPath)
	switch r.Method {
	case http.MethodGet:
		list, err := bm.List()
		if err != nil {
			errorResp(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, backupsResponse{Backups: list, Keep: s.KeepBackups})
	case http.MethodDelete:
		file := r.URL.Query().Get("file")
		if file == "" {
			errorResp(w, http.StatusBadRequest, "missing ?file= query")
			return
		}
		if err := bm.Delete(file); err != nil {
			errorResp(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": file})
	default:
		errorResp(w, http.StatusMethodNotAllowed, "GET or DELETE only")
	}
}

func (s *Server) handleBackupsRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResp(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResp(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.File == "" {
		errorResp(w, http.StatusBadRequest, "missing 'file' in body")
		return
	}
	bm := backup.New(s.ConfigPath)
	prev, err := bm.Restore(req.File)
	if err != nil {
		errorResp(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	resp := restoreResponse{Restored: req.File, BackupOfPrevious: prev}
	if rsErr := restartSingBox(s.InitScript); rsErr != nil {
		resp.RestartErr = rsErr.Error()
	} else {
		resp.Restarted = true
	}
	writeJSON(w, http.StatusOK, resp)
}

// =====================================================================
// helpers
// =====================================================================

// restartSingBox runs `<initScript> restart`. Errors include stderr text
// so the client gets a useful message instead of "exit status 1".
func restartSingBox(initScript string) error {
	if _, err := os.Stat(initScript); err != nil {
		return fmt.Errorf("init script not found at %s: %w", initScript, err)
	}
	cmd := exec.Command(initScript, "restart")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s restart: %w (%s)", initScript, err, strings.TrimSpace(string(out)))
	}
	return nil
}
