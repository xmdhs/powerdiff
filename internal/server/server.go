package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/xmdhs/powerdiff/internal/power"
)

//go:embed static
var staticFS embed.FS

type Config struct {
	Debug bool
}

type Server struct {
	mux   *http.ServeMux
	debug bool
}

func New(cfg Config) *Server {
	s := &Server{mux: http.NewServeMux(), debug: cfg.Debug}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !isLocalhost(r) {
			writeError(w, http.StatusForbidden, "只允许 localhost 访问 API", "access-check", nil)
			return
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.health)
	s.mux.HandleFunc("/api/schemes", s.schemes)
	s.mux.HandleFunc("/api/schemes/active", s.activeScheme)
	s.mux.HandleFunc("/api/schemes/", s.schemeAction)
	s.mux.HandleFunc("/api/overlays", s.overlays)
	s.mux.HandleFunc("/api/overlays/", s.overlayAction)
	s.mux.HandleFunc("/api/settings", s.settings)
	s.mux.HandleFunc("/api/settings/", s.settingAction)
	s.mux.HandleFunc("/api/diff/", s.diff)
	s.mux.HandleFunc("/api/export", s.exportXML)
	s.mux.HandleFunc("/api/import", s.importXML)
	s.mux.HandleFunc("/api/script", s.script)
	sub, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("/", http.FileServer(http.FS(sub)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "platform": power.Platform()})
}

func (s *Server) schemes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := power.ListSchemes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "list-schemes", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) activeScheme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	item, err := power.ActiveScheme()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "active-scheme", err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) schemeAction(w http.ResponseWriter, r *http.Request) {
	guid, action, ok := splitAction(r.URL.Path, "/api/schemes/")
	if !ok || action != "activate" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := power.SetActiveScheme(guid); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "activate-scheme", err)
		return
	}
	active, _ := power.ActiveScheme()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": active.GUID, "scheme": active})
}

func (s *Server) overlays(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	items, err := power.ListOverlays()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "list-overlays", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) overlayAction(w http.ResponseWriter, r *http.Request) {
	guid, action, ok := splitAction(r.URL.Path, "/api/overlays/")
	if !ok || action != "activate" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := power.SetActiveOverlay(guid); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "activate-overlay", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active": guid})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	scheme := r.URL.Query().Get("scheme")
	items, err := power.ListSettings(scheme)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "list-settings", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) settingAction(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/settings/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	guid := parts[0]
	if len(parts) == 1 && r.Method == http.MethodPut {
		var req power.UpdateSettingRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "decode-update-setting", err)
			return
		}
		value, err := power.UpdateSetting(guid, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "update-setting", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "value": value})
		return
	}
	if len(parts) == 2 && parts[1] == "hidden" && r.Method == http.MethodPut {
		var req power.HiddenRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "decode-hidden", err)
			return
		}
		if err := power.SetHidden(guid, req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "set-hidden", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if len(parts) == 2 && parts[1] == "possible-values" && r.Method == http.MethodPost {
		var req power.PossibleValueRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "decode-possible-value", err)
			return
		}
		if err := power.AddPossibleValue(guid, req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), "add-possible-value", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	methodNotAllowed(w)
}

func (s *Server) diff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	scheme := strings.TrimPrefix(r.URL.Path, "/api/diff/")
	compare := r.URL.Query().Get("compare")
	var items []power.DiffItem
	var err error
	if compare == "" || compare == "default" {
		items, err = power.DiffScheme(scheme, r.URL.Query().Get("all") == "1")
	} else {
		items, err = power.DiffAgainst(scheme, compare)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "diff", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) exportXML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	scheme := r.URL.Query().Get("scheme")
	data, err := power.ExportXML(scheme)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "export", err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="power-settings.xml"`)
	_, _ = w.Write(data)
}

func (s *Server) importXML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "read-import", err)
		return
	}
	result, err := power.ImportXML(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "import", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) script(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	content, err := power.ExportScript(r.URL.Query().Get("scheme"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "script", err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="power-settings.cmd"`)
	_, _ = w.Write([]byte(content))
}

func splitAction(path, prefix string) (guid, action string, ok bool) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, operation string, err error) {
	resp := map[string]any{"error": msg, "operation": operation}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		resp["code"] = uintptr(errno)
	}
	writeJSON(w, status, resp)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed", "method", nil)
}

func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
