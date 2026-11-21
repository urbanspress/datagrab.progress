package dbstudio

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"

	"groxy-desk/internal/security"
)

type ExportDiagramRequest struct {
	Target TargetSpec `json:"target"`
	Path   string     `json:"path"`
}

type diagramEntity struct {
	TableSchema
	ID            string
	Relationships []diagramRelationship
}

type diagramRelationship struct {
	Label    string
	TargetID string
}

var diagramHTML = template.Must(template.New("schema").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'">
<title>Database entity-relationship diagram</title>
<style>
:root { color-scheme: light dark; }
body { margin: 2rem; font: 1rem/1.5 system-ui, sans-serif; }
main { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: 1rem; }
article { border: 1px solid; border-radius: 0.5rem; padding: 1rem; overflow-wrap: anywhere; }
article:target { outline: 2px solid; outline-offset: 3px; }
h2 { margin-top: 0; }
:focus-visible { outline: 2px solid; outline-offset: 3px; }
</style>
</head>
<body>
<header><h1>Database entity-relationship diagram</h1><p>{{.Target}}</p></header>
<main>
{{range .Entities}}
<article id="{{.ID}}" tabindex="-1" aria-labelledby="{{.ID}}-name">
<h2 id="{{.ID}}-name">{{.Schema}}.{{.Name}}</h2>
<h3>Columns</h3>
<ul>{{range .Columns}}<li>{{.Name}} · {{.Type}}{{if .PrimaryKey}} · primary key{{end}}{{if .Nullable}} · nullable{{end}}</li>{{end}}</ul>
{{if .Relationships}}<h3>Foreign keys</h3><ul>{{range .Relationships}}
<li>{{if .TargetID}}<a href="#{{.TargetID}}">{{.Label}}</a>{{else}}{{.Label}}{{end}}</li>
{{end}}</ul>{{end}}
{{if .Indexes}}<h3>Indexes</h3><ul>{{range .Indexes}}<li>{{.}}</li>{{end}}</ul>{{end}}
</article>
{{else}}<p>No tables found.</p>{{end}}
</main>
</body>
</html>
`))

// ExportDiagram saves the read-only schema and its relationships as standalone HTML.
func (m *Manager) ExportDiagram(ctx context.Context, r ExportDiagramRequest) (map[string]any, error) {
	if !strings.EqualFold(filepath.Ext(r.Path), ".html") {
		return nil, security.NewError(security.CodeDeskE101, 422, "choose an .html workspace path for the diagram", nil)
	}
	_, path, err := security.ConfineWorkspacePath(m.workspaceRoot, r.Path)
	if err != nil {
		return nil, err
	}
	catalog, err := m.GetSchema(ctx, r.Target)
	if err != nil {
		return nil, err
	}
	tables := catalog["tables"].([]TableSchema)
	entities := make([]diagramEntity, len(tables))
	ids := make(map[[2]string]string, len(tables))
	for i, table := range tables {
		id := fmt.Sprintf("table-%d", i+1)
		entities[i] = diagramEntity{TableSchema: table, ID: id}
		ids[[2]string{table.Schema, table.Name}] = id
	}
	for i := range entities {
		entity := &entities[i]
		for _, key := range entity.ForeignKeys {
			schema := firstNonEmpty(key.RefSchema, entity.Schema)
			entity.Relationships = append(entity.Relationships, diagramRelationship{
				Label:    fmt.Sprintf("%s → %s.%s.%s", key.Column, schema, key.RefTable, key.RefColumn),
				TargetID: ids[[2]string{schema, key.RefTable}],
			})
		}
	}
	data := struct {
		Target   string
		Entities []diagramEntity
	}{catalog["target_label"].(string), entities}
	if err := m.writeExport(ctx, path, func(w io.Writer) error { return diagramHTML.Execute(w, data) }); err != nil {
		return nil, err
	}
	return map[string]any{"path": path, "engine": catalog["engine"], "tables_exported": fmt.Sprint(len(tables))}, nil
}
