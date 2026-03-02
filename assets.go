package fastmd

import "embed"

// WebFS embeds all web assets (templates + static files) into the binary.
//
//go:embed web
var WebFS embed.FS
