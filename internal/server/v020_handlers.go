package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"groxy-desk/internal/auditor"
	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/mcpbridge"
	"groxy-desk/internal/security"
)

func (s *Server) handleMCPTools(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"tools":                   s.mcp.ListTools(),
		"agent":                   groxybridge.AgentID,
		"model_switching_allowed": false,
	})
}

func (s *Server) handleMCPCall(w http.ResponseWriter, r *http.Request) {
	var req mcpbridge.CallRequest
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.mcp.CallTool(r.Context(), req)
	if err != nil {
		if security.ErrorStatus(err, 500) == http.StatusUnprocessableEntity {
			s.writeMCPError422(w, err)
			return
		}
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStudioDesignSystem(w http.ResponseWriter, r *http.Request) {
	studioURL := strings.TrimSpace(r.URL.Query().Get("studio_url"))
	dsMD, scMD, err := s.mcp.ReadDesignSystemAndContext(r.Context(), studioURL)
	if err != nil {
		s.writeError(w, err)
		return
	}
	cat := auditor.StudioCatalog(dsMD, scMD)
	s.writeJSON(w, http.StatusOK, cat)
}

type studioSynthesizeHTTPReq struct {
	auditor.SynthesizeRequest
	StudioURL string `json:"studio_url,omitempty"`
}

func (s *Server) handleStudioSynthesize(w http.ResponseWriter, r *http.Request) {
	var req studioSynthesizeHTTPReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	if strings.TrimSpace(req.DesignSystemMD) == "" || strings.TrimSpace(req.SiteContextMD) == "" {
		dsMD, scMD, err := s.mcp.ReadDesignSystemAndContext(r.Context(), req.StudioURL)
		if err != nil {
			s.writeError(w, err)
			return
		}
		if strings.TrimSpace(req.DesignSystemMD) == "" {
			req.DesignSystemMD = dsMD
		}
		if strings.TrimSpace(req.SiteContextMD) == "" {
			req.SiteContextMD = scMD
		}
	}
	res, err := auditor.SynthesizeLayout(req.SynthesizeRequest)
	if err != nil {
		s.writeError(w, security.NewError(mcpbridge.CodeDeskE422, 422, err.Error(), err))
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStudioLayoutApply(w http.ResponseWriter, r *http.Request) {
	var req mcpbridge.StudioApplyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	res, err := s.mcp.ApplyStudioLayout(r.Context(), req)
	if err != nil {
		if security.ErrorStatus(err, 500) == http.StatusUnprocessableEntity {
			s.writeMCPError422(w, err)
			return
		}
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGroxyAuditV2(w http.ResponseWriter, r *http.Request) {
	var in auditor.AuditInput
	if err := decodeJSONBody(r, &in); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.hydrateAuditWorkspaceContext(r, &in); err != nil {
		s.writeError(w, err)
		return
	}
	report := auditor.Audit(in)
	s.writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleGroxyBoost(w http.ResponseWriter, r *http.Request) {
	var req auditor.BoostRequest
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	auditBridge := auditor.AuditInput{
		TargetPath:     req.TargetPath,
		HTML:           req.HTML,
		GrexLayout:     req.GrexLayout,
		DesignSystemMD: req.DesignSystemMD,
		SiteContextMD:  req.SiteContextMD,
	}
	if err := s.hydrateAuditWorkspaceContext(r, &auditBridge); err != nil {
		s.writeError(w, err)
		return
	}
	req.TargetPath = auditBridge.TargetPath
	req.HTML = auditBridge.HTML
	req.DesignSystemMD = auditBridge.DesignSystemMD
	req.SiteContextMD = auditBridge.SiteContextMD

	proposal, err := auditor.Boost(req)
	if err != nil {
		s.writeError(w, security.NewError(mcpbridge.CodeDeskE422, 422, err.Error(), err))
		return
	}

	proposedBody := proposal.ProposedHTML
	fallbackBefore := req.HTML
	if proposedBody == "" && proposal.ProposedLayout != nil {
		b, _ := json.MarshalIndent(proposal.ProposedLayout, "", "  ")
		proposedBody = string(b)
	}
	if fallbackBefore == "" && req.GrexLayout != nil {
		b, _ := json.MarshalIndent(req.GrexLayout, "", "  ")
		fallbackBefore = string(b)
	}

	targetKey := proposal.TargetPath
	if strings.TrimSpace(targetKey) == "" {
		targetKey = "inline:boost"
	}

	appr, snap, _, err := s.approvals.IssueBoostToken(s.workspaceMgr.Root(), targetKey, proposedBody, fallbackBefore)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":                      true,
		"agent":                   groxybridge.AgentID,
		"model_switching_allowed": false,
		"reasoning_mode":          groxybridge.ReasoningDeep,
		"approval_required":       true,
		"approval_token":          appr.Token,
		"target_path":             appr.TargetPath,
		"snapshot":                snap,
		"proposal":                proposal,
	})
}

type boostApproveReq struct {
	ApprovalToken string  `json:"approval_token"`
	TargetPath    string  `json:"target_path"`
	Content       *string `json:"content,omitempty"`
}

func (s *Server) handleGroxyBoostApprove(w http.ResponseWriter, r *http.Request) {
	var req boostApproveReq
	if err := decodeJSONBody(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	target := strings.TrimSpace(req.TargetPath)
	if target == "" {
		target = "inline:boost"
	}
	res, err := s.approvals.ApplyMutationForAction(s.workspaceMgr.Root(), groxybridge.ActionBoostApprove, groxybridge.ApplyRequest{
		ApprovalToken: req.ApprovalToken,
		TargetPath:    target,
		Content:       req.Content,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, res)
}

func (s *Server) hydrateAuditWorkspaceContext(r *http.Request, in *auditor.AuditInput) error {
	if strings.TrimSpace(in.TargetPath) != "" &&
		!strings.HasPrefix(in.TargetPath, "studio:") &&
		!strings.HasPrefix(in.TargetPath, "cms:") &&
		!strings.HasPrefix(in.TargetPath, "inline:") {
		abs, rel, err := security.ConfineWorkspacePath(s.workspaceMgr.Root(), in.TargetPath)
		if err != nil {
			return err
		}
		in.TargetPath = rel
		if strings.TrimSpace(in.HTML) == "" && in.GrexLayout == nil {
			raw, readErr := os.ReadFile(abs)
			if readErr == nil {
				in.HTML = string(raw)
			}
		}
	}
	if strings.TrimSpace(in.DesignSystemMD) == "" || strings.TrimSpace(in.SiteContextMD) == "" {
		dsMD, scMD, err := s.mcp.ReadDesignSystemAndContext(r.Context(), "")
		if err == nil {
			if strings.TrimSpace(in.DesignSystemMD) == "" {
				in.DesignSystemMD = dsMD
			}
			if strings.TrimSpace(in.SiteContextMD) == "" {
				in.SiteContextMD = scMD
			}
		}
	}
	return nil
}

func (s *Server) writeMCPError422(w http.ResponseWriter, err error) {
	msg := s.redactor.Redact(err.Error())
	s.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"ok": false,
		"error": map[string]any{
			"code":    mcpbridge.CodeDeskE422,
			"message": msg,
		},
		"mcp_error": map[string]any{
			"code":    -32602,
			"message": msg,
		},
	})
}
