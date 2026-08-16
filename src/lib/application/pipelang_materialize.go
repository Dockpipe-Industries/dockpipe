package application

import (
	"time"

	"dockpipe/src/lib/application/internal/pipelangmaterialize"
)

type workflowTypeMapDoc struct {
	Types []string `yaml:"types"`
}

func dedupeAbsExistingDirs(paths []string) []string {
	return pipelangmaterialize.ExistingRoots(paths)
}

func detectPipeLangModuleRoot(startDir string) string {
	return pipelangmaterialize.DetectModuleRoot(startDir)
}

func readPipeFilesUnder(root string) (map[string][]byte, time.Time, error) {
	return pipelangmaterialize.ReadFilesUnder(root)
}

func materializePipeLangRoots(roots []string, force bool, outBase string) (int, error) {
	return pipelangmaterialize.MaterializeRoots(roots, force, outBase)
}
