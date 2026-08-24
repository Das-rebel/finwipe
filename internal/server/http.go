package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
)

//go:embed dashboard.html
var dashFS embed.FS

type Dashboard struct{ Store *Store }

func (d *Dashboard) Register(mux *http.ServeMux) {
	tmpl := template.Must(template.ParseFS(dashFS, "dashboard.html"))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		all, err := d.Store.All()
		if err != nil {
			http.Error(w, "db: "+err.Error(), 500)
			return
		}
		tmpl.Execute(w, all)
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		all, _ := d.Store.All()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(all)
	})
}

// MCPShim exposes the same operations over a minimal JSON-RPC surface so
// that modelcontextprotocol/go-sdk can be layered on without rework.
// Tools: list_requests, get_status, escalate.
func (d *Dashboard) MCPShim(mux *http.ServeMux) {
	mux.HandleFunc("/api/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		switch req.Method {
		case "list_requests":
			all, _ := d.Store.All()
			json.NewEncoder(w).Encode(map[string]any{"requests": all})
		case "escalate":
			var p struct{ DPRID string `json:"dpr_id"` }
			json.Unmarshal(req.Params, &p)
			err := d.Store.Transition(p.DPRID, "ESCALATED", "escalated via mcp shim")
			resp := map[string]any{"ok": err == nil}
			if err != nil { resp["error"] = err.Error() }
			json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "unknown method", 404)
		}
	})
}
