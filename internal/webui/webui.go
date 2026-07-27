// Package webui embeds the compiled Vue3 admin UI (see web/) so it ships
// inside the server binary — no separate static file deployment needed.
package webui

import "embed"

//go:embed all:dist
var Assets embed.FS
