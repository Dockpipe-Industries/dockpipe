//go:build windows

package remotecmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

const windowsTaskNamespace = "http://schemas.microsoft.com/windows/2004/02/mit/task"
const windowsTaskOwnerPath = "/Task/Principals/Principal/UserId"
const windowsTaskTriggerUserPath = "/Task/Triggers/LogonTrigger/UserId"
const maxWindowsTaskXML = 32 << 10

var windowsTaskNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

func prepareWindowsWorkerTask(ctx context.Context, name string, arguments []string, fingerprint string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	owner, err := windowsWorkerTaskOwner()
	if err != nil {
		return "", err
	}
	return buildWindowsWorkerTask(owner, name, arguments, fingerprint)
}

func windowsWorkerTaskOwner() (string, error) {
	sid, err := windowsWorkerTaskSID()
	if err != nil {
		return "", err
	}
	return sid.String(), nil
}

func windowsWorkerTaskSID() (*windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read Windows worker owner SID: %w", err)
	}
	return user.User.Sid, nil
}

func buildWindowsWorkerTask(owner, name string, arguments []string, fingerprint string) (string, error) {
	if !windowsTaskNamePattern.MatchString(name) {
		return "", errors.New("invalid Windows worker task name")
	}
	digest, err := hex.DecodeString(fingerprint)
	if err != nil || len(digest) != 32 {
		return "", errors.New("invalid Windows worker executable fingerprint")
	}
	if len(arguments) < 2 || !filepath.IsAbs(arguments[0]) {
		return "", errors.New("Windows worker task requires an absolute executable and arguments")
	}

	escaped := make([]string, len(arguments)-1)
	for index, argument := range arguments {
		// The scheduler expands these expressions in Exec actions. The reviewed
		// worker invocation must receive the literal executable and state path.
		if strings.Count(argument, "%") > 1 || strings.Contains(argument, "$(") || strings.ContainsAny(argument, "\x00\r\n") {
			return "", errors.New("Windows worker task argument contains a scheduler substitution or control character")
		}
		if index > 0 {
			escaped[index-1] = windows.EscapeArg(argument)
		}
	}

	escape := func(value string) string {
		var output bytes.Buffer
		_ = xml.EscapeText(&output, []byte(value))
		return output.String()
	}
	definition := fmt.Sprintf(`<Task version="1.3" xmlns="%s">
  <RegistrationInfo>
    <Description>Dockpipe executable sha256: %s</Description>
    <URI>\%s</URI>
  </RegistrationInfo>
  <Triggers>
    <RegistrationTrigger>
      <Enabled>true</Enabled>
      <Repetition>
        <Interval>PT1M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
      <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    </RegistrationTrigger>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <Repetition>
        <Interval>PT5M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
      <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
      <UserId>%s</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Worker">
      <UserId>%s</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <UseUnifiedSchedulingEngine>true</UseUnifiedSchedulingEngine>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Worker">
    <Exec>
      <Command>%s</Command>
      <Arguments>%s</Arguments>
    </Exec>
  </Actions>
</Task>`, windowsTaskNamespace, hex.EncodeToString(digest), name, escape(owner), escape(owner),
		escape(arguments[0]), escape(strings.Join(escaped, " ")))
	if len(definition) > maxWindowsTaskXML {
		return "", errors.New("Windows worker task definition exceeds size limit")
	}
	return definition, nil
}

func readWindowsWorkerTask(ctx context.Context, name, expected string) (exists bool, matches bool, err error) {
	if !windowsTaskNamePattern.MatchString(name) {
		return false, false, errors.New("invalid Windows worker task name")
	}
	response, err := runWindowsTaskScheduler(ctx, windowsTaskRequest{Action: "read", Name: name})
	if err != nil || !response.Exists {
		return false, false, err
	}
	matches, err = compareWindowsTask(response.XML, expected)
	return true, matches, err
}

func activateWindowsWorkerTask(ctx context.Context, name, expected string, replace bool) error {
	if !windowsTaskNamePattern.MatchString(name) {
		return errors.New("invalid Windows worker task name")
	}
	owner, err := windowsWorkerTaskOwner()
	if err != nil {
		return err
	}
	definition, err := windowsTaskSemantics(expected)
	if err != nil {
		return err
	}
	if definition[windowsTaskOwnerPath] != owner || definition["/Task/RegistrationInfo/URI"] != "\\"+name {
		return errors.New("Windows worker task definition does not match its current owner or registration name")
	}

	current, err := runWindowsTaskScheduler(ctx, windowsTaskRequest{Action: "read", Name: name})
	if err != nil {
		return err
	}
	var matches bool
	if current.Exists {
		currentDefinition, err := windowsTaskSemantics(current.XML)
		if err != nil {
			return err
		}
		if currentDefinition[windowsTaskOwnerPath] != owner {
			return errors.New("existing Windows worker task is not bound to the current owner SID")
		}
		matches, err = matchWindowsTaskDefinitions(currentDefinition, definition, windowsWorkerTaskAccount)
		if err != nil {
			return err
		}
		if !matches && !replace {
			return errors.New("existing Windows worker task differs; use remote service --role worker --replace")
		}
	}
	armRegistration := !matches || replace
	if !matches {
		current, err = runWindowsTaskScheduler(ctx, windowsTaskRequest{
			Action: "register", Name: name, Owner: owner, XML: expected, Current: current.XML,
		})
		if err != nil {
			return err
		}
		matches, err = compareWindowsTask(current.XML, expected)
		if err != nil {
			return err
		}
		if !current.Exists || !matches {
			return errors.New("registered Windows worker task differs from its expected definition")
		}
	}

	start := windowsTaskRequest{
		Action: "start", Name: name, Current: current.XML, Restart: replace,
	}
	if armRegistration {
		start.ArmRegistration = true
		start.Owner = owner
		start.XML = expected
	}
	started, err := runWindowsTaskScheduler(ctx, start)
	if err != nil || !armRegistration {
		return err
	}
	if !started.Exists {
		return errors.New("armed Windows worker task is missing")
	}
	matches, err = compareWindowsTask(started.XML, expected)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("armed Windows worker task differs from its expected definition")
	}
	return nil
}

type windowsTaskElement struct {
	XMLName    xml.Name
	Attributes []xml.Attr           `xml:",any,attr"`
	Value      string               `xml:",chardata"`
	Children   []windowsTaskElement `xml:",any"`
}

func compareWindowsTask(actual, expected string) (bool, error) {
	found, err := windowsTaskSemantics(actual)
	if err != nil {
		return false, err
	}
	want, err := windowsTaskSemantics(expected)
	if err != nil {
		return false, err
	}
	return matchWindowsTaskDefinitions(found, want, windowsWorkerTaskAccount)
}

func windowsWorkerTaskAccount() (owner, qualifiedAccount string, err error) {
	sid, err := windowsWorkerTaskSID()
	if err != nil {
		return "", "", err
	}
	account, domain, accountType, err := sid.LookupAccount("")
	if err != nil {
		return "", "", fmt.Errorf("resolve current Windows worker account: %w", err)
	}
	if accountType != windows.SidTypeUser || account == "" || domain == "" {
		return "", "", errors.New("current Windows worker SID has no qualified user account")
	}
	return sid.String(), domain + "\\" + account, nil
}

func matchWindowsTaskDefinitions(found, want map[string]string, currentAccount func() (string, string, error)) (bool, error) {
	if maps.Equal(found, want) {
		return true, nil
	}

	// Scheduler can persist the logon trigger as DOMAIN\User while retaining
	// the SID on the principal. Only the current token's own account is an alias.
	triggerUser := found[windowsTaskTriggerUserPath]
	domain, account, qualified := strings.Cut(triggerUser, "\\")
	wantedOwner := want[windowsTaskOwnerPath]
	if !qualified || domain == "" || account == "" || wantedOwner == "" ||
		found[windowsTaskOwnerPath] != wantedOwner || want[windowsTaskTriggerUserPath] != wantedOwner {
		return false, nil
	}

	owner, qualifiedAccount, err := currentAccount()
	if err != nil {
		return false, err
	}
	if owner != wantedOwner || !strings.EqualFold(triggerUser, qualifiedAccount) {
		return false, nil
	}

	canonical := maps.Clone(found)
	canonical[windowsTaskTriggerUserPath] = owner
	return maps.Equal(canonical, want), nil
}

func windowsTaskSemantics(source string) (map[string]string, error) {
	if len(source) == 0 || len(source) > maxWindowsTaskXML {
		return nil, errors.New("invalid Windows worker task XML size")
	}
	decoder := xml.NewDecoder(strings.NewReader(strings.TrimPrefix(source, "\ufeff")))
	// Task Scheduler returns a Unicode BSTR. Its XML declaration can retain
	// UTF-16 although PowerShell sends it to this process as UTF-8.
	decoder.CharsetReader = windowsTaskXMLCharsetReader
	var root windowsTaskElement
	if err := decoder.Decode(&root); err != nil || root.XMLName != (xml.Name{Space: windowsTaskNamespace, Local: "Task"}) {
		return nil, errors.New("invalid Windows worker task XML")
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		switch value := token.(type) {
		case xml.Comment:
			continue
		case xml.CharData:
			if strings.TrimSpace(string(value)) == "" {
				continue
			}
		}
		return nil, errors.New("unexpected data after Windows worker task XML")
	}

	semantics := make(map[string]string)
	seen := make(map[string]bool)
	if err := recordWindowsTaskElement(root, "", semantics, seen); err != nil {
		return nil, err
	}
	return semantics, nil
}

func windowsTaskXMLCharsetReader(charset string, body io.Reader) (io.Reader, error) {
	if strings.EqualFold(charset, "UTF-16") || strings.EqualFold(charset, "UTF-16LE") {
		return body, nil
	}
	return nil, errors.New("unexpected Windows worker task XML encoding")
}

// Optional values that the scheduler may insert or omit are compared using
// scheduler defaults. Other elements, principals, triggers and actions must
// still match, even when the readback carries the same executable fingerprint.
var windowsTaskDefaults = map[string]string{
	"/Task/Principals/Principal/RunLevel":                             "LeastPrivilege",
	"/Task/Settings/AllowStartOnDemand":                               "true",
	"/Task/Settings/AllowHardTerminate":                               "true",
	"/Task/Settings/MultipleInstancesPolicy":                          "IgnoreNew",
	"/Task/Settings/DisallowStartIfOnBatteries":                       "true",
	"/Task/Settings/StopIfGoingOnBatteries":                           "true",
	"/Task/Settings/StartWhenAvailable":                               "false",
	"/Task/Settings/RunOnlyIfIdle":                                    "false",
	"/Task/Settings/RunOnlyIfNetworkAvailable":                        "false",
	"/Task/Settings/WakeToRun":                                        "false",
	"/Task/Settings/Enabled":                                          "true",
	"/Task/Settings/Hidden":                                           "false",
	"/Task/Settings/Priority":                                         "7",
	"/Task/Settings/UseUnifiedSchedulingEngine":                       "false",
	"/Task/Settings/DisallowStartOnRemoteAppSession":                  "false",
	"/Task/Settings/DeleteExpiredTaskAfter":                           "0s",
	"/Task/Triggers/LogonTrigger/Enabled":                             "true",
	"/Task/Triggers/LogonTrigger/Repetition/StopAtDurationEnd":        "false",
	"/Task/Triggers/LogonTrigger/Delay":                               "0s",
	"/Task/Triggers/RegistrationTrigger/Enabled":                      "true",
	"/Task/Triggers/RegistrationTrigger/Repetition/StopAtDurationEnd": "false",
	"/Task/Triggers/RegistrationTrigger/Delay":                        "0s",
}

func recordWindowsTaskElement(element windowsTaskElement, parent string, semantics map[string]string, seen map[string]bool) error {
	key := parent + "/" + element.XMLName.Local
	if element.XMLName.Space != windowsTaskNamespace || strings.Count(key, "/") > 12 {
		return errors.New("Windows worker task has an unexpected namespace or element depth")
	}
	if seen[key] {
		if key == "/Task/Principals/Principal" || key == windowsTaskOwnerPath {
			return errors.New("Windows worker task has an ambiguous owner")
		}
		semantics[key+"#multiple"] = "true"
		return nil
	}
	seen[key] = true

	switch key {
	case "/Task/RegistrationInfo/Date", "/Task/RegistrationInfo/Author",
		"/Task/RegistrationInfo/Version", "/Task/RegistrationInfo/Source",
		"/Task/RegistrationInfo/Documentation", "/Task/Principals/Principal/DisplayName":
		return nil
	case "/Task/Settings/IdleSettings":
		// IdleSettings has no effect when RunOnlyIfIdle is false; that setting
		// is still verified separately.
		return nil
	}

	value := strings.TrimSpace(element.Value)
	switch key {
	case "/Task/Actions/Exec/Command":
		value = strings.ToLower(filepath.Clean(value))
	case "/Task/Actions/Exec/Arguments":
		arguments, err := windows.DecomposeCommandLine("dockpipe.exe " + value)
		if err != nil {
			return errors.New("invalid Windows worker task argument quoting")
		}
		encoded, err := json.Marshal(arguments[1:])
		if err != nil {
			return err
		}
		value = string(encoded)
	case "/Task/Triggers/LogonTrigger/Delay", "/Task/Triggers/LogonTrigger/ExecutionTimeLimit",
		"/Task/Triggers/LogonTrigger/Repetition/Interval", "/Task/Settings/ExecutionTimeLimit",
		"/Task/Triggers/RegistrationTrigger/Delay", "/Task/Triggers/RegistrationTrigger/ExecutionTimeLimit",
		"/Task/Triggers/RegistrationTrigger/Repetition/Interval",
		"/Task/Settings/RestartOnFailure/Interval", "/Task/Settings/DeleteExpiredTaskAfter":
		value = normalizeWindowsTaskDuration(value)
	}
	if defaultValue, known := windowsTaskDefaults[key]; known {
		if defaultValue == "true" || defaultValue == "false" {
			switch value {
			case "1":
				value = "true"
			case "0":
				value = "false"
			}
		}
		if value == defaultValue && len(element.Attributes) == 0 && len(element.Children) == 0 {
			return nil
		}
	}
	semantics[key] = value
	for _, attribute := range element.Attributes {
		if attribute.Name.Local == "xmlns" || attribute.Name.Space == "xmlns" ||
			(key == "/Task" && attribute.Name == (xml.Name{Local: "version"})) {
			continue
		}
		semantics[key+"/@"+attribute.Name.Space+":"+attribute.Name.Local] = attribute.Value
	}
	for _, child := range element.Children {
		if err := recordWindowsTaskElement(child, key, semantics, seen); err != nil {
			return err
		}
	}
	return nil
}

func normalizeWindowsTaskDuration(duration string) string {
	if tail, ok := strings.CutPrefix(duration, "PT"); ok {
		if value, err := time.ParseDuration(strings.ToLower(tail)); err == nil {
			return value.String()
		}
	}
	return duration
}

type windowsTaskRequest struct {
	Action          string `json:"action"`
	Name            string `json:"name"`
	Owner           string `json:"owner,omitempty"`
	XML             string `json:"xml,omitempty"`
	Current         string `json:"current,omitempty"`
	Restart         bool   `json:"restart,omitempty"`
	ArmRegistration bool   `json:"arm_registration,omitempty"`
}

type windowsTaskResponse struct {
	Exists bool   `json:"exists"`
	XML    string `json:"xml,omitempty"`
	Error  string `json:"error,omitempty"`
}

func runWindowsTaskScheduler(ctx context.Context, request windowsTaskRequest) (windowsTaskResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var response windowsTaskResponse
	if err := ctx.Err(); err != nil {
		return response, err
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return response, err
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return response, fmt.Errorf("find system Windows PowerShell: %w", err)
	}
	executable := filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")
	encoded := make([]byte, 0, len(windowsTaskSchedulerScript)*2)
	for _, unit := range utf16.Encode([]rune(windowsTaskSchedulerScript)) {
		encoded = binary.LittleEndian.AppendUint16(encoded, unit)
	}

	command := exec.CommandContext(ctx, executable, "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded))
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	command.Stdin = bytes.NewReader(requestJSON)
	command.WaitDelay = 2 * time.Second
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return response, ctx.Err()
		}
		return response, fmt.Errorf("run Windows Task Scheduler operation %s: %w", request.Action, err)
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return response, fmt.Errorf("invalid Windows Task Scheduler %s response", request.Action)
	}
	if response.Error != "" {
		return response, fmt.Errorf("Windows Task Scheduler %s: %s", request.Action, response.Error)
	}
	return response, nil
}

const windowsTaskSchedulerScript = `
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)

function Get-LocalTask($folder, $name) {
    try {
        return $folder.GetTask($name)
    } catch {
        for ($exception = $_.Exception; $null -ne $exception; $exception = $exception.InnerException) {
            if (('{0:X8}' -f $exception.HResult) -eq '80070002') {
                return $null
            }
        }
        throw
    }
}

function Read-TaskXML($task) {
    if ($null -eq $task) {
        return ''
    }
    $xml = [string]$task.Xml
    if ($xml.Length -eq 0 -or [Text.Encoding]::UTF8.GetByteCount($xml) -gt 32768) {
        throw [InvalidOperationException]::new('task XML exceeds its readback bound')
    }
    return $xml
}

try {
    $request = [Console]::In.ReadToEnd() | ConvertFrom-Json
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $folder = $service.GetFolder('\')
    $task = Get-LocalTask $folder $request.name
    $current = Read-TaskXML $task
    switch -Exact ($request.action) {
        'read' {
            $reply = @{exists = $null -ne $task; xml = $current}
        }
        'register' {
            if ($current -cne [string]$request.current) {
                $reply = @{error = 'task changed during registration; inspect it and retry'}
                break
            }
            # TASK_CREATE or TASK_UPDATE, never implicit create-or-update.
            $flags = if ($null -eq $task) { 2 } else { 4 }
            # 3 = TASK_LOGON_INTERACTIVE_TOKEN. No stored password.
            $null = $folder.RegisterTask($request.name, $request.xml, ($flags -bor 32), $request.owner, $null, 3, $null)
            $task = Get-LocalTask $folder $request.name
            $reply = @{exists = $null -ne $task; xml = (Read-TaskXML $task)}
        }
        'start' {
            if ($null -eq $task -or $current -cne [string]$request.current) {
                $reply = @{error = 'task changed before start; inspect it and retry'}
                break
            }
            if ($request.restart -and $task.State -in 2,4) {
                $task.Stop(0)
                for ($attempt = 0; $attempt -lt 25 -and $task.State -in 2,4; $attempt++) {
                    Start-Sleep -Milliseconds 80
                }
                if ($task.State -in 2,4) {
                    $reply = @{error = 'existing worker task did not stop for replacement'}
                    break
                }
            }
            if ($request.arm_registration) {
                # Arm recurring activation only after the suppressed registration
                # passed Go's semantic readback and its exact XML still matches.
                if ((Read-TaskXML $task) -cne [string]$request.current) {
                    $reply = @{error = 'task changed before arming; inspect it and retry'}
                    break
                }
                $task = $folder.RegisterTask($request.name, $request.xml, 4, $request.owner, $null, 3, $null)
                $reply = @{exists = $null -ne $task; xml = (Read-TaskXML $task)}
                break
            }
            # A demand start is idempotent for an existing queued/running task.
            if ($task.State -notin 2,4) {
                $null = $task.Run($null)
            }
            $reply = @{exists = $true}
        }
        default { $reply = @{error = 'invalid scheduler operation'} }
    }
} catch {
    $exception = $_.Exception
    while ($null -ne $exception.InnerException) { $exception = $exception.InnerException }
    $reply = @{error = ('scheduler operation failed (HRESULT 0x{0:X8})' -f $exception.HResult)}
}
[Console]::WriteLine(($reply | ConvertTo-Json -Compress))
`
