package provisioning

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFirstBootObservationPlanIsDeterministicBoundedAndInert(t *testing.T) {
	evidenceRoot := filepath.Join(t.TempDir(), "evidence")
	runtimeRoot := shortObservationRuntimeRoot(t)
	first, err := PlanFirstBootObservation(evidenceRoot, runtimeRoot, "run-001", "cohort-001")
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanFirstBootObservation(evidenceRoot, runtimeRoot, "run-001", "cohort-001")
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := first.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := second.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if firstJSON != secondJSON {
		t.Fatal("first-boot observation plan is not deterministic")
	}
	wantPath := filepath.Join(evidenceRoot, "run-001", "cohort-001", FirstBootObservationFilename)
	wantSocket := filepath.Join(runtimeRoot, "run-001", "cohort-001", FirstBootObservationSocketFilename)
	if first.EvidencePath != wantPath || first.SocketPath != wantSocket || first.GuestSource != "isa-serial/ttyS0" || first.Sink != "controller-owned-bounded-file" || first.Transport != "unix-stream-qemu-client" || first.EvidenceMode != 0o600 || !first.EvidenceExclusive || first.SocketMode != 0o600 || !first.SocketExclusive || first.MaxBytes != 4*1024*1024 || first.OverflowPolicy != "fail-closed-preserve-prefix" || !first.FsyncOnFailure || !first.FsyncParentOnFailure || !first.PassiveCapture || !first.ControllerListener || !first.QEMUClient || first.Reconnect || !first.StopAndJoin || first.SeedMutation || first.PrivatePayloadRead || first.Network || first.Execute || !first.AuthorizationRequired {
		t.Fatalf("first-boot observation policy changed: %+v", first)
	}
}

func TestFirstBootObservationPlanRejectsUnsafeInputsAndAuthorityExpansion(t *testing.T) {
	evidenceRoot := filepath.Join(t.TempDir(), "evidence")
	runtimeRoot := shortObservationRuntimeRoot(t)
	for name, root := range map[string]string{
		"relative": "relative/evidence",
		"unclean":  evidenceRoot + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(evidenceRoot),
		"comma":    evidenceRoot + ",changed",
		"newline":  evidenceRoot + "\nchanged",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := PlanFirstBootObservation(root, runtimeRoot, "run-001", "cohort-001"); err == nil {
				t.Fatal("expected unsafe evidence-root rejection")
			}
		})
	}
	if _, err := PlanFirstBootObservation(evidenceRoot, runtimeRoot, "run-001\nchanged", "cohort-001"); err == nil {
		t.Fatal("expected unsafe run identity rejection")
	}
	if _, err := PlanFirstBootObservation(evidenceRoot, runtimeRoot, "run-001", strings.Repeat("c", 129)); err == nil {
		t.Fatal("expected unsafe cohort identity rejection")
	}

	base, err := PlanFirstBootObservation(evidenceRoot, runtimeRoot, "run-001", "cohort-001")
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*FirstBootObservationPlan){
		"execute":              func(p *FirstBootObservationPlan) { p.Execute = true },
		"network":              func(p *FirstBootObservationPlan) { p.Network = true },
		"seed mutation":        func(p *FirstBootObservationPlan) { p.SeedMutation = true },
		"private payload read": func(p *FirstBootObservationPlan) { p.PrivatePayloadRead = true },
		"unbounded":            func(p *FirstBootObservationPlan) { p.MaxBytes++ },
		"transport":            func(p *FirstBootObservationPlan) { p.Transport = "ordinary-file" },
		"socket":               func(p *FirstBootObservationPlan) { p.SocketPath = filepath.Join(runtimeRoot, "other.sock") },
		"reconnect":            func(p *FirstBootObservationPlan) { p.Reconnect = true },
		"no join":              func(p *FirstBootObservationPlan) { p.StopAndJoin = false },
		"replace evidence":     func(p *FirstBootObservationPlan) { p.EvidenceExclusive = false },
		"widen mode":           func(p *FirstBootObservationPlan) { p.EvidenceMode = 0o640 },
		"widen socket mode":    func(p *FirstBootObservationPlan) { p.SocketMode = 0o660 },
		"replace socket":       func(p *FirstBootObservationPlan) { p.SocketExclusive = false },
		"substitute root":      func(p *FirstBootObservationPlan) { p.EvidenceRoot = filepath.Join(t.TempDir(), "other-evidence") },
		"no authorization":     func(p *FirstBootObservationPlan) { p.AuthorizationRequired = false },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if err := changed.Validate(); err == nil {
				t.Fatal("expected changed observation policy rejection")
			}
		})
	}
}

func shortObservationRuntimeRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(string(filepath.Separator), "tmp", "dpvm-observation")
	if volume := filepath.VolumeName(t.TempDir()); volume != "" {
		root = volume + string(filepath.Separator) + "dpvm-observation"
	}
	return root
}
