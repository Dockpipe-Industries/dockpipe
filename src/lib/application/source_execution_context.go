package application

import (
	"context"

	"dockpipe/src/lib/infrastructure"
)

// The legacy no-context invocation retains its existing source behavior. A real
// callback context selects the platform owner which can supervise cancellation.
func executeSourceHostScript(ctx context.Context, path string, environment []string) (map[string]string, error) {
	if ctx == nil {
		return infrastructure.SourceHostScript(path, environment)
	}

	return infrastructure.SourceHostScriptWithContext(ctx, path, environment)
}

func sourceHostScriptWithoutContext(source func(context.Context, string, []string) (map[string]string, error)) func(string, []string) (map[string]string, error) {
	return func(path string, environment []string) (map[string]string, error) {
		return source(nil, path, environment)
	}
}
