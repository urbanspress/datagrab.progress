package dbstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"groxy-desk/internal/security"
)

const maxBridgeResponse = 8 << 20

type bridgeSQLiteTarget struct {
	Engine     string `json:"engine"`
	SQLitePath string `json:"sqlite_path"`
}

type bridgeSQLiteColumn struct {
	ColumnMeta
	DefaultValue string `json:"default_value,omitempty"`
}

func bridgeSQLiteColumns(wire []bridgeSQLiteColumn) []ColumnMeta {
	columns := make([]ColumnMeta, 0, len(wire))
	for _, column := range wire {
		column.DefaultVal = column.DefaultValue
		columns = append(columns, column.ColumnMeta)
	}
	return columns
}

func bridgeTarget(target TargetSpec) (bridgeSQLiteTarget, error) {
	if target.DirectHost != "" || target.TunnelID == "" || target.BridgeBearer == "" ||
		strings.ContainsAny(target.BridgeBearer, "\r\n") {
		return bridgeSQLiteTarget{}, security.NewError(security.CodeDeskE405, 403, "remote SQLite requires a pinned Gashboard bridge and its bearer", nil)
	}
	path := target.SQLitePath
	if path == "" || strings.ContainsAny(path, "\x00\\:") || strings.HasPrefix(path, "/") {
		return bridgeSQLiteTarget{}, security.NewError(security.CodeGrabE401, 403, "remote SQLite requires a path under the bridge's SQLite root", nil)
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." || security.IsSensitiveSegment(component) {
			return bridgeSQLiteTarget{}, security.NewError(security.CodeGrabE401, 403, "remote SQLite path must be confined to the bridge's SQLite root", nil)
		}
	}
	return bridgeSQLiteTarget{Engine: "sqlite", SQLitePath: path}, nil
}

func (m *Manager) requestSQLiteBridge(ctx context.Context, target TargetSpec, endpoint string, payload, out any) error {
	tun := m.tunnels.activeTunnel(target.TunnelID)
	if tun == nil || tun.view.Engine != "sqlite-bridge" || tun.sshClient == nil {
		return security.NewError(security.CodeDeskE405, 403, "open an authenticated, pinned Gashboard bridge tunnel first", nil)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+tun.remoteAddr+endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	m.redactor.Register(target.BridgeBearer)
	request.Header.Set("Authorization", "Bearer "+target.BridgeBearer)
	request.Header.Set("Content-Type", "application/json")
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext:       m.tunnels.pinDial(tun),
	}
	res, err := transport.RoundTrip(request)
	if err != nil {
		return bridgeResponseError()
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return bridgeResponseError()
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBridgeResponse+1))
	if err != nil || len(data) > maxBridgeResponse || json.Unmarshal(data, out) != nil {
		return bridgeResponseError()
	}
	return nil
}

func bridgeResponseError() error {
	return security.NewError(security.CodeDeskE502, 502, "the pinned Gashboard bridge could not provide this read-only SQLite capability", nil)
}

func (m *Manager) remoteSQLiteSchema(ctx context.Context, target TargetSpec) (map[string]any, error) {
	payload, err := bridgeTarget(target)
	if err != nil {
		return nil, err
	}
	var result struct {
		OK     bool   `json:"ok"`
		Engine string `json:"engine"`
		Tables []struct {
			TableSchema
			Columns []bridgeSQLiteColumn `json:"columns"`
			Indexes []struct {
				Name string `json:"name"`
			} `json:"indexes"`
		} `json:"tables"`
	}
	if err := m.requestSQLiteBridge(ctx, target, "/v1/db/schema", payload, &result); err != nil {
		return nil, err
	}
	if !result.OK || result.Engine != "sqlite" || result.Tables == nil {
		return nil, bridgeResponseError()
	}
	tables := make([]TableSchema, 0, len(result.Tables))
	for _, table := range result.Tables {
		table.TableSchema.Columns = bridgeSQLiteColumns(table.Columns)
		for _, index := range table.Indexes {
			table.TableSchema.Indexes = append(table.TableSchema.Indexes, index.Name)
		}
		tables = append(tables, table.TableSchema)
	}
	return map[string]any{"ok": true, "engine": "sqlite-bridge", "target_label": target.SQLitePath + " via " + target.TunnelID, "tables": tables}, nil
}

func (m *Manager) remoteSQLiteRows(ctx context.Context, req RowsRequest) (map[string]any, error) {
	target := req.Target
	payload, err := bridgeTarget(target)
	if err != nil {
		return nil, err
	}
	// The explicit target shadows the Desk target, so browser credentials are
	// passed only in the Authorization header and Desk tunnel IDs stay local.
	request := struct {
		RowsRequest
		Target bridgeSQLiteTarget `json:"target"`
	}{RowsRequest: req, Target: payload}
	var result struct {
		OK         bool                 `json:"ok"`
		Engine     string               `json:"engine"`
		Table      string               `json:"table"`
		Page       string               `json:"page"`
		Limit      string               `json:"limit"`
		TotalRows  string               `json:"total_rows"`
		TotalPages string               `json:"total_pages"`
		Columns    []bridgeSQLiteColumn `json:"columns"`
		Rows       []map[string]*string `json:"rows"`
	}
	if err := m.requestSQLiteBridge(ctx, target, "/v1/db/rows", request, &result); err != nil {
		return nil, err
	}
	if !result.OK || result.Engine != "sqlite" || result.Table != req.Table || len(result.Rows) > MaxRowsPerPage {
		return nil, bridgeResponseError()
	}
	rows := make([]map[string]any, 0, len(result.Rows))
	for _, row := range result.Rows {
		values := make(map[string]any, len(row))
		for column, value := range row {
			if value == nil {
				values[column] = nil
			} else {
				values[column] = *value
			}
		}
		rows = append(rows, values)
	}
	return map[string]any{"ok": true, "engine": "sqlite-bridge", "table": result.Table, "page": result.Page, "limit": result.Limit,
		"total_rows": result.TotalRows, "total_pages": result.TotalPages, "columns": bridgeSQLiteColumns(result.Columns), "rows": rows}, nil
}
