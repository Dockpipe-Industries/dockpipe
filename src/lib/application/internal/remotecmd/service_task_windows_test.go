//go:build windows

package remotecmd

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestWindowsWorkerTaskRoundTripsQuotedArgumentsAndOwner(t *testing.T) {
	const owner = "S-1-5-21-1-2-3-1001"
	arguments := []string{
		`C:\Program Files\Dockpipe & Tools\dockpipe-worker.exe`,
		"--cli", `C:\Program Files\Dockpipe & Tools\dockpipe.exe`,
		"--state", `C:\Private State\Bjørn\`,
	}
	definition, err := buildWindowsWorkerTask(owner, "com.dockpipe.remote.worker.test", arguments, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	task, err := windowsTaskSemantics(definition)
	if err != nil {
		t.Fatal(err)
	}

	if got := task[windowsTaskOwnerPath]; got != owner {
		t.Fatalf("principal owner = %q, want %q", got, owner)
	}
	if got := task["/Task/Triggers/LogonTrigger/UserId"]; got != owner {
		t.Fatalf("logon trigger owner = %q, want %q", got, owner)
	}
	if got := task["/Task/Triggers/RegistrationTrigger/Repetition/Interval"]; got != "1m0s" {
		t.Fatalf("registration repetition interval = %q, want 1m0s", got)
	}
	if got := task["/Task/Principals/Principal/LogonType"]; got != "InteractiveToken" {
		t.Fatalf("logon type = %q, want InteractiveToken", got)
	}
	if got := task["/Task/Settings/ExecutionTimeLimit"]; got != "0s" {
		t.Fatalf("execution time limit = %q, want 0s", got)
	}
	if got := task["/Task/Settings/RestartOnFailure/Count"]; got != "3" {
		t.Fatalf("restart count = %q, want 3", got)
	}
	if got := task["/Task/Actions/Exec/Command"]; !strings.EqualFold(got, arguments[0]) {
		t.Fatalf("registered executable = %q, want %q", got, arguments[0])
	}
	var parsed []string
	if err := json.Unmarshal([]byte(task["/Task/Actions/Exec/Arguments"]), &parsed); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(parsed, arguments[1:]) {
		t.Fatalf("registered arguments = %q, want %q", parsed, arguments[1:])
	}
}

func TestWindowsWorkerTaskSemanticReadback(t *testing.T) {
	const owner = "S-1-5-21-1-2-3-1001"
	definition, err := buildWindowsWorkerTask(owner, "com.dockpipe.remote.worker.test",
		[]string{`C:\Dockpipe\dockpipe.exe`, "remote", "worker", "--state", `C:\Private State`},
		strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	readback := strings.NewReplacer(
		"<RegistrationInfo>", "<RegistrationInfo><Author>Scheduler account</Author>",
		"<Enabled>true</Enabled>", "<Enabled>1</Enabled>",
		"<StopAtDurationEnd>false</StopAtDurationEnd>", "",
		"<Interval>PT5M</Interval>", "<Interval>PT300S</Interval>",
		"<Interval>PT1M</Interval>", "<Interval>PT60S</Interval>",
		"<Settings>", "<Settings><IdleSettings><StopOnIdleEnd>true</StopOnIdleEnd></IdleSettings><Hidden>false</Hidden>",
	).Replace(definition)
	readback = "<?xml version=\"1.0\" encoding=\"UTF-16\"?>\r\n" + readback
	if matches, err := compareWindowsTask(readback, definition); err != nil || !matches {
		t.Fatalf("equivalent scheduler XML did not match: matches=%t err=%v", matches, err)
	}

	for _, test := range []struct {
		name string
		from string
		to   string
	}{
		{"other owner", owner, "S-1-5-21-1-2-3-1002"},
		{"privilege escalation", "<RunLevel>LeastPrivilege</RunLevel>", "<RunLevel>HighestAvailable</RunLevel>"},
		{"password logon", "InteractiveToken", "Password"},
		{"automatic startup removed", "<LogonTrigger>", "<LogonTrigger><EndBoundary>2026-01-01T00:00:00</EndBoundary>"},
		{"registration repetition changed", "<Interval>PT60S</Interval>", "<Interval>PT2M</Interval>"},
		{"registration repetition limited", "<Interval>PT60S</Interval>", "<Interval>PT60S</Interval><Duration>PT10M</Duration>"},
		{"parallel runs", "<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>", "<MultipleInstancesPolicy>Parallel</MultipleInstancesPolicy>"},
		{"different scheduling engine", "<UseUnifiedSchedulingEngine>true</UseUnifiedSchedulingEngine>", "<UseUnifiedSchedulingEngine>false</UseUnifiedSchedulingEngine>"},
		{"execution duration omitted", "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>", ""},
		{"extra executable", "</Exec>", "</Exec><Exec><Command>C:\\Other.exe</Command></Exec>"},
		{"different fingerprint", strings.Repeat("b", 64), strings.Repeat("c", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			drift := strings.Replace(readback, test.from, test.to, 1)
			if drift == readback {
				t.Fatal("test did not edit the generated definition")
			}
			if matches, err := compareWindowsTask(drift, definition); err != nil || matches {
				t.Fatalf("semantic drift did not return a mismatch: matches=%t err=%v", matches, err)
			}
		})
	}
}

func TestWindowsWorkerTaskMatchesNativeLogonTriggerAccount(t *testing.T) {
	const owner = "S-1-5-21-1-2-3-1001"
	const qualifiedAccount = `TESTPC\ScheduleOwner`
	definition, err := buildWindowsWorkerTask(owner, "com.dockpipe.remote.worker.test",
		[]string{`C:\Dockpipe\dockpipe.exe`, "remote", "worker", "--state", `C:\Private State`},
		strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	want, err := windowsTaskSemantics(definition)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name         string
		triggerUser  string
		currentOwner string
		lookupError  error
		matches      bool
	}{
		{name: "native account form", triggerUser: `testpc\scheduleowner`, currentOwner: owner, matches: true},
		{name: "same username on another domain", triggerUser: `OTHER\ScheduleOwner`, currentOwner: owner},
		{name: "another local account", triggerUser: `TESTPC\AnotherUser`, currentOwner: owner},
		{name: "ambiguous unqualified username", triggerUser: "ScheduleOwner", currentOwner: owner},
		{name: "different current SID", triggerUser: qualifiedAccount, currentOwner: "S-1-5-21-1-2-3-1002"},
		{name: "unavailable account resolution", triggerUser: qualifiedAccount, lookupError: errors.New("cannot resolve account")},
	} {
		t.Run(test.name, func(t *testing.T) {
			readback := strings.Replace(definition, "<UserId>"+owner+"</UserId>", "<UserId>"+test.triggerUser+"</UserId>", 1)
			readback = strings.NewReplacer(
				"<RunLevel>LeastPrivilege</RunLevel>", "",
				"<RunOnlyIfIdle>false</RunOnlyIfIdle>", "<IdleSettings><StopOnIdleEnd>true</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings>",
				"<RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>", "",
			).Replace(readback)
			found, err := windowsTaskSemantics(readback)
			if err != nil {
				t.Fatal(err)
			}
			matches, err := matchWindowsTaskDefinitions(found, want, func() (string, string, error) {
				return test.currentOwner, qualifiedAccount, test.lookupError
			})
			if matches != test.matches || !errors.Is(err, test.lookupError) {
				t.Fatalf("native scheduler comparison: matches=%t err=%v", matches, err)
			}
		})
	}
}

func TestWindowsWorkerTaskRejectsSchedulerSubstitutions(t *testing.T) {
	for _, path := range []string{`C:\%TEMP%\state`, `C:\$(Arg0)\state`, "C:\\some\nstate"} {
		_, err := buildWindowsWorkerTask("S-1-5-21-1-2-3-1001", "test-worker",
			[]string{`C:\Dockpipe\dockpipe.exe`, "remote", "worker", "--state", path}, strings.Repeat("a", 64))
		if err == nil {
			t.Fatalf("scheduler interpolation was accepted in %q", path)
		}
	}
}
