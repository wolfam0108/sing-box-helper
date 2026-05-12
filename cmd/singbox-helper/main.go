// Command singbox-helper is the entry point. It has two run modes:
//
//	# CLI: render a sing-box config from a URI, print or write to disk.
//	singbox-helper --from-uri 'hysteria2://pw@host:443'
//	singbox-helper --from-uri 'vless://...' --apply
//	singbox-helper --from-uri 'vless://...' --apply --out /tmp/sing-box.json
//
//	# HTTP server: expose REST API at the given address.
//	singbox-helper --serve
//	singbox-helper --serve --listen 0.0.0.0:8765
//
// Web UI (HTML) lands in v1.0-β; v1.0-α is JSON-only.
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
	"github.com/wolfam0108/sing-box-helper/internal/web"
)

func main() {
	var (
		serve        = flag.Bool("serve", false, "Run as HTTP server (otherwise process --from-uri and exit).")
		listen       = flag.String("listen", "0.0.0.0:8765", "HTTP listen address used with --serve.")
		fromURI      = flag.String("from-uri", "", "Node URI (vless://, hysteria2://, hy2://, socks5://). Required in CLI mode.")
		apply        = flag.Bool("apply", false, "Write the rendered config to disk (otherwise prints to stdout).")
		outPath      = flag.String("out", "/opt/etc/sing-box/config.json",
			"Output path used together with --apply.")
		settingsPath = flag.String("settings", "/opt/etc/singbox-helper/config.yaml",
			"Path to YAML settings file. Missing file = built-in defaults.")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"singbox-helper — manage sing-box config.json from a node URI\n\n"+
				"Usage:\n"+
				"  %s --from-uri <URI> [--apply [--out <path>]]    # one-shot CLI\n"+
				"  %s --serve [--listen 0.0.0.0:8765]             # HTTP API\n\nFlags:\n",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	settings, err := config.LoadSettings(*settingsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: settings load:", err, "(using defaults)")
		settings = config.DefaultSettings()
	}

	if *serve {
		if err := runServer(*listen, *settingsPath, settings); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if *fromURI == "" {
		fmt.Fprintln(os.Stderr, "error: --from-uri is required (or use --serve for HTTP API)")
		flag.Usage()
		os.Exit(2)
	}

	if err := run(*fromURI, *apply, *outPath, settings); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runServer(addr, settingsPath string, s config.Settings) error {
	srv := web.New(s)
	srv.SettingsPath = settingsPath
	fmt.Fprintln(os.Stderr, "singbox-helper HTTP API listening on", addr)
	fmt.Fprintln(os.Stderr, "Settings:", settingsPath)
	fmt.Fprintln(os.Stderr, "Endpoints: GET /api/status, POST /api/preview, POST /api/apply, GET /api/test, GET|POST /api/settings")
	return srv.ListenAndServe(addr)
}

func run(uri string, apply bool, outPath string, settings config.Settings) error {
	pn, err := parser.Parse(uri)
	if err != nil {
		return fmt.Errorf("parse URI: %w", err)
	}

	// Surface parser-emitted warnings on stderr so they're visible even in
	// --apply mode where stdout is silent.
	for _, note := range pn.Display.Notes {
		fmt.Fprintln(os.Stderr, "warning:", note)
	}

	rendered, err := config.Render(pn, settings)
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
