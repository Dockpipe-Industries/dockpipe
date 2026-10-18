package archivecodec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/process"
)

// Match the package transport ceiling; output has its own caller-supplied bound.
const maxExternalInputBytes int64 = 512 << 20
const externalTimeout = 10 * time.Minute
const maxDiagnosticBytes = 16 << 10

func openExternal(ctx context.Context, source io.Reader, codec string, maximum int64) (result io.ReadCloser, resultErr error) {
	callerContext := ctx
	ctx, cancel := context.WithTimeout(ctx, externalTimeout)
	defer cancel()
	selected, err := configuredProvider(ctx, codec)
	if err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp("", "dockpipe-archive-decoder-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, os.RemoveAll(directory))
		}
	}()
	executableName := "decoder"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	executable := filepath.Join(directory, executableName)
	budget := maxProviderBytes
	if err := copyPinned(ctx, selected.Executable, selected.SHA256, executable, &budget, true); err != nil {
		return nil, err
	}
	seen := map[string]bool{strings.ToLower(executableName): true, "input": true, "output": true}
	for _, file := range selected.Files {
		name := strings.ToLower(file.Name)
		if !validStagedName(file.Name) || seen[name] {
			return nil, fmt.Errorf("unsafe or duplicate archive decoder staged name %q", file.Name)
		}
		seen[name] = true
		if err := copyPinned(ctx, file.Path, file.SHA256, filepath.Join(directory, filepath.FromSlash(file.Name)), &budget, false); err != nil {
			return nil, err
		}
	}
	input, err := os.OpenFile(filepath.Join(directory, "input"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	count, err := io.Copy(input, io.LimitReader(contextReader{ctx: ctx, reader: source}, maxExternalInputBytes+1))
	if err != nil {
		return nil, err
	}
	if count > maxExternalInputBytes {
		return nil, errors.New("external archive input exceeds 512 MiB")
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	output, err := os.OpenFile(filepath.Join(directory, "output"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, output.Close())
		}
	}()
	bounded := &boundedOutput{ctx: ctx, output: output, remaining: maximum, cancel: cancel}
	diagnostic := &diagnosticWriter{}
	command := exec.CommandContext(ctx, executable, selected.Args...)
	command.Dir = directory
	command.Env = decoderEnvironment(directory)
	command.Stdin = input
	command.Stdout = bounded
	command.Stderr = diagnostic
	command.WaitDelay = time.Second
	err = process.Run(command)
	if bounded.failure != nil {
		return nil, bounded.failure
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("archive decoder %q failed: %w; %s", codec, err, diagnostic.String())
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return &externalReader{file: output, directory: directory, ctx: callerContext}, nil
}

type externalReader struct {
	file      *os.File
	directory string
	ctx       context.Context
	closed    bool
}

func (reader *externalReader) Read(buffer []byte) (int, error) {
	return (contextReader{ctx: reader.ctx, reader: reader.file}).Read(buffer)
}

func (reader *externalReader) Close() error {
	if reader.closed {
		return nil
	}
	reader.closed = true
	return errors.Join(reader.file.Close(), os.RemoveAll(reader.directory))
}

type boundedOutput struct {
	ctx       context.Context
	output    io.Writer
	remaining int64
	cancel    context.CancelFunc
	failure   error
}

func (writer *boundedOutput) Write(buffer []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		writer.failure = err
		return 0, err
	}
	if int64(len(buffer)) > writer.remaining {
		writer.failure = ErrDecodedLimit
		writer.cancel()
		return 0, writer.failure
	}
	count, err := writer.output.Write(buffer)
	writer.remaining -= int64(count)
	if err != nil {
		writer.failure = err
		writer.cancel()
	}
	return count, err
}

type diagnosticWriter struct {
	data      []byte
	truncated bool
}

func (writer *diagnosticWriter) Write(buffer []byte) (int, error) {
	count := len(buffer)
	remaining := maxDiagnosticBytes - len(writer.data)
	if count > remaining {
		writer.truncated = true
		buffer = buffer[:remaining]
	}
	writer.data = append(writer.data, buffer...)
	return count, nil
}

func (writer *diagnosticWriter) String() string {
	result := string(writer.data)
	if writer.truncated {
		result += " [stderr truncated]"
	}
	return result
}

func decoderEnvironment(directory string) []string {
	// Never forward loader, shell, language runtime, credential or codec overrides.
	environment := []string{"HOME=" + directory, "TMPDIR=" + directory, "TMP=" + directory, "TEMP=" + directory}
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		environment = append(environment, "SystemRoot="+root, "WINDIR="+root,
			"PATH="+filepath.Join(root, "System32"))
	} else {
		environment = append(environment, "PATH=/usr/bin:/bin")
	}
	if runtime.GOOS == "darwin" {
		environment = append(environment, "DYLD_LIBRARY_PATH="+directory+string(os.PathListSeparator)+filepath.Join(directory, "lib"))
	} else if runtime.GOOS != "windows" {
		environment = append(environment, "LD_LIBRARY_PATH="+directory+string(os.PathListSeparator)+filepath.Join(directory, "lib"))
	}
	return environment
}
