// Package packagecatalog reads HTTPS package catalogs and installs verified package archives.
package packagecatalog

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

const maxManifestBytes = 8 << 20

var safeToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

type Entry struct {
	packagebuild.StoreArtifact
	Kind string `json:"kind"`
}

type Catalog struct {
	Manifest string  `json:"manifest"`
	Platform string  `json:"platform"`
	Packages []Entry `json:"packages"`
}

// Client owns request policy; callers supply no transport options through catalog data.
type Client struct {
	http *http.Client
}

func NewClient() *Client {
	client := &http.Client{Timeout: 5 * time.Minute}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host || req.URL.User != nil {
			return fmt.Errorf("package redirect must stay on the original HTTPS origin")
		}
		return nil
	}
	return &Client{http: client}
}

func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("package remote must be an HTTPS manifest URL without credentials, query, or fragment")
	}
	return u, nil
}

// childURL permits only relative paths beneath the manifest's directory.
func childURL(base *url.URL, reference string) (*url.URL, error) {
	if reference == "" || strings.ContainsAny(reference, "\\:%?#") || strings.HasPrefix(reference, "/") {
		return nil, fmt.Errorf("unsafe catalog path %q", reference)
	}
	for _, part := range strings.Split(reference, "/") {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("unsafe catalog path %q", reference)
		}
	}
	return base.ResolveReference(&url.URL{Path: reference}), nil
}

func (c *Client) get(ctx context.Context, target *url.URL) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "dockpipe/package-catalog")
	response, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("package remote returned HTTP %d for %s", response.StatusCode, target)
	}
	return response, nil
}

func (c *Client) readJSON(ctx context.Context, target *url.URL, out any) error {
	response, err := c.get(ctx, target)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxManifestBytes {
		return fmt.Errorf("package manifest exceeds size limit")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid package manifest at %s: %w", target, err)
	}
	return nil
}

// Load accepts a latest pointer, release manifest, or platform store manifest.
// Latest pointer paths are origin-relative; release store paths are release-relative.
func (c *Client) Load(ctx context.Context, remote, platform string) (Catalog, error) {
	out := Catalog{Platform: platform, Packages: []Entry{}}
	target, err := parseURL(remote)
	if err != nil {
		return out, err
	}
	for depth := 0; depth < 3; depth++ {
		var document struct {
			packagebuild.StoreBuildManifest
			Manifest string `json:"manifest"`
			Stores   map[string]struct {
				Manifest string `json:"manifest"`
			} `json:"stores"`
		}
		if err := c.readJSON(ctx, target, &document); err != nil {
			return out, err
		}
		if document.Manifest != "" && depth == 0 {
			origin := *target
			origin.Path = "/"
			origin.RawPath = ""
			target, err = childURL(&origin, document.Manifest)
		} else if document.Schema != 1 {
			return out, fmt.Errorf("unsupported package catalog schema %d", document.Schema)
		} else if document.Stores != nil {
			store, exists := document.Stores[platform]
			if !exists {
				return out, fmt.Errorf("package remote has no store for %s", platform)
			}
			target, err = childURL(target, store.Manifest)
		} else {
			out.Manifest = target.String()
			if document.Packages.Core != nil {
				out.Packages = append(out.Packages, Entry{*document.Packages.Core, "core"})
			}
			for _, entry := range document.Packages.Workflows {
				out.Packages = append(out.Packages, Entry{entry, "workflow"})
			}
			for _, entry := range document.Packages.Resolvers {
				out.Packages = append(out.Packages, Entry{entry, "resolver"})
			}
			seen := make(map[string]bool)
			for _, entry := range out.Packages {
				if err := validateEntry(entry); err != nil {
					return out, err
				}
				key := entry.Kind + "/" + entry.Name
				if seen[key] {
					return out, fmt.Errorf("duplicate package %s", key)
				}
				seen[key] = true
			}
			if len(out.Packages) == 0 {
				return out, fmt.Errorf("package store is empty")
			}
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
	return out, fmt.Errorf("package catalog indirection limit exceeded")
}

func validateEntry(entry Entry) error {
	if !safeToken.MatchString(entry.Name) || !safeToken.MatchString(entry.Version) || strings.Contains(entry.Name, "..") {
		return fmt.Errorf("invalid package name or version")
	}
	prefix := "dockpipe-" + entry.Kind + "-"
	if entry.Kind != "core" {
		prefix += entry.Name + "-"
	}
	if entry.Kind != "core" && entry.Kind != "workflow" && entry.Kind != "resolver" {
		return fmt.Errorf("invalid package kind %q", entry.Kind)
	}
	if entry.Tarball != prefix+entry.Version+".tar.gz" || path.Base(entry.Tarball) != entry.Tarball {
		return fmt.Errorf("package filename does not match identity: %q", entry.Tarball)
	}
	digest, err := hex.DecodeString(entry.SHA256)
	if err != nil || len(digest) != 32 {
		return fmt.Errorf("package %s needs a SHA-256 checksum", entry.Name)
	}
	return nil
}
