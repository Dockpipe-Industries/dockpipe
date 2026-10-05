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
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	publicationCohortOptInEnv    = "DORKPIPE_SQLITE_PUBLICATION_COHORT"
	publicationCohortChildEnv    = "DORKPIPE_SQLITE_PUBLICATION_CHILD"
	publicationCohortDatabaseEnv = "DORKPIPE_SQLITE_PUBLICATION_DATABASE"
	publicationCohortCycles      = 10000
	publicationSessionID         = "pipeon-sqlite-native-publication-cohort"
)

type publicationCommand struct {
	Cycle     int    `json:"cycle"`
	Operation string `json:"operation"`
}

type publicationResponse struct {
	Cycle      int    `json:"cycle"`
	Operation  string `json:"operation"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	Revision   int64  `json:"revision,omitempty"`
	Digest     string `json:"digest,omitempty"`
	SQLiteCode int    `json:"sqlite_code,omitempty"`
}

type publicationLine struct {
	text string
	err  error
}

type publicationChild struct {
	role    string
	command *exec.Cmd
	stdin   io.WriteCloser
	lines   chan publicationLine
	stderr  bytes.Buffer
}

type publicationWriterState struct {
	database *evidenceDatabase
	tx       *sql.Tx
	cancel   context.CancelFunc
	cycle    int
	newRow   aggregateRow
}

func TestWindowsNativeSQLitePublicationCohort(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("Windows/amd64 native evidence only")
	}
	if os.Getenv(publicationCohortOptInEnv) != "1" {
		t.Skip("set DORKPIPE_SQLITE_PUBLICATION_COHORT=1 to run the 10,000-cycle publication cohort")
	}
	if evidenceCGOEnabled || os.Getenv("CGO_ENABLED") != "0" {
		t.Fatalf("publication cohort requires a !cgo test binary and CGO_ENABLED=0; compiled_cgo=%t env=%q", evidenceCGOEnabled, os.Getenv("CGO_ENABLED"))
	}

	fixtureRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize publication fixture root: %v", err)
	}
	fixtureRoot = filepath.Clean(fixtureRoot)
	hostFacts, err := collectAndProtectWindowsHost(fixtureRoot)
	if err != nil {
		t.Fatalf("qualify Windows publication host: %v", err)
	}
	t.Logf("host windows_build=%s arch=%s drive=%s filesystem=%s ntfs_version=%s volume=%s go=%s root_protection={%s}",
		hostFacts.WindowsBuild, hostFacts.Architecture, hostFacts.DriveType, hostFacts.FileSystem,
		hostFacts.NTFSVersion, hostFacts.VolumeID, hostFacts.GoVersion, hostFacts.Protection)

	databasePath := prepareEvidenceFile(t, fixtureRoot, "publication")
	ctx, cancel := context.WithTimeout(context.Background(), 29*time.Minute)
	defer cancel()

	initialRow := publicationRow(1)
	database := mustOpenEvidenceDatabase(t, ctx, databasePath)
	mustCreateSelectedSchema(t, ctx, database.conn)
	mustInsertAndCommit(t, ctx, database.conn, initialRow, databasePath)
	mustRequireExactRow(t, ctx, database.conn, initialRow)
	if err := requireQuickCheck(ctx, database.conn); err != nil {
		t.Fatalf("initial publication quick_check: %v", err)
	}
	mustCloseEvidenceDatabase(t, database)
	preTreeHash, err := stablePublicationTreeHash(fixtureRoot)
	if err != nil {
		t.Fatalf("capture stable pre-cohort file tree: %v", err)
	}

	writer := startPublicationChild(t, "writer", databasePath)
	reader := startPublicationChild(t, "reader", databasePath)
	t.Cleanup(func() {
		writer.forceStop()
		reader.forceStop()
	})

	startedAt := time.Now()
	oldRow, oldReads, busyResults, newReads, protectedJournals := runPublicationCohortCycles(t, ctx, writer, reader, databasePath, initialRow)

	stopCycle := publicationCohortCycles + 1
	if err := writer.stop(stopCycle); err != nil {
		t.Fatalf("stop publication writer: %v", err)
	}
	if err := reader.stop(stopCycle); err != nil {
		t.Fatalf("stop publication reader: %v", err)
	}

	elapsed := time.Since(startedAt)
	postTreeHash, err := stablePublicationTreeHash(fixtureRoot)
	if err != nil {
		t.Fatalf("capture stable post-cohort file tree: %v", err)
	}
	if oldReads != publicationCohortCycles || busyResults != publicationCohortCycles || newReads != publicationCohortCycles || protectedJournals != publicationCohortCycles {
		t.Fatalf("publication counters old=%d busy=%d new=%d protected_journal=%d want=%d each", oldReads, busyResults, newReads, protectedJournals, publicationCohortCycles)
	}
	if oldRow.Revision != int64(publicationCohortCycles+1) {
		t.Fatalf("final publication revision = %d, want %d", oldRow.Revision, publicationCohortCycles+1)
	}
	if elapsed > 30*time.Minute {
		t.Fatalf("publication cohort elapsed %s, exceeds 30m", elapsed)
	}

	t.Logf("sqlite version=%s source_id=%s vfs=%s cycles=%d old_reads=%d busy_or_locked=%d new_reads=%d ambiguous_reads=0 revision_gaps_or_duplicates=0 digest_mismatches=0 protected_journals=%d",
		selectedSQLiteVersion, selectedSQLiteSourceID, selectedNativeVFS(), publicationCohortCycles,
		oldReads, busyResults, newReads, protectedJournals)
	t.Logf("publication initial_revision=%d initial_digest=%s final_revision=%d final_digest=%s elapsed=%s pre_tree_sha256=%s post_tree_sha256=%s",
		initialRow.Revision, hex.EncodeToString(initialRow.SHA256), oldRow.Revision, hex.EncodeToString(oldRow.SHA256),
		elapsed.Round(time.Millisecond), preTreeHash, postTreeHash)
}

func TestWindowsSQLitePublicationCohortChild(t *testing.T) {
	role := os.Getenv(publicationCohortChildEnv)
	if role == "" {
		t.Skip("publication-cohort child-process helper")
	}
	if runtime.GOARCH != "amd64" || evidenceCGOEnabled {
		t.Fatalf("publication child requires Windows/amd64 with CGo disabled; arch=%s cgo=%t", runtime.GOARCH, evidenceCGOEnabled)
	}
	databasePath := filepath.Clean(os.Getenv(publicationCohortDatabaseEnv))
	if databasePath == "." || !filepath.IsAbs(databasePath) {
		t.Fatalf("publication child database path must be absolute: %q", databasePath)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 4096)
	switch role {
	case "writer":
		runPublicationWriter(t, scanner, databasePath)
	case "reader":
		runPublicationReader(t, scanner, databasePath)
	default:
		t.Fatalf("unknown publication child role %q", role)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("publication %s command stream: %v", role, err)
	}
}

func publicationRow(revision int64) aggregateRow {
	payload := []byte(fmt.Sprintf(`{"adapter":"codex_app_server","outcome_unknown":true,"pipeon_session_id":"%s","revision":%d}`, publicationSessionID, revision))
	row := selectedRow(revision, payload)
	row.SessionID = publicationSessionID
	return row
}

func startPublicationChild(t *testing.T, role, databasePath string) *publicationChild {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestWindowsSQLitePublicationCohortChild$", "-test.count=1", "-test.timeout=30m")
	command.Env = publicationChildEnvironment(role, databasePath)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create publication %s stdin: %v", role, err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("create publication %s stdout: %v", role, err)
	}
	child := &publicationChild{role: role, command: command, stdin: stdin, lines: make(chan publicationLine, 8)}
	command.Stderr = &child.stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start publication %s child: %v", role, err)
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024), 4096)
		for scanner.Scan() {
			child.lines <- publicationLine{text: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			child.lines <- publicationLine{err: err}
		} else {
			child.lines <- publicationLine{err: io.EOF}
		}
		close(child.lines)
	}()
	return child
}

func publicationChildEnvironment(role, databasePath string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, publicationCohortChildEnv+"=") || strings.HasPrefix(item, publicationCohortDatabaseEnv+"=") {
			continue
		}
		environment = append(environment, item)
	}
	return append(environment, publicationCohortChildEnv+"="+role, publicationCohortDatabaseEnv+"="+databasePath)
}

func (child *publicationChild) exchange(command publicationCommand) (publicationResponse, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return publicationResponse{}, fmt.Errorf("encode command: %w", err)
	}
	if len(payload) > 1024 {
		return publicationResponse{}, errors.New("encoded command exceeds protocol bound")
	}
	if _, err := child.stdin.Write(append(payload, '\n')); err != nil {
		return publicationResponse{}, fmt.Errorf("write command: %w", err)
	}
	select {
	case line, ok := <-child.lines:
		if !ok {
			return publicationResponse{}, fmt.Errorf("%s response stream closed", child.role)
		}
		if line.err != nil {
			return publicationResponse{}, fmt.Errorf("%s response stream: %w", child.role, line.err)
		}
		var response publicationResponse
		if err := decodeStrictPublicationJSON(line.text, &response); err != nil {
			return publicationResponse{}, fmt.Errorf("decode %s response %q: %w", child.role, boundedText(line.text), err)
		}
		if response.Cycle != command.Cycle || response.Operation != command.Operation {
			return publicationResponse{}, fmt.Errorf("%s response out of order: command=%+v response=%+v", child.role, command, response)
		}
		if response.Status == "error" {
			return publicationResponse{}, fmt.Errorf("%s child error: %s", child.role, response.Error)
		}
		return response, nil
	case <-time.After(30 * time.Second):
		return publicationResponse{}, fmt.Errorf("%s response timed out for cycle=%d operation=%s", child.role, command.Cycle, command.Operation)
	}
}

func (child *publicationChild) stop(cycle int) error {
	response, err := child.exchange(publicationCommand{Cycle: cycle, Operation: "shutdown"})
	if err != nil {
		return err
	}
	if response.Status != "stopped" {
		return fmt.Errorf("%s shutdown status = %q, want stopped", child.role, response.Status)
	}
	if err := child.stdin.Close(); err != nil {
		return fmt.Errorf("close %s command stream: %w", child.role, err)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.command.Wait() }()
	select {
	case err := <-waited:
		if err != nil {
			return fmt.Errorf("wait for %s child: %w stderr=%s", child.role, err, boundedText(child.stderr.String()))
		}
		return nil
	case <-time.After(15 * time.Second):
		_ = child.command.Process.Kill()
		<-waited
		return fmt.Errorf("%s child did not exit after shutdown", child.role)
	}
}

func (child *publicationChild) forceStop() {
	if child == nil || child.command == nil || child.command.Process == nil || (child.command.ProcessState != nil && child.command.ProcessState.Exited()) {
		return
	}
	_ = child.command.Process.Kill()
	_, _ = child.command.Process.Wait()
}

func runPublicationWriter(t *testing.T, scanner *bufio.Scanner, databasePath string) {
	t.Helper()
	expectedCycle := 1
	state := &publicationWriterState{}
	defer state.close()
	for scanner.Scan() {
		command := decodePublicationCommand(t, scanner.Text())
		if command.Operation == "shutdown" {
			if command.Cycle != expectedCycle || state.tx != nil {
				t.Fatalf("writer shutdown out of order: cycle=%d expected=%d active=%t", command.Cycle, expectedCycle, state.tx != nil)
			}
			writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "stopped"})
			return
		}
		if command.Cycle != expectedCycle {
			t.Fatalf("writer command cycle=%d, want %d", command.Cycle, expectedCycle)
		}
		switch command.Operation {
		case "stage":
			if state.tx != nil {
				t.Fatalf("writer cycle %d received duplicate stage", command.Cycle)
			}
			if err := state.stage(databasePath, command.Cycle); err != nil {
				writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
				return
			}
			writePublicationResponse(t, rowPublicationResponse(command, "staged", state.newRow))
		case "commit":
			if state.tx == nil || state.cycle != command.Cycle {
				t.Fatalf("writer cycle %d commit without exact staged transaction", command.Cycle)
			}
			committedRow := state.newRow
			if err := state.commit(databasePath); err != nil {
				writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
				return
			}
			writePublicationResponse(t, rowPublicationResponse(command, "released", committedRow))
			expectedCycle++
		default:
			t.Fatalf("writer cycle %d unexpected operation %q", command.Cycle, command.Operation)
		}
	}
}

func (state *publicationWriterState) stage(databasePath string, cycle int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		cancel()
		return fmt.Errorf("open: %w", err)
	}
	fail := func(err error) error {
		cancel()
		_ = database.conn.Close()
		_ = database.db.Close()
		return err
	}
	oldRow := publicationRow(int64(cycle))
	current, err := readAggregateRow(ctx, database.conn)
	if err != nil || !sameRow(current, oldRow) {
		return fail(fmt.Errorf("pre-state revision=%d err=%v, want exact revision %d", current.Revision, err, oldRow.Revision))
	}
	if err := requireSchemaObjectCount(ctx, database.conn, 1); err != nil {
		return fail(fmt.Errorf("schema: %w", err))
	}
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		return fail(fmt.Errorf("begin exclusive: %w", err))
	}
	newRow := publicationRow(oldRow.Revision + 1)
	result, err := tx.ExecContext(ctx,
		"UPDATE app_server_aggregate SET revision=?, canonical_json=?, canonical_sha256=? WHERE singleton=? AND pipeon_session_id=? AND revision=? AND canonical_sha256=?",
		newRow.Revision, newRow.Payload, newRow.SHA256,
		oldRow.Singleton, oldRow.SessionID, oldRow.Revision, oldRow.SHA256,
	)
	if err != nil {
		_ = tx.Rollback()
		return fail(fmt.Errorf("execute CAS: %w", err))
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		_ = tx.Rollback()
		return fail(fmt.Errorf("CAS affected=%d err=%v, want 1", affected, err))
	}
	staged, err := readAggregateRow(ctx, tx)
	if err != nil || !sameRow(staged, newRow) {
		_ = tx.Rollback()
		return fail(fmt.Errorf("staged row revision=%d err=%v, want exact revision %d", staged.Revision, err, newRow.Revision))
	}
	if _, err := requirePublicationJournal(databasePath); err != nil {
		_ = tx.Rollback()
		return fail(err)
	}
	state.database = database
	state.tx = tx
	state.cancel = cancel
	state.cycle = cycle
	state.newRow = newRow
	return nil
}

func (state *publicationWriterState) commit(databasePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := state.tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	state.tx = nil
	state.cancel()
	state.cancel = nil
	current, err := readAggregateRow(ctx, state.database.conn)
	if err != nil || !sameRow(current, state.newRow) {
		return fmt.Errorf("post-commit row revision=%d err=%v, want exact revision %d", current.Revision, err, state.newRow.Revision)
	}
	if err := requireSchemaObjectCount(ctx, state.database.conn, 1); err != nil {
		return fmt.Errorf("post-commit schema: %w", err)
	}
	if err := requireOnlyMain(ctx, state.database.conn, databasePath); err != nil {
		return fmt.Errorf("post-commit database_list: %w", err)
	}
	if err := closePublicationDatabase(state.database); err != nil {
		return err
	}
	state.database = nil
	state.cycle = 0
	state.newRow = aggregateRow{}
	if err := requirePublicationSiblings(filepath.Dir(databasePath), false); err != nil {
		return err
	}
	return nil
}

func (state *publicationWriterState) close() {
	if state.tx != nil {
		_ = state.tx.Rollback()
	}
	if state.cancel != nil {
		state.cancel()
	}
	if state.database != nil {
		_ = state.database.conn.Close()
		_ = state.database.db.Close()
	}
}

func runPublicationReader(t *testing.T, scanner *bufio.Scanner, databasePath string) {
	t.Helper()
	expectedCycle := 1
	phase := 0
	for scanner.Scan() {
		command := decodePublicationCommand(t, scanner.Text())
		if command.Operation == "shutdown" {
			if command.Cycle != expectedCycle || phase != 0 {
				t.Fatalf("reader shutdown out of order: cycle=%d expected=%d phase=%d", command.Cycle, expectedCycle, phase)
			}
			writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "stopped"})
			return
		}
		if command.Cycle != expectedCycle {
			t.Fatalf("reader command cycle=%d, want %d", command.Cycle, expectedCycle)
		}
		switch {
		case phase == 0 && command.Operation == "read_old":
			row, err := readPublicationRow(databasePath, publicationRow(int64(command.Cycle)), false)
			if err != nil {
				writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
				return
			}
			writePublicationResponse(t, rowPublicationResponse(command, "row", row))
			phase = 1
		case phase == 1 && command.Operation == "expect_busy":
			code, err := requirePublicationBusy(databasePath)
			if err != nil {
				writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
				return
			}
			writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "busy_or_locked", SQLiteCode: code})
			phase = 2
		case phase == 2 && command.Operation == "read_new":
			row, err := readPublicationRow(databasePath, publicationRow(int64(command.Cycle+1)), true)
			if err != nil {
				writePublicationResponse(t, publicationResponse{Cycle: command.Cycle, Operation: command.Operation, Status: "error", Error: boundedText(err.Error())})
				return
			}
			writePublicationResponse(t, rowPublicationResponse(command, "row", row))
			phase = 0
			expectedCycle++
		default:
			t.Fatalf("reader cycle %d operation=%q is out of order at phase=%d", command.Cycle, command.Operation, phase)
		}
	}
}

func readPublicationRow(databasePath string, want aggregateRow, checkIntegrity bool) (aggregateRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		return aggregateRow{}, fmt.Errorf("open: %w", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	row, err := readAggregateRow(ctx, database.conn)
	if err != nil || !sameRow(row, want) {
		return aggregateRow{}, fmt.Errorf("row revision=%d err=%v, want exact revision %d", row.Revision, err, want.Revision)
	}
	if err := requireSchemaObjectCount(ctx, database.conn, 1); err != nil {
		return aggregateRow{}, fmt.Errorf("schema: %w", err)
	}
	if err := requireOnlyMain(ctx, database.conn, databasePath); err != nil {
		return aggregateRow{}, fmt.Errorf("database_list: %w", err)
	}
	if checkIntegrity {
		if err := requireQuickCheck(ctx, database.conn); err != nil {
			return aggregateRow{}, fmt.Errorf("quick_check: %w", err)
		}
	}
	if err := closePublicationDatabase(database); err != nil {
		return aggregateRow{}, err
	}
	if err := requirePublicationSiblings(filepath.Dir(databasePath), false); err != nil {
		return aggregateRow{}, err
	}
	return row, nil
}

func requirePublicationBusy(databasePath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		if code, ok := sqlitePrimaryCode(err); ok && (code == 5 || code == 6) {
			return code, nil
		}
		return 0, fmt.Errorf("open returned non-contention error: %w", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	_, err = readAggregateRow(ctx, database.conn)
	if err != nil {
		if code, ok := sqlitePrimaryCode(err); ok && (code == 5 || code == 6) {
			return code, nil
		}
		return 0, fmt.Errorf("read returned non-contention error: %w", err)
	}
	return 0, errors.New("reader observed a row while the exclusive writer was live")
}

func closePublicationDatabase(database *evidenceDatabase) error {
	if database == nil {
		return nil
	}
	if err := database.conn.Close(); err != nil {
		_ = database.db.Close()
		return fmt.Errorf("close dedicated connection: %w", err)
	}
	if err := database.db.Close(); err != nil {
		return fmt.Errorf("close database handle: %w", err)
	}
	return nil
}

func requirePublicationJournal(databasePath string) (string, error) {
	journalPath := databasePath + "-journal"
	info, err := os.Lstat(journalPath)
	if err != nil {
		return "", fmt.Errorf("inspect publication journal: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("publication journal is not a regular file: mode=%s", info.Mode())
	}
	protection, err := requireWindowsPrivatePath(journalPath)
	if err != nil {
		return "", fmt.Errorf("publication journal protection: %w", err)
	}
	if err := requirePublicationSiblings(filepath.Dir(databasePath), true); err != nil {
		return "", err
	}
	return protection, nil
}

func requirePublicationSiblings(directory string, requireJournal bool) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read publication siblings: %w", err)
	}
	seenMain := false
	seenJournal := false
	for _, entry := range entries {
		switch entry.Name() {
		case "aggregate.sqlite":
			seenMain = true
		case "aggregate.sqlite-journal":
			seenJournal = true
		default:
			return fmt.Errorf("unexpected publication sibling %q", entry.Name())
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("publication sibling %q is not a regular file", entry.Name())
		}
		if _, err := requireWindowsPrivatePath(filepath.Join(directory, entry.Name())); err != nil {
			return fmt.Errorf("publication sibling %q protection: %w", entry.Name(), err)
		}
	}
	if !seenMain || (requireJournal && !seenJournal) {
		return fmt.Errorf("publication siblings main=%t journal=%t require_journal=%t", seenMain, seenJournal, requireJournal)
	}
	return nil
}

func stablePublicationTreeHash(root string) (string, error) {
	first, err := publicationTreeHash(root)
	if err != nil {
		return "", err
	}
	second, err := publicationTreeHash(root)
	if err != nil {
		return "", err
	}
	if first != second {
		return "", fmt.Errorf("file tree changed while quiescent: first=%s second=%s", first, second)
	}
	return first, nil
}

func publicationTreeHash(root string) (string, error) {
	if _, err := requireWindowsPrivatePath(root); err != nil {
		return "", fmt.Errorf("fixture root protection: %w", err)
	}
	rows := make([]string, 0, 3)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("file tree contains symlink %q", relative)
		}
		kind := "file"
		if entry.IsDir() {
			kind = "directory"
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("file tree contains non-regular entry %q mode=%s", relative, info.Mode())
		}
		protection, err := requireWindowsPrivatePath(path)
		if err != nil {
			return err
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\t%d\t%s", relative, kind, info.Size(), protection))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(rows)
	digest := sha256Bytes([]byte(strings.Join(rows, "\n") + "\n"))
	return hex.EncodeToString(digest), nil
}

func decodePublicationCommand(t *testing.T, line string) publicationCommand {
	t.Helper()
	if len(line) == 0 || len(line) > 4096 {
		t.Fatalf("publication command length = %d, want 1..4096", len(line))
	}
	var command publicationCommand
	if err := decodeStrictPublicationJSON(line, &command); err != nil {
		t.Fatalf("decode publication command: %v", err)
	}
	if command.Cycle < 1 || command.Operation == "" || len(command.Operation) > 32 {
		t.Fatalf("invalid publication command: %+v", command)
	}
	return command
}

func decodeStrictPublicationJSON(line string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writePublicationResponse(t *testing.T, response publicationResponse) {
	t.Helper()
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("encode publication response: %v", err)
	}
	if len(payload) > 1024 {
		t.Fatalf("publication response exceeds protocol bound: %d", len(payload))
	}
	if _, err := fmt.Fprintln(os.Stdout, string(payload)); err != nil {
		t.Fatalf("write publication response: %v", err)
	}
}

func rowPublicationResponse(command publicationCommand, status string, row aggregateRow) publicationResponse {
	return publicationResponse{
		Cycle:     command.Cycle,
		Operation: command.Operation,
		Status:    status,
		Revision:  row.Revision,
		Digest:    hex.EncodeToString(row.SHA256),
	}
}

func requirePublicationResponse(t *testing.T, response publicationResponse, cycle int, operation, status string, row aggregateRow) {
	t.Helper()
	wantDigest := hex.EncodeToString(row.SHA256)
	if response.Cycle != cycle || response.Operation != operation || response.Status != status || response.Revision != row.Revision || response.Digest != wantDigest || response.SQLiteCode != 0 {
		t.Fatalf("cycle %d %s response mismatch: got=%+v want_status=%s want_revision=%d want_digest=%s", cycle, operation, response, status, row.Revision, wantDigest)
	}
}
