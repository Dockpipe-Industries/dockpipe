module dorkpipe.orchestrator

go 1.25

require dockpipe v0.0.0

require (
	github.com/lib/pq v1.10.9
	golang.org/x/sys v0.47.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.56.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	golang.org/x/term v0.27.0 // indirect
	modernc.org/libc v1.74.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace dockpipe => ../../..
