//go:build linux

package sqliteevidence

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const linuxEvidenceOptInEnv = "DORKPIPE_SQLITE_LINUX_EVIDENCE"

var linuxExpectedCompileOptions = []string{
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

func TestLinuxNativeSQLiteSmoke(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("Linux/amd64 native evidence only")
	}
	if !goVersionAtLeast(runtime.Version(), 1, 25) {
		t.Fatalf("native evidence requires Go 1.25 or later; got %s", runtime.Version())
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

	var qualification *linuxQualification
	runNativeSQLiteSmoke(t, nativeSmokePlatform{
		name:          "Linux",
		optInEnv:      linuxEvidenceOptInEnv,
		childTestName: "TestLinuxSQLiteEvidenceChild",
		qualifyRoot: func(root string) (string, error) {
			var evidence string
			qualification, evidence, err = qualifyLinuxFixtureRoot(root)
			return evidence, err
		},
		prepareFile: func(t *testing.T, _ string, name string) string {
			t.Helper()
			path, err := qualification.prepareEvidenceFile(name)
			if err != nil {
				t.Fatalf("prepare Linux %s evidence file: %v", name, err)
			}
			return path
		},
		requireJournal: func(t *testing.T, databasePath string) {
			t.Helper()
			if err := qualification.requireJournal(databasePath + "-journal"); err != nil {
				t.Fatalf("require Linux journal: %v", err)
			}
		},
		inspectJournalAfterCommit: func(t *testing.T, databasePath string) {
			t.Helper()
			journalPath := databasePath + "-journal"
			fact, err := qualification.requirePath(journalPath, "file", 0o600)
			if err != nil {
				t.Fatalf("inspect Linux post-commit journal metadata: %v", err)
			}
			t.Logf("journal after_commit=present size=%d mode=%#o owner=%d:%d mount_id=%d device=%d:%d contents_opened_or_hashed=false", fact.Size, fact.Mode, fact.UID, fact.GID, fact.MountID, fact.DeviceMajor, fact.DeviceMinor)
		},
		requireSiblings: func(t *testing.T, directory string, requireJournal bool) {
			t.Helper()
			if err := qualification.requireSiblings(directory, requireJournal); err != nil {
				t.Fatalf("require Linux database siblings: %v", err)
			}
		},
		stableTreeHash: func(_ string) (string, error) {
			return qualification.stableTreeHash()
		},
		rootIdentity: func(_ string) (string, error) {
			return qualification.rootIdentity()
		},
		validateCompileOptions:   requireLinuxCompileOptions,
		cleanCommitAfterRecovery: true,
	})
}

func TestLinuxSQLiteEvidenceChild(t *testing.T) {
	root := filepath.Clean(os.Getenv(childFixtureRootEnv))
	if os.Getenv(childRoleEnv) == "" {
		t.Skip("Linux child-process helper")
	}
	qualification, _, err := qualifyLinuxFixtureRoot(root)
	if err != nil {
		t.Fatalf("qualify Linux child fixture: %v", err)
	}
	runSQLiteEvidenceChild(t, "linux", qualification.validateChildPath, qualification.requireJournal, true)
}

func requireLinuxCompileOptions(got []string) error {
	if len(got) != 56 || len(linuxExpectedCompileOptions) != 56 || strings.Join(got, "\n") != strings.Join(linuxExpectedCompileOptions, "\n") {
		return fmt.Errorf("compile-option set mismatch: got[%d]=%s", len(got), strings.Join(got, ","))
	}
	return nil
}

func requireLinuxModuleGraph() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("read working directory: %w", err)
	}
	moduleRoot := filepath.Clean(filepath.Join(workingDirectory, "..", ".."))
	modPayload, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	sumPayload, err := os.ReadFile(filepath.Join(moduleRoot, "go.sum"))
	if err != nil {
		return "", fmt.Errorf("read go.sum: %w", err)
	}
	modText := string(modPayload)
	sumText := string(sumPayload)
	for _, required := range []string{"\ngo 1.25\n", "\n\tgolang.org/x/sys v0.47.0\n", "\n\tmodernc.org/sqlite v1.56.0\n", "\n\tmodernc.org/libc v1.74.4 // indirect\n"} {
		if !strings.Contains(modText, required) {
			return "", fmt.Errorf("go.mod missing exact selected line %q", strings.TrimSpace(required))
		}
	}
	for _, required := range []string{
		"golang.org/x/sys v0.47.0 h1:o7XGOvZQCADBQQ4Y7VNq2dRWQR7JmOUW8Kxx4ZsNgWs=",
		"modernc.org/libc v1.74.4 h1:fX1Omw4o2/1C2iRkkIsrQTasJQldLhRmuPreXLoWs9k=",
		"modernc.org/sqlite v1.56.0 h1:/D8e2RfFqoy/Zc6PuC76U28zFwmI/sYx1Kjm4yEn9e0=",
	} {
		if !strings.Contains(sumText, required+"\n") {
			return "", fmt.Errorf("go.sum missing exact selected checksum for %q", strings.Fields(required)[0]+" "+strings.Fields(required)[1])
		}
	}
	modDigest := sha256.Sum256(modPayload)
	sumDigest := sha256.Sum256(sumPayload)
	return fmt.Sprintf("language=go1.25 selected=[golang.org/x/sys@v0.47.0,modernc.org/libc@v1.74.4,modernc.org/sqlite@v1.56.0] go_mod_sha256=%x go_sum_sha256=%x", modDigest, sumDigest), nil
}

func goVersionAtLeast(version string, wantMajor, wantMinor int) bool {
	version = strings.TrimPrefix(version, "go")
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minorDigits := strings.TrimLeftFunc(parts[1], func(character rune) bool { return character < '0' || character > '9' })
	minorEnd := 0
	for minorEnd < len(minorDigits) && minorDigits[minorEnd] >= '0' && minorDigits[minorEnd] <= '9' {
		minorEnd++
	}
	if minorEnd == 0 {
		return false
	}
	minor, err := strconv.Atoi(minorDigits[:minorEnd])
	return err == nil && (major > wantMajor || major == wantMajor && minor >= wantMinor)
}
