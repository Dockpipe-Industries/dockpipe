//go:build linux

package sqliteevidence

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	sqlite "modernc.org/sqlite"
)

const (
	vmHarnessRoleEnv       = "DORKPIPE_SQLITE_VM_HARNESS_ROLE"
	vmHarnessSchema        = "dockpipe.sqlite-vm-harness-command.v1"
	vmCheckpointSchema     = "dockpipe.sqlite-vm-checkpoint-evidence.v1"
	vmRecoverySchema       = "dockpipe.sqlite-vm-recovery-evidence.v1"
	vmQualificationRoot    = "/var/lib/dockpipe-qualification"
	vmHarnessMaxInputBytes = 16 * 1024
)

var vmHarnessIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)

var vmDurabilityBoundaries = map[string]int64{
	"after-stage-before-commit":        1,
	"inside-commit-hook-before-phase1": 1,
	"after-commit-before-reload":       2,
	"after-validation-before-ack":      2,
}

type vmHarnessCommand struct {
	Schema   string `json:"schema"`
	CohortID string `json:"cohort_id"`
	TrialID  string `json:"trial_id"`
	Boundary string `json:"boundary"`
	Attempt  int    `json:"attempt"`
	Root     string `json:"root"`
}

type vmHarnessEvidence struct {
	Schema               string `json:"schema"`
	CohortID             string `json:"cohort_id"`
	TrialID              string `json:"trial_id"`
	Boundary             string `json:"boundary"`
	Attempt              int    `json:"attempt"`
	Root                 string `json:"root"`
	RootIdentity         string `json:"root_identity"`
	Database             string `json:"database"`
	ExpectedRevision     int64  `json:"expected_revision"`
	ObservedRevision     int64  `json:"observed_revision,omitempty"`
	ObservedDigest       string `json:"observed_digest,omitempty"`
	PreMetadataSHA256    string `json:"pre_metadata_sha256"`
	PostMetadataSHA256   string `json:"post_metadata_sha256,omitempty"`
	CompileOptionsSHA256 string `json:"compile_options_sha256"`
	SQLiteVersion        string `json:"sqlite_version"`
	SQLiteSourceID       string `json:"sqlite_source_id"`
	VFS                  string `json:"vfs"`
	QuickCheck           string `json:"quick_check,omitempty"`
	Retries              int    `json:"retries"`
	Replays              int    `json:"replays"`
	Repairs              int    `json:"repairs"`
	Fallbacks            int    `json:"fallbacks"`
}

// TestMain turns a go-test binary into the reviewed, test-only VM harness only
// when the exact private role variable is present. Normal package tests retain
// the ordinary testing entrypoint.
func TestMain(m *testing.M) {
	role := os.Getenv(vmHarnessRoleEnv)
	if role == "" {
		os.Exit(m.Run())
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || evidenceCGOEnabled {
		fmt.Fprintln(os.Stderr, "VM durability harness requires linux/amd64 with CGO disabled")
		os.Exit(2)
	}
	command, err := decodeVMHarnessCommand(os.Stdin)
	if err == nil {
		switch role {
		case "checkpoint":
			err = runVMCheckpoint(command, os.Stdout)
		case "recovery":
			err = runVMRecovery(command, os.Stdout)
		default:
			err = fmt.Errorf("unknown VM durability harness role %q", role)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestLinuxVMDurabilityHarnessContract(t *testing.T) {
	for boundary, revision := range vmDurabilityBoundaries {
		if !vmHarnessIDPattern.MatchString(boundary) || revision != 1 && revision != 2 {
			t.Fatalf("invalid reviewed VM durability boundary %q=%d", boundary, revision)
		}
	}
	if len(vmDurabilityBoundaries) != 4 {
		t.Fatalf("VM durability boundary count = %d, want 4", len(vmDurabilityBoundaries))
	}
}

func TestVMParentDirectoryLifecycleIsPrivateAndStable(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "cohort")
	if err := ensureVMParentDirectory(parent); err != nil {
		t.Fatal(err)
	}
	if err := ensureVMParentDirectory(parent); err != nil {
		t.Fatalf("reuse exact private parent: %v", err)
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("parent directory changed: info=%v err=%v", info, err)
	}
	link := filepath.Join(t.TempDir(), "linked-cohort")
	if err := os.Symlink(parent, link); err != nil {
		t.Fatal(err)
	}
	if err := ensureVMParentDirectory(link); err == nil {
		t.Fatal("expected linked parent rejection")
	}
}

func decodeVMHarnessCommand(reader io.Reader) (vmHarnessCommand, error) {
	var command vmHarnessCommand
	decoder := json.NewDecoder(io.LimitReader(reader, vmHarnessMaxInputBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil {
		return command, fmt.Errorf("decode VM harness command: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return command, fmt.Errorf("VM harness command contains trailing JSON")
	}
	if command.Schema != vmHarnessSchema || !vmHarnessIDPattern.MatchString(command.CohortID) || !vmHarnessIDPattern.MatchString(command.TrialID) || command.Attempt < 1 || command.Attempt > 3 {
		return command, fmt.Errorf("VM harness command identity is invalid")
	}
	if _, ok := vmDurabilityBoundaries[command.Boundary]; !ok {
		return command, fmt.Errorf("VM harness durability boundary is not reviewed")
	}
	wantRoot := filepath.Join(vmQualificationRoot, "cohorts", command.CohortID, command.Boundary, fmt.Sprintf("attempt-%d", command.Attempt))
	if command.Root != wantRoot || !filepath.IsAbs(command.Root) || filepath.Clean(command.Root) != command.Root {
		return command, fmt.Errorf("VM harness root does not match the exact cohort, boundary, and attempt")
	}
	return command, nil
}

func runVMCheckpoint(command vmHarnessCommand, output io.Writer) error {
	if _, err := os.Lstat(command.Root); !os.IsNotExist(err) {
		return fmt.Errorf("VM trial root is not fresh")
	}
	cohortRoot := filepath.Join(vmQualificationRoot, "cohorts", command.CohortID)
	boundaryRoot := filepath.Join(cohortRoot, command.Boundary)
	for _, directory := range []string{cohortRoot, boundaryRoot} {
		if err := ensureVMParentDirectory(directory); err != nil {
			return err
		}
	}
	for _, directory := range []string{command.Root, filepath.Join(command.Root, "sqlite"), filepath.Join(command.Root, "evidence")} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return fmt.Errorf("create VM trial directory: %w", err)
		}
		if err := syncVMDirectory(filepath.Dir(directory)); err != nil {
			return err
		}
	}
	sqliteRoot := filepath.Join(command.Root, "sqlite")
	qualification, _, err := qualifyLinuxFixtureRoot(sqliteRoot)
	if err != nil {
		return fmt.Errorf("qualify VM SQLite root: %w", err)
	}
	rootIdentity, err := qualification.rootIdentity()
	if err != nil {
		return err
	}
	databasePath, err := qualification.prepareEvidenceFile("main")
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		return err
	}
	if err := requireLinuxCompileOptions(database.compileOptions); err != nil {
		return err
	}
	if err := createVMSelectedStore(ctx, database.conn, databasePath, failureRow(command.TrialID, 1), qualification); err != nil {
		closePublicationDatabase(database)
		return err
	}
	if err := closePublicationDatabase(database); err != nil {
		return err
	}
	preHash, err := qualification.stableTreeHash()
	if err != nil {
		return err
	}
	database, err = openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		return err
	}
	oldRow, newRow := failureRow(command.TrialID, 1), failureRow(command.TrialID, 2)
	evidence := vmHarnessEvidence{
		Schema: vmCheckpointSchema, CohortID: command.CohortID, TrialID: command.TrialID,
		Boundary: command.Boundary, Attempt: command.Attempt, Root: command.Root,
		RootIdentity: rootIdentity, Database: databasePath,
		ExpectedRevision: vmDurabilityBoundaries[command.Boundary], PreMetadataSHA256: preHash,
		CompileOptionsSHA256: vmCompileOptionsSHA256(database.compileOptions),
		SQLiteVersion:        selectedSQLiteVersion, SQLiteSourceID: selectedSQLiteSourceID, VFS: selectedNativeVFS(),
	}
	emitAndHold := func() error {
		if err := durableVMJSON(filepath.Join(command.Root, "evidence", "checkpoint.json"), evidence); err != nil {
			return err
		}
		if err := writeVMHarnessEvidence(output, evidence); err != nil {
			return err
		}
		select {}
	}
	switch command.Boundary {
	case "inside-commit-hook-before-phase1":
		if err := database.conn.Raw(func(driverConn any) error {
			hooker, ok := driverConn.(sqlite.HookRegisterer)
			if !ok {
				return fmt.Errorf("pinned driver connection %T lacks commit-hook support", driverConn)
			}
			hooker.RegisterCommitHook(func() int32 {
				if err := emitAndHold(); err != nil {
					panic(err)
				}
				return 1
			})
			return nil
		}); err != nil {
			return err
		}
		tx, err := beginVMCAS(ctx, database, databasePath, oldRow, newRow, qualification)
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit returned before hard power: %w", err)
		}
		return errors.New("commit returned before hard power")
	case "after-stage-before-commit":
		if _, err := beginVMCAS(ctx, database, databasePath, oldRow, newRow, qualification); err != nil {
			return err
		}
		return emitAndHold()
	case "after-commit-before-reload", "after-validation-before-ack":
		tx, err := beginVMCAS(ctx, database, databasePath, oldRow, newRow, qualification)
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit VM durability transition: %w", err)
		}
		if command.Boundary == "after-validation-before-ack" {
			if err := requireFailureStore(ctx, database, databasePath, newRow); err != nil {
				return err
			}
			if err := closePublicationDatabase(database); err != nil {
				return err
			}
			if err := qualification.requireSiblings(filepath.Dir(databasePath), true); err != nil {
				return err
			}
		}
		return emitAndHold()
	default:
		return fmt.Errorf("unreachable durability boundary")
	}
}

func ensureVMParentDirectory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return fmt.Errorf("create VM cohort directory: %w", err)
		}
		return syncVMDirectory(filepath.Dir(path))
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("VM cohort directory is not an exact private directory")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Geteuid() || int(stat.Gid) != os.Getegid() {
		return fmt.Errorf("VM cohort directory ownership changed")
	}
	return nil
}

func runVMRecovery(command vmHarnessCommand, output io.Writer) error {
	if info, err := os.Lstat(command.Root); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("VM recovery root is absent or invalid")
	}
	sqliteRoot := filepath.Join(command.Root, "sqlite")
	qualification, _, err := qualifyLinuxFixtureRoot(sqliteRoot)
	if err != nil {
		return err
	}
	rootIdentity, err := qualification.rootIdentity()
	if err != nil {
		return err
	}
	checkpoint, err := readVMCheckpoint(filepath.Join(command.Root, "evidence", "checkpoint.json"))
	if err != nil {
		return err
	}
	if checkpoint.CohortID != command.CohortID || checkpoint.TrialID != command.TrialID || checkpoint.Boundary != command.Boundary || checkpoint.Attempt != command.Attempt || checkpoint.Root != command.Root || checkpoint.RootIdentity != rootIdentity {
		return fmt.Errorf("VM checkpoint evidence identity changed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, checkpoint.Database)
	if err != nil {
		return err
	}
	if err := requireLinuxCompileOptions(database.compileOptions); err != nil {
		return err
	}
	want := failureRow(command.TrialID, checkpoint.ExpectedRevision)
	if err := requireFailureStore(ctx, database, checkpoint.Database, want); err != nil {
		return fmt.Errorf("fresh recovery-only validation: %w", err)
	}
	if err := closePublicationDatabase(database); err != nil {
		return err
	}
	if err := qualification.requireSiblings(filepath.Dir(checkpoint.Database), true); err != nil {
		return err
	}
	postHash, err := qualification.stableTreeHash()
	if err != nil {
		return err
	}
	evidence := checkpoint
	evidence.Schema = vmRecoverySchema
	evidence.ObservedRevision = want.Revision
	evidence.ObservedDigest = hex.EncodeToString(want.SHA256)
	evidence.PostMetadataSHA256 = postHash
	evidence.QuickCheck = "ok"
	if err := durableVMJSON(filepath.Join(command.Root, "evidence", "recovery.json"), evidence); err != nil {
		return err
	}
	return writeVMHarnessEvidence(output, evidence)
}

func createVMSelectedStore(ctx context.Context, conn *sql.Conn, path string, row aggregateRow, qualification *linuxQualification) error {
	if err := requireSchemaObjectCount(ctx, conn, 0); err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, selectedSchema); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version=1"); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO app_server_aggregate (singleton, pipeon_session_id, revision, canonical_json, canonical_sha256) VALUES (?, ?, ?, ?, ?)", row.Singleton, row.SessionID, row.Revision, row.Payload, row.SHA256); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := qualification.requireJournal(path + "-journal"); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return requireFailureStore(ctx, &evidenceDatabase{conn: conn, path: path}, path, row)
}

func beginVMCAS(ctx context.Context, database *evidenceDatabase, path string, oldRow, newRow aggregateRow, qualification *linuxQualification) (*sql.Tx, error) {
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	got, err := readAggregateRow(ctx, tx)
	if err != nil || !sameRow(got, oldRow) {
		_ = tx.Rollback()
		return nil, fmt.Errorf("VM transaction observation mismatch: revision=%d err=%v", got.Revision, err)
	}
	result, err := tx.ExecContext(ctx, selectedCASStatement, newRow.Revision, newRow.Payload, newRow.SHA256, oldRow.Singleton, oldRow.SessionID, oldRow.Revision, oldRow.SHA256)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		_ = tx.Rollback()
		return nil, fmt.Errorf("VM CAS affected=%d err=%v, want 1", affected, err)
	}
	got, err = readAggregateRow(ctx, tx)
	if err != nil || !sameRow(got, newRow) {
		_ = tx.Rollback()
		return nil, fmt.Errorf("VM staged row mismatch: revision=%d err=%v", got.Revision, err)
	}
	if err := qualification.requireJournal(path + "-journal"); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func readVMCheckpoint(path string) (vmHarnessEvidence, error) {
	var evidence vmHarnessEvidence
	data, err := os.ReadFile(path)
	if err != nil {
		return evidence, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return evidence, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || evidence.Schema != vmCheckpointSchema {
		return evidence, fmt.Errorf("checkpoint evidence is malformed")
	}
	return evidence, nil
}

func writeVMHarnessEvidence(output io.Writer, evidence vmHarnessEvidence) error {
	writer := bufio.NewWriterSize(output, 32*1024)
	if err := json.NewEncoder(writer).Encode(evidence); err != nil {
		return err
	}
	return writer.Flush()
}

func durableVMJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncVMDirectory(filepath.Dir(path))
}

func syncVMDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func vmCompileOptionsSHA256(options []string) string {
	copy := append([]string(nil), options...)
	sort.Strings(copy)
	digest := sha256.Sum256([]byte(strings.Join(copy, "\n") + "\n"))
	return hex.EncodeToString(digest[:])
}
