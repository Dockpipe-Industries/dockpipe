package pipelang

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dockpipe/tests/containedexec"
)

func TestGeneratedBundleSealedSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux sealed bundle prototype")
	}
	path := filepath.Join(t.TempDir(), "original")
	original := []byte("verified native bytes")
	if err := os.WriteFile(path, original, 0500); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(original)
	sealed, err := sealGeneratedExecutable(path, hex.EncodeToString(hash[:]))
	if err != nil {
		t.Fatal(err)
	}
	defer sealed.Close()
	if _, err := sealed.WriteAt([]byte("modified"), 0); err == nil {
		t.Fatal("sealed bytes were writable")
	}
	if err := sealed.Truncate(0); err == nil {
		t.Fatal("sealed executable could shrink")
	}
	if err := sealed.Truncate(999); err == nil {
		t.Fatal("sealed executable could grow")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0500); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(sealed)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("cache path replacement changed sealed snapshot")
	}
	if bad, err := sealGeneratedExecutable(path, hex.EncodeToString(hash[:])); err == nil {
		bad.Close()
		t.Fatal("wrong binary digest was accepted")
	}
}

func TestGeneratedBundleCurrentOracles(t *testing.T) {
	testGeneratedBundleCurrentOracles(t, false, false)
}

func TestGeneratedOrdinaryBundleCurrentOracles(t *testing.T) {
	testGeneratedBundleCurrentOracles(t, true, false)
}

func TestGeneratedBinaryBundleCurrentOracles(t *testing.T) {
	testGeneratedBundleCurrentOracles(t, false, true)
}

func testGeneratedBundleCurrentOracles(t *testing.T, ordinary, binaryFixture bool) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux sealed bundle prototype")
	}
	if mode := os.Getenv("PIPELANG_BUNDLE_PROBE"); mode != "" {
		mode = strings.TrimPrefix(mode, "ordinary-")
		supportSource := finiteSharedOracle
		if binaryFixture {
			supportSource = finiteBinaryOracle
		}
		oracle := strings.Replace(supportSource, "t.Helper()", `t.Helper(); invocations++; if invocations > 2 { t.Fatal("shared process state leaked") }`, 1) + "\nvar invocations int\n"
		if mode == "support" {
			oracle += "\n// changed shared checking code\n"
		}
		for i := 0; i < 16; i++ {
			expectedValue := fmt.Sprint(i)
			expectedTrace := []string{"trace" + fmt.Sprint(i)}
			if binaryFixture {
				expectedValue = "\x00☃\n" + expectedValue
				switch i {
				case 0:
					expectedTrace = nil
				case 1:
					expectedTrace = []string{}
				case 2:
					expectedTrace = []string{"", "\x00☃\n", "last"}
				}
			}
			traceSource := "[]string{" + quotedStrings(expectedTrace) + "}"
			if expectedTrace == nil {
				traceSource = "nil"
			}
			source := fmt.Sprintf("package generated; var calls, traceCalls int; func Value() string { calls++; if calls != 1 { panic(\"state leaked\") }; return %q }; func Trace() []string { traceCalls++; if traceCalls != 1 { panic(\"trace state leaked\") }; return %s }", expectedValue, traceSource)
			if mode == "source" && i == 0 {
				source += "; const changed = true"
			}
			checks := []byte("package generated; import (\"testing\";\"pipelang-generated-check/oracle\"); func TestCurrent(t *testing.T) { oracle.Values(t,\"oracle.json\",1,func(int)string{return Value()}); oracle.Traces(t,\"oracle.json\",1,func(int)[]string{return Trace()}) }")
			value := expectedValue
			if mode == "oracle" && i == 15 {
				value = "wrong"
			}
			trace := "trace" + fmt.Sprint(i)
			if mode == "trace" && i == 15 {
				trace = "wrong-trace"
			}
			fixture := []byte(fmt.Sprintf("[{\"Value\":%q,\"Trace\":[%q]}]", value, trace))
			if binaryFixture {
				if mode == "trace" && i == 15 {
					expectedTrace = []string{trace}
				}
				fixture = encodeFiniteBinaryOracle([]finiteConditionalOracleCase{{Value: value, Trace: expectedTrace}})
				if i == 15 {
					switch mode {
					case "truncated":
						fixture = fixture[:len(fixture)-1]
					case "trailing":
						fixture = append(fixture, 0)
					case "count":
						fixture[0] = 2
					case "length":
						fixture[4], fixture[5], fixture[6], fixture[7] = 255, 255, 255, 255
					}
				}
			}
			var support = []byte(oracle)
			if ordinary {
				// The same regression covers ordinary packages: the assertion is
				// local, while the fixture and process must still be fresh on hits.
				checks = []byte(`package generated; import ("testing"; "os"; "encoding/json")
func TestCurrent(t *testing.T) {
 data,err:=os.ReadFile("oracle.json"); if err!=nil {t.Fatal(err)}
 var wants []struct{Value string}; if err:=json.Unmarshal(data,&wants);err!=nil {t.Fatal(err)}
 if len(wants)!=1 {t.Fatal("fixture count")}; if got:=Value();got!=wants[0].Value {t.Fatalf("got %q want %q",got,wants[0].Value)}
 if err:=os.WriteFile("oracle.json",[]byte("changed by child"),0600);err!=nil {t.Fatal(err)}
}`)
				support = nil
			}
			if !queueGeneratedBatchWithOracle(t, []byte(source), checks, map[string][]byte{"oracle.json": fixture}, support) {
				t.Fatal("bundle rejected")
			}
		}
		return
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	buildCache := t.TempDir()
	if err := os.Chmod(buildCache, 0700); err != nil {
		t.Fatal(err)
	}
	prepare := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-p=1", "testing", "encoding/json", "reflect", "strings")
	prepare.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=", "GOCACHE="+buildCache, "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off")
	if output, _, err := measureGeneratedBuild(prepare); err != nil {
		t.Fatalf("prepare probe dependencies: %v\n%s", err, output)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		mode      string
		hit, fail bool
	}{{"initial", false, false}, {"repeat", true, false}, {"oracle", true, true}, {"trace", true, true}, {"source", false, false}, {"support", false, false}, {"ordinary-initial", false, false}, {"ordinary-repeat", true, false}, {"ordinary-oracle", true, true}, {"ordinary-source", false, false}, {"truncated", true, true}, {"trailing", true, true}, {"count", true, true}, {"length", true, true}}
	for _, tc := range cases {
		malformed := tc.mode == "truncated" || tc.mode == "trailing" || tc.mode == "count" || tc.mode == "length"
		if malformed && !binaryFixture {
			continue
		}
		if strings.HasPrefix(tc.mode, "ordinary-") != ordinary {
			continue
		}
		cmd := exec.Command(executable, "-test.run", "^"+t.Name()+"$", "-test.count=1", "-test.v", "-test.timeout=25s")
		cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=", "PIPELANG_NATIVE_BUNDLE=1", "PIPELANG_GENERATED_BATCH=1", "PIPELANG_COMPILED_CACHE="+root, "PIPELANG_BUNDLE_PROBE="+tc.mode, "PIPELANG_BUNDLE_BUILD_CACHE="+buildCache)
		output, _, err := containedexec.Measure(cmd)
		if (err != nil) != tc.fail {
			t.Fatalf("%s: %v\n%s", tc.mode, err, output)
		}
		if !strings.Contains(string(output), fmt.Sprintf("packages=16 cache_hit=%t", tc.hit)) {
			t.Fatalf("%s did not use expected bundle:\n%s", tc.mode, output)
		}
		failure := "want \"wrong\""
		if tc.mode == "trace" {
			failure = "want [wrong-trace]"
		}
		if malformed {
			failure = "oracle "
		}
		if tc.fail && !strings.Contains(string(output), failure) {
			t.Fatalf("unexpected oracle failure: %s", output)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			count++
		}
	}
	want := 3
	if ordinary {
		want = 2
	}
	if count != want {
		t.Fatalf("got %d retained bundles; want %d build identities", count, want)
	}
}
