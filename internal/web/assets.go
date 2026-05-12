package web

import "embed"

// assetsFS holds the static UI files (HTML, CSS, JS) shipped inside the
// binary. The "assets" prefix is stripped at serve time so URLs are clean
// (/style.css instead of /assets/style.css).
//
//go:embed assets
var assetsFS embed.FS
