// Package docs embeds the application's documentation: the API description swag writes from the
// routes' comments and the architecture map kept with the code.
package docs

import (
	_ "embed"

	"github.com/MyAppLibs/go-pkg/starter"
)

//go:embed swagger.json
var api []byte

//go:embed architecture.md
var architecture []byte

// Docs returns the API description and the architecture map, as the starter serves them.
func Docs() starter.Docs {
	return starter.Docs{API: api, Architecture: architecture}
}
