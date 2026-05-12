package web

import (
	"io/fs"
	"strings"
	"testing"
)

// TestAssetsEmbedded ensures the //go:embed directive actually captured
// index.html, style.css and app.js. If this fails, the rest of the UI is
// going to 404.
func TestAssetsEmbedded(t *testing.T) {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		t.Fatalf("fs.Sub(assetsFS, \"assets\"): %v", err)
	}
	for _, want := range []string{"index.html", "style.css", "app.js"} {
		f, err := sub.Open(want)
		if err != nil {
			// Dump what we did capture, for debugging.
			entries, _ := fs.ReadDir(assetsFS, ".")
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Fatalf("missing %q in embedded assets. fs root entries: %v", want, strings.Join(names, ", "))
		}
		_ = f.Close()
	}
}
