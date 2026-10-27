package remotecmd

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/hardwarekey"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

//go:embed hardware_ceremony.html
var hardwareCeremonyPage string

type hardwareCeremonyOptions struct {
	Title   string `json:"title"`
	Details string `json:"details"`
	Kind    string `json:"kind"`
	Options any    `json:"options"`
}

// The bridge holds no machine/admin secrets in browser storage. Only the short
// lived local ceremony capability goes to the browser; broker calls stay native.
func runHardwareCeremony(ctx context.Context, begin func() (hardwareCeremonyOptions, error), finish func([]byte) (bool, error)) error {
	listener, err := net.Listen("tcp4", hardwarekey.Listen)
	if err != nil {
		return fmt.Errorf("hardware-key browser port is unavailable: %w", err)
	}
	defer listener.Close()
	token, err := remoteio.Secret()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, hardwarekey.CeremonyLifetime*2)
	defer cancel()
	done := make(chan error, 1)
	handler := hardwareCeremonyHandler(token, begin, finish, done)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8 << 10}
	failures := make(chan error, 1)
	go func() { failures <- server.Serve(listener) }()
	defer func() {
		shutdown, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
		_ = server.Close()
	}()
	address := hardwarekey.Origin + "/#" + token
	fmt.Fprintf(os.Stderr, "Open this local page to verify your physical security key:\n%s\n", address)
	launcher, cleanup, launchErr := prepareHardwareCeremonyLauncher(address)
	if launchErr == nil {
		defer func() {
			if err := cleanup(); err != nil {
				fmt.Fprintln(os.Stderr, "Could not remove the private hardware-key browser launcher.")
			}
		}()
		openHardwareCeremonyBrowser(launcher)
	} else {
		fmt.Fprintln(os.Stderr, "Could not prepare a private browser launcher; open the printed link manually.")
	}
	select {
	case err := <-done:
		return err
	case err := <-failures:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func hardwareCeremonyHandler(token string, begin func() (hardwareCeremonyOptions, error), finish func([]byte) (bool, error), done chan<- error) http.Handler {
	scriptStart := strings.Index(hardwareCeremonyPage, "<script>") + len("<script>")
	scriptEnd := strings.Index(hardwareCeremonyPage, "</script>")
	scriptDigest := sha256.Sum256([]byte(hardwareCeremonyPage[scriptStart:scriptEnd]))
	policy := "default-src 'none'; script-src 'sha256-" + base64.StdEncoding.EncodeToString(scriptDigest[:]) + "'; connect-src 'self'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	// Serialize the options/finish state. A second browser request cannot create
	// multiple effects or race registration against existing-key verification.
	var mutex sync.Mutex
	completed := false
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", policy)
		if r.Host != "localhost:47832" || r.URL.RawQuery != "" {
			http.Error(w, "invalid local ceremony host", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, hardwareCeremonyPage)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Origin") != hardwarekey.Origin ||
			subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Dockpipe-Ceremony")), []byte(token)) != 1 {
			http.Error(w, "local ceremony authorization required", http.StatusForbidden)
			return
		}
		mutex.Lock()
		defer mutex.Unlock()
		if completed {
			http.Error(w, "ceremony finished", http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/options":
			options, err := begin()
			if err != nil {
				http.Error(w, "cannot prepare hardware-key ceremony; inspect the CLI", http.StatusConflict)
				completed = true
				done <- err
				return
			}
			_ = json.NewEncoder(w).Encode(options)
		case "/finish":
			response, err := io.ReadAll(http.MaxBytesReader(w, r.Body, hardwarekey.MaxResponseBytes))
			if err != nil {
				http.Error(w, "response too large", http.StatusRequestEntityTooLarge)
				return
			}
			next, err := finish(response)
			if err != nil {
				http.Error(w, "hardware-key verification failed; inspect the CLI", http.StatusConflict)
				completed = true
				done <- err
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]bool{"next": next})
			if !next {
				completed = true
				done <- nil
			}
		default:
			http.NotFound(w, r)
		}
	})
}

// Browser opener arguments can be visible to other local users. Put the secret
// redirect inside an owner-only file, and launch only that file's public path.
func prepareHardwareCeremonyLauncher(address string) (string, func() error, error) {
	temporaryRoot, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", nil, err
	}
	temporaryRoot, err = filepath.Abs(temporaryRoot)
	if err != nil {
		return "", nil, err
	}
	identifier, err := remoteio.Secret()
	if err != nil {
		return "", nil, err
	}
	directory := filepath.Join(temporaryRoot, "dockpipe-key-"+identifier)
	if err := infrastructure.PreparePrivateDirectory(directory); err != nil {
		return "", nil, err
	}
	cleanup := func() error { return os.RemoveAll(directory) }
	prepared := false
	defer func() {
		if !prepared {
			_ = cleanup()
		}
	}()
	encodedAddress, err := json.Marshal(address)
	if err != nil {
		return "", nil, err
	}
	script := "location.replace(" + string(encodedAddress) + ");"
	digest := sha256.Sum256([]byte(script))
	policy := "default-src 'none'; script-src 'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'; base-uri 'none'; form-action 'none'"
	page := "<!doctype html><meta charset=\"utf-8\"><meta name=\"referrer\" content=\"no-referrer\">" +
		"<meta http-equiv=\"Content-Security-Policy\" content=\"" + policy + "\">" +
		"<title>Dockpipe hardware-key pairing</title><script>" + script + "</script>"
	path := filepath.Join(directory, "open.html")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", nil, err
	}
	_, writeErr := io.WriteString(file, page)
	closeErr := file.Close()
	if writeErr != nil {
		return "", nil, writeErr
	}
	if closeErr != nil {
		return "", nil, closeErr
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	launcher := (&url.URL{Scheme: "file", Path: uriPath}).String()
	prepared = true
	return launcher, cleanup, nil
}

// The browser is an independent user application. Finishing or cancelling this
// ceremony must not kill it; the private local server still expires promptly.
func openHardwareCeremonyBrowser(fileURI string) {
	command := hardwareCeremonyBrowserCommand(fileURI)
	if command == nil {
		return
	}
	if err := command.Start(); err != nil {
		// The printed localhost link remains usable when desktop opening is absent.
		return
	}
	go func() { _ = command.Wait() }()
}

func hardwareCeremonyBrowserCommand(fileURI string) *exec.Cmd {
	parsed, err := url.Parse(fileURI)
	if err != nil || parsed.Scheme != "file" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", fileURI)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", fileURI)
	default:
		command = exec.Command("xdg-open", fileURI)
	}
	return command
}
