package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wolfam0108/sing-box-helper/internal/config"
	"github.com/wolfam0108/sing-box-helper/internal/parser"
	"github.com/wolfam0108/sing-box-helper/internal/probe"
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

	bak, err := backupIfExists(s.ConfigPath)
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
				if _, bakErr := backupIfExists(s.ConfigPath); bakErr == nil {
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
// helpers
// =====================================================================

// backupIfExists copies cfgPath into cfgPath.bak-<timestamp>, returning
// the backup path. No-op if cfgPath doesn't exist.
func backupIfExists(cfgPath string) (string, error) {
	src, err := os.Open(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer src.Close()

	bak := cfgPath + ".bak-" + time.Now().Format("20060102-150405")
	dst, err := os.OpenFile(bak, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return bak, nil
}

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
