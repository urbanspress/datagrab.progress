package groxybridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"groxy-desk/internal/security"
)

const (
	// ActionApplyMutation identifies an approval token bound to mutating a workspace file.
	ActionApplyMutation = "apply_mutation"
	// ActionGrabmanInit identifies an approval token bound to initializing a grabman workspace.
	ActionGrabmanInit = "grabman_init"
	// ActionMcpWrite identifies an approval token bound to an MCP or CMS write invocation.
	ActionMcpWrite = "mcp_write"
	// ActionStudioLayoutApply identifies an approval token bound to dispatching a studio Grex layout.
	ActionStudioLayoutApply = "studio_layout_apply"
	// ActionBoostApprove identifies an approval token bound to applying a Boost remediation diff.
	ActionBoostApprove = "boost_approve"

	defaultApprovalTTL = 15 * time.Minute
)

var validSnapshotIDPattern = regexp.MustCompile(`^snap-[a-f0-9]{16,64}$`)

// SnapshotRecord records an immutable pre-mutation SHA-256 backup of a workspace target.
type SnapshotRecord struct {
	SnapshotID   string `json:"snapshot_id"`
	TargetPath   string `json:"target_path"`
	Existed      bool   `json:"existed"`
	Virtual      bool   `json:"virtual,omitempty"`
	BeforeSHA256 string `json:"before_sha256"`
	ByteLength   string `json:"byte_length"`
	CreatedAt    string `json:"created_at"`
	Restored     bool   `json:"restored"`
	RestoredAt   string `json:"restored_at,omitempty"`
}

// ApprovalRecord tracks a single-use, target-bound GROXY-SAFE-001 approval token.
type ApprovalRecord struct {
	Token           string
	Action          string
	TargetPath      string
	ProposedSHA256  string
	ProposedContent string
	HasProposed     bool
	SnapshotID      string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	Used            bool
	UsedAt          time.Time
}

// ApplyRequest describes a user-approved workspace mutation request.
type ApplyRequest struct {
	ApprovalToken string  `json:"approval_token"`
	TargetPath    string  `json:"target_path"`
	Content       *string `json:"content,omitempty"`
}

// ApplyResult summarizes an executed file mutation and its rollback snapshot reference.
type ApplyResult struct {
	OK            bool           `json:"ok"`
	TargetPath    string         `json:"target_path"`
	SnapshotID    string         `json:"snapshot_id"`
	AppliedSHA256 string         `json:"applied_sha256"`
	ByteLength    string         `json:"byte_length"`
	Snapshot      SnapshotRecord `json:"snapshot"`
}

// RestoreResult summarizes a 1-click snapshot rollback operation.
type RestoreResult struct {
	OK              bool           `json:"ok"`
	SnapshotID      string         `json:"snapshot_id"`
	TargetPath      string         `json:"target_path"`
	RestoredExisted bool           `json:"restored_existed"`
	RestoredSHA256  string         `json:"restored_sha256"`
	RestoredContent string         `json:"restored_content,omitempty"`
	Snapshot        SnapshotRecord `json:"snapshot"`
}

// ApprovalManager enforces GROXY-SAFE-001 single-use target-bound approvals and pre-mutation SHA-256 snapshots.
type ApprovalManager struct {
	mu          sync.Mutex
	snapshotDir string
	redactor    *security.SecretRedactor
	tokens      map[string]*ApprovalRecord
	snapshots   map[string]*SnapshotRecord
}

// NewApprovalManager initializes an approval and snapshot store backed by stateDir/snapshots.
func NewApprovalManager(stateDir string, redactor *security.SecretRedactor) (*ApprovalManager, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, security.NewError(security.CodeDeskE101, 500, "state directory cannot be empty", nil)
	}
	absState, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed to resolve state directory", err)
	}
	snapDir := filepath.Join(absState, "snapshots")
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		return nil, security.NewError(security.CodeDeskE101, 500, "failed to initialize snapshot directory", err)
	}
	if redactor == nil {
		redactor = security.NewSecretRedactor()
	}

	mgr := &ApprovalManager{
		snapshotDir: snapDir,
		redactor:    redactor,
		tokens:      make(map[string]*ApprovalRecord),
		snapshots:   make(map[string]*SnapshotRecord),
	}
	if err := mgr.loadExistingSnapshots(); err != nil {
		return nil, err
	}
	return mgr, nil
}

func (m *ApprovalManager) loadExistingSnapshots() error {
	entries, err := os.ReadDir(m.snapshotDir)
	if err != nil {
		return security.NewError(security.CodeDeskE101, 500, "failed to read snapshot metadata directory", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validSnapshotIDPattern.MatchString(id) {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(m.snapshotDir, entry.Name()))
		if readErr != nil {
			continue
		}
		var rec SnapshotRecord
		if json.Unmarshal(raw, &rec) == nil && validSnapshotIDPattern.MatchString(rec.SnapshotID) {
			copyRec := rec
			m.snapshots[rec.SnapshotID] = &copyRec
		}
	}
	return nil
}

// CreateSnapshot captures an immutable pre-mutation SHA-256 snapshot of a workspace target.
func (m *ApprovalManager) CreateSnapshot(workspaceRoot, targetPath string) (SnapshotRecord, string, error) {
	absTarget, relTarget, err := security.ConfineWorkspacePath(workspaceRoot, targetPath)
	if err != nil {
		return SnapshotRecord{}, "", err
	}

	existed := false
	var beforeBytes []byte
	var beforeSHA string

	info, statErr := os.Stat(absTarget)
	if statErr == nil {
		if info.IsDir() {
			return SnapshotRecord{}, "", security.NewError(security.CodeDeskE101, 400, "snapshot target must be a file, not a directory", nil)
		}
		beforeBytes, err = os.ReadFile(absTarget)
		if err != nil {
			return SnapshotRecord{}, "", security.NewError(security.CodeDeskE101, 500, "failed to read file for pre-mutation snapshot", err)
		}
		existed = true
		beforeSHA = security.SHA256Hex(beforeBytes)
	} else if !os.IsNotExist(statErr) {
		return SnapshotRecord{}, "", security.NewError(security.CodeDeskE101, 500, "failed to stat target file", statErr)
	}

	randHex, err := security.RandomTokenHex(16)
	if err != nil {
		return SnapshotRecord{}, "", security.NewError(security.CodeDeskE101, 500, "failed to generate snapshot id", err)
	}
	snapID := "snap-" + randHex
	rec := SnapshotRecord{
		SnapshotID:   snapID,
		TargetPath:   relTarget,
		Existed:      existed,
		BeforeSHA256: beforeSHA,
		ByteLength:   strconv.Itoa(len(beforeBytes)),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.persistSnapshotLocked(rec, beforeBytes); err != nil {
		return SnapshotRecord{}, "", err
	}
	copyRec := rec
	m.snapshots[snapID] = &copyRec
	return rec, string(beforeBytes), nil
}

func (m *ApprovalManager) persistSnapshotLocked(rec SnapshotRecord, content []byte) error {
	metaPath := filepath.Join(m.snapshotDir, rec.SnapshotID+".json")
	dataPath := filepath.Join(m.snapshotDir, rec.SnapshotID+".bak")

	metaBytes, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return security.NewError(security.CodeDeskE101, 500, "failed to serialize snapshot metadata", err)
	}
	if rec.Existed {
		if err := os.WriteFile(dataPath, content, 0o600); err != nil {
			return security.NewError(security.CodeDeskE101, 500, "failed to write snapshot backup bytes", err)
		}
	}
	if err := os.WriteFile(metaPath, metaBytes, 0o600); err != nil {
		return security.NewError(security.CodeDeskE101, 500, "failed to write snapshot metadata", err)
	}
	return nil
}

// CreateVirtualSnapshot persists a SHA-256 snapshot for non-disk targets (e.g., studio slots or CMS entries).
func (m *ApprovalManager) CreateVirtualSnapshot(targetKey, beforeContent string) (SnapshotRecord, error) {
	norm := normalizeTarget(targetKey)
	if norm == "" || norm == "." {
		norm = "studio:inline"
	}
	raw := []byte(beforeContent)
	randHex, err := security.RandomTokenHex(16)
	if err != nil {
		return SnapshotRecord{}, security.NewError(security.CodeDeskE101, 500, "failed to generate snapshot id", err)
	}
	snapID := "snap-" + randHex
	rec := SnapshotRecord{
		SnapshotID:   snapID,
		TargetPath:   norm,
		Existed:      true,
		Virtual:      true,
		BeforeSHA256: security.SHA256Hex(raw),
		ByteLength:   strconv.Itoa(len(raw)),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.persistSnapshotLocked(rec, raw); err != nil {
		return SnapshotRecord{}, err
	}
	copyRec := rec
	m.snapshots[snapID] = &copyRec
	return rec, nil
}

// IssueMutationToken captures a pre-mutation snapshot and mints a target-bound single-use token.
func (m *ApprovalManager) IssueMutationToken(workspaceRoot, targetPath, proposedContent string) (ApprovalRecord, SnapshotRecord, string, error) {
	return m.issueMutationTokenForAction(ActionApplyMutation, workspaceRoot, targetPath, proposedContent)
}

// IssueBoostToken captures a disk or virtual snapshot and mints a single-use ActionBoostApprove token.
func (m *ApprovalManager) IssueBoostToken(workspaceRoot, targetPath, proposedContent, fallbackBeforeContent string) (ApprovalRecord, SnapshotRecord, string, error) {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" || strings.HasPrefix(trimmed, "studio:") || strings.HasPrefix(trimmed, "cms:") || strings.HasPrefix(trimmed, "inline:") {
		key := trimmed
		if key == "" {
			key = "inline:boost"
		}
		snap, err := m.CreateVirtualSnapshot(key, fallbackBeforeContent)
		if err != nil {
			return ApprovalRecord{}, SnapshotRecord{}, "", err
		}
		rawToken, err := security.RandomTokenHex(24)
		if err != nil {
			return ApprovalRecord{}, SnapshotRecord{}, "", security.NewError(security.CodeDeskE101, 500, "failed to mint boost approval token", err)
		}
		token := "groxy-appr-" + rawToken
		m.redactor.Register(token)
		now := time.Now().UTC()
		rec := &ApprovalRecord{
			Token:           token,
			Action:          ActionBoostApprove,
			TargetPath:      snap.TargetPath,
			ProposedSHA256:  security.SHA256Hex([]byte(proposedContent)),
			ProposedContent: proposedContent,
			HasProposed:     true,
			SnapshotID:      snap.SnapshotID,
			CreatedAt:       now,
			ExpiresAt:       now.Add(defaultApprovalTTL),
		}
		m.mu.Lock()
		m.tokens[token] = rec
		m.mu.Unlock()
		return *rec, snap, fallbackBeforeContent, nil
	}
	return m.issueMutationTokenForAction(ActionBoostApprove, workspaceRoot, trimmed, proposedContent)
}

func (m *ApprovalManager) issueMutationTokenForAction(action, workspaceRoot, targetPath, proposedContent string) (ApprovalRecord, SnapshotRecord, string, error) {
	snap, currentContent, err := m.CreateSnapshot(workspaceRoot, targetPath)
	if err != nil {
		return ApprovalRecord{}, SnapshotRecord{}, "", err
	}

	rawToken, err := security.RandomTokenHex(24)
	if err != nil {
		return ApprovalRecord{}, SnapshotRecord{}, "", security.NewError(security.CodeDeskE101, 500, "failed to mint approval token", err)
	}
	token := "groxy-appr-" + rawToken
	m.redactor.Register(token)

	now := time.Now().UTC()
	rec := &ApprovalRecord{
		Token:           token,
		Action:          action,
		TargetPath:      snap.TargetPath,
		ProposedSHA256:  security.SHA256Hex([]byte(proposedContent)),
		ProposedContent: proposedContent,
		HasProposed:     true,
		SnapshotID:      snap.SnapshotID,
		CreatedAt:       now,
		ExpiresAt:       now.Add(defaultApprovalTTL),
	}

	m.mu.Lock()
	m.tokens[token] = rec
	m.mu.Unlock()

	return *rec, snap, currentContent, nil
}

// IssueActionToken mints a single-use approval token scoped to an action and target path (e.g., grabman init).
func (m *ApprovalManager) IssueActionToken(action, targetPath string) (ApprovalRecord, error) {
	normTarget := normalizeTarget(targetPath)
	if strings.TrimSpace(action) == "" {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE101, 400, "approval action cannot be empty", nil)
	}
	rawToken, err := security.RandomTokenHex(24)
	if err != nil {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE101, 500, "failed to mint action approval token", err)
	}
	token := "groxy-appr-" + rawToken
	m.redactor.Register(token)

	now := time.Now().UTC()
	rec := &ApprovalRecord{
		Token:      token,
		Action:     action,
		TargetPath: normTarget,
		CreatedAt:  now,
		ExpiresAt:  now.Add(defaultApprovalTTL),
	}

	m.mu.Lock()
	m.tokens[token] = rec
	m.mu.Unlock()

	return *rec, nil
}

// ConsumeToken validates and atomically marks a single-use target-bound token as consumed.
// Returns 403 DESK-E403 on missing/unknown/expired/mismatched tokens and 409 DESK-E409 on replay.
func (m *ApprovalManager) ConsumeToken(token, expectedAction, expectedTarget string) (ApprovalRecord, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE403, 403, "missing approval_token", nil)
	}
	normExpected := normalizeTarget(expectedTarget)

	m.mu.Lock()
	defer m.mu.Unlock()

	var matched *ApprovalRecord
	for storedToken, rec := range m.tokens {
		if security.ConstantTimeEqual(storedToken, trimmed) {
			matched = rec
			break
		}
	}
	if matched == nil {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE403, 403, "invalid or unrecognized approval_token", nil)
	}
	if matched.Used {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE409, 409, "approval_token already consumed (replay rejected)", nil)
	}
	if time.Now().UTC().After(matched.ExpiresAt) {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE403, 403, "approval_token has expired", nil)
	}
	if matched.Action != expectedAction {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE403, 403, "approval_token action mismatch", nil)
	}
	if matched.TargetPath != normExpected {
		return ApprovalRecord{}, security.NewError(security.CodeDeskE403, 403, "approval_token target mismatch", nil)
	}

	matched.Used = true
	matched.UsedAt = time.Now().UTC()
	copyRec := *matched
	return copyRec, nil
}

// ApplyMutation verifies workspace confinement and single-use approval, writes atomically, and returns snapshot metadata.
func (m *ApprovalManager) ApplyMutation(workspaceRoot string, req ApplyRequest) (ApplyResult, error) {
	return m.ApplyMutationForAction(workspaceRoot, ActionApplyMutation, req)
}

// ApplyMutationForAction applies a file or virtual mutation gated by expectedAction + GROXY-SAFE-001 token.
func (m *ApprovalManager) ApplyMutationForAction(workspaceRoot, expectedAction string, req ApplyRequest) (ApplyResult, error) {
	trimmedTarget := strings.TrimSpace(req.TargetPath)
	isVirtual := strings.HasPrefix(trimmedTarget, "studio:") || strings.HasPrefix(trimmedTarget, "cms:") || strings.HasPrefix(trimmedTarget, "inline:")
	if isVirtual {
		norm := normalizeTarget(trimmedTarget)
		rec, err := m.ConsumeToken(req.ApprovalToken, expectedAction, norm)
		if err != nil {
			return ApplyResult{}, err
		}
		finalContent := rec.ProposedContent
		if req.Content != nil {
			finalContent = *req.Content
		} else if !rec.HasProposed {
			return ApplyResult{}, security.NewError(security.CodeDeskE101, 400, "mutation content is required", nil)
		}
		m.mu.Lock()
		snapRec, ok := m.snapshots[rec.SnapshotID]
		var snapCopy SnapshotRecord
		if ok {
			snapCopy = *snapRec
		}
		m.mu.Unlock()
		return ApplyResult{
			OK:            true,
			TargetPath:    norm,
			SnapshotID:    rec.SnapshotID,
			AppliedSHA256: security.SHA256Hex([]byte(finalContent)),
			ByteLength:    strconv.Itoa(len(finalContent)),
			Snapshot:      snapCopy,
		}, nil
	}

	absTarget, relTarget, err := security.ConfineWorkspacePath(workspaceRoot, req.TargetPath)
	if err != nil {
		return ApplyResult{}, err
	}

	rec, err := m.ConsumeToken(req.ApprovalToken, expectedAction, relTarget)
	if err != nil {
		return ApplyResult{}, err
	}

	finalContent := rec.ProposedContent
	if req.Content != nil {
		finalContent = *req.Content
	} else if !rec.HasProposed {
		return ApplyResult{}, security.NewError(security.CodeDeskE101, 400, "mutation content is required", nil)
	}

	if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
		return ApplyResult{}, security.NewError(security.CodeDeskE101, 500, "failed to prepare target parent directory", err)
	}

	if err := writeFileAtomic(absTarget, []byte(finalContent), 0o644); err != nil {
		return ApplyResult{}, security.NewError(security.CodeDeskE101, 500, "failed to write approved file mutation", err)
	}

	m.mu.Lock()
	snapRec, ok := m.snapshots[rec.SnapshotID]
	var snapCopy SnapshotRecord
	if ok {
		snapCopy = *snapRec
	}
	m.mu.Unlock()

	dataBytes := []byte(finalContent)
	return ApplyResult{
		OK:            true,
		TargetPath:    relTarget,
		SnapshotID:    rec.SnapshotID,
		AppliedSHA256: security.SHA256Hex(dataBytes),
		ByteLength:    strconv.Itoa(len(dataBytes)),
		Snapshot:      snapCopy,
	}, nil
}

// ListSnapshots returns all recorded snapshots newest-first.
func (m *ApprovalManager) ListSnapshots() []SnapshotRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]SnapshotRecord, 0, len(m.snapshots))
	for _, snap := range m.snapshots {
		out = append(out, *snap)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

// RestoreSnapshot restores the workspace file back to its pre-mutation SHA-256 snapshot.
func (m *ApprovalManager) RestoreSnapshot(workspaceRoot, snapshotID string) (RestoreResult, error) {
	id := strings.TrimSpace(snapshotID)
	if !validSnapshotIDPattern.MatchString(id) {
		return RestoreResult{}, security.NewError(security.CodeDeskE101, 400, "invalid snapshot_id format", nil)
	}

	m.mu.Lock()
	snapPtr, ok := m.snapshots[id]
	if !ok {
		m.mu.Unlock()
		return RestoreResult{}, security.NewError(security.CodeDeskE101, 404, "snapshot not found", nil)
	}
	snap := *snapPtr
	m.mu.Unlock()

	if snap.Virtual {
		dataPath := filepath.Join(m.snapshotDir, snap.SnapshotID+".bak")
		origBytes, err := os.ReadFile(dataPath)
		if err != nil {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "failed to read virtual snapshot backup bytes", err)
		}
		restoredSHA := security.SHA256Hex(origBytes)
		if restoredSHA != snap.BeforeSHA256 {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "virtual snapshot backup checksum mismatch", nil)
		}
		m.mu.Lock()
		if cur, exists := m.snapshots[id]; exists {
			cur.Restored = true
			cur.RestoredAt = time.Now().UTC().Format(time.RFC3339Nano)
			_ = m.persistSnapshotLocked(*cur, origBytes)
			snap = *cur
		}
		m.mu.Unlock()
		return RestoreResult{
			OK:              true,
			SnapshotID:      snap.SnapshotID,
			TargetPath:      snap.TargetPath,
			RestoredExisted: true,
			RestoredSHA256:  restoredSHA,
			RestoredContent: string(origBytes),
			Snapshot:        snap,
		}, nil
	}

	absTarget, relTarget, err := security.ConfineWorkspacePath(workspaceRoot, snap.TargetPath)
	if err != nil {
		return RestoreResult{}, err
	}

	var restoredSHA string
	if !snap.Existed {
		if rmErr := os.Remove(absTarget); rmErr != nil && !os.IsNotExist(rmErr) {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "failed to remove file created after snapshot", rmErr)
		}
	} else {
		dataPath := filepath.Join(m.snapshotDir, snap.SnapshotID+".bak")
		origBytes, err := os.ReadFile(dataPath)
		if err != nil {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "failed to read snapshot backup bytes", err)
		}
		if gotSHA := security.SHA256Hex(origBytes); gotSHA != snap.BeforeSHA256 {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "snapshot backup integrity checksum mismatch", nil)
		}
		if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "failed to prepare restore directory", err)
		}
		if err := writeFileAtomic(absTarget, origBytes, 0o644); err != nil {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "failed to restore snapshot bytes", err)
		}
		verifyBytes, err := os.ReadFile(absTarget)
		if err != nil {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "failed to verify restored file", err)
		}
		restoredSHA = security.SHA256Hex(verifyBytes)
		if restoredSHA != snap.BeforeSHA256 {
			return RestoreResult{}, security.NewError(security.CodeDeskE101, 500, "restored file checksum mismatch", nil)
		}
	}

	m.mu.Lock()
	if cur, exists := m.snapshots[id]; exists {
		cur.Restored = true
		cur.RestoredAt = time.Now().UTC().Format(time.RFC3339Nano)
		var content []byte
		if cur.Existed {
			content, _ = os.ReadFile(filepath.Join(m.snapshotDir, cur.SnapshotID+".bak"))
		}
		_ = m.persistSnapshotLocked(*cur, content)
		snap = *cur
	}
	m.mu.Unlock()

	return RestoreResult{
		OK:              true,
		SnapshotID:      snap.SnapshotID,
		TargetPath:      relTarget,
		RestoredExisted: snap.Existed,
		RestoredSHA256:  restoredSHA,
		Snapshot:        snap,
	}, nil
}

func normalizeTarget(target string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "./")
	cleaned = strings.Trim(cleaned, "/")
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func writeFileAtomic(dest string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".groxy-desk-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	cleanup = false
	return nil
}
