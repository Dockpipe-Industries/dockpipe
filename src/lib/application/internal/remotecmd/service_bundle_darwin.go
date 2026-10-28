package remotecmd

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// plutil supports both XML and binary app plists. Use the system tool directly;
// neither PATH nor a workflow can substitute the metadata reader.
func serviceBundleIdentifier(ctx context.Context, executable string) (string, error) {
	path := serviceBundleInfoPath(executable)
	if path == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", "-expect", "string", "-o", "-", path)
	output := &bundleIdentifierOutput{}
	command.Stdout = output
	command.WaitDelay = time.Second
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", errors.New("cannot read enclosing application bundle identifier")
	}
	identifier := strings.TrimSuffix(output.buffer.String(), "\n")
	if err := validateServiceBundleIdentifier(identifier); err != nil {
		return "", err
	}
	return identifier, nil
}

type bundleIdentifierOutput struct {
	buffer bytes.Buffer
}

func (output *bundleIdentifierOutput) Write(data []byte) (int, error) {
	if output.buffer.Len()+len(data) > 256 {
		return 0, errors.New("application bundle identifier exceeds 255 bytes")
	}
	return output.buffer.Write(data)
}
