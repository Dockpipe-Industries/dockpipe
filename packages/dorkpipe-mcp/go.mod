module dorkpipe.mcp

go 1.25.0

toolchain go1.26.9

require (
	dockpipe v0.0.0
	dorkpipe.orchestrator v0.0.0
)

require (
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace dockpipe => ../..

replace dorkpipe.orchestrator => ../dorkpipe/lib
