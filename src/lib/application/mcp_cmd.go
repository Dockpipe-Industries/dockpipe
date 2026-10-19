package application

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/mcpregistry"
	mcpmodel "dockpipe/src/lib/model/mcp"
)

const mcpUsage = `dockpipe mcp registry search [--search text] [--cursor cursor] [--limit 1..100]
dockpipe mcp registry versions --name namespace/server
dockpipe mcp registry show --name namespace/server [--version version|latest]
dockpipe mcp config validate [--workdir path]

Read-only discovery and project configuration validation. No installation, server
connection, process launch, secret resolution or gateway exposure is performed.
`

type mcpRegistryReader interface {
	Search(context.Context, mcpregistry.Query) (mcpmodel.RegistryPage, error)
	Versions(context.Context, string) (mcpmodel.RegistryPage, error)
	Version(context.Context, string, string) (mcpmodel.RegistryRecord, error)
}

func cmdMCP(args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return runMCP(ctx, args, os.Stdout, mcpregistry.NewClient())
}

func runMCP(ctx context.Context, args []string, output io.Writer, client mcpRegistryReader) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(output, mcpUsage)
		return err
	}
	if len(args) < 2 {
		return usageError(mcpUsage)
	}
	if args[0] == "config" && args[1] == "validate" {
		return validateMCPConfig(args[2:], output)
	}
	if args[0] != "registry" {
		return usageError(mcpUsage)
	}
	flags := flag.NewFlagSet("mcp registry", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	name := flags.String("name", "", "registry server name")
	version := flags.String("version", "latest", "registry version")
	search := flags.String("search", "", "name substring")
	cursor := flags.String("cursor", "", "opaque pagination cursor")
	limit := flags.Int("limit", 30, "page size")
	if err := flags.Parse(args[2:]); err != nil {
		return usageError(mcpUsage)
	}
	if flags.NArg() != 0 {
		return usageError(mcpUsage)
	}
	allowed := map[string]map[string]bool{
		"search":   {"search": true, "cursor": true, "limit": true},
		"versions": {"name": true},
		"show":     {"name": true, "version": true},
	}
	invalidOption := false
	flags.Visit(func(option *flag.Flag) {
		if !allowed[args[1]][option.Name] {
			invalidOption = true
		}
	})
	if invalidOption {
		return usageError(mcpUsage)
	}
	query := mcpregistry.Query{Search: *search, Cursor: *cursor, Limit: *limit}
	var result any
	var err error
	switch args[1] {
	case "search":
		if *name != "" || *version != "latest" {
			return fmt.Errorf("search does not accept a name or version")
		}
		result, err = client.Search(ctx, query)
	case "versions":
		if *name == "" || *search != "" || *version != "latest" {
			return fmt.Errorf("versions requires --name and does not accept --search or --version")
		}
		result, err = client.Versions(ctx, *name)
	case "show":
		if *name == "" || *search != "" || *cursor != "" || *limit != 30 {
			return fmt.Errorf("show requires --name and accepts only --version")
		}
		result, err = client.Version(ctx, *name, *version)
	default:
		return usageError(mcpUsage)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func validateMCPConfig(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("mcp config validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workdir := flags.String("workdir", ".", "project directory")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usageError(mcpUsage)
	}
	root, err := domain.FindProjectRootWithDockpipeConfig(*workdir)
	if err != nil {
		return err
	}
	config, err := domain.LoadDockpipeProjectConfig(root)
	if err != nil {
		return err
	}
	if config == nil || config.MCP == nil {
		return fmt.Errorf("project has no mcp configuration")
	}
	// Validation intentionally returns no secret bindings, configured paths or URLs.
	return json.NewEncoder(output).Encode(struct {
		Schema string `json:"schema"`
		Valid  bool   `json:"valid"`
		Count  int    `json:"configured_servers"`
		Active bool   `json:"servers_started"`
	}{"dockpipe.mcp.validation/v1", true, len(config.MCP.Servers), false})
}
