// Package mcpregistry reads public Official MCP Registry metadata. It does not
// install packages, resolve secrets, connect servers, or grant tool authority.
package mcpregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcpdomain "dockpipe/src/lib/domain/mcp"
	mcpmodel "dockpipe/src/lib/model/mcp"
)

// Match the existing package catalog's 8 MiB metadata budget. One operation reads
// one page, never recursively follows cursors, and has a 15-second total deadline.
const maxResponseBytes = 8 << 20
const operationTimeout = 15 * time.Second

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{
		Timeout: operationTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// The official registry API has stable URLs; redirects add unnecessary
			// origin/credential ambiguity, so callers must inspect them explicitly.
			return fmt.Errorf("MCP registry redirects are not followed")
		},
	}}
}

type Query struct {
	Search string
	Cursor string
	Limit  int
}

type wireRecord struct {
	Server json.RawMessage `json:"server"`
	Meta   map[string]struct {
		Status   string `json:"status"`
		IsLatest bool   `json:"isLatest"`
	} `json:"_meta"`
}

type wirePage struct {
	Servers  []wireRecord `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
	} `json:"metadata"`
}

func (c *Client) Search(ctx context.Context, query Query) (mcpmodel.RegistryPage, error) {
	values, err := queryValues(query)
	if err != nil {
		return mcpmodel.RegistryPage{}, err
	}
	values.Set("version", "latest")
	return c.page(ctx, "/v0.1/servers", values)
}

func (c *Client) Versions(ctx context.Context, name string) (mcpmodel.RegistryPage, error) {
	if err := mcpdomain.ValidateRegistryName(name); err != nil {
		return mcpmodel.RegistryPage{}, err
	}
	page, err := c.page(ctx, "/v0.1/servers/"+url.PathEscape(name)+"/versions", nil)
	if err != nil {
		return mcpmodel.RegistryPage{}, err
	}
	for _, server := range page.Servers {
		if server.Name != name {
			return mcpmodel.RegistryPage{}, fmt.Errorf("MCP registry version history returned a different server")
		}
	}
	return page, nil
}

func (c *Client) Version(ctx context.Context, name, version string) (mcpmodel.RegistryRecord, error) {
	if err := mcpdomain.ValidateRegistryName(name); err != nil {
		return mcpmodel.RegistryRecord{}, err
	}
	if err := mcpdomain.ValidateRegistryVersion(version); err != nil {
		return mcpmodel.RegistryRecord{}, err
	}
	var record wireRecord
	path := "/v0.1/servers/" + url.PathEscape(name) + "/versions/" + url.PathEscape(version)
	if err := c.read(ctx, path, nil, &record); err != nil {
		return mcpmodel.RegistryRecord{}, err
	}
	normalized, err := normalize(record)
	if err == nil && (normalized.Name != name || (version != "latest" && normalized.Version != version)) {
		err = fmt.Errorf("MCP registry returned a different server or version")
	}
	return normalized, err
}

func (c *Client) page(ctx context.Context, path string, query url.Values) (mcpmodel.RegistryPage, error) {
	var wire wirePage
	if err := c.read(ctx, path, query, &wire); err != nil {
		return mcpmodel.RegistryPage{}, err
	}
	if wire.Servers == nil || len(wire.Servers) > 100 || len(wire.Metadata.NextCursor) > 4096 {
		return mcpmodel.RegistryPage{}, fmt.Errorf("invalid or oversized MCP registry page")
	}
	page := mcpmodel.RegistryPage{Servers: make([]mcpmodel.RegistryRecord, 0, len(wire.Servers)), NextCursor: wire.Metadata.NextCursor}
	seen := map[string]bool{}
	for _, record := range wire.Servers {
		normalized, err := normalize(record)
		if err != nil {
			return mcpmodel.RegistryPage{}, err
		}
		key := normalized.Name + "\x00" + normalized.Version
		if seen[key] {
			return mcpmodel.RegistryPage{}, fmt.Errorf("duplicate server version in MCP registry page")
		}
		seen[key] = true
		page.Servers = append(page.Servers, normalized)
	}
	return page, nil
}

func (c *Client) read(ctx context.Context, path string, query url.Values, out any) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	target := mcpmodel.OfficialRegistry + path
	if len(query) != 0 {
		target += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "dockpipe/mcp-registry")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("MCP registry request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP registry returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("MCP registry response failed: %w", err)
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("MCP registry response exceeds 8 MiB")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("invalid MCP registry JSON")
	}
	return nil
}

func normalize(record wireRecord) (mcpmodel.RegistryRecord, error) {
	var server struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(record.Server, &server); err != nil {
		return mcpmodel.RegistryRecord{}, fmt.Errorf("invalid MCP registry server document")
	}
	if mcpdomain.ValidateRegistryName(server.Name) != nil || mcpdomain.ValidateRegistryVersion(server.Version) != nil || server.Version == "latest" {
		return mcpmodel.RegistryRecord{}, fmt.Errorf("invalid MCP registry server identity")
	}
	meta, exists := record.Meta["io.modelcontextprotocol.registry/official"]
	if !exists || (meta.Status != "active" && meta.Status != "deprecated" && meta.Status != "deleted") {
		return mcpmodel.RegistryRecord{}, fmt.Errorf("missing or unsupported official registry status")
	}
	return mcpmodel.RegistryRecord{
		Source: mcpmodel.OfficialRegistry, Name: server.Name, Version: server.Version,
		Title: server.Title, Description: server.Description, Status: meta.Status,
		IsLatest: meta.IsLatest, Document: record.Server,
	}, nil
}

func queryValues(query Query) (url.Values, error) {
	if query.Limit == 0 {
		query.Limit = 30
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Search) > 512 || len(query.Cursor) > 4096 ||
		strings.ContainsAny(query.Search+query.Cursor, "\x00\r\n") {
		return nil, fmt.Errorf("MCP registry query exceeds search, cursor or page-size limits")
	}
	values := url.Values{"limit": {fmt.Sprint(query.Limit)}}
	if query.Search != "" {
		values.Set("search", query.Search)
	}
	if query.Cursor != "" {
		values.Set("cursor", query.Cursor)
	}
	return values, nil
}
