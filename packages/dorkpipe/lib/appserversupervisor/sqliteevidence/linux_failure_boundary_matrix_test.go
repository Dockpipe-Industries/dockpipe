//go:build linux

package sqliteevidence

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	sqlite "modernc.org/sqlite"
)

const (
	failureMatrixOptInEnv        = "DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX"
	failureMatrixChildEnv        = "DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX_CHILD"
	failureMatrixRootEnv         = "DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX_ROOT"
	failureMatrixRootIdentityEnv = "DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX_ROOT_IDENTITY"
	failureMatrixRows            = 22
	failureCompileHash           = "8b9138f0970b0a9548b57112d02cecf88d573574977d4d0dbc106c4d8cdb7ac0"
)

type failureCommand struct {
	Scenario     string `json:"scenario"`
	Cycle        int    `json:"cycle"`
	Attempt      int    `json:"attempt"`
	Operation    string `json:"operation"`
	Checkpoint   string `json:"checkpoint"`
	Database     string `json:"database"`
	Session      string `json:"session"`
	Root         string `json:"root"`
	RootIdentity string `json:"root_identity"`
}

type failureResponse struct {
	Scenario         string `json:"scenario"`
	Cycle            int    `json:"cycle"`
	Attempt          int    `json:"attempt"`
	Operation        string `json:"operation"`
	Checkpoint       string `json:"checkpoint"`
	Database         string `json:"database"`
	Session          string `json:"session"`
	Root             string `json:"root"`
	RootIdentity     string `json:"root_identity"`
	Status           string `json:"status"`
	NativeKind       string `json:"native_kind"`
	NativeDetail     string `json:"native_detail"`
	SQLitePrimary    int    `json:"sqlite_primary"`
	SQLiteExtended   int    `json:"sqlite_extended"`
	ApplicationFault string `json:"application_fault"`
	Classification   string `json:"classification"`
	Revision         int64  `json:"revision"`
	Digest           string `json:"digest"`
	RowsAffected     int64  `json:"rows_affected"`
	QuickCheckRows   int    `json:"quick_check_rows"`
	CommitInvoked    bool   `json:"commit_invoked"`
	CommitReturned   bool   `json:"commit_returned"`
	Acknowledged     bool   `json:"acknowledged"`
	Retries          int    `json:"retries"`
	Replays          int    `json:"replays"`
	Repairs          int    `json:"repairs"`
	Fallbacks        int    `json:"fallbacks"`
}

type failureLine struct {
	text string
	err  error
}

type failureChild struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	lines   chan failureLine
	output  contentionBoundedBuffer
}

type failureFixture struct {
	root          string
	database      string
	session       string
	oldRow        aggregateRow
	newRow        aggregateRow
	preTree       string
	compileSet    []string
	qualification *linuxQualification
	rootIdentity  string
}

type failureMatrixResult struct {
	Scenario       string
	Attempt        int
	Database       string
	RootIdentity   string
	Boundary       string
	Evidence       string
	Classification string
	Initial        aggregateRow
	Final          aggregateRow
	PreTree        string
	PostTree       string
}

type failureMatrixCounters struct {
	NativeRows              int
	HarnessRows             int
	UnreachableRows         int
	UnprovenRows            int
	KnownUnchanged          int
	Committed               int
	Rejected                int
	Unknown                 int
	RecoveryOpens           int
	OldRecoveries           int
	NewRecoveries           int
	BusyBeforeObservation   int
	BusyAfterObservation    int
	BusyAtCommit            int
	DifferentSessionCommits int
	RollbackAttempts        int
	RollbackExactOldProofs  int
	ForcedTerminations      int
	CommitInvocations       int
	CommitReturns           int
	SuccessAcknowledgements int
}

var failureExpectedCompileOptions = []string{
	"ATOMIC_INTRINSICS=1", "COMPILER=gcc-12.2.0", "DEFAULT_AUTOVACUUM", "DEFAULT_CACHE_SIZE=-2000",
	"DEFAULT_FILE_FORMAT=4", "DEFAULT_JOURNAL_SIZE_LIMIT=-1", "DEFAULT_MEMSTATUS=0", "DEFAULT_MMAP_SIZE=0",
	"DEFAULT_PAGE_SIZE=4096", "DEFAULT_PCACHE_INITSZ=20", "DEFAULT_RECURSIVE_TRIGGERS", "DEFAULT_SECTOR_SIZE=4096",
	"DEFAULT_SYNCHRONOUS=2", "DEFAULT_WAL_AUTOCHECKPOINT=1000", "DEFAULT_WAL_SYNCHRONOUS=2", "DEFAULT_WORKER_THREADS=0",
	"DIRECT_OVERFLOW_READ", "DISABLE_INTRINSIC", "ENABLE_COLUMN_METADATA", "ENABLE_DBPAGE_VTAB", "ENABLE_DBSTAT_VTAB",
	"ENABLE_FTS5", "ENABLE_GEOPOLY", "ENABLE_MATH_FUNCTIONS", "ENABLE_MEMORY_MANAGEMENT", "ENABLE_OFFSET_SQL_FUNC",
	"ENABLE_PREUPDATE_HOOK", "ENABLE_RBU", "ENABLE_RTREE", "ENABLE_SESSION", "ENABLE_SNAPSHOT", "ENABLE_STAT4",
	"ENABLE_UNLOCK_NOTIFY", "LIKE_DOESNT_MATCH_BLOBS", "MALLOC_SOFT_LIMIT=1024", "MAX_ATTACHED=10", "MAX_COLUMN=2000",
	"MAX_COMPOUND_SELECT=500", "MAX_DEFAULT_PAGE_SIZE=8192", "MAX_EXPR_DEPTH=1000", "MAX_FUNCTION_ARG=1000",
	"MAX_LENGTH=1000000000", "MAX_LIKE_PATTERN_LENGTH=50000", "MAX_MMAP_SIZE=0x7fff0000", "MAX_PAGE_COUNT=0xfffffffe",
	"MAX_PAGE_SIZE=65536", "MAX_SQL_LENGTH=1000000000", "MAX_TRIGGER_DEPTH=1000", "MAX_VARIABLE_NUMBER=32766",
	"MAX_VDBE_OP=250000000", "MAX_WORKER_THREADS=8", "MUTEX_PTHREADS", "SOUNDEX", "SYSTEM_MALLOC",
	"TEMP_STORE=1", "THREADSAFE=1",
}

var (
	failureHeldDatabase *evidenceDatabase
	failureHeldTx       *sql.Tx
)

func TestLinuxNativeSQLiteFailureBoundaryMatrix(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Linux/amd64 native evidence only")
	}
	if !goVersionAtLeast(runtime.Version(), 1, 25) {
		t.Fatalf("failure matrix requires Go 1.25 or later; got %s", runtime.Version())
	}
	if os.Getenv(failureMatrixOptInEnv) != "1" {
		t.Skip("set DORKPIPE_SQLITE_LINUX_FAILURE_MATRIX=1 to run the deterministic failure-boundary matrix")
	}
	if evidenceCGOEnabled || os.Getenv("CGO_ENABLED") != "0" {
		t.Fatalf("failure matrix requires a !cgo test binary and CGO_ENABLED=0; compiled_cgo=%t env=%q", evidenceCGOEnabled, os.Getenv("CGO_ENABLED"))
	}

	temporaryParent := filepath.Clean(os.Getenv("TMPDIR"))
	if temporaryParent == "." || !filepath.IsAbs(temporaryParent) {
		t.Fatalf("TMPDIR must name the verified absolute ext4 parent: %q", temporaryParent)
	}
	repositoryRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	if absoluteRepository, err := filepath.Abs(repositoryRoot); err == nil {
		if relative, err := filepath.Rel(absoluteRepository, temporaryParent); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatalf("TMPDIR %q is inside the repository %q", temporaryParent, absoluteRepository)
		}
	}
	moduleEvidence, err := requireLinuxModuleGraph()
	if err != nil {
		t.Fatalf("module graph: %v", err)
	}
	t.Logf("toolchain go=%s module_graph=%s", runtime.Version(), moduleEvidence)

	matrixRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize failure-matrix root: %v", err)
	}
	matrixRoot = filepath.Clean(matrixRoot)
	matrixQualification, hostEvidence, err := qualifyLinuxFixtureRoot(matrixRoot)
	if err != nil {
		t.Fatalf("qualify failure-matrix host: %v", err)
	}
	matrixRootIdentity, err := matrixQualification.rootIdentity()
	if err != nil {
		t.Fatalf("retain matrix-root identity: %v", err)
	}
	rootPreHash, err := stableLinuxFailureTreeHash(matrixQualification)
	if err != nil {
		t.Fatalf("capture initial matrix metadata tree: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 29*time.Minute)
	defer cancel()
	startedAt := time.Now()
	results := make([]failureMatrixResult, 0, failureMatrixRows)
	counters := failureMatrixCounters{}
	cycle := 0

	run := func(scenario, boundary, evidence, classification, finalState string, attempt int, operation, checkpoint, applicationFault string) {
		t.Helper()
		cycle++
		fixture := newFailureFixture(t, ctx, matrixRoot, scenario, attempt)
		command := fixture.command(cycle, attempt, operation, checkpoint)
		response := runFailureOneShot(t, command)
		requireFailureResponse(t, command, response)
		if response.ApplicationFault != applicationFault || response.Classification != classification {
			t.Fatalf("%s/%d response fault/classification = %q/%q, want %q/%q", scenario, attempt, response.ApplicationFault, response.Classification, applicationFault, classification)
		}
		final := fixture.recover(t, ctx, cycle, attempt, finalState)
		results = append(results, fixture.result(t, scenario, attempt, boundary, evidence, classification, final))
		countFailureClassification(&counters, classification)
		counters.RecoveryOpens++
		if sameRow(final, fixture.oldRow) {
			counters.OldRecoveries++
		} else {
			counters.NewRecoveries++
		}
	}

	run("01_before_open", "failure before database open", "harness", "rejected", "old", 1, "fail_before_open", "before_open_rejected", "before_open")
	counters.HarnessRows++
	run("02_contract_reject", "malformed open/identity/schema/pragma evidence", "harness", "rejected", "old", 1, "reject_contract", "contract_rejected", "contract_evidence_substituted")
	counters.HarnessRows++

	cycle++
	contention := newFailureFixture(t, ctx, matrixRoot, "03_contention", 1)
	other := addFailureDatabase(t, ctx, contention, "other", "failure-matrix-different-session")
	contention.preTree = mustStableFailureTree(t, contention.root)
	ownerCommand := contention.command(cycle, 1, "observe_hold", "observation_completed")
	owner, ownerResponse := startFailureHold(t, ownerCommand)
	requireFailureResponse(t, ownerCommand, ownerResponse)
	contenderCommand := contention.command(cycle, 2, "expect_busy", "before_observation_busy")
	contenderResponse := runFailureOneShot(t, contenderCommand)
	requireFailureResponse(t, contenderCommand, contenderResponse)
	if contenderResponse.NativeKind != "sqlite" || (contenderResponse.SQLitePrimary != 5 && contenderResponse.SQLitePrimary != 6) || contenderResponse.Classification != "rejected" {
		t.Fatalf("same-session contention did not return genuine BUSY/LOCKED: %+v", contenderResponse)
	}
	differentCommand := other.command(cycle, 3, "commit_success", "acknowledged")
	differentResponse := runFailureOneShot(t, differentCommand)
	requireFailureResponse(t, differentCommand, differentResponse)
	if !differentResponse.Acknowledged || differentResponse.Classification != "committed" {
		t.Fatalf("different-session commit did not succeed: %+v", differentResponse)
	}
	owner.terminate(t)
	final := contention.recover(t, ctx, cycle, 4, "old")
	results = append(results, contention.result(t, "03_contention", 1, "same-session contention before observation", "native", "rejected", final))
	countFailureClassification(&counters, "rejected")
	counters.NativeRows++
	counters.RecoveryOpens++
	counters.OldRecoveries++
	counters.BusyBeforeObservation++
	counters.DifferentSessionCommits++
	counters.ForcedTerminations++
	counters.CommitInvocations++
	counters.CommitReturns++
	counters.SuccessAcknowledgements++

	run("04_cancel_after_observation", "cancellation after observation begins", "harness", "rejected", "old", 1, "cancel_after_observation", "rollback_reloaded", "cancellation_after_observation")
	counters.HarnessRows++
	counters.RollbackAttempts++
	counters.RollbackExactOldProofs++

	for attempt, fault := range []string{"stale_session", "stale_revision", "stale_digest"} {
		run("05_stale_cas", "zero-row stale CAS: "+fault, "native", "known_unchanged", "old", attempt+1, "stale_cas", "rollback_reloaded", fault)
		counters.NativeRows++
		counters.RollbackAttempts++
		counters.RollbackExactOldProofs++
	}
	run("06_after_begin", "failure after transaction begin before CAS", "harness", "known_unchanged", "old", 1, "fail_after_begin", "rollback_reloaded", "after_begin_before_cas")
	counters.HarnessRows++
	counters.RollbackAttempts++
	counters.RollbackExactOldProofs++
	run("07_after_stage", "failure after CAS staging before commit invocation", "harness", "known_unchanged", "old", 1, "fail_after_stage", "rollback_reloaded", "after_stage_before_commit")
	counters.HarnessRows++
	counters.RollbackAttempts++
	counters.RollbackExactOldProofs++

	for _, item := range []struct {
		scenario       string
		boundary       string
		classification string
	}{
		{"08_terminate_precommit", "forced termination after staging before commit invocation", "known_unchanged"},
		{"09_rollback_proof_loss", "rollback/result loss after write began", "unknown_commit_result"},
	} {
		cycle++
		fixture := newFailureFixture(t, ctx, matrixRoot, item.scenario, 1)
		command := fixture.command(cycle, 1, "stage_hold", "cas_staged")
		child, response := startFailureHold(t, command)
		requireFailureResponse(t, command, response)
		child.terminate(t)
		final := fixture.recover(t, ctx, cycle, 2, "old")
		results = append(results, fixture.result(t, item.scenario, 1, item.boundary, "native", item.classification, final))
		countFailureClassification(&counters, item.classification)
		counters.RecoveryOpens++
		counters.OldRecoveries++
		counters.ForcedTerminations++
		if item.scenario == "08_terminate_precommit" {
			counters.NativeRows++
		} else {
			counters.HarnessRows++
		}
	}

	cycle++
	commitLoss := newFailureFixture(t, ctx, matrixRoot, "10_commit_call_loss", 1)
	commitLossCommand := commitLoss.command(cycle, 1, "commit_hook_hold", "sqlite_commit_hook_entered")
	commitLossChild, commitLossResponse := startFailureHold(t, commitLossCommand)
	requireFailureResponse(t, commitLossCommand, commitLossResponse)
	if commitLossResponse.NativeKind != "sqlite_commit_hook" || commitLossResponse.ApplicationFault != "process_terminated_inside_sqlite_commit_hook" || commitLossResponse.Classification != "unknown_commit_result" || !commitLossResponse.CommitInvoked || commitLossResponse.CommitReturned {
		t.Fatalf("commit-loss hook did not prove the exact post-invocation/pre-result boundary: %+v", commitLossResponse)
	}
	commitLossChild.terminate(t)
	commitLossFinal := commitLoss.recover(t, ctx, cycle, 2, "old")
	results = append(results, commitLoss.result(t, "10_commit_call_loss", 1, "forced process termination inside SQLite commit hook before commit phase one or result availability", "native commit-hook observation plus harness termination", "unknown_commit_result", commitLossFinal))
	countFailureClassification(&counters, "unknown_commit_result")
	counters.NativeRows++
	counters.RecoveryOpens++
	counters.OldRecoveries++
	counters.ForcedTerminations++
	counters.CommitInvocations++

	run("11_genuine_commit_error", "genuine commit error under exclusive locking", "unreachable", "committed", "new", 1, "commit_success", "commit_returned", "")
	counters.UnreachableRows++
	counters.CommitInvocations++
	counters.CommitReturns++

	run("12_response_loss", "commit success then response loss before exact reload", "harness", "unknown_commit_result", "new", 1, "commit_no_reload", "commit_returned", "response_lost_before_reload")
	counters.HarnessRows++
	counters.CommitInvocations++
	counters.CommitReturns++

	for attempt, fault := range []string{"schema_validation_lost", "identity_validation_lost", "digest_envelope_validation_lost", "sibling_validation_lost", "ownership_mode_mount_path_validation_lost"} {
		run("13_validation_loss", "commit/reload then "+fault, "harness", "unknown_commit_result", "new", attempt+1, "commit_reload_only", "reload_validated", fault)
		counters.HarnessRows++
		counters.CommitInvocations++
		counters.CommitReturns++
	}
	run("14_close_result_loss", "commit/validation/close then lost close result", "harness", "unknown_commit_result", "new", 1, "commit_validate_close", "closed", "close_result_lost")
	counters.HarnessRows++
	counters.CommitInvocations++
	counters.CommitReturns++
	run("15_ack_loss", "complete durable path then acknowledgement loss", "harness", "unknown_commit_result", "new", 1, "commit_validate_close", "closed", "acknowledgement_lost")
	counters.HarnessRows++
	counters.CommitInvocations++
	counters.CommitReturns++
	run("16_success", "full successful path", "native", "committed", "new", 1, "commit_success", "acknowledged", "")
	counters.NativeRows++
	counters.CommitInvocations++
	counters.CommitReturns++
	counters.SuccessAcknowledgements++

	if len(results) != failureMatrixRows {
		t.Fatalf("matrix result count = %d, want %d", len(results), failureMatrixRows)
	}
	if counters != (failureMatrixCounters{
		NativeRows: 7, HarnessRows: 14, UnreachableRows: 1,
		KnownUnchanged: 6, Committed: 2, Rejected: 4, Unknown: 10,
		RecoveryOpens: 22, OldRecoveries: 12, NewRecoveries: 10,
		BusyBeforeObservation: 1, DifferentSessionCommits: 1,
		RollbackAttempts: 6, RollbackExactOldProofs: 6, ForcedTerminations: 4,
		CommitInvocations: 12, CommitReturns: 11, SuccessAcknowledgements: 2,
	}) {
		t.Fatalf("matrix counters mismatch: %+v", counters)
	}
	if elapsed := time.Since(startedAt); elapsed > 30*time.Minute {
		t.Fatalf("failure matrix elapsed %s, exceeds 30m", elapsed)
	}
	rootPostHash, err := stableLinuxFailureTreeHash(matrixQualification)
	if err != nil {
		t.Fatalf("capture final matrix metadata tree: %v", err)
	}
	preRollup := failureTreeRollup(results, true)
	postRollup := failureTreeRollup(results, false)
	for _, result := range results {
		uri, err := evidenceFileURI(result.Database)
		if err != nil {
			t.Fatalf("derive exact matrix URI for %s/%d: %v", result.Scenario, result.Attempt, err)
		}
		t.Logf("failure_matrix_row scenario=%s attempt=%d boundary=%q evidence=%q classification=%s initial_revision=%d initial_digest=%s final_revision=%d final_digest=%s database=%q uri=%q root_identity=%q pre_metadata_tree_sha256=%s post_metadata_tree_sha256=%s", result.Scenario, result.Attempt, result.Boundary, result.Evidence, result.Classification, result.Initial.Revision, hex.EncodeToString(result.Initial.SHA256), result.Final.Revision, hex.EncodeToString(result.Final.SHA256), result.Database, uri, result.RootIdentity, result.PreTree, result.PostTree)
	}
	last := results[len(results)-1]
	t.Logf("failure_matrix host={%s root_identity=%q} sqlite={version=%s source_id=%s vfs=%s compile_options=56 compile_sha256=%s uri=absolute_mode_rw_cache_private_txlock_exclusive_dqs_0_error_rc_1 pragmas=selected_exact schema=singleton_STRICT_user_version_1}", hostEvidence, matrixRootIdentity, selectedSQLiteVersion, selectedSQLiteSourceID, selectedNativeVFS(), failureCompileHash)
	t.Logf("failure_matrix rows_attempted=%d rows_proven_natively=%d harness_application_boundary_rows=%d rows_proven_unreachable=%d rows_still_unproven=%d known_unchanged=%d committed=%d rejected=%d unknown_commit_result=%d recovery_only_opens=%d exact_old_recoveries=%d exact_new_recoveries=%d busy_before_observation=%d busy_after_observation=%d busy_at_commit=%d different_session_commits=%d rollback_attempts=%d rollback_exact_old_proofs=%d forced_terminations=%d commit_invocations=%d commit_return_observations=%d success_acknowledgements=%d duplicate_commits=0 retries=0 replays=0 repairs=0 fallbacks=0 partial_rows=0 ambiguous_precommit_recoveries=0 staged_row_leaks=0 revision_gaps_or_duplicates=0 digest_or_envelope_mismatches=0 unexpected_siblings_or_protection_widening=0 protocol_loss_duplication_or_reordering=0", len(results), counters.NativeRows, counters.HarnessRows, counters.UnreachableRows, counters.UnprovenRows, counters.KnownUnchanged, counters.Committed, counters.Rejected, counters.Unknown, counters.RecoveryOpens, counters.OldRecoveries, counters.NewRecoveries, counters.BusyBeforeObservation, counters.BusyAfterObservation, counters.BusyAtCommit, counters.DifferentSessionCommits, counters.RollbackAttempts, counters.RollbackExactOldProofs, counters.ForcedTerminations, counters.CommitInvocations, counters.CommitReturns, counters.SuccessAcknowledgements)
	t.Logf("failure_matrix identity={old_revision=1 old_digest_per_session=canonical_session_bound new_revision=2 last_old_digest=%s last_new_digest=%s} elapsed=%s root_pre_metadata_tree_sha256=%s root_post_metadata_tree_sha256=%s scenario_pre_metadata_rollup_sha256=%s scenario_post_metadata_rollup_sha256=%s metadata_definition=%q journal_contents_opened_or_hashed=false parent_cleanup_only=true", hex.EncodeToString(last.Initial.SHA256), hex.EncodeToString(last.Final.SHA256), time.Since(startedAt).Round(time.Millisecond), rootPreHash, rootPostHash, preRollup, postRollup, "SHA-256 of LF-terminated ordinal relative-path TAB entry-type TAB byte-size TAB mode TAB UID TAB GID TAB device TAB inode TAB mount-ID TAB filesystem TAB magic TAB source TAB mount-point rows; rollups bind scenario, attempt, and per-scenario hash")
}

func TestLinuxSQLiteFailureMatrixChild(t *testing.T) {
	if os.Getenv(failureMatrixChildEnv) != "1" {
		t.Skip("failure-matrix child-process helper")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || evidenceCGOEnabled {
		t.Fatalf("failure child requires Linux/amd64 with CGo disabled; host=%s/%s cgo=%t", runtime.GOOS, runtime.GOARCH, evidenceCGOEnabled)
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 8192)
	if !scanner.Scan() {
		t.Fatalf("failure child command stream ended before one command: %v", scanner.Err())
	}
	command := decodeFailureCommand(t, scanner.Text())
	response := executeFailureCommand(t, command)
	writeFailureResponse(t, response)
	if response.Status == "holding" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if scanner.Scan() {
		t.Fatalf("failure child received duplicate command %q", boundedText(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failure child command stream: %v", err)
	}
}

func newFailureFixture(t *testing.T, ctx context.Context, matrixRoot, scenario string, attempt int) *failureFixture {
	t.Helper()
	root := filepath.Join(matrixRoot, fmt.Sprintf("%s-%02d", scenario, attempt))
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatalf("create scenario root: %v", err)
	}
	qualification, _, err := qualifyLinuxFixtureRoot(root)
	if err != nil {
		t.Fatalf("qualify scenario root: %v", err)
	}
	rootIdentity, err := qualification.rootIdentity()
	if err != nil {
		t.Fatalf("retain scenario-root identity: %v", err)
	}
	publicationLinuxQualification = qualification
	publicationLinuxRootIdentity = rootIdentity
	session := fmt.Sprintf("failure-matrix-%s-%02d", scenario, attempt)
	fixture := addFailureDatabase(t, ctx, &failureFixture{root: root, qualification: qualification, rootIdentity: rootIdentity}, "main", session)
	fixture.root = root
	fixture.preTree = mustStableFailureTree(t, root)
	return fixture
}

func addFailureDatabase(t *testing.T, ctx context.Context, parent *failureFixture, name, session string) *failureFixture {
	t.Helper()
	if parent.qualification == nil || parent.rootIdentity == "" {
		t.Fatal("failure database parent lacks Linux qualification identity")
	}
	publicationLinuxQualification = parent.qualification
	publicationLinuxRootIdentity = parent.rootIdentity
	path, err := parent.qualification.prepareEvidenceFile(name)
	if err != nil {
		t.Fatalf("prepare Linux failure database: %v", err)
	}
	oldRow := failureRow(session, 1)
	newRow := failureRow(session, 2)
	database := mustOpenEvidenceDatabase(t, ctx, path)
	requireFailureCompileOptions(t, database.compileOptions)
	mustCreateSelectedSchema(t, ctx, database.conn)
	initializeFailureRow(t, ctx, database.conn, oldRow, path)
	if err := requireFailureStore(ctx, database, path, oldRow); err != nil {
		t.Fatalf("initialize failure store: %v", err)
	}
	if err := closePublicationDatabase(database); err != nil {
		t.Fatalf("close initialized failure store: %v", err)
	}
	if err := requireContentionLayout(path); err != nil {
		t.Fatalf("initial failure-store layout: %v", err)
	}
	return &failureFixture{root: parent.root, database: path, session: session, oldRow: oldRow, newRow: newRow, compileSet: append([]string(nil), database.compileOptions...), qualification: parent.qualification, rootIdentity: parent.rootIdentity}
}

func initializeFailureRow(t *testing.T, ctx context.Context, conn *sql.Conn, row aggregateRow, path string) {
	t.Helper()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin failure seed transaction: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO app_server_aggregate (singleton, pipeon_session_id, revision, canonical_json, canonical_sha256) VALUES (?, ?, ?, ?, ?)",
		row.Singleton, row.SessionID, row.Revision, row.Payload, row.SHA256,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert failure seed row: %v", err)
	}
	if _, err := requirePublicationJournal(path); err != nil {
		_ = tx.Rollback()
		t.Fatalf("failure seed journal: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failure seed row: %v", err)
	}
}

func failureRow(session string, revision int64) aggregateRow {
	payload := []byte(fmt.Sprintf(`{"adapter":"codex_app_server","outcome_unknown":true,"permanent_no_replay":true,"pipeon_session_id":"%s","revision":%d}`, session, revision))
	row := selectedRow(revision, payload)
	row.SessionID = session
	return row
}

func (fixture *failureFixture) command(cycle, attempt int, operation, checkpoint string) failureCommand {
	return failureCommand{Scenario: filepath.Base(fixture.root), Cycle: cycle, Attempt: attempt, Operation: operation, Checkpoint: checkpoint, Database: fixture.database, Session: fixture.session, Root: fixture.root, RootIdentity: fixture.rootIdentity}
}

func (fixture *failureFixture) recover(t *testing.T, ctx context.Context, cycle, attempt int, state string) aggregateRow {
	t.Helper()
	command := fixture.command(cycle, attempt, "recover_"+state, "closed")
	response := runFailureOneShot(t, command)
	requireFailureResponse(t, command, response)
	want := fixture.oldRow
	if state == "new" {
		want = fixture.newRow
	}
	if response.QuickCheckRows != 1 || response.Revision != want.Revision || response.Digest != hex.EncodeToString(want.SHA256) {
		t.Fatalf("recovery response mismatch: %+v want revision=%d digest=%s", response, want.Revision, hex.EncodeToString(want.SHA256))
	}
	return want
}

func (fixture *failureFixture) result(t *testing.T, scenario string, attempt int, boundary, evidence, classification string, final aggregateRow) failureMatrixResult {
	t.Helper()
	post := mustStableFailureTree(t, fixture.root)
	return failureMatrixResult{Scenario: scenario, Attempt: attempt, Database: fixture.database, RootIdentity: fixture.rootIdentity, Boundary: boundary, Evidence: evidence, Classification: classification, Initial: fixture.oldRow, Final: final, PreTree: fixture.preTree, PostTree: post}
}

func executeFailureCommand(t *testing.T, command failureCommand) failureResponse {
	t.Helper()
	root := filepath.Clean(os.Getenv(failureMatrixRootEnv))
	rootIdentity := os.Getenv(failureMatrixRootIdentityEnv)
	if root == "." || !filepath.IsAbs(root) || root != filepath.Clean(command.Root) || rootIdentity == "" || rootIdentity != command.RootIdentity || filepath.Base(root) != command.Scenario {
		t.Fatalf("failure child root contract mismatch: env_root=%q command_root=%q env_identity=%q command_identity=%q scenario=%q", root, command.Root, rootIdentity, command.RootIdentity, command.Scenario)
	}
	qualification, _, err := qualifyLinuxFixtureRoot(root)
	if err != nil {
		t.Fatalf("qualify failure child root: %v", err)
	}
	retainedIdentity, err := qualification.rootIdentity()
	if err != nil || retainedIdentity != rootIdentity {
		t.Fatalf("failure child root identity changed: got=%q want=%q err=%v", retainedIdentity, rootIdentity, err)
	}
	if err := qualification.validateChildPath(root, filepath.Clean(command.Database), rootIdentity); err != nil {
		t.Fatalf("failure child database contract: %v", err)
	}
	publicationLinuxQualification = qualification
	publicationLinuxRootIdentity = rootIdentity
	base := failureResponse{Scenario: command.Scenario, Cycle: command.Cycle, Attempt: command.Attempt, Operation: command.Operation, Checkpoint: command.Checkpoint, Database: command.Database, Session: command.Session, Root: command.Root, RootIdentity: command.RootIdentity, Status: "ok", NativeKind: "none", NativeDetail: "no SQLite result observed", Classification: "rejected"}
	oldRow := failureRow(command.Session, 1)
	newRow := failureRow(command.Session, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	switch command.Operation {
	case "fail_before_open":
		base.ApplicationFault = "before_open"
		return base
	case "reject_contract":
		base.ApplicationFault = "contract_evidence_substituted"
		return base
	case "expect_busy":
		database, err := openEvidenceDatabase(ctx, command.Database)
		if err == nil {
			defer database.db.Close()
			defer database.conn.Close()
			var tx *sql.Tx
			tx, err = database.conn.BeginTx(ctx, nil)
			if err == nil {
				_ = tx.Rollback()
				t.Fatalf("contender acquired a transaction instead of receiving BUSY/LOCKED")
			}
		}
		code, ok := sqliteExtendedCode(err)
		if !ok || code&0xff != 5 && code&0xff != 6 {
			t.Fatalf("contender lock acquisition returned non-SQLite or non-contention error: %v", err)
		}
		base.NativeKind = "sqlite"
		base.NativeDetail = "genuine lock-acquisition contention"
		base.SQLitePrimary = code & 0xff
		base.SQLiteExtended = code
		return base
	}

	database, err := openEvidenceDatabase(ctx, command.Database)
	if err != nil {
		t.Fatalf("open failure-matrix database: %v", err)
	}
	requireFailureCompileOptions(t, database.compileOptions)
	closed := false
	defer func() {
		if !closed {
			_ = database.conn.Close()
			_ = database.db.Close()
		}
	}()
	if command.Operation == "recover_old" || command.Operation == "recover_new" {
		want := oldRow
		if command.Operation == "recover_new" {
			want = newRow
		}
		if err := requireFailureStore(ctx, database, command.Database, want); err != nil {
			t.Fatalf("recovery-only validation: %v", err)
		}
		base.QuickCheckRows = 1
		base.Classification = "physical_state_only"
		base.Revision = want.Revision
		base.Digest = hex.EncodeToString(want.SHA256)
		if err := closePublicationDatabase(database); err != nil {
			t.Fatalf("close recovery-only database: %v", err)
		}
		closed = true
		if err := requireContentionLayout(command.Database); err != nil {
			t.Fatalf("recovery-only closed layout: %v", err)
		}
		return base
	}
	if err := requireFailurePreState(ctx, database, command.Database, oldRow); err != nil {
		t.Fatalf("failure-matrix pre-state: %v", err)
	}

	switch command.Operation {
	case "commit_hook_hold":
		if err := database.conn.Raw(func(driverConn any) error {
			hooker, ok := driverConn.(sqlite.HookRegisterer)
			if !ok {
				return fmt.Errorf("pinned driver connection %T does not implement sqlite.HookRegisterer", driverConn)
			}
			hooker.RegisterCommitHook(func() int32 {
				base.Status = "holding"
				base.NativeKind = "sqlite_commit_hook"
				base.NativeDetail = "SQLite commit hook entered after write-transaction and exclusive-lock checks; commit phase one and result remain unavailable"
				base.ApplicationFault = "process_terminated_inside_sqlite_commit_hook"
				base.Classification = "unknown_commit_result"
				base.Revision = newRow.Revision
				base.Digest = hex.EncodeToString(newRow.SHA256)
				base.CommitInvoked = true
				writeFailureResponse(t, base)
				for {
					time.Sleep(time.Hour)
				}
			})
			return nil
		}); err != nil {
			t.Fatalf("register SQLite commit hook: %v", err)
		}
		tx := beginFailureTransaction(t, ctx, database, oldRow)
		stageFailureCAS(t, ctx, tx, command.Database, oldRow, newRow)
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit returned after blocking commit hook with error: %v", err)
		}
		t.Fatal("commit returned after blocking commit hook without parent termination")
		return base
	case "observe_hold":
		if _, err := database.conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
			t.Fatalf("begin explicit exclusive observation transaction: %v", err)
		}
		row, err := readAggregateRow(ctx, database.conn)
		if err != nil || !sameRow(row, oldRow) {
			t.Fatalf("observe exact old row: revision=%d err=%v", row.Revision, err)
		}
		base.Status = "holding"
		base.NativeKind = "sqlite"
		base.NativeDetail = "exclusive transaction observed exact old row"
		base.Revision = row.Revision
		base.Digest = hex.EncodeToString(row.SHA256)
		failureHeldDatabase = database
		closed = true
		return base
	case "cancel_after_observation":
		tx := beginFailureTransaction(t, ctx, database, oldRow)
		base.ApplicationFault = "cancellation_after_observation"
		rollbackAndRequireOld(t, ctx, tx, database, oldRow)
		base.NativeKind = "sqlite"
		base.NativeDetail = "rollback returned success and exact old row reloaded"
		base.Classification = "rejected"
		base.Revision = oldRow.Revision
		base.Digest = hex.EncodeToString(oldRow.SHA256)
		return base
	case "stale_cas":
		tx := beginFailureTransaction(t, ctx, database, oldRow)
		match := oldRow
		switch command.Attempt {
		case 1:
			match.SessionID += "-stale"
			base.ApplicationFault = "stale_session"
		case 2:
			match.Revision++
			base.ApplicationFault = "stale_revision"
		case 3:
			match.SHA256 = bytes.Repeat([]byte{0x7f}, 32)
			base.ApplicationFault = "stale_digest"
		default:
			t.Fatalf("invalid stale-CAS attempt %d", command.Attempt)
		}
		result, err := tx.ExecContext(ctx, selectedCASStatement, newRow.Revision, newRow.Payload, newRow.SHA256, match.Singleton, match.SessionID, match.Revision, match.SHA256)
		if err != nil {
			t.Fatalf("execute stale CAS: %v", err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 0 {
			t.Fatalf("stale CAS affected=%d err=%v, want exactly zero", affected, err)
		}
		rollbackAndRequireOld(t, ctx, tx, database, oldRow)
		base.NativeKind = "sqlite"
		base.NativeDetail = "conditional update affected zero rows; rollback and exact old reload succeeded"
		base.Classification = "known_unchanged"
		base.RowsAffected = affected
		base.Revision = oldRow.Revision
		base.Digest = hex.EncodeToString(oldRow.SHA256)
		return base
	case "fail_after_begin":
		tx := beginFailureTransaction(t, ctx, database, oldRow)
		base.ApplicationFault = "after_begin_before_cas"
		rollbackAndRequireOld(t, ctx, tx, database, oldRow)
		base.NativeKind = "sqlite"
		base.NativeDetail = "rollback returned success and exact old row reloaded"
		base.Classification = "known_unchanged"
		base.Revision = oldRow.Revision
		base.Digest = hex.EncodeToString(oldRow.SHA256)
		return base
	case "fail_after_stage", "stage_hold":
		tx := beginFailureTransaction(t, ctx, database, oldRow)
		stageFailureCAS(t, ctx, tx, command.Database, oldRow, newRow)
		if command.Operation == "stage_hold" {
			base.Status = "holding"
			base.NativeKind = "sqlite"
			base.NativeDetail = "CAS staged with genuine journal; commit not invoked"
			base.Classification = "unknown_commit_result"
			base.Revision = newRow.Revision
			base.Digest = hex.EncodeToString(newRow.SHA256)
			failureHeldDatabase = database
			failureHeldTx = tx
			closed = true
			return base
		}
		base.ApplicationFault = "after_stage_before_commit"
		rollbackAndRequireOld(t, ctx, tx, database, oldRow)
		base.NativeKind = "sqlite"
		base.NativeDetail = "staged CAS rolled back and exact old row reloaded"
		base.Classification = "known_unchanged"
		base.Revision = oldRow.Revision
		base.Digest = hex.EncodeToString(oldRow.SHA256)
		return base
	case "commit_no_reload", "commit_reload_only", "commit_validate_close", "commit_success":
		tx := beginFailureTransaction(t, ctx, database, oldRow)
		stageFailureCAS(t, ctx, tx, command.Database, oldRow, newRow)
		base.CommitInvoked = true
		commitErr := tx.Commit()
		base.CommitReturned = true
		if commitErr != nil {
			code, _ := sqliteExtendedCode(commitErr)
			base.NativeKind = "sqlite"
			base.NativeDetail = boundedText(commitErr.Error())
			base.SQLitePrimary = code & 0xff
			base.SQLiteExtended = code
			base.Classification = "unknown_commit_result"
			return base
		}
		base.NativeKind = "sqlite"
		base.NativeDetail = "genuine commit result success"
		base.Classification = "committed"
		base.Revision = newRow.Revision
		base.Digest = hex.EncodeToString(newRow.SHA256)
		if command.Operation == "commit_no_reload" {
			base.ApplicationFault = "response_lost_before_reload"
			base.Classification = "unknown_commit_result"
			return base
		}
		if err := requireFailureExactRow(ctx, database.conn, newRow); err != nil {
			t.Fatalf("post-commit exact reload: %v", err)
		}
		if command.Operation == "commit_reload_only" {
			base.ApplicationFault = failureValidationFault(command.Attempt)
			base.Classification = "unknown_commit_result"
			return base
		}
		if err := requireFailureStore(ctx, database, command.Database, newRow); err != nil {
			t.Fatalf("post-commit complete validation: %v", err)
		}
		base.QuickCheckRows = 1
		if err := closePublicationDatabase(database); err != nil {
			t.Fatalf("close committed failure store: %v", err)
		}
		closed = true
		if err := requireContentionLayout(command.Database); err != nil {
			t.Fatalf("closed committed layout: %v", err)
		}
		if command.Operation == "commit_validate_close" {
			if command.Scenario == "14_close_result_loss-01" {
				base.ApplicationFault = "close_result_lost"
			} else {
				base.ApplicationFault = "acknowledgement_lost"
			}
			base.Classification = "unknown_commit_result"
			return base
		}
		base.Acknowledged = true
		return base
	default:
		t.Fatalf("unknown failure-matrix operation %q", command.Operation)
	}
	return base
}

const selectedCASStatement = "UPDATE app_server_aggregate SET revision=?, canonical_json=?, canonical_sha256=? WHERE singleton=? AND pipeon_session_id=? AND revision=? AND canonical_sha256=?"

func beginFailureTransaction(t *testing.T, ctx context.Context, database *evidenceDatabase, oldRow aggregateRow) *sql.Tx {
	t.Helper()
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin failure transaction: %v", err)
	}
	row, err := readAggregateRow(ctx, tx)
	if err != nil || !sameRow(row, oldRow) {
		_ = tx.Rollback()
		t.Fatalf("transaction observation mismatch: revision=%d err=%v", row.Revision, err)
	}
	return tx
}

func stageFailureCAS(t *testing.T, ctx context.Context, tx *sql.Tx, path string, oldRow, newRow aggregateRow) {
	t.Helper()
	result, err := tx.ExecContext(ctx, selectedCASStatement, newRow.Revision, newRow.Payload, newRow.SHA256, oldRow.Singleton, oldRow.SessionID, oldRow.Revision, oldRow.SHA256)
	if err != nil {
		t.Fatalf("stage failure CAS: %v", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		t.Fatalf("failure CAS affected=%d err=%v, want 1", affected, err)
	}
	row, err := readAggregateRow(ctx, tx)
	if err != nil || !sameRow(row, newRow) {
		t.Fatalf("staged failure row mismatch: revision=%d err=%v", row.Revision, err)
	}
	if _, err := requirePublicationJournal(path); err != nil {
		t.Fatalf("staged failure journal: %v", err)
	}
}

func rollbackAndRequireOld(t *testing.T, ctx context.Context, tx *sql.Tx, database *evidenceDatabase, oldRow aggregateRow) {
	t.Helper()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("explicit rollback: %v", err)
	}
	if err := requireFailureExactRow(ctx, database.conn, oldRow); err != nil {
		t.Fatalf("exact old row after rollback: %v", err)
	}
}

func requireFailureStore(ctx context.Context, database *evidenceDatabase, path string, want aggregateRow) error {
	if err := requireFailurePreState(ctx, database, path, want); err != nil {
		return err
	}
	if err := requireQuickCheck(ctx, database.conn); err != nil {
		return fmt.Errorf("quick_check: %w", err)
	}
	return nil
}

func requireFailurePreState(ctx context.Context, database *evidenceDatabase, path string, want aggregateRow) error {
	if err := requireContentionSchema(ctx, database.conn); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if err := requireOnlyMain(ctx, database.conn, path); err != nil {
		return fmt.Errorf("database identity: %w", err)
	}
	if err := requireFailureExactRow(ctx, database.conn, want); err != nil {
		return err
	}
	if err := requireContentionLayout(path); err != nil {
		return err
	}
	return nil
}

func requireFailureExactRow(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, want aggregateRow) error {
	row, err := readAggregateRow(ctx, queryer)
	if err != nil {
		return fmt.Errorf("read aggregate row: %w", err)
	}
	if !sameRow(row, want) {
		return fmt.Errorf("aggregate row mismatch: got revision=%d session=%q digest=%s want revision=%d session=%q digest=%s", row.Revision, row.SessionID, hex.EncodeToString(row.SHA256), want.Revision, want.SessionID, hex.EncodeToString(want.SHA256))
	}
	var canonical map[string]any
	decoder := json.NewDecoder(bytes.NewReader(row.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&canonical); err != nil {
		return fmt.Errorf("decode canonical payload: %w", err)
	}
	if decoder.More() {
		return errors.New("canonical payload contains trailing JSON")
	}
	reencoded, err := json.Marshal(canonical)
	if err != nil || !bytes.Equal(reencoded, row.Payload) {
		return fmt.Errorf("payload is not exact canonical JSON: err=%v", err)
	}
	if canonical["adapter"] != "codex_app_server" || canonical["outcome_unknown"] != true || canonical["permanent_no_replay"] != true || canonical["pipeon_session_id"] != want.SessionID || canonical["revision"] != float64(want.Revision) || len(canonical) != 5 {
		return fmt.Errorf("canonical lifecycle envelope mismatch: %+v", canonical)
	}
	return nil
}

func requireFailureCompileOptions(t *testing.T, got []string) {
	t.Helper()
	if len(got) != 56 || len(failureExpectedCompileOptions) != 56 || strings.Join(got, "\n") != strings.Join(failureExpectedCompileOptions, "\n") {
		t.Fatalf("compile-option set mismatch: got[%d]=%s", len(got), strings.Join(got, ","))
	}
	digest := sha256Bytes([]byte(strings.Join(got, "\n") + "\n"))
	if hex.EncodeToString(digest) != failureCompileHash {
		t.Fatalf("compile-option hash = %s, want %s", hex.EncodeToString(digest), failureCompileHash)
	}
}

func sqliteExtendedCode(err error) (int, bool) {
	primary, ok := sqlitePrimaryCode(err)
	if !ok {
		return 0, false
	}
	var sqliteError interface{ Code() int }
	if errors.As(err, &sqliteError) {
		return sqliteError.Code(), true
	}
	return primary, true
}

func failureValidationFault(attempt int) string {
	faults := []string{"schema_validation_lost", "identity_validation_lost", "digest_envelope_validation_lost", "sibling_validation_lost", "ownership_mode_mount_path_validation_lost"}
	if attempt < 1 || attempt > len(faults) {
		return "invalid_validation_attempt"
	}
	return faults[attempt-1]
}

func startFailureChild(t *testing.T, commandMessage failureCommand) *failureChild {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestLinuxSQLiteFailureMatrixChild$", "-test.count=1", "-test.timeout=30m")
	command.Env = failureChildEnvironment(commandMessage)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create failure-child stdin: %v", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		t.Fatalf("create failure-child protocol stream: %v", err)
	}
	child := &failureChild{command: command, stdin: stdin, lines: make(chan failureLine, 4)}
	command.Stdout = &child.output
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		t.Fatalf("start failure child: %v", err)
	}
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024), 8192)
		for scanner.Scan() {
			child.lines <- failureLine{text: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			child.lines <- failureLine{err: err}
		} else {
			child.lines <- failureLine{err: io.EOF}
		}
		close(child.lines)
	}()
	return child
}

func runFailureOneShot(t *testing.T, command failureCommand) failureResponse {
	t.Helper()
	child := startFailureChild(t, command)
	response := child.exchange(t, command)
	if err := child.stdin.Close(); err != nil {
		t.Fatalf("close failure-child input: %v", err)
	}
	child.finish(t, false)
	return response
}

func startFailureHold(t *testing.T, command failureCommand) (*failureChild, failureResponse) {
	t.Helper()
	child := startFailureChild(t, command)
	t.Cleanup(child.forceStop)
	response := child.exchange(t, command)
	if response.Status != "holding" {
		child.terminate(t)
		t.Fatalf("hold operation status = %q, want holding", response.Status)
	}
	return child, response
}

func (child *failureChild) forceStop() {
	if child == nil || child.command == nil || child.command.Process == nil || (child.command.ProcessState != nil && child.command.ProcessState.Exited()) {
		return
	}
	_ = child.command.Process.Kill()
	_ = child.stdin.Close()
	_, _ = child.command.Process.Wait()
}

func (child *failureChild) exchange(t *testing.T, command failureCommand) failureResponse {
	t.Helper()
	payload, err := json.Marshal(command)
	if err != nil || len(payload) > 4096 {
		t.Fatalf("encode bounded failure command: len=%d err=%v", len(payload), err)
	}
	if _, err := child.stdin.Write(append(payload, '\n')); err != nil {
		t.Fatalf("write failure command: %v", err)
	}
	select {
	case line, ok := <-child.lines:
		if !ok || line.err != nil {
			t.Fatalf("failure response stream ended: ok=%t err=%v output=%s", ok, line.err, child.output.String())
		}
		var response failureResponse
		if err := decodeStrictPublicationJSON(line.text, &response); err != nil {
			t.Fatalf("decode failure response %q: %v", boundedText(line.text), err)
		}
		return response
	case <-time.After(30 * time.Second):
		child.terminate(t)
		t.Fatalf("failure response timed out: scenario=%s attempt=%d operation=%s output=%s", command.Scenario, command.Attempt, command.Operation, child.output.String())
		return failureResponse{}
	}
}

func (child *failureChild) finish(t *testing.T, expectKilled bool) {
	t.Helper()
	waited := make(chan error, 1)
	go func() { waited <- child.command.Wait() }()
	waitComplete := false
	streamComplete := false
	var waitErr, protocolErr error
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for !waitComplete || !streamComplete {
		select {
		case err := <-waited:
			waitErr = err
			waitComplete = true
		case line, ok := <-child.lines:
			if !ok || errors.Is(line.err, io.EOF) {
				streamComplete = true
				continue
			}
			if protocolErr == nil {
				if line.err != nil {
					protocolErr = line.err
				} else {
					protocolErr = fmt.Errorf("duplicate or out-of-order response %q", boundedText(line.text))
				}
			}
		case <-deadline.C:
			_ = child.command.Process.Kill()
			t.Fatalf("failure child did not terminate; output=%s", child.output.String())
		}
	}
	if protocolErr != nil {
		t.Fatalf("failure protocol stream: %v output=%s", protocolErr, child.output.String())
	}
	if expectKilled {
		if waitErr == nil {
			t.Fatalf("forced-termination child exited successfully")
		}
	} else if waitErr != nil {
		t.Fatalf("failure child exited unsuccessfully: %v output=%s", waitErr, child.output.String())
	}
}

func (child *failureChild) terminate(t *testing.T) {
	t.Helper()
	if child.command.Process == nil || child.command.ProcessState != nil {
		t.Fatalf("failure child is not live before termination")
	}
	if err := child.command.Process.Kill(); err != nil {
		t.Fatalf("force-terminate failure child: %v", err)
	}
	_ = child.stdin.Close()
	child.finish(t, true)
}

func failureChildEnvironment(command failureCommand) []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, failureMatrixChildEnv+"=") || strings.HasPrefix(item, failureMatrixRootEnv+"=") || strings.HasPrefix(item, failureMatrixRootIdentityEnv+"=") {
			continue
		}
		environment = append(environment, item)
	}
	return append(environment,
		failureMatrixChildEnv+"=1",
		failureMatrixRootEnv+"="+command.Root,
		failureMatrixRootIdentityEnv+"="+command.RootIdentity,
	)
}

func decodeFailureCommand(t *testing.T, line string) failureCommand {
	t.Helper()
	if len(line) == 0 || len(line) > 8192 {
		t.Fatalf("failure command length = %d, want 1..8192", len(line))
	}
	var command failureCommand
	if err := decodeStrictPublicationJSON(line, &command); err != nil {
		t.Fatalf("decode failure command: %v", err)
	}
	if command.Scenario == "" || len(command.Scenario) > 96 || command.Cycle < 1 || command.Cycle > failureMatrixRows || command.Attempt < 1 || command.Attempt > 8 || command.Operation == "" || len(command.Operation) > 48 || command.Checkpoint == "" || len(command.Checkpoint) > 48 || command.Session == "" || len(command.Session) > 256 || !filepath.IsAbs(command.Database) || filepath.Clean(command.Database) != command.Database || filepath.Base(command.Database) != "aggregate.sqlite" || !filepath.IsAbs(command.Root) || filepath.Clean(command.Root) != command.Root || filepath.Base(command.Root) != command.Scenario || command.RootIdentity == "" || len(command.RootIdentity) > 256 {
		t.Fatalf("invalid failure command: %+v", command)
	}
	databaseRole := filepath.Base(filepath.Dir(command.Database))
	wantSession := "failure-matrix-" + command.Scenario
	if databaseRole == "other" && command.Scenario == "03_contention-01" {
		wantSession = "failure-matrix-different-session"
	}
	if databaseRole != "main" && databaseRole != "other" || command.Session != wantSession {
		t.Fatalf("failure command role/session mismatch: role=%q session=%q want=%q", databaseRole, command.Session, wantSession)
	}
	return command
}

func writeFailureResponse(t *testing.T, response failureResponse) {
	t.Helper()
	payload, err := json.Marshal(response)
	if err != nil || len(payload) > 4096 {
		t.Fatalf("encode bounded failure response: len=%d err=%v", len(payload), err)
	}
	if _, err := fmt.Fprintln(os.Stderr, string(payload)); err != nil {
		t.Fatalf("write failure response: %v", err)
	}
}

func requireFailureResponse(t *testing.T, command failureCommand, response failureResponse) {
	t.Helper()
	if response.Scenario != command.Scenario || response.Cycle != command.Cycle || response.Attempt != command.Attempt || response.Operation != command.Operation || response.Checkpoint != command.Checkpoint || response.Database != command.Database || response.Session != command.Session || response.Root != command.Root || response.RootIdentity != command.RootIdentity {
		t.Fatalf("failure response identity mismatch: command=%+v response=%+v", command, response)
	}
	if response.Status != "ok" && response.Status != "holding" && response.Status != "unproven" {
		t.Fatalf("failure response status %q is not allowlisted", response.Status)
	}
	if response.Retries != 0 || response.Replays != 0 || response.Repairs != 0 || response.Fallbacks != 0 {
		t.Fatalf("failure response attempted forbidden action: %+v", response)
	}
}

func countFailureClassification(counters *failureMatrixCounters, classification string) {
	switch classification {
	case "known_unchanged":
		counters.KnownUnchanged++
	case "committed":
		counters.Committed++
	case "rejected":
		counters.Rejected++
	case "unknown_commit_result":
		counters.Unknown++
	default:
		panic("unknown failure classification: " + classification)
	}
}

func mustStableFailureTree(t *testing.T, root string) string {
	t.Helper()
	qualification, _, err := qualifyLinuxFixtureRoot(root)
	if err != nil {
		t.Fatalf("qualify stable failure metadata tree: %v", err)
	}
	hash, err := stableLinuxFailureTreeHash(qualification)
	if err != nil {
		t.Fatalf("capture stable failure metadata tree: %v", err)
	}
	return hash
}

func stableLinuxFailureTreeHash(qualification *linuxQualification) (string, error) {
	first, err := linuxFailureTreeHash(qualification)
	if err != nil {
		return "", err
	}
	second, err := linuxFailureTreeHash(qualification)
	if err != nil {
		return "", err
	}
	if first != second {
		return "", fmt.Errorf("failure metadata tree changed while quiescent: first=%s second=%s", first, second)
	}
	return first, nil
}

func linuxFailureTreeHash(qualification *linuxQualification) (string, error) {
	rows := make([]string, 0, failureMatrixRows*5)
	err := filepath.WalkDir(qualification.FixtureRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		kind := "file"
		mode := uint16(0o600)
		if entry.IsDir() {
			kind = "directory"
			mode = 0o700
		}
		fact, err := qualification.requirePath(path, kind, mode)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(qualification.FixtureRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if err := requireLinuxFailureTreePath(relative, kind); err != nil {
			return err
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\t%d\t%#o\t%d\t%d\t%d:%d\t%d\t%d\t%s\t%#x\t%s\t%s", relative, fact.Kind, fact.Size, fact.Mode, fact.UID, fact.GID, fact.DeviceMajor, fact.DeviceMinor, fact.Inode, fact.MountID, qualification.Mount.FileSystem, fact.FSMagic, qualification.Mount.Source, qualification.Mount.MountPoint))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rows)
	return hex.EncodeToString(sha256Bytes([]byte(strings.Join(rows, "\n") + "\n"))), nil
}

func requireLinuxFailureTreePath(relative, kind string) error {
	if relative == "." {
		if kind != "directory" {
			return fmt.Errorf("failure metadata root kind = %s, want directory", kind)
		}
		return nil
	}
	parts := strings.Split(relative, "/")
	if len(parts) == 1 {
		if kind != "directory" || parts[0] != "main" && parts[0] != "other" && !isLinuxFailureScenarioDirectory(parts[0]) {
			return fmt.Errorf("failure metadata first-level path %q is not a directory", relative)
		}
		return nil
	}
	if parts[0] == "main" || parts[0] == "other" {
		if len(parts) == 2 && kind == "file" && (parts[1] == "aggregate.sqlite" || parts[1] == "aggregate.sqlite-journal") {
			return nil
		}
		return fmt.Errorf("unexpected scenario metadata path %q", relative)
	}
	if !isLinuxFailureScenarioDirectory(parts[0]) {
		return fmt.Errorf("unexpected matrix scenario directory %q", parts[0])
	}
	if len(parts) == 2 && kind == "directory" && (parts[1] == "main" || parts[1] == "other") {
		return nil
	}
	if len(parts) == 3 && kind == "file" && (parts[1] == "main" || parts[1] == "other") && (parts[2] == "aggregate.sqlite" || parts[2] == "aggregate.sqlite-journal") {
		return nil
	}
	return fmt.Errorf("unexpected matrix metadata path %q", relative)
}

func isLinuxFailureScenarioDirectory(name string) bool {
	for _, scenario := range []string{
		"01_before_open-01", "02_contract_reject-01", "03_contention-01", "04_cancel_after_observation-01",
		"05_stale_cas-01", "05_stale_cas-02", "05_stale_cas-03", "06_after_begin-01", "07_after_stage-01",
		"08_terminate_precommit-01", "09_rollback_proof_loss-01", "10_commit_call_loss-01",
		"11_genuine_commit_error-01", "12_response_loss-01",
		"13_validation_loss-01", "13_validation_loss-02", "13_validation_loss-03", "13_validation_loss-04", "13_validation_loss-05",
		"14_close_result_loss-01", "15_ack_loss-01", "16_success-01",
	} {
		if name == scenario {
			return true
		}
	}
	return false
}

func failureTreeRollup(results []failureMatrixResult, pre bool) string {
	rows := make([]string, 0, len(results))
	for _, result := range results {
		hash := result.PostTree
		if pre {
			hash = result.PreTree
		}
		rows = append(rows, fmt.Sprintf("%s\t%d\t%s", result.Scenario, result.Attempt, hash))
	}
	sort.Strings(rows)
	return hex.EncodeToString(sha256Bytes([]byte(strings.Join(rows, "\n") + "\n")))
}
