package environment

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure/process"
)

type loginProgress struct {
	mu      sync.Mutex
	pending string
	domain  *url.URL
	shown   bool
	output  io.Writer
	open    func(string)
}

func (progress *loginProgress) Write(data []byte) (int, error) {
	progress.mu.Lock()
	defer progress.mu.Unlock()
	if len(progress.pending)+len(data) > 16384 {
		return 0, errors.New("login output exceeded limit")
	}
	progress.pending += string(data)
	for {
		line, remainder, found := strings.Cut(progress.pending, "\n")
		if !found {
			break
		}
		progress.pending = remainder
		if progress.shown {
			continue
		}
		for _, field := range strings.Fields(line) {
			parsed, err := url.Parse(field)
			if err != nil || parsed.Scheme != "https" || parsed.Host != progress.domain.Host || parsed.User != nil ||
				parsed.Path != strings.TrimSuffix(progress.domain.Path, "/api")+"/login" || parsed.Fragment != "" {
				continue
			}
			query, err := url.ParseQuery(parsed.RawQuery)
			if err != nil || len(query) != 1 || len(query["callback_port"]) != 1 {
				continue
			}
			port, err := strconv.Atoi(query.Get("callback_port"))
			if err != nil || port < 1 || port > 65535 {
				continue
			}
			// Reconstruct the only public challenge shape we allow; never echo
			// provider diagnostics, credentials, or arbitrary query strings.
			challenge := *progress.domain
			challenge.Path = parsed.Path
			challenge.RawPath = ""
			challenge.RawQuery = url.Values{"callback_port": {strconv.Itoa(port)}}.Encode()
			fmt.Fprintf(progress.output, "[dockpipe] Sign in through your browser: %s\n", challenge.String())
			progress.shown = true
			progress.open(challenge.String())
			break
		}
	}
	return len(data), nil
}

func runBrowserLogin(ctx context.Context, arguments, environment []string, output io.Writer) ([]byte, error) {
	var domain *url.URL
	for _, argument := range arguments {
		if value, found := strings.CutPrefix(argument, "--domain="); found {
			domain, _ = url.Parse(value)
		}
	}
	if domain == nil {
		return nil, errors.New("missing login domain")
	}
	progress := &loginProgress{domain: domain, output: output, open: func(challenge string) {
		// Infisical opens the browser itself on macOS/Windows, but only prints
		// the URL on Linux. A failed opener leaves the clickable URL available.
		if runtime.GOOS == "linux" {
			openContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = process.RunNested(exec.CommandContext(openContext, "xdg-open", challenge))
		}
	}}
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Env = environment
	// No terminal input: never ask for passwords or pasted session tokens.
	command.Stdout = progress
	command.Stderr = progress
	command.WaitDelay = time.Second
	if err := process.RunNested(command); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("browser login failed (provider output withheld)")
	}
	return nil, nil
}
