//go:build windows

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
	"strings"
	"testing"
	"time"
)

const (
	contentionCohortOptInEnv    = "DORKPIPE_SQLITE_CONTENTION_COHORT"
	contentionCohortChildEnv    = "DORKPIPE_SQLITE_CONTENTION_CHILD"
	contentionCohortDatabaseEnv = "DORKPIPE_SQLITE_CONTENTION_DATABASE"
	contentionCohortCycles      = 1000
	contentionSameSessionID     = "pipeon-sqlite-native-contention-same"
	contentionOtherSessionID    = "pipeon-sqlite-native-contention-other"
)

type contentionCommand struct {
	Cycle     int    `json:"cycle"`
	Operation string `json:"operation"`
}

type contentionResponse struct {
	Cycle      int    `json:"cycle"`
	Operation  string `json:"operation"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Revision   int64  `json:"revision,omitempty"`
	Digest     string `json:"digest,omitempty"`
	SQLiteCode int    `json:"sqlite_code,omitempty"`
}

type contentionLine struct {
	text string
	err  error
}

type contentionChild struct {
	role    string
	command *exec.Cmd
	stdin   io.WriteCloser
	lines   chan contentionLine
	output  contentionBoundedBuffer
}

type contentionBoundedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (buffer *contentionBoundedBuffer) Write(payload []byte) (int, error) {
	const limit = 4096
	originalLength := len(payload)
	remaining := limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(payload) > remaining {
			payload = payload[:remaining]
			buffer.truncated = true
		}
		_, _ = buffer.buffer.Write(payload)
	} else if originalLength > 0 {
		buffer.truncated = true
	}
	return originalLength, nil
}

func (buffer *contentionBoundedBuffer) String() string {
	if buffer.truncated {
		return buffer.buffer.String() + "...[truncated]"
	}
	return buffer.buffer.String()
}

func TestWindowsNativeSQLiteContentionCohort(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("Windows/amd64 native evidence only")
	}
	if os.Getenv(contentionCohortOptInEnv) != "1" {
		t.Skip("set DORKPIPE_SQLITE_CONTENTION_COHORT=1 to run the 1,000-cycle contention cohort")
	}
	if evidenceCGOEnabled || os.Getenv("CGO_ENABLED") != "0" {
		t.Fatalf("contention cohort requires a !cgo test binary and CGO_ENABLED=0; compiled_cgo=%t env=%q", evidenceCGOEnabled, os.Getenv("CGO_ENABLED"))
	}

	fixtureRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize contention fixture root: %v", err)
	}
	fixtureRoot = filepath.Clean(fixtureRoot)
	hostFacts, err := collectAndProtectWindowsHost(fixtureRoot)
	if err != nil {
		t.Fatalf("qualify Windows contention host: %v", err)
	}
	t.Logf("host windows_build=%s arch=%s drive=%s filesystem=%s ntfs_version=%s volume=%s go=%s root_protection={%s}",
		hostFacts.WindowsBuild, hostFacts.Architecture, hostFacts.DriveType, hostFacts.FileSystem,
		hostFacts.NTFSVersion, hostFacts.VolumeID, hostFacts.GoVersion, hostFacts.Protection)

	samePath := prepareEvidenceFile(t, fixtureRoot, "same-session")
	otherPath := prepareEvidenceFile(t, fixtureRoot, "different-session")
	ctx, cancel := context.WithTimeout(context.Background(), 29*time.Minute)
	defer cancel()

	initialSame := contentionRow(contentionSameSessionID, 1)
	initialOther := contentionRow(contentionOtherSessionID, 1)
	sameOptions := initializeContentionDatabase(t, ctx, samePath, initialSame)
	otherOptions := initializeContentionDatabase(t, ctx, otherPath, initialOther)
	if strings.Join(sameOptions, "\n") != strings.Join(otherOptions, "\n") {
		t.Fatal("same-session and different-session SQLite compile options differ")
	}
	t.Logf("sqlite version=%s source_id=%s vfs=%s uri={absolute mode=rw cache=private _txlock=exclusive _dqs=0 _error_rc=1} pragmas={journal_mode=delete synchronous=3 fullfsync=1 temp_store=2 mmap_size=0 busy_timeout=0 foreign_keys=1 trusted_schema=0 cell_size_check=1 locking_mode=exclusive page_size=4096} schema={singleton_strict user_version=1} compile_options[%d]=%s",
		selectedSQLiteVersion, selectedSQLiteSourceID, selectedNativeVFS(), len(sameOptions), strings.Join(sameOptions, ","))

	preTreeHash, err := stablePublicationTreeHash(fixtureRoot)
	if err != nil {
		t.Fatalf("capture stable pre-cohort metadata tree: %v", err)
	}

	var activeOwner *contentionChild
	t.Cleanup(func() {
		if activeOwner != nil {
			activeOwner.forceStop()
		}
	})

	startedAt := time.Now()
	sameRow := initialSame
	otherRow := initialOther
	ownersStaged := 0
	protectedJournals := 0
	busyResults := 0
	otherCommits := 0
	forcedTerminations := 0
	oldRecoveries := 0
	postRecoveryCommits := 0

	for cycle := 1; cycle <= contentionCohortCycles; cycle++ {
		if err := ctx.Err(); err != nil {
			t.Fatalf("contention cohort deadline before cycle %d: %v", cycle, err)
		}
		if sameRow.Revision != int64(cycle) || otherRow.Revision != int64(cycle) {
			t.Fatalf("cycle %d pre-state revisions same=%d other=%d, want %d", cycle, sameRow.Revision, otherRow.Revision, cycle)
		}

		owner, err := startContentionChild("owner", samePath)
		if err != nil {
			t.Fatalf("cycle %d start owner: %v", cycle, err)
		}
		activeOwner = owner
		stagedRow := contentionRow(contentionSameSessionID, sameRow.Revision+1)
		ownerResponse, err := owner.exchange(contentionCommand{Cycle: cycle, Operation: "stage_next"})
		if err != nil {
			t.Fatalf("cycle %d owner-stage protocol: %v", cycle, err)
		}
		if err := requireContentionRowResponse(ownerResponse, cycle, "stage_next", "staged_live", stagedRow); err != nil {
			t.Fatalf("cycle %d owner-stage response: %v", cycle, err)
		}
		ownersStaged++

		if _, err := requirePublicationJournal(samePath); err != nil {
			t.Fatalf("cycle %d live same-session journal: %v", cycle, err)
		}
		protectedJournals++

		contenderResponse, err := runContentionOneShot("contender", samePath, contentionCommand{Cycle: cycle, Operation: "expect_busy"})
		if err != nil {
			t.Fatalf("cycle %d contender protocol: %v", cycle, err)
		}
		if contenderResponse.Cycle != cycle || contenderResponse.Operation != "expect_busy" || contenderResponse.Status != "busy_or_locked" || (contenderResponse.SQLiteCode != 5 && contenderResponse.SQLiteCode != 6) || contenderResponse.Revision != 0 || contenderResponse.Digest != "" {
			t.Fatalf("cycle %d contender response mismatch: %+v", cycle, contenderResponse)
		}
		busyResults++

		nextOther := contentionRow(contentionOtherSessionID, otherRow.Revision+1)
		otherResponse, err := runContentionOneShot("different_writer", otherPath, contentionCommand{Cycle: cycle, Operation: "commit_different"})
		if err != nil {
			t.Fatalf("cycle %d different-session writer protocol: %v", cycle, err)
		}
		if err := requireContentionRowResponse(otherResponse, cycle, "commit_different", "committed", nextOther); err != nil {
			t.Fatalf("cycle %d different-session response: %v", cycle, err)
		}
		otherRow = nextOther
		otherCommits++

		if err := owner.terminate(); err != nil {
			t.Fatalf("cycle %d force-terminate owner: %v", cycle, err)
		}
		activeOwner = nil
		forcedTerminations++

		recoveryResponse, err := runContentionOneShot("recovery", samePath, contentionCommand{Cycle: cycle, Operation: "recover_old"})
		if err != nil {
			t.Fatalf("cycle %d recovery protocol: %v", cycle, err)
		}
		if err := requireContentionRowResponse(recoveryResponse, cycle, "recover_old", "old_row_recovered", sameRow); err != nil {
			t.Fatalf("cycle %d recovery response: %v", cycle, err)
		}
		oldRecoveries++

		writerResponse, err := runContentionOneShot("clean_writer", samePath, contentionCommand{Cycle: cycle, Operation: "commit_same"})
		if err != nil {
			t.Fatalf("cycle %d clean-writer protocol: %v", cycle, err)
		}
		if err := requireContentionRowResponse(writerResponse, cycle, "commit_same", "committed", stagedRow); err != nil {
			t.Fatalf("cycle %d clean-writer response: %v", cycle, err)
		}
		sameRow = stagedRow
		postRecoveryCommits++
	}

	elapsed := time.Since(startedAt)
	postTreeHash, err := stablePublicationTreeHash(fixtureRoot)
	if err != nil {
		t.Fatalf("capture stable post-cohort metadata tree: %v", err)
	}
	if ownersStaged != contentionCohortCycles || protectedJournals != contentionCohortCycles || busyResults != contentionCohortCycles || otherCommits != contentionCohortCycles || forcedTerminations != contentionCohortCycles || oldRecoveries != contentionCohortCycles || postRecoveryCommits != contentionCohortCycles {
		t.Fatalf("contention counters staged=%d journals=%d busy=%d other_commits=%d terminations=%d old_recoveries=%d same_commits=%d want=%d each",
			ownersStaged, protectedJournals, busyResults, otherCommits, forcedTerminations, oldRecoveries, postRecoveryCommits, contentionCohortCycles)
	}
	if sameRow.Revision != initialSame.Revision+contentionCohortCycles || otherRow.Revision != initialOther.Revision+contentionCohortCycles {
		t.Fatalf("final revisions same=%d other=%d, want initial+%d", sameRow.Revision, otherRow.Revision, contentionCohortCycles)
	}
	if elapsed > 30*time.Minute {
		t.Fatalf("contention cohort elapsed %s, exceeds 30m", elapsed)
	}

	t.Logf("contention cycles=%d owner_transactions_staged=%d protected_live_journals=%d same_session_busy_or_locked=%d different_session_commits=%d forced_owner_terminations=%d exact_old_row_recoveries=%d post_recovery_same_session_commits=%d ambiguous_recoveries=0 staged_row_leaks=0 revision_gaps_or_duplicates=0 digest_or_envelope_mismatches=0 unexpected_siblings_or_protection_widening=0 child_protocol_loss_duplication_or_reordering=0",
		contentionCohortCycles, ownersStaged, protectedJournals, busyResults, otherCommits, forcedTerminations, oldRecoveries, postRecoveryCommits)
	t.Logf("contention same_initial_revision=%d same_initial_digest=%s same_final_revision=%d same_final_digest=%s other_initial_revision=%d other_initial_digest=%s other_final_revision=%d other_final_digest=%s elapsed=%s pre_metadata_tree_sha256=%s post_metadata_tree_sha256=%s",
		initialSame.Revision, hex.EncodeToString(initialSame.SHA256), sameRow.Revision, hex.EncodeToString(sameRow.SHA256),
		initialOther.Revision, hex.EncodeToString(initialOther.SHA256), otherRow.Revision, hex.EncodeToString(otherRow.SHA256),
		elapsed.Round(time.Millisecond), preTreeHash, postTreeHash)
}

func TestWindowsSQLiteContentionCohortChild(t *testing.T) {
	role := os.Getenv(contentionCohortChildEnv)
	if role == "" {
		t.Skip("contention-cohort child-process helper")
	}
	if runtime.GOARCH != "amd64" || evidenceCGOEnabled {
		t.Fatalf("contention child requires Windows/amd64 with CGo disabled; arch=%s cgo=%t", runtime.GOARCH, evidenceCGOEnabled)
	}
	databasePath := filepath.Clean(os.Getenv(contentionCohortDatabaseEnv))
	if databasePath == "." || !filepath.IsAbs(databasePath) {
		t.Fatalf("contention child database path must be absolute: %q", databasePath)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 4096)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			t.Fatalf("contention %s command stream: %v", role, err)
		}
		t.Fatalf("contention %s command stream ended before one command", role)
	}
	command := decodeContentionCommand(t, scanner.Text())
	var response contentionResponse
	var err error
	switch role {
	case "owner":
		runContentionOwner(t, databasePath, command)
		return
	case "contender":
		response, err = runContentionContender(databasePath, command)
	case "different_writer":
		response, err = runContentionWriter(databasePath, command, contentionOtherSessionID, "commit_different")
	case "recovery":
		response, err = runContentionRecovery(databasePath, command)
	case "clean_writer":
		response, err = runContentionWriter(databasePath, command, contentionSameSessionID, "commit_same")
	default:
		t.Fatalf("unknown contention child role %q", role)
	}
	if err != nil {
		response = contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())}
	}
	writeContentionResponse(t, response)
	if scanner.Scan() {
		t.Fatalf("contention %s received duplicate command %q", role, boundedText(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("contention %s command stream: %v", role, err)
	}
}

func contentionRow(sessionID string, revision int64) aggregateRow {
	payload := []byte(fmt.Sprintf(`{"adapter":"codex_app_server","outcome_unknown":true,"pipeon_session_id":"%s","replay_forbidden":true,"revision":%d}`, sessionID, revision))
	row := selectedRow(revision, payload)
	row.SessionID = sessionID
	return row
}

func initializeContentionDatabase(t *testing.T, ctx context.Context, databasePath string, initial aggregateRow) []string {
	t.Helper()
	database := mustOpenEvidenceDatabase(t, ctx, databasePath)
	mustCreateSelectedSchema(t, ctx, database.conn)
	mustInsertAndCommit(t, ctx, database.conn, initial, databasePath)
	mustRequireExactRow(t, ctx, database.conn, initial)
	if err := requireQuickCheck(ctx, database.conn); err != nil {
		t.Fatalf("initial contention quick_check for %s: %v", initial.SessionID, err)
	}
	options := append([]string(nil), database.compileOptions...)
	mustCloseEvidenceDatabase(t, database)
	if err := requireContentionLayout(databasePath); err != nil {
		t.Fatalf("initial contention layout for %s: %v", initial.SessionID, err)
	}
	return options
}

func startContentionChild(role, databasePath string) (*contentionChild, error) {
	command := exec.Command(os.Args[0], "-test.run=^TestWindowsSQLiteContentionCohortChild$", "-test.count=1", "-test.timeout=30m")
	command.Env = contentionChildEnvironment(role, databasePath)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create %s stdin: %w", role, err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("create %s protocol stream: %w", role, err)
	}
	child := &contentionChild{role: role, command: command, stdin: stdin, lines: make(chan contentionLine, 8)}
	command.Stdout = &child.output
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start %s child: %w", role, err)
	}
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024), 4096)
		for scanner.Scan() {
			child.lines <- contentionLine{text: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			child.lines <- contentionLine{err: err}
		} else {
			child.lines <- contentionLine{err: io.EOF}
		}
		close(child.lines)
	}()
	return child, nil
}

func contentionChildEnvironment(role, databasePath string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, contentionCohortChildEnv+"=") || strings.HasPrefix(item, contentionCohortDatabaseEnv+"=") {
			continue
		}
		environment = append(environment, item)
	}
	return append(environment, contentionCohortChildEnv+"="+role, contentionCohortDatabaseEnv+"="+databasePath)
}

func runContentionOneShot(role, databasePath string, command contentionCommand) (contentionResponse, error) {
	child, err := startContentionChild(role, databasePath)
	if err != nil {
		return contentionResponse{}, err
	}
	response, exchangeErr := child.exchange(command)
	closeErr := child.stdin.Close()
	waitErr := child.finish(false)
	if exchangeErr != nil {
		return contentionResponse{}, exchangeErr
	}
	if closeErr != nil {
		return contentionResponse{}, fmt.Errorf("close %s command stream: %w", role, closeErr)
	}
	if waitErr != nil {
		return contentionResponse{}, waitErr
	}
	return response, nil
}

func (child *contentionChild) exchange(command contentionCommand) (contentionResponse, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return contentionResponse{}, fmt.Errorf("encode command: %w", err)
	}
	if len(payload) > 1024 {
		return contentionResponse{}, errors.New("encoded command exceeds protocol bound")
	}
	if _, err := child.stdin.Write(append(payload, '\n')); err != nil {
		return contentionResponse{}, fmt.Errorf("write %s command: %w", child.role, err)
	}
	select {
	case line, ok := <-child.lines:
		if !ok {
			return contentionResponse{}, fmt.Errorf("%s response stream closed", child.role)
		}
		if line.err != nil {
			return contentionResponse{}, fmt.Errorf("%s response stream: %w output=%s", child.role, line.err, child.output.String())
		}
		var response contentionResponse
		if err := decodeStrictPublicationJSON(line.text, &response); err != nil {
			return contentionResponse{}, fmt.Errorf("decode %s response %q: %w", child.role, boundedText(line.text), err)
		}
		if response.Cycle != command.Cycle || response.Operation != command.Operation {
			return contentionResponse{}, fmt.Errorf("%s response out of order: command=%+v response=%+v", child.role, command, response)
		}
		if response.Status == "error" {
			return response, fmt.Errorf("%s child error: %s", child.role, response.Error)
		}
		if response.Error != "" {
			return response, fmt.Errorf("%s successful response carried an error: %q", child.role, response.Error)
		}
		return response, nil
	case <-time.After(30 * time.Second):
		return contentionResponse{}, fmt.Errorf("%s response timed out for cycle=%d operation=%s output=%s", child.role, command.Cycle, command.Operation, child.output.String())
	}
}

func (child *contentionChild) finish(expectKilled bool) error {
	waited := make(chan error, 1)
	go func() { waited <- child.command.Wait() }()
	var waitErr error
	waitComplete := false
	streamComplete := false
	var protocolErr error
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for !waitComplete || !streamComplete {
		select {
		case err := <-waited:
			waitErr = err
			waitComplete = true
		case line, ok := <-child.lines:
			if !ok {
				streamComplete = true
				continue
			}
			if errors.Is(line.err, io.EOF) {
				streamComplete = true
				continue
			}
			if protocolErr == nil {
				if line.err != nil {
					protocolErr = fmt.Errorf("%s response stream: %w", child.role, line.err)
				} else {
					protocolErr = fmt.Errorf("%s emitted duplicate or post-response line %q", child.role, boundedText(line.text))
				}
			}
		case <-deadline.C:
			if child.command.Process != nil {
				_ = child.command.Process.Kill()
			}
			_ = child.stdin.Close()
			return fmt.Errorf("%s child did not terminate cleanly; output=%s", child.role, child.output.String())
		}
	}
	if protocolErr != nil {
		return fmt.Errorf("%w output=%s", protocolErr, child.output.String())
	}
	if expectKilled {
		if waitErr == nil {
			return fmt.Errorf("%s owner unexpectedly exited successfully", child.role)
		}
		return nil
	}
	if waitErr != nil {
		return fmt.Errorf("wait for %s child: %w output=%s", child.role, waitErr, child.output.String())
	}
	return nil
}

func (child *contentionChild) terminate() error {
	if child.command.Process == nil || child.command.ProcessState != nil {
		return fmt.Errorf("owner is not live before forced termination")
	}
	if err := child.command.Process.Kill(); err != nil {
		return err
	}
	_ = child.stdin.Close()
	return child.finish(true)
}

func (child *contentionChild) forceStop() {
	if child == nil || child.command == nil || child.command.Process == nil || (child.command.ProcessState != nil && child.command.ProcessState.Exited()) {
		return
	}
	_ = child.command.Process.Kill()
	_ = child.stdin.Close()
	_, _ = child.command.Process.Wait()
}

func runContentionOwner(t *testing.T, databasePath string, command contentionCommand) {
	t.Helper()
	if command.Operation != "stage_next" {
		t.Fatalf("owner cycle %d operation = %q, want stage_next", command.Cycle, command.Operation)
	}
	ctx := context.Background()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		writeContentionResponse(t, contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(fmt.Sprintf("open: %v", err))})
		return
	}
	oldRow := contentionRow(contentionSameSessionID, int64(command.Cycle))
	newRow := contentionRow(contentionSameSessionID, oldRow.Revision+1)
	if err := requireContentionPreState(ctx, database, databasePath, oldRow); err != nil {
		writeContentionResponse(t, contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
		return
	}
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		writeContentionResponse(t, contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(fmt.Sprintf("begin exclusive: %v", err))})
		return
	}
	if err := executeContentionCAS(ctx, tx, oldRow, newRow); err != nil {
		writeContentionResponse(t, contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
		return
	}
	staged, err := readAggregateRow(ctx, tx)
	if err != nil || !sameRow(staged, newRow) {
		writeContentionResponse(t, contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(fmt.Sprintf("staged row revision=%d err=%v, want exact revision %d", staged.Revision, err, newRow.Revision))})
		return
	}
	if _, err := requirePublicationJournal(databasePath); err != nil {
		writeContentionResponse(t, contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
		return
	}
	writeContentionResponse(t, contentionRowResponse(command, "staged_live", newRow))
	for {
		time.Sleep(time.Hour)
	}
}

func runContentionContender(databasePath string, command contentionCommand) (contentionResponse, error) {
	if command.Operation != "expect_busy" {
		return contentionResponse{}, fmt.Errorf("contender cycle %d operation = %q, want expect_busy", command.Cycle, command.Operation)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		if code, ok := sqlitePrimaryCode(err); ok && (code == 5 || code == 6) {
			return contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "busy_or_locked", SQLiteCode: code}, nil
		}
		return contentionResponse{}, fmt.Errorf("open returned non-contention error: %w", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	row, err := readAggregateRow(ctx, database.conn)
	if err != nil {
		if code, ok := sqlitePrimaryCode(err); ok && (code == 5 || code == 6) {
			return contentionResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "busy_or_locked", SQLiteCode: code}, nil
		}
		return contentionResponse{}, fmt.Errorf("read returned non-contention error: %w", err)
	}
	return contentionResponse{}, fmt.Errorf("contender observed row revision=%d instead of receiving BUSY/LOCKED", row.Revision)
}

func runContentionRecovery(databasePath string, command contentionCommand) (contentionResponse, error) {
	if command.Operation != "recover_old" {
		return contentionResponse{}, fmt.Errorf("recovery cycle %d operation = %q, want recover_old", command.Cycle, command.Operation)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		return contentionResponse{}, fmt.Errorf("open for hot-journal recovery: %w", err)
	}
	oldRow := contentionRow(contentionSameSessionID, int64(command.Cycle))
	stagedRow := contentionRow(contentionSameSessionID, oldRow.Revision+1)
	if err := requireContentionSchema(ctx, database.conn); err != nil {
		return contentionResponse{}, fmt.Errorf("schema: %w", err)
	}
	if err := requireOnlyMain(ctx, database.conn, databasePath); err != nil {
		return contentionResponse{}, fmt.Errorf("database identity: %w", err)
	}
	row, err := readAggregateRow(ctx, database.conn)
	if err != nil {
		return contentionResponse{}, fmt.Errorf("reload old row: %w", err)
	}
	if !sameRow(row, oldRow) {
		return contentionResponse{}, fmt.Errorf("recovered row revision=%d digest=%s, want exact old revision=%d digest=%s", row.Revision, hex.EncodeToString(row.SHA256), oldRow.Revision, hex.EncodeToString(oldRow.SHA256))
	}
	if sameRow(row, stagedRow) {
		return contentionResponse{}, fmt.Errorf("terminated owner's staged revision %d leaked", stagedRow.Revision)
	}
	if err := requireQuickCheck(ctx, database.conn); err != nil {
		return contentionResponse{}, fmt.Errorf("quick_check: %w", err)
	}
	if err := closePublicationDatabase(database); err != nil {
		return contentionResponse{}, err
	}
	if err := requireContentionLayout(databasePath); err != nil {
		return contentionResponse{}, err
	}
	return contentionRowResponse(command, "old_row_recovered", oldRow), nil
}

func runContentionWriter(databasePath string, command contentionCommand, sessionID, operation string) (contentionResponse, error) {
	if command.Operation != operation {
		return contentionResponse{}, fmt.Errorf("writer cycle %d operation = %q, want %s", command.Cycle, command.Operation, operation)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		return contentionResponse{}, fmt.Errorf("open: %w", err)
	}
	oldRow := contentionRow(sessionID, int64(command.Cycle))
	newRow := contentionRow(sessionID, oldRow.Revision+1)
	if err := requireContentionPreState(ctx, database, databasePath, oldRow); err != nil {
		return contentionResponse{}, err
	}
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		return contentionResponse{}, fmt.Errorf("begin exclusive: %w", err)
	}
	if err := executeContentionCAS(ctx, tx, oldRow, newRow); err != nil {
		_ = tx.Rollback()
		return contentionResponse{}, err
	}
	staged, err := readAggregateRow(ctx, tx)
	if err != nil || !sameRow(staged, newRow) {
		_ = tx.Rollback()
		return contentionResponse{}, fmt.Errorf("staged row revision=%d err=%v, want exact revision %d", staged.Revision, err, newRow.Revision)
	}
	if _, err := requirePublicationJournal(databasePath); err != nil {
		_ = tx.Rollback()
		return contentionResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return contentionResponse{}, fmt.Errorf("commit: %w", err)
	}
	committed, err := readAggregateRow(ctx, database.conn)
	if err != nil || !sameRow(committed, newRow) {
		return contentionResponse{}, fmt.Errorf("committed row revision=%d err=%v, want exact revision %d", committed.Revision, err, newRow.Revision)
	}
	if err := requireContentionSchema(ctx, database.conn); err != nil {
		return contentionResponse{}, fmt.Errorf("post-commit schema: %w", err)
	}
	if err := requireOnlyMain(ctx, database.conn, databasePath); err != nil {
		return contentionResponse{}, fmt.Errorf("post-commit database identity: %w", err)
	}
	if err := requireQuickCheck(ctx, database.conn); err != nil {
		return contentionResponse{}, fmt.Errorf("post-commit quick_check: %w", err)
	}
	if err := closePublicationDatabase(database); err != nil {
		return contentionResponse{}, err
	}
	if err := requireContentionLayout(databasePath); err != nil {
		return contentionResponse{}, err
	}
	return contentionRowResponse(command, "committed", newRow), nil
}

func requireContentionPreState(ctx context.Context, database *evidenceDatabase, databasePath string, want aggregateRow) error {
	if err := requireContentionSchema(ctx, database.conn); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	if err := requireOnlyMain(ctx, database.conn, databasePath); err != nil {
		return fmt.Errorf("database identity: %w", err)
	}
	if err := requireContentionLayout(databasePath); err != nil {
		return err
	}
	current, err := readAggregateRow(ctx, database.conn)
	if err != nil || !sameRow(current, want) {
		return fmt.Errorf("pre-state revision=%d err=%v, want exact revision %d", current.Revision, err, want.Revision)
	}
	return nil
}

func requireContentionSchema(ctx context.Context, conn *sql.Conn) error {
	if err := requireSchemaObjectCount(ctx, conn, 1); err != nil {
		return err
	}
	var userVersion int64
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
		return err
	}
	if userVersion != 1 {
		return fmt.Errorf("user_version = %d, want 1", userVersion)
	}
	return nil
}

func executeContentionCAS(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, oldRow, newRow aggregateRow) error {
	result, err := tx.ExecContext(ctx,
		"UPDATE app_server_aggregate SET revision=?, canonical_json=?, canonical_sha256=? WHERE singleton=? AND pipeon_session_id=? AND revision=? AND canonical_sha256=?",
		newRow.Revision, newRow.Payload, newRow.SHA256,
		oldRow.Singleton, oldRow.SessionID, oldRow.Revision, oldRow.SHA256,
	)
	if err != nil {
		return fmt.Errorf("execute CAS: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return fmt.Errorf("CAS affected=%d err=%v, want 1", affected, err)
	}
	return nil
}

func requireContentionLayout(databasePath string) error {
	if _, err := requireWindowsPrivatePath(filepath.Dir(databasePath)); err != nil {
		return fmt.Errorf("database directory protection: %w", err)
	}
	if err := requirePublicationSiblings(filepath.Dir(databasePath), false); err != nil {
		return err
	}
	return nil
}

func decodeContentionCommand(t *testing.T, line string) contentionCommand {
	t.Helper()
	if len(line) == 0 || len(line) > 4096 {
		t.Fatalf("contention command length = %d, want 1..4096", len(line))
	}
	var command contentionCommand
	if err := decodeStrictPublicationJSON(line, &command); err != nil {
		t.Fatalf("decode contention command: %v", err)
	}
	if command.Cycle < 1 || command.Cycle > contentionCohortCycles || command.Operation == "" || len(command.Operation) > 32 {
		t.Fatalf("invalid contention command: %+v", command)
	}
	return command
}

func writeContentionResponse(t *testing.T, response contentionResponse) {
	t.Helper()
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("encode contention response: %v", err)
	}
	if len(payload) > 1024 {
		t.Fatalf("contention response exceeds protocol bound: %d", len(payload))
	}
	if _, err := fmt.Fprintln(os.Stderr, string(payload)); err != nil {
		t.Fatalf("write contention response: %v", err)
	}
}

func contentionRowResponse(command contentionCommand, status string, row aggregateRow) contentionResponse {
	return contentionResponse{
		Cycle:     command.Cycle,
		Operation: command.Operation,
		Status:    status,
		Revision:  row.Revision,
		Digest:    hex.EncodeToString(row.SHA256),
	}
}

func requireContentionRowResponse(response contentionResponse, cycle int, operation, status string, row aggregateRow) error {
	wantDigest := hex.EncodeToString(row.SHA256)
	if response.Cycle != cycle || response.Operation != operation || response.Status != status || response.Revision != row.Revision || response.Digest != wantDigest || response.SQLiteCode != 0 || response.Error != "" {
		return fmt.Errorf("got=%+v want_status=%s want_revision=%d want_digest=%s", response, status, row.Revision, wantDigest)
	}
	return nil
}
