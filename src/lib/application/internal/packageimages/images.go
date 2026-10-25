package packageimages

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/application/internal/imageartifact"
	"dockpipe/src/lib/application/internal/textvalue"
	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

// Run lists planned and indexed image artifacts using the supplied Docker availability query.
func Run(args []string, exists func(string) (bool, error)) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageImagesUsageText)
		return nil
	}
	workdir, err := parsePackageImagesWorkdir(args)
	if err != nil {
		return err
	}
	rows, err := CollectRows(workdir, exists)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		root, _ := infrastructure.ImageArtifactIndexDir(workdir)
		fmt.Fprintf(os.Stderr, "[dockpipe] no image artifacts found (%s)\n", root)
		return nil
	}
	for _, row := range rows {
		r := row.Artifact
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			trimDigestPrefix(r.Fingerprint),
			row.Status,
			textvalue.FirstNonEmpty(strings.TrimSpace(r.ArtifactState), string(domain.ImageArtifactPlanned)),
			strings.TrimSpace(r.Source), strings.TrimSpace(r.ImageRef),
			strings.TrimSpace(r.WorkflowName), strings.TrimSpace(r.PackageName),
			strings.TrimSpace(r.StepID), strings.TrimSpace(r.ImageKey),
		)
	}
	return nil
}

func parsePackageImagesWorkdir(args []string) (string, error) {
	var workdir string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return "", fmt.Errorf("unknown option %s", args[i])
		default:
			return "", fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if strings.TrimSpace(workdir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return filepath.Abs(workdir)
}

// Row combines an image artifact with its current availability status.
type Row struct {
	Artifact domain.ImageArtifactManifest
	Status   string
}

// CollectRows merges planned package artifacts with their indexed runtime receipts.
func CollectRows(workdir string, existsFn func(string) (bool, error)) ([]Row, error) {
	planned, err := collectPlannedImageArtifactsFromPackages(workdir)
	if err != nil {
		return nil, err
	}
	indexed, err := collectIndexedImageArtifacts(workdir)
	if err != nil {
		return nil, err
	}
	indexByLogicalKey := map[string]domain.ImageArtifactManifest{}
	for _, r := range indexed {
		indexByLogicalKey[imageArtifactLogicalKey(r)] = r
	}
	rows := make([]Row, 0, len(planned)+len(indexed))
	seenLogicalKeys := map[string]struct{}{}
	for _, p := range planned {
		key := imageArtifactLogicalKey(p)
		seenLogicalKeys[key] = struct{}{}
		indexed, ok := indexByLogicalKey[key]
		switch {
		case ok && strings.TrimSpace(indexed.Fingerprint) == strings.TrimSpace(p.Fingerprint):
			rows = append(rows, Row{Artifact: indexed, Status: imageArtifactAvailabilityStatus(indexed, existsFn)})
		case ok:
			rows = append(rows, Row{Artifact: p, Status: "stale"})
		default:
			rows = append(rows, Row{Artifact: p, Status: imageArtifactAvailabilityStatus(p, existsFn)})
		}
	}
	for _, r := range indexed {
		if _, ok := seenLogicalKeys[imageArtifactLogicalKey(r)]; ok {
			continue
		}
		rows = append(rows, Row{Artifact: r, Status: textvalue.FirstNonEmpty(imageArtifactAvailabilityStatus(r, existsFn), "orphaned")})
	}
	sort.Slice(rows, func(i, j int) bool {
		a := rows[i].Artifact
		b := rows[j].Artifact
		for _, cmp := range []struct{ x, y string }{
			{a.WorkflowName, b.WorkflowName},
			{a.PackageName, b.PackageName},
			{a.StepID, b.StepID},
			{a.ImageKey, b.ImageKey},
			{a.ImageRef, b.ImageRef},
			{a.Fingerprint, b.Fingerprint},
		} {
			if cmp.x != cmp.y {
				return cmp.x < cmp.y
			}
		}
		return rows[i].Status < rows[j].Status
	})
	return rows, nil
}

func imageArtifactLogicalKey(r domain.ImageArtifactManifest) string {
	return strings.Join([]string{
		strings.TrimSpace(r.WorkflowName),
		strings.TrimSpace(r.PackageName),
		strings.TrimSpace(r.StepID),
		strings.TrimSpace(r.ImageKey),
		strings.TrimSpace(r.Source),
		strings.TrimSpace(r.ImageRef),
	}, "\x00")
}

func imageArtifactAvailabilityStatus(r domain.ImageArtifactManifest, existsFn func(string) (bool, error)) string {
	state := domain.ImageArtifactState(strings.TrimSpace(r.ArtifactState))
	switch {
	case state == domain.ImageArtifactPlanned:
		return "planned"
	case state == domain.ImageArtifactReferenced:
		return "referenced"
	case state != domain.ImageArtifactMaterialized && state != domain.ImageArtifactCached:
		return textvalue.FirstNonEmpty(string(state), "unknown")
	}
	if strings.TrimSpace(r.ImageRef) == "" || existsFn == nil {
		return "unknown"
	}
	ok, err := existsFn(strings.TrimSpace(r.ImageRef))
	if err != nil {
		return "docker-error"
	}
	if !ok {
		return "missing"
	}
	return "ready"
}

func collectPlannedImageArtifactsFromPackages(workdir string) ([]domain.ImageArtifactManifest, error) {
	workflowDir, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return nil, err
	}
	matches, err := packagebuild.ListPackageArchivePaths(workflowDir, "workflow")
	if err != nil {
		return nil, err
	}
	var out []domain.ImageArtifactManifest
	for _, tgz := range matches {
		paths, err := packagebuild.ListTarGzMemberPaths(tgz)
		if err != nil {
			return nil, err
		}
		for _, entry := range paths {
			if !imageartifact.IsTarEntry(entry) {
				continue
			}
			b, err := packagebuild.ReadFileFromTarGz(tgz, entry)
			if err != nil {
				return nil, err
			}
			var r domain.ImageArtifactManifest
			if err := json.Unmarshal(b, &r); err != nil {
				return nil, fmt.Errorf("%s:%s: %w", tgz, entry, err)
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func collectIndexedImageArtifacts(workdir string) ([]domain.ImageArtifactManifest, error) {
	root, err := infrastructure.ImageArtifactIndexDir(workdir)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, "by-fingerprint")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]domain.ImageArtifactManifest, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r domain.ImageArtifactManifest
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, e.Name()), err)
		}
		out = append(out, r)
	}
	return out, nil
}

func trimDigestPrefix(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "sha256:")
}

const packageImagesUsageText = `dockpipe package images [--workdir <path>]

Lists Docker image artifacts known to the project. The command merges planned image
artifacts from compiled workflow tarballs with materialized/cached receipts under
bin/.dockpipe/internal/images/by-fingerprint.

Output columns (tab-separated):
  fingerprint, status, state, source, image_ref, workflow, package, step_id, image_key

Statuses:
  ready, missing, stale, planned, referenced, docker-error

`
