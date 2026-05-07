// api/internal/handler/landing/embed.go
package landing

import (
	"embed"
	"io/fs"
)

//go:embed templates
var templatesDir embed.FS

//go:embed static
var staticDir embed.FS

// Templates returns a sub-FS rooted at the templates directory.
func Templates() fs.FS {
	sub, err := fs.Sub(templatesDir, "templates")
	if err != nil {
		panic(err)
	}
	return sub
}

// Static returns a sub-FS rooted at the static directory (CSS, etc.).
func Static() fs.FS {
	sub, err := fs.Sub(staticDir, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
