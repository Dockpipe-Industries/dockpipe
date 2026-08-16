package packagecompile

import (
	"fmt"
	"os"
	"path/filepath"

	"dockpipe/src/lib/application/internal/pipelangmaterialize"
	"dockpipe/src/lib/application/internal/treecopy"
	"dockpipe/src/lib/pipelang"
)

func compileWorkflowOneFromPipe(workdir, workflowDir string, force bool) error {
	pipePath := filepath.Join(workflowDir, "config.pipe")
	files, _, err := pipelangmaterialize.ReadFilesUnder(workflowDir)
	if err != nil {
		return err
	}
	entryClass := ""
	if data, ok := files[pipePath]; ok {
		if program, err := pipelang.Parse(data); err == nil && len(program.Classes) > 0 {
			entryClass = program.Classes[0].Name
		}
	}
	compiled, err := pipelang.CompileFiles(files, entryClass)
	if err != nil {
		return fmt.Errorf("%s: %w", pipePath, err)
	}
	staging, err := os.MkdirTemp("", "dockpipe-wf-pipe-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := treecopy.Copy(workflowDir, staging); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, "config.yml"), compiled.WorkflowYAML, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, ".pipelang.bindings.json"), compiled.BindingsJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, ".pipelang.bindings.env"), compiled.BindingsEnv, 0o644); err != nil {
		return err
	}
	return compileWorkflowOne(workdir, staging, filepath.Base(workflowDir), force)
}
