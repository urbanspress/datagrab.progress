package server

import (
	"net/http"
	"strings"

	"groxy-desk/internal/author"
	"groxy-desk/internal/dbstudio"
	"groxy-desk/internal/dns"
	"groxy-desk/internal/fleet"
)

func (s *Server) registerV030Routes(mux *http.ServeMux) {
	// Step 5 Fleet & Grabtaculous Bridge endpoints
	mux.Handle("GET /api/fleet/nodes", s.authCSRF(http.HandlerFunc(s.handleFleetNodes)))
	mux.Handle("POST /api/fleet/nodes/verify", s.authCSRF(http.HandlerFunc(s.handleFleetVerify)))
	mux.Handle("POST /api/fleet/dna/validate", s.authCSRF(http.HandlerFunc(s.handleFleetDNAValidate)))
	mux.Handle("POST /api/fleet/deploy/plan", s.authCSRF(http.HandlerFunc(s.handleFleetDeployPlan)))
	mux.Handle("POST /api/fleet/deploy/apply", s.authCSRF(http.HandlerFunc(s.handleFleetDeployApply)))
	mux.Handle("POST /api/fleet/snapshots/restore", s.authCSRF(http.HandlerFunc(s.handleFleetSnapshotRestore)))

	// Loopback SSH DB Tunnels & Visual DB Explorer endpoints
	mux.Handle("GET /api/db/tunnels", s.authCSRF(http.HandlerFunc(s.handleDBTunnels)))
	mux.Handle("POST /api/db/tunnels/open", s.authCSRF(http.HandlerFunc(s.handleDBTunnelOpen)))
	mux.Handle("POST /api/db/tunnels/close", s.authCSRF(http.HandlerFunc(s.handleDBTunnelClose)))
	mux.Handle("GET /api/db/schema", s.authCSRF(http.HandlerFunc(s.handleDBSchema)))
	mux.Handle("POST /api/db/schema", s.authCSRF(http.HandlerFunc(s.handleDBSchema)))
	mux.Handle("POST /api/db/rows", s.authCSRF(http.HandlerFunc(s.handleDBRows)))
	mux.Handle("POST /api/db/query", s.authCSRF(http.HandlerFunc(s.handleDBQuery)))
	mux.Handle("POST /api/db/mutate/plan", s.authCSRF(http.HandlerFunc(s.handleDBMutatePlan)))
	mux.Handle("POST /api/db/mutate/apply", s.authCSRF(http.HandlerFunc(s.handleDBMutateApply)))
	mux.Handle("POST /api/db/snapshots/restore", s.authCSRF(http.HandlerFunc(s.handleDBSnapshotRestore)))

	// Encrypted Namecheap DNS Vault & Domain -> DNS -> Auto-HTTPS endpoints
	mux.Handle("GET /api/dns/vault", s.authCSRF(http.HandlerFunc(s.handleDNSVaultStatus)))
	mux.Handle("POST /api/dns/vault/save", s.authCSRF(http.HandlerFunc(s.handleDNSVaultSave)))
	mux.Handle("POST /api/dns/plan", s.authCSRF(http.HandlerFunc(s.handleDNSPlan)))
	mux.Handle("POST /api/dns/apply", s.authCSRF(http.HandlerFunc(s.handleDNSApply)))
	mux.Handle("POST /api/dns/https/preflight", s.authCSRF(http.HandlerFunc(s.handleHTTPSPreflight)))
	mux.Handle("POST /api/dns/https/apply", s.authCSRF(http.HandlerFunc(s.handleHTTPSApply)))
	mux.Handle("POST /api/dns/snapshots/restore", s.authCSRF(http.HandlerFunc(s.handleDNSSnapshotRestore)))

	// 1-Click Local Author Mode & Loopback Ollama Runtime endpoints
	mux.Handle("POST /api/author/init/plan", s.authCSRF(http.HandlerFunc(s.handleAuthorInitPlan)))
	mux.Handle("POST /api/author/init/apply", s.authCSRF(http.HandlerFunc(s.handleAuthorInitApply)))
	mux.Handle("GET /api/author/ollama/status", s.authCSRF(http.HandlerFunc(s.handleAuthorOllamaStatus)))
	mux.Handle("POST /api/author/ollama/start", s.authCSRF(http.HandlerFunc(s.handleAuthorOllamaStart)))
}

func handleJSONAction[Req any, Res any](s *Server, w http.ResponseWriter, r *http.Request, fn func(Req) (Res, error)) {
	var req Req
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := fn(req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleFleetNodes(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"nodes": s.fleetMgr.ListNodes(),
	})
}

func (s *Server) handleFleetVerify(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req fleet.UpsertNodeRequest) (map[string]any, error) {
		return s.fleetMgr.VerifyAndSaveNode(r.Context(), req)
	})
}

func (s *Server) handleFleetDNAValidate(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req fleet.DNAValidateRequest) (fleet.DNAValidationResult, error) {
		return s.fleetMgr.ValidateSiteDNA(r.Context(), req)
	})
}

func (s *Server) handleFleetDeployPlan(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req fleet.DeployPlanRequest) (fleet.DeployPlanResult, error) {
		return s.fleetMgr.PlanDeploy(r.Context(), req)
	})
}

func (s *Server) handleFleetDeployApply(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req fleet.DeployApplyRequest) (map[string]any, error) {
		return s.fleetMgr.ApplyDeploy(r.Context(), req)
	})
}

func (s *Server) handleFleetSnapshotRestore(w http.ResponseWriter, r *http.Request) {
	type restoreReq struct {
		SnapshotID   string `json:"snapshot_id"`
		ControlURL   string `json:"control_url,omitempty"`
		ControlToken string `json:"control_token,omitempty"`
	}
	handleJSONAction(s, w, r, func(req restoreReq) (map[string]any, error) {
		return s.fleetMgr.RestoreDeploySnapshot(r.Context(), req.SnapshotID, req.ControlURL, req.ControlToken)
	})
}

func (s *Server) handleDBTunnels(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"tunnels": s.dbStudio.Tunnels().ListTunnels(),
	})
}

func (s *Server) handleDBTunnelOpen(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dbstudio.OpenTunnelRequest) (map[string]any, error) {
		view, err := s.dbStudio.Tunnels().OpenTunnel(req)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "tunnel": view}, nil
	})
}

func (s *Server) handleDBTunnelClose(w http.ResponseWriter, r *http.Request) {
	type closeReq struct {
		TunnelID string `json:"tunnel_id"`
	}
	handleJSONAction(s, w, r, func(req closeReq) (map[string]any, error) {
		closed := s.dbStudio.Tunnels().CloseTunnel(req.TunnelID)
		return map[string]any{"ok": true, "closed": closed, "tunnel_id": strings.TrimSpace(req.TunnelID)}, nil
	})
}

func (s *Server) handleDBSchema(w http.ResponseWriter, r *http.Request) {
	var target dbstudio.TargetSpec
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		target = dbstudio.TargetSpec{
			Engine:     strings.TrimSpace(q.Get("engine")),
			SQLitePath: strings.TrimSpace(q.Get("sqlite_path")),
			TunnelID:   strings.TrimSpace(q.Get("tunnel_id")),
			DirectHost: strings.TrimSpace(q.Get("host")),
			DBUser:     strings.TrimSpace(q.Get("db_user")),
			DBName:     strings.TrimSpace(q.Get("db_name")),
			SchemaName: strings.TrimSpace(q.Get("schema_name")),
		}
	} else {
		if err := decodeJSONBody(r, &target); err != nil {
			s.writeError(w, err)
			return
		}
	}
	res, err := s.dbStudio.GetSchema(r.Context(), target)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleDBRows(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dbstudio.RowsRequest) (map[string]any, error) {
		return s.dbStudio.BrowseRows(r.Context(), req)
	})
}

func (s *Server) handleDBQuery(w http.ResponseWriter, r *http.Request) {
	type queryReq struct {
		Target dbstudio.TargetSpec `json:"target"`
		SQL    string              `json:"sql"`
	}
	handleJSONAction(s, w, r, func(req queryReq) (map[string]any, error) {
		return s.dbStudio.RunReadOnlyQuery(r.Context(), req.Target, req.SQL)
	})
}

func (s *Server) handleDBMutatePlan(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dbstudio.MutatePlanRequest) (map[string]any, error) {
		return s.dbStudio.PlanMutation(r.Context(), req)
	})
}

func (s *Server) handleDBMutateApply(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dbstudio.MutateApplyRequest) (map[string]any, error) {
		return s.dbStudio.ApplyMutation(r.Context(), req)
	})
}

func (s *Server) handleDBSnapshotRestore(w http.ResponseWriter, r *http.Request) {
	type restoreReq struct {
		Target     dbstudio.TargetSpec `json:"target"`
		SnapshotID string              `json:"snapshot_id"`
	}
	handleJSONAction(s, w, r, func(req restoreReq) (map[string]any, error) {
		return s.dbStudio.RestoreSnapshot(r.Context(), req.Target, req.SnapshotID)
	})
}

func (s *Server) handleDNSVaultStatus(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"vault": s.dnsMgr.GetVaultMaskedStatus(),
	})
}

func (s *Server) handleDNSVaultSave(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dns.VaultSaveRequest) (map[string]any, error) {
		return s.dnsMgr.SaveVault(req)
	})
}

func (s *Server) handleDNSPlan(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dns.DNSPlanRequest) (map[string]any, error) {
		return s.dnsMgr.PlanDNS(r.Context(), req)
	})
}

func (s *Server) handleDNSApply(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dns.DNSApplyRequest) (map[string]any, error) {
		return s.dnsMgr.ApplyDNS(r.Context(), req)
	})
}

func (s *Server) handleHTTPSPreflight(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dns.HTTPSPreflightRequest) (map[string]any, error) {
		return s.dnsMgr.PreflightHTTPS(r.Context(), req)
	})
}

func (s *Server) handleHTTPSApply(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req dns.HTTPSApplyRequest) (map[string]any, error) {
		return s.dnsMgr.ApplyHTTPS(r.Context(), req)
	})
}

func (s *Server) handleDNSSnapshotRestore(w http.ResponseWriter, r *http.Request) {
	type restoreReq struct {
		SnapshotID string `json:"snapshot_id"`
	}
	handleJSONAction(s, w, r, func(req restoreReq) (map[string]any, error) {
		return s.dnsMgr.RestoreDNSSnapshot(r.Context(), req.SnapshotID)
	})
}

func (s *Server) handleAuthorInitPlan(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req author.InitPlanRequest) (map[string]any, error) {
		return s.authorMgr.PlanInit(req)
	})
}

func (s *Server) handleAuthorInitApply(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req author.InitApplyRequest) (map[string]any, error) {
		return s.authorMgr.ApplyInit(req)
	})
}

func (s *Server) handleAuthorOllamaStatus(w http.ResponseWriter, r *http.Request) {
	overrideURL := strings.TrimSpace(r.URL.Query().Get("url"))
	res, err := s.authorMgr.OllamaStatus(r.Context(), overrideURL)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAuthorOllamaStart(w http.ResponseWriter, r *http.Request) {
	handleJSONAction(s, w, r, func(req author.OllamaStartRequest) (map[string]any, error) {
		return s.authorMgr.StartOllama(r.Context(), req)
	})
}
