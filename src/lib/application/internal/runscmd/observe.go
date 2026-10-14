package runscmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/operationrecord"
)

func observe(args []string) error {
	workdir := os.Getenv("DOCKPIPE_WORKDIR")
	system, resources := false, false
	container := ""
	execution := ""
	global := false
	explicitWorkdir := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--workdir":
			index++
			if index == len(args) {
				return fmt.Errorf("--workdir requires a path")
			}
			workdir = args[index]
			explicitWorkdir = true
		case "--execution":
			index++
			if index == len(args) {
				return fmt.Errorf("--execution requires an existing ID")
			}
			execution = args[index]
		case "--container":
			index++
			if index == len(args) {
				return fmt.Errorf("--container requires an associated resource ID")
			}
			container = args[index]
		case "--global":
			global = true
		case "--system":
			system = true
		case "--resources":
			resources = true
		case "--json": // Snapshots always use the reusable JSON contract.
		default:
			return fmt.Errorf("unexpected observability argument %q", args[index])
		}
	}
	if global && explicitWorkdir {
		return fmt.Errorf("--global and --workdir are mutually exclusive")
	}
	if global {
		workdir = ""
	}
	if workdir == "" && !global {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	monitorContext, stopMonitoring := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopMonitoring()
	ctx, cancel := context.WithTimeout(monitorContext, 3*time.Second)
	defer cancel()
	snapshot, err := infrastructure.ReadExecutionSnapshot(ctx, workdir, system, resources)
	if err != nil {
		return err
	}
	if !system && execution != "" {
		for index := range snapshot.Executions {
			record := &snapshot.Executions[index]
			if record.ID != execution || record.Log == "" {
				continue
			}
			events, readErr := operationrecord.ReadOperationSummaries(record.Log)
			if readErr != nil {
				record.LogNotice = "Operation log unavailable."
			} else {
				record.OperationEvents = events
				record.LogNotice = "Shared workflow operation log; free-form output is omitted."
			}
		}
	}
	if resources && !system && container != "" {
		infrastructure.CollectExecutionContainer(ctx, &snapshot, container)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(snapshot)
}

// associate is the runtime-facing sidecar API; it cannot launch or stop a process.
func associate(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("runs associate requires runtime, resource ID and PID")
	}
	pid, err := strconv.Atoi(args[2])
	if err != nil {
		return fmt.Errorf("invalid resource PID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return infrastructure.RegisterExecutionResource(ctx, os.Getenv("DOCKPIPE_WORKDIR"), os.Getenv("DOCKPIPE_RUN_ID"), args[0], args[1], pid)
}
