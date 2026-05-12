// Command singbox-helper is the CLI entry point. v0.5 exposes a single
// task: take a node URI, render a sing-box config.json, and either print
// it to stdout or write it to disk (with backup).
//
// Examples:
//
//	# Just print the JSON, no side effects.
//	singbox-helper --from-uri 'hysteria2://pw@host:443'
//
//	# Write to default location, backing up the previous file.
//	singbox-helper --from-uri 'vless://...' --apply
//
//	# Custom output path.
//	singbox-helper --from-uri 'vless://...' --apply --out /tmp/sing-box.json
//
// Web UI and HTTP API will arrive in v1.0.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/wolfam0108/sing-box-helper/internal/config"
	"github.com/wolfam0108/sing-box-helper/internal/parser"
)

func main() {
	var (
		fromURI = flag.String("from-uri", "", "Node URI (vless://, hysteria2://, hy2://). Required.")
		apply   = flag.Bool("apply", false, "Write the rendered config to disk (otherwise prints to stdout).")
		outPath = flag.String("out", "/opt/etc/sing-box/config.json",
			"Output path used together with --apply.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"singbox-helper — generate sing-box config.json from a node URI\n\n"+
				"Usage:\n  %s --from-uri <URI> [--apply [--out <path>]]\n\nFlags:\n",
			filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if *fromURI == "" {
		fmt.Fprintln(os.Stderr, "error: --from-uri is required")
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*fromURI, *apply, *outPath); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(uri string, apply bool, outPath string) error {
	pn, err := parser.Parse(uri)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}

	// Surface parser-emitted warnings on stderr so they're visible even in
	// --apply mode where stdout is silent.
	for _, note := range pn.Display.Notes {
		fmt.Fprintln(os.Stderr, "warning:", note)
	}

	rendered, err := config.Render(pn, config.DefaultSettings())
	if err != nil {
		return fmt.Errorf("render config: %w", err)
	}

	if !apply {
		_, err := os.Stdout.Write(append(rendered, '\n'))
		return err
	}

	if err := backupIfExists(outPath); err != nil {
		return fmt.Errorf("backup existing config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}
	if err := os.WriteFile(outPath, rendered, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Fprintf(os.Stderr, "Wrote %d bytes to %s\n", len(rendered), outPath)
	fmt.Fprintln(os.Stderr,
		"Reminder: restart sing-box for changes to take effect, e.g.:")
	fmt.Fprintln(os.Stderr,
		"  /opt/etc/init.d/S99sing-box restart")
	return nil
}

// backupIfExists copies outPath into outPath.bak-<timestamp> next to it,
// so the previous config is recoverable. If outPath doesn't exist it's a
// no-op (first install).
func backupIfExists(outPath string) error {
	src, err := os.Open(outPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer src.Close()

	bak := outPath + ".bak-" + time.Now().Format("20060102-150405")
	dst, err := os.OpenFile(bak, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Backup: %s\n", bak)
	return nil
}
