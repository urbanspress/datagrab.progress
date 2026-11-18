// Package mcpbridge provides Groxy Desk's unified, workspace-confined bridge
// across Core GRAB v1.1.0 stdio MCP tools, GrabLab CMS v1.0.1 HTTP MCP tools,
// and Grex Theme Builder v1.0.0 layout dispatch.
package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"groxy-desk/internal/auditor"
	"groxy-desk/internal/groxybridge"
	"groxy-desk/internal/security"
)

const (
	CodeDeskE422 = "DESK-E422"
	maxHTTPReply = 4 * 1024 * 1024
)

// ToolDescriptor describes one tool exposed across Core GRAB, GrabLab CMS, or Grex Theme Builder.
type ToolDescriptor struct {
	Name             string         `json:"name"`
	Layer            string         `json:"layer"` // core-grab | grablab-cms | grex-builder
	Description      string         `json:"description"`
	Mode             string         `json:"mode"` // read-only | write
	RequiresApproval bool           `json:"requires_approval"`
	InputSchema      map[string]any `json:"input_schema,omitempty"`
}

// CallRequest invokes an MCP tool via POST /api/mcp/call.
type CallRequest struct {
	Tool          string         `json:"tool"`
	Arguments     map[string]any `json:"arguments,omitempty"`
	CMSURL        string         `json:"cms_url,omitempty"`
	CMSToken      string         `json:"cms_token,omitempty"`
	StudioURL     string         `json:"studio_url,omitempty"`
	StudioCSRF    string         `json:"studio_csrf,omitempty"`
	Prepare       bool           `json:"prepare,omitempty"`
	ApprovalToken string         `json:"approval_token,omitempty"`
	TargetPath    string         `json:"target_path,omitempty"`
}

// CallResponse returns structured MCP output or GROXY-SAFE-001 approval challenge.
type CallResponse struct {
	OK               bool                       `json:"ok"`
	Tool             string                     `json:"tool"`
	Layer            string                     `json:"layer"`
	Status           string                     `json:"status"` // ok | approval_required
	ReadOnly         bool                       `json:"read_only"`
	AllowWriteFlag   bool                       `json:"allow_write_flag"`
	StatusEnforced   string                     `json:"status_enforced,omitempty"`
	ApprovalRequired bool                       `json:"approval_required"`
	ApprovalToken    string                     `json:"approval_token,omitempty"`
	TargetPath       string                     `json:"target_path,omitempty"`
	Snapshot         *groxybridge.SnapshotRecord `json:"snapshot,omitempty"`
	Result           any                        `json:"result,omitempty"`
	Command          map[string]any             `json:"command,omitempty"`
}

// StudioApplyRequest dispatches an AI-synthesized layout via POST /api/studio/layout/apply.
type StudioApplyRequest struct {
	Slot          string            `json:"slot,omitempty"`
	Summary       string            `json:"summary,omitempty"`
	StudioURL     string            `json:"studio_url,omitempty"`
	StudioCSRF    string            `json:"studio_csrf,omitempty"`
	GrexLayout    *auditor.GrexNode `json:"grex_layout,omitempty"`
	Layout        *auditor.GrexNode `json:"layout,omitempty"`
	Prepare       bool              `json:"prepare,omitempty"`
	ApprovalToken string            `json:"approval_token,omitempty"`
	TargetPath    string            `json:"target_path,omitempty"`
}

// Bridge manages Core GRAB stdio MCP calls and loopback CMS/Studio HTTP dispatches.
type Bridge struct {
	workspaceRoot string
	grabBin       string
	stateDir      string
	approvals     *groxybridge.ApprovalManager
	redactor      *security.SecretRedactor
	httpClient    *http.Client
}

// New constructs a validated MCP bridge for the confined workspace.
func New(workspaceRoot, grabBin, stateDir string, approvals *groxybridge.ApprovalManager, redactor *security.SecretRedactor) (*Bridge, error) {
	if approvals == nil {
		return nil, fmt.Errorf("approval manager is required")
	}
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return &Bridge{
		workspaceRoot: absRoot,
		grabBin:       grabBin,
		stateDir:      stateDir,
		approvals:     approvals,
		redactor:      redactor,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}, nil
}

// ListTools returns all 13 unified tools across Core GRAB v1.1.0, GrabLab CMS v1.0.1,
// and Grex Theme Builder v1.0.0.
func (b *Bridge) ListTools() []ToolDescriptor {
	return []ToolDescriptor{
		{
			Name:             "grab_check",
			Layer:            "core-grab",
			Description:      "Statically validate a .grab program and optional .grab.html template without executing code.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "grab_format",
			Layer:            "core-grab",
			Description:      "Canonical 2-space GRAB formatter. Read-only default; write: true requires GROXY-SAFE-001 approval.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "grab_inspect_routes",
			Layer:            "core-grab",
			Description:      "Inspect declared HTTP routes, params, and templates in a .grab file without executing code.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "grab_inspect_schema",
			Layer:            "core-grab",
			Description:      "Inspect grab.data.json, grab.api.json, and grab.platform.json without leaking env secrets.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "grab_scaffold",
			Layer:            "core-grab",
			Description:      "Generate canonically formatted .grab/.grab.html/grab.api.json starter files (write gated unless dry_run: true).",
			Mode:             "write",
			RequiresApproval: true,
		},
		{
			Name:             "list_content_types",
			Layer:            "grablab-cms",
			Description:      "Discover registered GrabLab CMS content types (post, page, custom types).",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "query_entries",
			Layer:            "grablab-cms",
			Description:      "Query GrabLab CMS entries with filters, taxonomy terms, and ADR-025c fields.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "create_entry_draft",
			Layer:            "grablab-cms",
			Description:      "Create a CMS entry strictly enforcing status_enforced: \"draft\" and WCAG AA layout validation.",
			Mode:             "write",
			RequiresApproval: true,
		},
		{
			Name:             "update_entry_layout",
			Layer:            "grablab-cms",
			Description:      "Update an existing entry's Grex layout after strict 20-element & WCAG AA validation (422 / -32602 on violations).",
			Mode:             "write",
			RequiresApproval: true,
		},
		{
			Name:             "list_media",
			Layer:            "grablab-cms",
			Description:      "Discover GrabLab CMS media assets for alt-text enrichment and safe insertion.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "list_menus",
			Layer:            "grablab-cms",
			Description:      "Discover navigation menus by location for SITE-CONTEXT.md internal linking.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "studio_design_system",
			Layer:            "grex-builder",
			Description:      "Read live GRAB-DESIGN-SYSTEM.md tokens, classes, SITE-CONTEXT.md, and 8x5 Grex styles.",
			Mode:             "read-only",
			RequiresApproval: false,
		},
		{
			Name:             "studio_apply_layout",
			Layer:            "grex-builder",
			Description:      "Dispatch an approved Grex layout with actor: \"groxy-ai\" returning applyAiLayout undo metadata.",
			Mode:             "write",
			RequiresApproval: true,
		},
	}
}

// LookupTool checks if name belongs to the unified 13-tool whitelist.
func (b *Bridge) LookupTool(name string) (ToolDescriptor, bool) {
	trimmed := strings.TrimSpace(name)
	for _, t := range b.ListTools() {
		if t.Name == trimmed {
			return t, true
		}
	}
	return ToolDescriptor{}, false
}

// CallTool executes or prepares a whitelisted Core GRAB, GrabLab CMS, or Grex Studio MCP tool.
func (b *Bridge) CallTool(ctx context.Context, req CallRequest) (CallResponse, error) {
	desc, ok := b.LookupTool(req.Tool)
	if !ok {
		return CallResponse{}, security.NewError(security.CodeDeskE101, 400, fmt.Sprintf("unsupported MCP tool %q", req.Tool), nil)
	}
	if req.CMSToken != "" {
		b.redactor.Register(req.CMSToken)
	}
	if req.StudioCSRF != "" {
		b.redactor.Register(req.StudioCSRF)
	}

	args := cloneMap(req.Arguments)
	sanitizedArgs, canonicalTarget, isWrite, err := b.sanitizeAndClassify(desc, args)
	if err != nil {
		return CallResponse{}, err
	}

	if strings.TrimSpace(req.TargetPath) != "" && isWrite {
		// Enforce target binding against explicit caller target if passed, confined cleanly if workspace path.
		explicitNorm := normalizeMCPTarget(req.TargetPath)
		if desc.Layer == "core-grab" {
			if _, rel, cErr := security.ConfineWorkspacePath(b.workspaceRoot, req.TargetPath); cErr != nil {
				return CallResponse{}, cErr
			} else {
				explicitNorm = rel
			}
		}
		if explicitNorm != canonicalTarget {
			return CallResponse{}, security.NewError(security.CodeDeskE403, 403, "approval_token target mismatch", nil)
		}
	}

	// Pre-flight validation for Grex layout arguments BEFORE token consumption or network dispatch!
	if desc.Name == "create_entry_draft" || desc.Name == "update_entry_layout" || desc.Name == "studio_apply_layout" {
		if rawTree := firstMapValue(sanitizedArgs, "grex_layout", "layout", "nextTree", "tree"); rawTree != nil {
			if vs, vErr := validateRawGrexLayout(rawTree); vErr != nil {
				return CallResponse{}, security.NewError(CodeDeskE422, 422, vErr.Error(), nil)
			} else if len(vs) > 0 {
				return CallResponse{}, newValidationError(vs)
			}
		}
	}

	if isWrite {
		if req.Prepare {
			snap, appr, err := b.prepareWriteApproval(desc, canonicalTarget, sanitizedArgs)
			if err != nil {
				return CallResponse{}, err
			}
			return CallResponse{
				OK:               true,
				Tool:             desc.Name,
				Layer:            desc.Layer,
				Status:           "approval_required",
				ReadOnly:         false,
				AllowWriteFlag:   false,
				ApprovalRequired: true,
				ApprovalToken:    appr.Token,
				TargetPath:       canonicalTarget,
				Snapshot:         &snap,
			}, nil
		}

		// Direct write call MUST present valid single-use target-bound approval_token (GROXY-SAFE-001).
		if _, err := b.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionMcpWrite, canonicalTarget); err != nil {
			return CallResponse{}, err
		}
	}

	switch desc.Layer {
	case "core-grab":
		res, err := b.callCoreGrabMCP(ctx, desc.Name, sanitizedArgs, isWrite)
		if err != nil {
			return CallResponse{}, err
		}
		return CallResponse{
			OK:             true,
			Tool:           desc.Name,
			Layer:          desc.Layer,
			Status:         "ok",
			ReadOnly:       !isWrite,
			AllowWriteFlag: isWrite,
			TargetPath:     canonicalTarget,
			Result:         res,
		}, nil

	case "grablab-cms":
		if strings.TrimSpace(req.CMSURL) == "" {
			return CallResponse{}, security.NewError(security.CodeDeskE101, 400, "cms_url (127.0.0.1 loopback worker URL) is required for GrabLab CMS tools", nil)
		}
		if err := ValidateLoopbackHTTPURL(req.CMSURL); err != nil {
			return CallResponse{}, err
		}
		if desc.Name == "create_entry_draft" {
			// Always enforce draft status regardless of caller args.
			sanitizedArgs["status"] = "draft"
		}
		payload, statusCode, err := b.callLoopbackCMSTool(ctx, req.CMSURL, req.CMSToken, desc.Name, sanitizedArgs)
		if err != nil {
			return CallResponse{}, err
		}
		if statusCode == http.StatusUnprocessableEntity {
			return CallResponse{}, newUpstream422Error(payload)
		}
		enforced := ""
		if desc.Name == "create_entry_draft" {
			enforced = "draft"
			if pm, ok := payload.(map[string]any); ok {
				if se, ok := pm["status_enforced"].(string); ok && se != "" {
					enforced = se
				} else {
					pm["status_enforced"] = "draft"
				}
			}
		}
		return CallResponse{
			OK:             true,
			Tool:           desc.Name,
			Layer:          desc.Layer,
			Status:         "ok",
			ReadOnly:       !isWrite,
			AllowWriteFlag: isWrite,
			StatusEnforced: enforced,
			TargetPath:     canonicalTarget,
			Result:         payload,
		}, nil

	case "grex-builder":
		if desc.Name == "studio_design_system" {
			dsMD, scMD, err := b.ReadDesignSystemAndContext(ctx, req.StudioURL)
			if err != nil {
				return CallResponse{}, err
			}
			return CallResponse{
				OK:       true,
				Tool:     desc.Name,
				Layer:    desc.Layer,
				Status:   "ok",
				ReadOnly: true,
				Result:   auditor.StudioCatalog(dsMD, scMD),
			}, nil
		}
		// studio_apply_layout
		slot := strArg(sanitizedArgs, "slot")
		if slot == "" {
			slot = "main"
		}
		summary := strArg(sanitizedArgs, "summary")
		rawTree := firstMapValue(sanitizedArgs, "grex_layout", "layout", "nextTree", "tree")
		node, err := decodeGrexNode(rawTree)
		if err != nil {
			return CallResponse{}, security.NewError(CodeDeskE422, 422, "invalid Grex layout JSON: "+err.Error(), nil)
		}
		res, cmd, err := b.dispatchStudioLayout(ctx, req.StudioURL, req.StudioCSRF, slot, summary, node)
		if err != nil {
			return CallResponse{}, err
		}
		return CallResponse{
			OK:             true,
			Tool:           desc.Name,
			Layer:          desc.Layer,
			Status:         "ok",
			ReadOnly:       false,
			AllowWriteFlag: true,
			TargetPath:     canonicalTarget,
			Result:         res,
			Command:        cmd,
		}, nil
	}

	return CallResponse{}, security.NewError(security.CodeDeskE101, 500, "unhandled tool layer", nil)
}

// ReadDesignSystemAndContext reads live GET /api/design-system.md (when studioURL is supplied
// cleanly on 127.0.0.1) or falls back to workspace GRAB-DESIGN-SYSTEM.md and SITE-CONTEXT.md.
func (b *Bridge) ReadDesignSystemAndContext(ctx context.Context, studioURL string) (string, string, error) {
	dsMD := ""
	if strings.TrimSpace(studioURL) != "" {
		if err := ValidateLoopbackHTTPURL(studioURL); err != nil {
			return "", "", err
		}
		endpoint := strings.TrimRight(strings.TrimSpace(studioURL), "/") + "/api/design-system.md"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", "", security.NewError(security.CodeDeskE101, 400, "invalid studio design-system URL", err)
		}
		resp, err := b.httpClient.Do(req)
		if err == nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxHTTPReply))
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && len(body) > 0 {
				dsMD = string(body)
			}
		}
	}
	if dsMD == "" {
		if raw, err := b.readConfinedOptionalFile("GRAB-DESIGN-SYSTEM.md"); err == nil && strings.TrimSpace(raw) != "" {
			dsMD = raw
		} else {
			dsMD = auditor.DefaultDesignSystemMarkdown
		}
	}
	scMD, _ := b.readConfinedOptionalFile("SITE-CONTEXT.md")
	return dsMD, scMD, nil
}

// ApplyStudioLayout handles POST /api/studio/layout/apply with WCAG/Grex 422 validation,
// GROXY-SAFE-001 approval token + snapshot, actor: "groxy-ai", and applyAiLayout 1-click undo.
func (b *Bridge) ApplyStudioLayout(ctx context.Context, req StudioApplyRequest) (CallResponse, error) {
	if req.StudioCSRF != "" {
		b.redactor.Register(req.StudioCSRF)
	}
	node := req.GrexLayout
	if node == nil {
		node = req.Layout
	}
	if node == nil {
		return CallResponse{}, security.NewError(CodeDeskE422, 422, "grex_layout or layout is required", nil)
	}
	if vs := auditor.ValidateGrexTree(node); len(vs) > 0 {
		return CallResponse{}, newValidationError(vs)
	}

	slot := strings.TrimSpace(req.Slot)
	if slot == "" {
		slot = "main"
	}
	targetKey := "studio:layout:" + slot
	if strings.TrimSpace(req.TargetPath) != "" && normalizeMCPTarget(req.TargetPath) != normalizeMCPTarget(targetKey) {
		return CallResponse{}, security.NewError(security.CodeDeskE403, 403, "approval_token target mismatch", nil)
	}

	if req.Prepare {
		prevJSON, _ := b.readStudioStateJSON(slot)
		snap, err := b.approvals.CreateVirtualSnapshot(targetKey, prevJSON)
		if err != nil {
			return CallResponse{}, err
		}
		appr, err := b.approvals.IssueActionToken(groxybridge.ActionStudioLayoutApply, targetKey)
		if err != nil {
			return CallResponse{}, err
		}
		appr.SnapshotID = snap.SnapshotID
		return CallResponse{
			OK:               true,
			Tool:             "studio_apply_layout",
			Layer:            "grex-builder",
			Status:           "approval_required",
			ReadOnly:         false,
			ApprovalRequired: true,
			ApprovalToken:    appr.Token,
			TargetPath:       targetKey,
			Snapshot:         &snap,
		}, nil
	}

	if _, err := b.approvals.ConsumeToken(req.ApprovalToken, groxybridge.ActionStudioLayoutApply, targetKey); err != nil {
		return CallResponse{}, err
	}

	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		summary = "Applied semantic zero-div layout via Groxy AI Studio"
	}
	res, cmd, err := b.dispatchStudioLayout(ctx, req.StudioURL, req.StudioCSRF, slot, summary, node)
	if err != nil {
		return CallResponse{}, err
	}
	return CallResponse{
		OK:             true,
		Tool:           "studio_apply_layout",
		Layer:          "grex-builder",
		Status:         "ok",
		ReadOnly:       false,
		AllowWriteFlag: true,
		TargetPath:     targetKey,
		Result:         res,
		Command:        cmd,
	}, nil
}

func (b *Bridge) sanitizeAndClassify(desc ToolDescriptor, args map[string]any) (map[string]any, string, bool, error) {
	pathKeys := []string{"file", "template", "db_config", "platform_config", "api_contract", "target"}
	var primaryRel string
	for _, k := range pathKeys {
		raw := strArg(args, k)
		if raw == "" {
			continue
		}
		_, rel, err := security.ConfineWorkspacePath(b.workspaceRoot, raw)
		if err != nil {
			return nil, "", false, err
		}
		args[k] = rel
		if primaryRel == "" || k == "target" || k == "file" {
			primaryRel = rel
		}
	}

	isWrite := desc.Mode == "write"
	switch desc.Name {
	case "grab_format":
		writeBool, _ := args["write"].(bool)
		isWrite = writeBool
		if primaryRel == "" {
			primaryRel = "inline:format"
		}
	case "grab_scaffold":
		dryRun, _ := args["dry_run"].(bool)
		isWrite = !dryRun
	case "create_entry_draft":
		slug := strings.TrimSpace(strArg(args, "slug"))
		if slug == "" {
			slug = "untitled-draft"
		}
		primaryRel = "cms:draft:" + strings.ToLower(slug)
	case "update_entry_layout":
		idStr := strings.TrimSpace(fmt.Sprint(args["entry_id"]))
		if idStr == "" || idStr == "<nil>" {
			idStr = "0"
		}
		primaryRel = "cms:entry:" + idStr
	case "studio_apply_layout":
		slot := strings.TrimSpace(strArg(args, "slot"))
		if slot == "" {
			slot = "main"
		}
		primaryRel = "studio:layout:" + slot
	}
	if primaryRel == "" {
		primaryRel = "mcp:" + desc.Name
	}
	return args, primaryRel, isWrite, nil
}

func (b *Bridge) prepareWriteApproval(desc ToolDescriptor, target string, args map[string]any) (groxybridge.SnapshotRecord, groxybridge.ApprovalRecord, error) {
	var snap groxybridge.SnapshotRecord
	var err error
	if desc.Layer == "core-grab" && !strings.HasPrefix(target, "inline:") && !strings.HasPrefix(target, "mcp:") {
		snap, _, err = b.approvals.CreateSnapshot(b.workspaceRoot, target)
	} else {
		beforeJSON, _ := json.Marshal(args)
		snap, err = b.approvals.CreateVirtualSnapshot(target, string(beforeJSON))
	}
	if err != nil {
		return groxybridge.SnapshotRecord{}, groxybridge.ApprovalRecord{}, err
	}
	appr, err := b.approvals.IssueActionToken(groxybridge.ActionMcpWrite, target)
	if err != nil {
		return groxybridge.SnapshotRecord{}, groxybridge.ApprovalRecord{}, err
	}
	return snap, appr, nil
}

func (b *Bridge) callCoreGrabMCP(ctx context.Context, toolName string, args map[string]any, allowWrite bool) (any, error) {
	if strings.TrimSpace(b.grabBin) == "" {
		return nil, security.NewError(security.CodeDeskE502, 503, "GRAB binary is not configured", nil)
	}
	argv := []string{"mcp", "--workspace", b.workspaceRoot}
	if allowWrite {
		argv = append(argv, "--allow-write")
	}
	for flagName, fileName := range map[string]string{
		"--platform-config": "grab.platform.json",
		"--db-config":       "grab.data.json",
		"--api-contract":    "grab.api.json",
	} {
		if _, _, err := security.ConfineWorkspacePath(b.workspaceRoot, fileName); err == nil {
			if _, statErr := os.Stat(filepath.Join(b.workspaceRoot, fileName)); statErr == nil {
				argv = append(argv, flagName, fileName)
			}
		}
	}

	if err := security.ValidateSafeCommandBinary(b.grabBin, argv); err != nil {
		return nil, err
	}

	cCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Structured argv execution only; zero shell string interpolation.
	cmd := exec.CommandContext(cCtx, b.grabBin, argv...)
	cmd.Dir = b.workspaceRoot

	req1 := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"groxy-desk","version":"1.0.1"}}}`
	req2 := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`
	callObj := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}
	callBytes, err := json.Marshal(callObj)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 400, "failed to encode MCP JSON-RPC arguments", err)
	}
	inputPayload := req1 + "\n" + req2 + "\n" + string(callBytes) + "\n"
	cmd.Stdin = strings.NewReader(inputPayload)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := b.redactor.Redact(strings.TrimSpace(stderr.String()))
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, security.NewError(security.CodeDeskE502, 500, fmt.Sprintf("grab mcp failed: %s", errMsg), err)
	}

	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 0, 16*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rpcResp struct {
			ID     any `json:"id"`
			Result struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &rpcResp); err != nil {
			continue
		}
		idNum, ok := rpcResp.ID.(float64)
		if !ok || int(idNum) != 2 {
			continue
		}
		if rpcResp.Error != nil {
			return nil, security.NewError(security.CodeDeskE101, 422, b.redactor.Redact(rpcResp.Error.Message), nil)
		}
		textPayload := ""
		if len(rpcResp.Result.Content) > 0 {
			textPayload = b.redactor.Redact(rpcResp.Result.Content[0].Text)
		}
		var parsed any
		if textPayload != "" && json.Unmarshal([]byte(textPayload), &parsed) == nil {
			if rpcResp.Result.IsError {
				return parsed, security.NewError(security.CodeDeskE101, 422, extractErrorSummary(parsed, textPayload), nil)
			}
			return parsed, nil
		}
		if rpcResp.Result.IsError {
			return nil, security.NewError(security.CodeDeskE101, 422, firstString(textPayload, "grab mcp tool reported an error"), nil)
		}
		return map[string]any{"text": textPayload}, nil
	}
	return nil, security.NewError(security.CodeDeskE502, 502, "grab mcp returned no tool response", nil)
}

func (b *Bridge) callLoopbackCMSTool(ctx context.Context, cmsURL, cmsToken, toolName string, args map[string]any) (any, int, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(cmsURL), "/") + "/api/mcp/tools/" + url.PathEscape(toolName)
	bodyBytes, err := json.Marshal(map[string]any{
		"arguments": args,
	})
	if err != nil {
		return nil, 0, security.NewError(security.CodeDeskE101, 400, "failed to marshal CMS MCP request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, security.NewError(security.CodeDeskE101, 400, "invalid CMS MCP request URL", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cmsToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cmsToken))
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, 0, security.NewError(security.CodeDeskE502, 502, "failed to contact local GrabLab CMS MCP worker", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPReply))
	if err != nil {
		return nil, resp.StatusCode, security.NewError(security.CodeDeskE502, 502, "failed reading GrabLab CMS MCP response", err)
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		parsed = map[string]any{"raw": b.redactor.Redact(string(raw))}
	}
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return parsed, resp.StatusCode, nil
	}
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, security.NewError(security.CodeDeskE101, resp.StatusCode, extractErrorSummary(parsed, string(raw)), nil)
	}
	return parsed, resp.StatusCode, nil
}

func (b *Bridge) dispatchStudioLayout(ctx context.Context, studioURL, studioCSRF, slot, summary string, node *auditor.GrexNode) (any, map[string]any, error) {
	prevRaw, _ := b.readStudioStateJSON(slot)
	var beforeNode any
	if prevRaw != "" {
		_ = json.Unmarshal([]byte(prevRaw), &beforeNode)
	}

	afterBytes, _ := json.Marshal(node)
	_ = b.writeStudioStateJSON(slot, afterBytes)

	undoCommand := map[string]any{
		"type":    "applyAiLayout",
		"slot":    slot,
		"actor":   "groxy-ai",
		"summary": summary,
		"command": map[string]any{
			"type": "applyAiLayout",
			"slot": slot,
			"tree": node,
		},
		"inverse": map[string]any{
			"type":     "applyAiLayout",
			"slot":     slot,
			"_restore": true,
			"tree":     beforeNode,
		},
		"before": beforeNode,
		"after":  node,
	}

	if strings.TrimSpace(studioURL) == "" {
		return map[string]any{
			"ok":      true,
			"actor":   "groxy-ai",
			"slot":    slot,
			"command": undoCommand,
			"tree":    node,
		}, undoCommand, nil
	}

	if err := ValidateLoopbackHTTPURL(studioURL); err != nil {
		return nil, nil, err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(studioURL), "/") + "/api/mcp/layout"
	payloadBytes, err := json.Marshal(map[string]any{
		"slot":     slot,
		"actor":    "groxy-ai",
		"summary":  summary,
		"layout":   node,
		"nextTree": node,
	})
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, nil, security.NewError(security.CodeDeskE101, 400, "invalid Grex Theme Builder URL", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(studioCSRF) != "" {
		req.Header.Set("X-CSRF-Token", strings.TrimSpace(studioCSRF))
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, nil, security.NewError(security.CodeDeskE502, 502, "failed contacting local Grex Theme Builder worker", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxHTTPReply))
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, nil, newUpstream422Error(parsed)
	}
	if resp.StatusCode >= 400 {
		return nil, nil, security.NewError(security.CodeDeskE101, resp.StatusCode, extractErrorSummary(parsed, string(raw)), nil)
	}
	if cmd, ok := parsed["command"].(map[string]any); ok {
		return parsed, cmd, nil
	}
	return parsed, undoCommand, nil
}

func (b *Bridge) readStudioStateJSON(slot string) (string, error) {
	if strings.TrimSpace(b.stateDir) == "" {
		return "null", nil
	}
	p := filepath.Join(b.stateDir, "studio-slot-"+sanitizeSlot(slot)+".json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return "null", err
	}
	return string(raw), nil
}

func (b *Bridge) writeStudioStateJSON(slot string, raw []byte) error {
	if strings.TrimSpace(b.stateDir) == "" {
		return nil
	}
	if err := os.MkdirAll(b.stateDir, 0o700); err != nil {
		return err
	}
	p := filepath.Join(b.stateDir, "studio-slot-"+sanitizeSlot(slot)+".json")
	return os.WriteFile(p, raw, 0o600)
}

func (b *Bridge) readConfinedOptionalFile(relName string) (string, error) {
	abs, _, err := security.ConfineWorkspacePath(b.workspaceRoot, relName)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ValidateLoopbackHTTPURL enforces loopback-only http(s) URLs (DESK-E501).
func ValidateLoopbackHTTPURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "*") {
		return security.NewError(security.CodeDeskE501, 403, "non-loopback or wildcard URL rejected", nil)
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return security.NewError(security.CodeDeskE501, 403, "invalid URL format", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return security.NewError(security.CodeDeskE501, 403, "only http/https loopback URLs are allowed", nil)
	}
	hostPort := u.Host
	if !strings.Contains(hostPort, ":") {
		if scheme == "https" {
			hostPort = net.JoinHostPort(u.Hostname(), "443")
		} else {
			hostPort = net.JoinHostPort(u.Hostname(), "80")
		}
	}
	if _, err := security.ValidateLoopbackBind(hostPort); err != nil {
		return err
	}
	return nil
}

func validateRawGrexLayout(raw any) ([]auditor.ValidationViolation, error) {
	node, err := decodeGrexNode(raw)
	if err != nil {
		return nil, err
	}
	return auditor.ValidateGrexTree(node), nil
}

func decodeGrexNode(raw any) (*auditor.GrexNode, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var node auditor.GrexNode
	if err := json.Unmarshal(b, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func newValidationError(vs []auditor.ValidationViolation) error {
	msg := "Grex layout or WCAG AA validation failed"
	if len(vs) > 0 {
		msg = fmt.Sprintf("Grex layout or WCAG AA validation failed (code=-32602): %s", vs[0].Message)
	}
	return security.NewError(CodeDeskE422, 422, msg, nil)
}

func newUpstream422Error(payload any) error {
	msg := extractErrorSummary(payload, "Grex layout or WCAG AA validation failed (-32602)")
	if !strings.Contains(msg, "-32602") {
		msg = msg + " (code=-32602)"
	}
	return security.NewError(CodeDeskE422, 422, msg, nil)
}

func extractErrorSummary(payload any, fallback string) string {
	if m, ok := payload.(map[string]any); ok {
		if me, ok := m["mcp_error"].(map[string]any); ok {
			if msg, ok := me["message"].(string); ok && msg != "" {
				return msg
			}
		}
		if e, ok := m["error"].(string); ok && e != "" {
			return e
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "request failed"
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func strArg(m map[string]any, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func firstMapValue(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstString(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func normalizeMCPTarget(t string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(t, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "./")
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func sanitizeSlot(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "main"
	}
	return b.String()
}

// Unused import safety helper for strconv when formatting integer args if needed.
var _ = strconv.Itoa
