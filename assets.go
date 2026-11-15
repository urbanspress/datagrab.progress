package groxydesk

import (
	"embed"
	"io/fs"
)

//go:embed views public
var embeddedAssets embed.FS

// ViewsFS returns the embedded Pure Grex JS v1.0.0 view templates subtree.
func ViewsFS() (fs.FS, error) {
	return fs.Sub(embeddedAssets, "views")
}

// PublicFS returns the embedded Pure Grex JS v1.0.0 static assets subtree.
func PublicFS() (fs.FS, error) {
	return fs.Sub(embeddedAssets, "public")
}
