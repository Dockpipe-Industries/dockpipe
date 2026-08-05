package sqliteevidence

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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
	selectedSQLiteVersion  = "3.53.3"
	selectedSQLiteSourceID = "2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62"
	evidenceOptInEnv       = "DORKPIPE_SQLITE_EVIDENCE"
	childRoleEnv           = "DORKPIPE_SQLITE_EVIDENCE_CHILD"
	childDatabaseEnv       = "DORKPIPE_SQLITE_EVIDENCE_DATABASE"
	childFixtureRootEnv    = "DORKPIPE_SQLITE_EVIDENCE_FIXTURE_ROOT"
	childRunTokenEnv       = "DORKPIPE_SQLITE_EVIDENCE_RUN_TOKEN"
	childRootIdentityEnv   = "DORKPIPE_SQLITE_EVIDENCE_ROOT_IDENTITY"
	childReadyMarker       = "SQLITE_EVIDENCE_OWNER_READY"
	selectedSessionID      = "pipeon-sqlite-native-smoke"
)

const selectedSchema = `CREATE TABLE app_server_aggregate (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    pipeon_session_id TEXT NOT NULL CHECK (length(pipeon_session_id) BETWEEN 1 AND 256),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    canonical_json BLOB NOT NULL CHECK (length(canonical_json) BETWEEN 1 AND 16384),
    canonical_sha256 BLOB NOT NULL CHECK (length(canonical_sha256) = 32)
) STRICT`

var (
	payloadRevision1 = []byte(`{"adapter":"codex_app_server","outcome_unknown":true,"pipeon_session_id":"pipeon-sqlite-native-smoke","revision":1}`)
	payloadRevision2 = []byte(`{"adapter":"codex_app_server","outcome_unknown":true,"pipeon_session_id":"pipeon-sqlite-native-smoke","revision":2}`)
	payloadRevision3 = []byte(`{"adapter":"codex_app_server","outcome_unknown":true,"pipeon_session_id":"pipeon-sqlite-native-smoke","revision":3}`)
)

type windowsHostFacts struct {
	WindowsBuild string
	Architecture string
	DriveType    string
	FileSystem   string
	NTFSVersion  string
	VolumeID     string
	GoVersion    string
	Protection   string
}

type aggregateRow struct {
	Singleton int64
	SessionID string
	Revision  int64
	Payload   []byte
	SHA256    []byte
}

type evidenceDatabase struct {
	db             *sql.DB
	conn           *sql.Conn
	path           string
	dsn            string
	compileOptions []string
}

type nativeSmokePlatform struct {
	name                      string
	optInEnv                  string
	childTestName             string
	qualifyRoot               func(string) (string, error)
	prepareFile               func(*testing.T, string, string) string
	requireJournal            func(*testing.T, string)
	inspectJournalAfterCommit func(*testing.T, string)
	requireSiblings           func(*testing.T, string, bool)
	stableTreeHash            func(string) (string, error)
	rootIdentity              func(string) (string, error)
	validateCompileOptions    func([]string) error
	cleanCommitAfterRecovery  bool
}

type evidenceChildSpec struct {
	testName     string
	fixtureRoot  string
	runToken     string
	rootIdentity string
}

func TestWindowsNativeSQLiteSmoke(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("Windows/amd64 native evidence only")
	}
	runNativeSQLiteSmoke(t, nativeSmokePlatform{
		name:                      "Windows",
		optInEnv:                  evidenceOptInEnv,
		childTestName:             "TestSQLiteEvidenceChild",
		qualifyRoot:               qualifyWindowsSmokeRoot,
		prepareFile:               prepareEvidenceFile,
		requireJournal:            mustRequireJournal,
		inspectJournalAfterCommit: mustInspectJournalAfterCommit,
		requireSiblings:           mustRequireSiblings,
	})
}

func qualifyWindowsSmokeRoot(root string) (string, error) {
	hostFacts, err := collectAndProtectWindowsHost(root)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("windows_build=%s arch=%s drive=%s filesystem=%s ntfs_version=%s volume=%s go=%s root_protection={%s}",
		hostFacts.WindowsBuild, hostFacts.Architecture, hostFacts.DriveType, hostFacts.FileSystem,
		hostFacts.NTFSVersion, hostFacts.VolumeID, hostFacts.GoVersion, hostFacts.Protection), nil
}

func runNativeSQLiteSmoke(t *testing.T, platform nativeSmokePlatform) {
	t.Helper()
	if os.Getenv(platform.optInEnv) != "1" {
		t.Skipf("set %s=1 to run the bounded native smoke lane", platform.optInEnv)
	}
	if evidenceCGOEnabled || os.Getenv("CGO_ENABLED") != "0" {
		t.Fatalf("native evidence requires a !cgo test binary and CGO_ENABLED=0; compiled_cgo=%t env=%q", evidenceCGOEnabled, os.Getenv("CGO_ENABLED"))
	}
	startedAt := time.Now()
	fixtureRoot, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize fixture root: %v", err)
	}
	fixtureRoot = filepath.Clean(fixtureRoot)
	hostEvidence, err := platform.qualifyRoot(fixtureRoot)
	if err != nil {
		t.Fatalf("qualify %s host: %v", platform.name, err)
	}
	t.Logf("host %s", hostEvidence)

	mainPath := platform.prepareFile(t, fixtureRoot, "main")
	otherPath := platform.prepareFile(t, fixtureRoot, "other")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	mainDB := mustOpenEvidenceDatabase(t, ctx, mainPath)
	t.Logf("sqlite version=%s source_id=%s vfs=%s uri=%q", selectedSQLiteVersion, selectedSQLiteSourceID, selectedNativeVFS(), mainDB.dsn)
	t.Logf("sqlite compile_options[%d]=%s", len(mainDB.compileOptions), strings.Join(mainDB.compileOptions, ","))
	if platform.validateCompileOptions != nil {
		if err := platform.validateCompileOptions(mainDB.compileOptions); err != nil {
			t.Fatalf("validate compile options: %v", err)
		}
	}
	mustRequireOnlyMain(t, ctx, mainDB.conn, mainPath)
	platform.requireSiblings(t, filepath.Dir(mainPath), false)
	mustCreateSelectedSchema(t, ctx, mainDB.conn)

	revision1 := selectedRow(1, payloadRevision1)
	mustInsertAndCommitWithJournal(t, ctx, mainDB.conn, revision1, mainPath, platform.requireJournal)
	mustRequireExactRow(t, ctx, mainDB.conn, revision1)
	platform.inspectJournalAfterCommit(t, mainPath)

	revision2 := selectedRow(2, payloadRevision2)
	mustCASAndCommitWithJournal(t, ctx, mainDB.conn, revision1, revision2, mainPath, platform.requireJournal)
	mustRequireExactRow(t, ctx, mainDB.conn, revision2)
	mustRequireSchema(t, ctx, mainDB.conn)
	mustRequireOnlyMain(t, ctx, mainDB.conn, mainPath)
	platform.inspectJournalAfterCommit(t, mainPath)
	mustCloseEvidenceDatabase(t, mainDB)

	otherDB := mustOpenEvidenceDatabase(t, ctx, otherPath)
	mustCreateSelectedSchema(t, ctx, otherDB.conn)
	mustCloseEvidenceDatabase(t, otherDB)

	preTreeHash := "not_recorded"
	if platform.stableTreeHash != nil {
		preTreeHash, err = platform.stableTreeHash(fixtureRoot)
		if err != nil {
			t.Fatalf("capture pre-contention metadata tree: %v", err)
		}
	}
	runToken := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", fixtureRoot, time.Now().UnixNano()))))
	rootIdentity := ""
	if platform.rootIdentity != nil {
		rootIdentity, err = platform.rootIdentity(fixtureRoot)
		if err != nil {
			t.Fatalf("capture fixture-root identity: %v", err)
		}
	}
	child := evidenceChildSpec{testName: platform.childTestName, fixtureRoot: fixtureRoot, runToken: runToken, rootIdentity: rootIdentity}
	owner := startOwnerChild(t, mainPath, child)
	t.Cleanup(func() {
		if owner.ProcessState == nil || !owner.ProcessState.Exited() {
			_ = owner.Process.Kill()
			_, _ = owner.Process.Wait()
		}
	})
	platform.requireJournal(t, mainPath)
	mustRunChild(t, "contend", mainPath, "same_database_busy_or_locked", child)
	mustRunChild(t, "other", otherPath, "different_database_succeeded", child)
	if err := owner.Process.Kill(); err != nil {
		t.Fatalf("force-terminate exclusive owner: %v", err)
	}
	if err := owner.Wait(); err == nil {
		t.Fatal("force-terminated owner unexpectedly exited successfully")
	}
	mustRunChild(t, "recover", mainPath, "recovery_succeeded", child)
	finalRow := revision2
	var cleanDB *evidenceDatabase
	if platform.cleanCommitAfterRecovery {
		cleanDB = mustOpenEvidenceDatabase(t, ctx, mainPath)
		mustRequireExactRow(t, ctx, cleanDB.conn, revision2)
		mustCASAndCommitWithJournal(t, ctx, cleanDB.conn, revision2, selectedRow(3, payloadRevision3), mainPath, platform.requireJournal)
		finalRow = selectedRow(3, payloadRevision3)
		mustRequireExactRow(t, ctx, cleanDB.conn, finalRow)
		mustRequireSchema(t, ctx, cleanDB.conn)
		if err := requireQuickCheck(ctx, cleanDB.conn); err != nil {
			t.Fatalf("clean writer quick_check: %v", err)
		}
		t.Logf("clean commit revision=%d digest=%x", finalRow.Revision, finalRow.SHA256)
	}
	platform.requireSiblings(t, filepath.Dir(mainPath), true)
	platform.inspectJournalAfterCommit(t, mainPath)
	postTreeHash := "not_recorded"
	if platform.stableTreeHash != nil {
		postTreeHash, err = platform.stableTreeHash(fixtureRoot)
		if err != nil {
			t.Fatalf("capture post-recovery metadata tree: %v", err)
		}
	}
	if cleanDB != nil {
		mustCloseEvidenceDatabase(t, cleanDB)
	}
	t.Logf("contention same_database=BUSY_or_LOCKED different_database=success forced_termination=lock_released recovery=quick_check_ok recovery_revision=%d clean_commit_revision=%d retries=0 replays=0 repairs=0 fallbacks=0 inferred_acknowledgements=0", revision2.Revision, finalRow.Revision)
	t.Logf("smoke initial_revision=%d initial_digest=%x final_revision=%d final_digest=%x pre_metadata_tree_sha256=%s post_metadata_tree_sha256=%s elapsed=%s journal_contents_opened_or_hashed=false parent_cleanup_only=true", revision1.Revision, revision1.SHA256, finalRow.Revision, finalRow.SHA256, preTreeHash, postTreeHash, time.Since(startedAt).Round(time.Millisecond))
}

func TestSQLiteEvidenceChild(t *testing.T) {
	runSQLiteEvidenceChild(t, "windows", nil, requireWindowsChildJournal, false)
}

func runSQLiteEvidenceChild(t *testing.T, expectedOS string, validatePath func(string, string, string) error, requireJournal func(string) error, strictRecoveryOld bool) {
	t.Helper()
	role := os.Getenv(childRoleEnv)
	if role == "" {
		t.Skip("child-process helper")
	}
	if runtime.GOOS != expectedOS || runtime.GOARCH != "amd64" || evidenceCGOEnabled {
		t.Fatalf("child requires %s/amd64 with CGO disabled; os=%s arch=%s cgo=%t", expectedOS, runtime.GOOS, runtime.GOARCH, evidenceCGOEnabled)
	}
	databasePath := filepath.Clean(os.Getenv(childDatabaseEnv))
	if databasePath == "." || !filepath.IsAbs(databasePath) {
		t.Fatalf("child database path must be absolute: %q", databasePath)
	}
	fixtureRoot := filepath.Clean(os.Getenv(childFixtureRootEnv))
	runToken := os.Getenv(childRunTokenEnv)
	if fixtureRoot == "." || !filepath.IsAbs(fixtureRoot) || len(runToken) != 64 {
		t.Fatalf("child fixture identity is invalid: root=%q token_length=%d", fixtureRoot, len(runToken))
	}
	if validatePath != nil {
		if err := validatePath(fixtureRoot, databasePath, os.Getenv(childRootIdentityEnv)); err != nil {
			t.Fatalf("child path validation: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	switch role {
	case "contend":
		childRequireContention(t, ctx, databasePath)
	case "other":
		childUseDifferentDatabase(t, ctx, databasePath)
	case "owner":
		childHoldExclusiveOwner(t, ctx, databasePath, requireJournal)
	case "recover":
		childRecover(t, ctx, databasePath, strictRecoveryOld)
	default:
		t.Fatalf("unknown child role %q", role)
	}
}

func requireWindowsChildJournal(journalPath string) error {
	info, err := os.Lstat(journalPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("owner live journal: info=%v err=%v", info, err)
	}
	if _, err := requireWindowsPrivatePath(journalPath); err != nil {
		return fmt.Errorf("owner live journal protection: %w", err)
	}
	return nil
}

func prepareEvidenceFile(t *testing.T, root, name string) string {
	t.Helper()
	directory := filepath.Join(root, name)
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create %s evidence directory: %v", name, err)
	}
	if err := setWindowsPrivateDirectory(directory); err != nil {
		t.Fatalf("protect %s evidence directory: %v", name, err)
	}
	databasePath := filepath.Join(directory, "aggregate.sqlite")
	file, err := os.OpenFile(databasePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("pre-create private %s database: %v", name, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close private %s database: %v", name, err)
	}
	if info, err := os.Stat(databasePath); err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("private %s database is not an empty regular file: info=%v err=%v", name, info, err)
	}
	if protection, err := requireWindowsPrivatePath(databasePath); err != nil {
		t.Fatalf("verify private %s database: %v", name, err)
	} else {
		t.Logf("%s database protection={%s}", name, protection)
	}
	mustRequireSiblings(t, directory, false)
	return databasePath
}

func selectedRow(revision int64, payload []byte) aggregateRow {
	digest := sha256.Sum256(payload)
	return aggregateRow{
		Singleton: 1,
		SessionID: selectedSessionID,
		Revision:  revision,
		Payload:   bytes.Clone(payload),
		SHA256:    bytes.Clone(digest[:]),
	}
}

func mustOpenEvidenceDatabase(t *testing.T, ctx context.Context, path string) *evidenceDatabase {
	t.Helper()
	database, err := openEvidenceDatabase(ctx, path)
	if err != nil {
		t.Fatalf("open evidence database: %v", err)
	}
	return database
}

func openEvidenceDatabase(ctx context.Context, path string) (*evidenceDatabase, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("database path is not canonical absolute: %q", path)
	}
	dsn, err := evidenceFileURI(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("construct database handle: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open dedicated connection: %w", err)
	}
	result := &evidenceDatabase{db: db, conn: conn, path: path, dsn: dsn}
	fail := func(err error) (*evidenceDatabase, error) {
		_ = conn.Close()
		_ = db.Close()
		return nil, err
	}
	var version, sourceID string
	if err := conn.QueryRowContext(ctx, "SELECT sqlite_version(), sqlite_source_id()").Scan(&version, &sourceID); err != nil {
		return fail(fmt.Errorf("query SQLite identity: %w", err))
	}
	if version != selectedSQLiteVersion || sourceID != selectedSQLiteSourceID {
		return fail(fmt.Errorf("SQLite identity = %q / %q, want %q / %q", version, sourceID, selectedSQLiteVersion, selectedSQLiteSourceID))
	}
	if _, err := conn.ExecContext(ctx, `SELECT "dqs_must_be_disabled"`); err == nil {
		return fail(errors.New("_dqs=0 was ignored: unresolved double-quoted string was accepted"))
	}
	if err := applySelectedPragmas(ctx, conn); err != nil {
		return fail(err)
	}
	options, err := readCompileOptions(ctx, conn)
	if err != nil {
		return fail(err)
	}
	result.compileOptions = options
	if err := requireOnlyMain(ctx, conn, path); err != nil {
		return fail(err)
	}
	return result, nil
}

func evidenceFileURI(path string) (string, error) {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) || clean != path {
		return "", fmt.Errorf("URI path is not canonical absolute: %q", path)
	}
	uriPath := filepath.ToSlash(clean)
	if runtime.GOOS == "windows" {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath}
	query := url.Values{}
	query.Set("mode", "rw")
	query.Set("cache", "private")
	query.Set("vfs", selectedNativeVFS())
	query.Set("_txlock", "exclusive")
	query.Set("_dqs", "0")
	query.Set("_error_rc", "1")
	uri.RawQuery = query.Encode()
	return uri.String(), nil
}

func applySelectedPragmas(ctx context.Context, conn *sql.Conn) error {
	textPragmas := []struct {
		name string
		set  string
		want string
	}{
		{"journal_mode", "DELETE", "delete"},
		{"locking_mode", "EXCLUSIVE", "exclusive"},
	}
	for _, item := range textPragmas {
		var got string
		if err := conn.QueryRowContext(ctx, "PRAGMA "+item.name+"="+item.set).Scan(&got); err != nil {
			return fmt.Errorf("apply PRAGMA %s=%s: %w", item.name, item.set, err)
		}
		if got != item.want {
			return fmt.Errorf("PRAGMA %s set result = %q, want %q", item.name, got, item.want)
		}
		if err := conn.QueryRowContext(ctx, "PRAGMA "+item.name).Scan(&got); err != nil {
			return fmt.Errorf("read back PRAGMA %s: %w", item.name, err)
		}
		if got != item.want {
			return fmt.Errorf("PRAGMA %s readback = %q, want %q", item.name, got, item.want)
		}
	}
	integerPragmas := []struct {
		name string
		set  string
		want int64
	}{
		{"synchronous", "EXTRA", 3},
		{"fullfsync", "ON", 1},
		{"temp_store", "MEMORY", 2},
		{"mmap_size", "0", 0},
		{"busy_timeout", "0", 0},
		{"foreign_keys", "ON", 1},
		{"trusted_schema", "OFF", 0},
		{"cell_size_check", "ON", 1},
		{"page_size", "4096", 4096},
	}
	for _, item := range integerPragmas {
		if _, err := conn.ExecContext(ctx, "PRAGMA "+item.name+"="+item.set); err != nil {
			return fmt.Errorf("apply PRAGMA %s=%s: %w", item.name, item.set, err)
		}
		var got int64
		if err := conn.QueryRowContext(ctx, "PRAGMA "+item.name).Scan(&got); err != nil {
			return fmt.Errorf("read back PRAGMA %s: %w", item.name, err)
		}
		if got != item.want {
			return fmt.Errorf("PRAGMA %s readback = %d, want %d", item.name, got, item.want)
		}
	}
	return nil
}

func readCompileOptions(ctx context.Context, conn *sql.Conn) ([]string, error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA compile_options")
	if err != nil {
		return nil, fmt.Errorf("query compile options: %w", err)
	}
	defer rows.Close()
	var options []string
	for rows.Next() {
		var option string
		if err := rows.Scan(&option); err != nil {
			return nil, fmt.Errorf("scan compile option: %w", err)
		}
		if option == "" || len(option) > 160 || len(options) >= 256 {
			return nil, fmt.Errorf("compile options exceed evidence bounds")
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate compile options: %w", err)
	}
	if len(options) == 0 {
		return nil, errors.New("compile options are empty")
	}
	sort.Strings(options)
	return options, nil
}

func mustCreateSelectedSchema(t *testing.T, ctx context.Context, conn *sql.Conn) {
	t.Helper()
	if err := requireSchemaObjectCount(ctx, conn, 0); err != nil {
		t.Fatalf("require empty schema: %v", err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin schema transaction: %v", err)
	}
	if _, err := tx.ExecContext(ctx, selectedSchema); err != nil {
		_ = tx.Rollback()
		t.Fatalf("create selected schema: %v", err)
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version=1"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("set user_version: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit selected schema: %v", err)
	}
	mustRequireSchema(t, ctx, conn)
}

func mustRequireSchema(t *testing.T, ctx context.Context, conn *sql.Conn) {
	t.Helper()
	if err := requireSchemaObjectCount(ctx, conn, 1); err != nil {
		t.Fatalf("require selected schema: %v", err)
	}
	var userVersion int64
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil || userVersion != 1 {
		t.Fatalf("user_version = %d err=%v, want 1", userVersion, err)
	}
}

func requireSchemaObjectCount(ctx context.Context, conn *sql.Conn, want int) error {
	rows, err := conn.QueryContext(ctx, "SELECT type, name, tbl_name, sql FROM sqlite_schema ORDER BY type, name")
	if err != nil {
		return fmt.Errorf("query schema objects: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var objectType, name, tableName string
		var sqlText sql.NullString
		if err := rows.Scan(&objectType, &name, &tableName, &sqlText); err != nil {
			return fmt.Errorf("scan schema object: %w", err)
		}
		count++
		if want == 1 && (objectType != "table" || name != "app_server_aggregate" || tableName != name || !sqlText.Valid || strings.Join(strings.Fields(sqlText.String), " ") != strings.Join(strings.Fields(selectedSchema), " ")) {
			return fmt.Errorf("unexpected schema object type=%q name=%q table=%q sql=%q", objectType, name, tableName, sqlText.String)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema objects: %w", err)
	}
	if count != want {
		return fmt.Errorf("schema object count = %d, want %d", count, want)
	}
	return nil
}

func mustInsertAndCommit(t *testing.T, ctx context.Context, conn *sql.Conn, row aggregateRow, path string) {
	mustInsertAndCommitWithJournal(t, ctx, conn, row, path, mustRequireJournal)
}

func mustInsertAndCommitWithJournal(t *testing.T, ctx context.Context, conn *sql.Conn, row aggregateRow, path string, requireJournal func(*testing.T, string)) {
	t.Helper()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin insert transaction: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO app_server_aggregate (singleton, pipeon_session_id, revision, canonical_json, canonical_sha256) VALUES (?, ?, ?, ?, ?)",
		row.Singleton, row.SessionID, row.Revision, row.Payload, row.SHA256,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert selected row: %v", err)
	}
	requireJournal(t, path)
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit selected row: %v", err)
	}
}

func mustCASAndCommit(t *testing.T, ctx context.Context, conn *sql.Conn, oldRow, newRow aggregateRow, path string) {
	mustCASAndCommitWithJournal(t, ctx, conn, oldRow, newRow, path, mustRequireJournal)
}

func mustCASAndCommitWithJournal(t *testing.T, ctx context.Context, conn *sql.Conn, oldRow, newRow aggregateRow, path string, requireJournal func(*testing.T, string)) {
	t.Helper()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin CAS transaction: %v", err)
	}
	result, err := tx.ExecContext(ctx,
		"UPDATE app_server_aggregate SET revision=?, canonical_json=?, canonical_sha256=? WHERE singleton=? AND pipeon_session_id=? AND revision=? AND canonical_sha256=?",
		newRow.Revision, newRow.Payload, newRow.SHA256, oldRow.Singleton, oldRow.SessionID, oldRow.Revision, oldRow.SHA256,
	)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("execute selected CAS: %v", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		_ = tx.Rollback()
		t.Fatalf("selected CAS affected %d rows err=%v, want 1", affected, err)
	}
	requireJournal(t, path)
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit selected CAS: %v", err)
	}
}

func mustRequireExactRow(t *testing.T, ctx context.Context, conn *sql.Conn, want aggregateRow) {
	t.Helper()
	got, err := readAggregateRow(ctx, conn)
	if err != nil {
		t.Fatalf("reload selected row: %v", err)
	}
	if !sameRow(got, want) {
		t.Fatalf("reloaded row mismatch: got revision=%d session=%q payload=%x sha=%x", got.Revision, got.SessionID, got.Payload, got.SHA256)
	}
}

func readAggregateRow(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (aggregateRow, error) {
	var row aggregateRow
	err := queryer.QueryRowContext(ctx,
		"SELECT singleton, pipeon_session_id, revision, canonical_json, canonical_sha256 FROM app_server_aggregate",
	).Scan(&row.Singleton, &row.SessionID, &row.Revision, &row.Payload, &row.SHA256)
	if err != nil {
		return aggregateRow{}, err
	}
	var count int
	if err := queryer.QueryRowContext(ctx, "SELECT count(*) FROM app_server_aggregate").Scan(&count); err != nil {
		return aggregateRow{}, err
	}
	if count != 1 {
		return aggregateRow{}, fmt.Errorf("aggregate row count = %d, want 1", count)
	}
	return row, nil
}

func sameRow(got, want aggregateRow) bool {
	return got.Singleton == want.Singleton &&
		got.SessionID == want.SessionID &&
		got.Revision == want.Revision &&
		bytes.Equal(got.Payload, want.Payload) &&
		bytes.Equal(got.SHA256, want.SHA256) &&
		bytes.Equal(got.SHA256, sha256Bytes(got.Payload))
}

func sha256Bytes(payload []byte) []byte {
	digest := sha256.Sum256(payload)
	return digest[:]
}

func mustRequireOnlyMain(t *testing.T, ctx context.Context, conn *sql.Conn, path string) {
	t.Helper()
	if err := requireOnlyMain(ctx, conn, path); err != nil {
		t.Fatalf("require only main database: %v", err)
	}
}

func requireOnlyMain(ctx context.Context, conn *sql.Conn, path string) error {
	rows, err := conn.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return fmt.Errorf("query database_list: %w", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var sequence int
		var name, file string
		if err := rows.Scan(&sequence, &name, &file); err != nil {
			return fmt.Errorf("scan database_list: %w", err)
		}
		count++
		if sequence != 0 || name != "main" || !samePhysicalFile(file, path) {
			return fmt.Errorf("database_list entry = (%d,%q,%q), want only main %q", sequence, name, file, path)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate database_list: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("database_list count = %d, want 1", count)
	}
	return nil
}

func samePhysicalFile(first, second string) bool {
	firstInfo, firstErr := os.Stat(filepath.Clean(first))
	secondInfo, secondErr := os.Stat(filepath.Clean(second))
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

func mustRequireJournal(t *testing.T, databasePath string) {
	t.Helper()
	journalPath := databasePath + "-journal"
	deadline := time.Now().Add(2 * time.Second)
	for {
		info, err := os.Lstat(journalPath)
		if err == nil && info.Mode().IsRegular() {
			break
		}
		if err != nil && !os.IsNotExist(err) {
			t.Fatalf("inspect live journal: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("SQLite journal was not present while transaction was live: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	protection, err := requireWindowsPrivatePath(journalPath)
	if err != nil {
		t.Fatalf("verify live journal protection: %v", err)
	}
	mustRequireSiblings(t, filepath.Dir(databasePath), true)
	t.Logf("journal live protection={%s}", protection)
}

func mustInspectJournalAfterCommit(t *testing.T, databasePath string) {
	t.Helper()
	journalPath := databasePath + "-journal"
	info, err := os.Lstat(journalPath)
	if os.IsNotExist(err) {
		t.Log("journal after_commit=absent")
		return
	}
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("inspect post-commit journal: info=%v err=%v", info, err)
	}
	protection, err := requireWindowsPrivatePath(journalPath)
	if err != nil {
		t.Fatalf("verify post-commit journal protection: %v", err)
	}
	t.Logf("journal after_commit=present size=%d protection={%s}", info.Size(), protection)
}

func mustRequireSiblings(t *testing.T, directory string, requireJournal bool) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read database siblings: %v", err)
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
			t.Fatalf("unexpected database sibling %q", entry.Name())
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			t.Fatalf("database sibling %q is not a regular file", entry.Name())
		}
	}
	if !seenMain || (requireJournal && !seenJournal) {
		t.Fatalf("database siblings main=%t journal=%t require_journal=%t", seenMain, seenJournal, requireJournal)
	}
}

func mustCloseEvidenceDatabase(t *testing.T, database *evidenceDatabase) {
	t.Helper()
	if err := database.conn.Close(); err != nil {
		t.Fatalf("close dedicated connection: %v", err)
	}
	if err := database.db.Close(); err != nil {
		t.Fatalf("close database handle: %v", err)
	}
}

func startOwnerChild(t *testing.T, databasePath string, child evidenceChildSpec) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^"+child.testName+"$", "-test.count=1", "-test.timeout=30s")
	command.Env = evidenceChildEnvironment("owner", databasePath, child)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("create owner stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start exclusive owner child: %v", err)
	}
	ready := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == childReadyMarker {
				ready <- nil
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ready <- err
			return
		}
		ready <- fmt.Errorf("owner exited before readiness marker; stderr=%s", boundedText(stderr.String()))
	}()
	select {
	case err := <-ready:
		if err != nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
			t.Fatalf("exclusive owner readiness: %v", err)
		}
	case <-time.After(10 * time.Second):
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		t.Fatalf("exclusive owner readiness timed out; stderr=%s", boundedText(stderr.String()))
	}
	return command
}

func mustRunChild(t *testing.T, role, databasePath, wantMarker string, child evidenceChildSpec) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+child.testName+"$", "-test.count=1", "-test.timeout=12s")
	command.Env = evidenceChildEnvironment(role, databasePath, child)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s child failed: %v output=%s", role, err, boundedText(string(output)))
	}
	if ctx.Err() != nil {
		t.Fatalf("%s child timed out: %v", role, ctx.Err())
	}
	if !strings.Contains(string(output), wantMarker) {
		t.Fatalf("%s child output missing %q: %s", role, wantMarker, boundedText(string(output)))
	}
	t.Logf("%s child result=%s", role, strings.TrimSpace(boundedText(string(output))))
}

func evidenceChildEnvironment(role, databasePath string, child evidenceChildSpec) []string {
	environment := make([]string, 0, len(os.Environ())+5)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, childRoleEnv+"=") || strings.HasPrefix(item, childDatabaseEnv+"=") || strings.HasPrefix(item, childFixtureRootEnv+"=") || strings.HasPrefix(item, childRunTokenEnv+"=") || strings.HasPrefix(item, childRootIdentityEnv+"=") {
			continue
		}
		environment = append(environment, item)
	}
	return append(environment,
		childRoleEnv+"="+role,
		childDatabaseEnv+"="+databasePath,
		childFixtureRootEnv+"="+child.fixtureRoot,
		childRunTokenEnv+"="+child.runToken,
		childRootIdentityEnv+"="+child.rootIdentity,
	)
}

func boundedText(value string) string {
	const limit = 4096
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "...[truncated]"
}

func childRequireContention(t *testing.T, ctx context.Context, databasePath string) {
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		if code, ok := sqlitePrimaryCode(err); ok && (code == 5 || code == 6) {
			fmt.Printf("same_database_busy_or_locked code=%d\n", code)
			return
		}
		t.Fatalf("same-database open returned non-contention error: %v", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		if code, ok := sqlitePrimaryCode(err); ok && (code == 5 || code == 6) {
			fmt.Printf("same_database_busy_or_locked code=%d\n", code)
			return
		}
		t.Fatalf("same-database transaction returned non-contention error: %v", err)
	}
	_ = tx.Rollback()
	t.Fatal("same-database contender acquired an exclusive transaction while owner was live")
}

func childUseDifferentDatabase(t *testing.T, ctx context.Context, databasePath string) {
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		t.Fatalf("open different database: %v", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin different-database transaction: %v", err)
	}
	if err := requireQuickCheck(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("different-database quick_check: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit different-database transaction: %v", err)
	}
	fmt.Println("different_database_succeeded")
}

func childHoldExclusiveOwner(t *testing.T, ctx context.Context, databasePath string, requireJournal func(string) error) {
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		t.Fatalf("owner open: %v", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	revision2 := selectedRow(2, payloadRevision2)
	revision3 := selectedRow(3, payloadRevision3)
	current, err := readAggregateRow(ctx, database.conn)
	if err != nil || !sameRow(current, revision2) {
		t.Fatalf("owner pre-state mismatch: revision=%d err=%v", current.Revision, err)
	}
	tx, err := database.conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("owner begin exclusive: %v", err)
	}
	result, err := tx.ExecContext(ctx,
		"UPDATE app_server_aggregate SET revision=?, canonical_json=?, canonical_sha256=? WHERE singleton=? AND pipeon_session_id=? AND revision=? AND canonical_sha256=?",
		revision3.Revision, revision3.Payload, revision3.SHA256,
		revision2.Singleton, revision2.SessionID, revision2.Revision, revision2.SHA256,
	)
	if err != nil {
		t.Fatalf("owner stage CAS: %v", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		t.Fatalf("owner CAS affected=%d err=%v", affected, err)
	}
	if err := requireJournal(databasePath + "-journal"); err != nil {
		t.Fatalf("owner live journal: %v", err)
	}
	fmt.Println(childReadyMarker)
	for {
		time.Sleep(time.Second)
	}
}

func childRecover(t *testing.T, ctx context.Context, databasePath string, strictOld bool) {
	database, err := openEvidenceDatabase(ctx, databasePath)
	if err != nil {
		t.Fatalf("recovery open: %v", err)
	}
	defer database.db.Close()
	defer database.conn.Close()
	if err := requireQuickCheck(ctx, database.conn); err != nil {
		t.Fatalf("recovery quick_check: %v", err)
	}
	if err := requireSchemaObjectCount(ctx, database.conn, 1); err != nil {
		t.Fatalf("recovery schema: %v", err)
	}
	row, err := readAggregateRow(ctx, database.conn)
	if err != nil {
		t.Fatalf("recovery reload: %v", err)
	}
	if strictOld && !sameRow(row, selectedRow(2, payloadRevision2)) {
		t.Fatalf("recovery row is not the exact old canonical row: revision=%d session=%q payload=%x sha=%x", row.Revision, row.SessionID, row.Payload, row.SHA256)
	}
	if !strictOld && !sameRow(row, selectedRow(2, payloadRevision2)) && !sameRow(row, selectedRow(3, payloadRevision3)) {
		t.Fatalf("recovery row is not allowlisted: revision=%d session=%q payload=%x sha=%x", row.Revision, row.SessionID, row.Payload, row.SHA256)
	}
	fmt.Printf("recovery_succeeded revision=%d\n", row.Revision)
}

func requireQuickCheck(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) error {
	rows, err := queryer.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return err
		}
		count++
		if result != "ok" {
			return fmt.Errorf("quick_check = %q", result)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("quick_check row count = %d, want 1", count)
	}
	return nil
}

func sqlitePrimaryCode(err error) (int, bool) {
	var sqliteError *sqlite.Error
	if !errors.As(err, &sqliteError) {
		return 0, false
	}
	return sqliteError.Code() & 0xff, true
}
