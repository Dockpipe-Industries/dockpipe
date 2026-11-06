package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"text/template"
	"time"
)

type inventoryContainerFormatter struct {
	index           int
	rootOrSizeCalls *int
}

func (formatter inventoryContainerFormatter) ID() string {
	return fmt.Sprintf("%064x", formatter.index+1)
}

func (inventoryContainerFormatter) Names() string {
	return "application"
}

func (inventoryContainerFormatter) State() string {
	return "running"
}

func (inventoryContainerFormatter) Status() string {
	return "Up 1 hour"
}

func (inventoryContainerFormatter) Image() string {
	return "example/application:1"
}

func (inventoryContainerFormatter) Ports() string {
	return "127.0.0.1:8080->80/tcp"
}

func (inventoryContainerFormatter) RunningFor() string {
	return "1 hour ago"
}

func (formatter inventoryContainerFormatter) Size() string {
	(*formatter.rootOrSizeCalls)++
	return "expensive unused filesystem size"
}

func (formatter inventoryContainerFormatter) MarshalJSON() ([]byte, error) {
	(*formatter.rootOrSizeCalls)++
	return []byte(`{"unrequested":"SECRET"}`), nil
}

func renderInventoryFormat(t *testing.T, arguments []string, records []any, expectedFields int) []byte {
	t.Helper()
	format := ""
	for index, argument := range arguments {
		if argument == "--format" && index+1 < len(arguments) {
			format = arguments[index+1]
		}
	}
	if format == "" || strings.Contains(format, "{{json .}}") {
		t.Fatal("inventory must select explicit fields")
	}
	parsed, err := template.New("inventory").Funcs(template.FuncMap{
		"json": func(value any) (string, error) {
			data, err := json.Marshal(value)
			return string(data), err
		},
	}).Parse(format)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	for _, record := range records {
		var row bytes.Buffer
		if err := parsed.Execute(&row, record); err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if json.Unmarshal(row.Bytes(), &fields) != nil || len(fields) != expectedFields {
			t.Fatal("inventory format emitted the wrong field set")
		}
		output.Write(row.Bytes())
		output.WriteByte('\n')
	}
	return output.Bytes()
}

func TestDockerInventoryUsesLightweightAllowlistedFormats(t *testing.T) {
	rootOrSizeCalls := 0
	var records []any
	for index := 0; index < 13; index++ {
		records = append(records, inventoryContainerFormatter{index: index, rootOrSizeCalls: &rootOrSizeCalls})
	}
	queries := 0
	snapshot := collectDockerInventory(context.Background(), func(ctx context.Context, arguments ...string) ([]byte, error) {
		queries++
		deadline, exists := ctx.Deadline()
		if !exists || time.Until(deadline) > 3*time.Second {
			t.Fatal("inventory aggregate deadline missing")
		}
		switch arguments[0] + " " + arguments[1] {
		case "container ls":
			disabled := false
			for _, argument := range arguments {
				if strings.HasPrefix(argument, "--size") {
					if argument != "--size=false" || disabled {
						t.Fatal("container size collection is not explicitly disabled")
					}
					disabled = true
				}
			}
			if !disabled {
				t.Fatal("container size collection must be explicitly disabled")
			}
			return renderInventoryFormat(t, arguments, records, 7), nil
		case "network ls":
			return renderInventoryFormat(t, arguments, []any{map[string]string{"ID": "network-id", "Name": "application-network", "Driver": "bridge", "Scope": "local", "Labels": "SECRET"}}, 4), nil
		case "volume ls":
			return renderInventoryFormat(t, arguments, []any{map[string]string{"Name": "application-data", "Driver": "local", "Mountpoint": "SECRET"}}, 2), nil
		case "container inspect":
			return nil, nil
		default:
			t.Fatal("unexpected inventory operation")
			return nil, nil
		}
	})
	if !snapshot.Available || snapshot.Truncated || len(snapshot.Containers) != 13 || len(snapshot.Networks) != 1 || len(snapshot.Volumes) != 1 || queries != 4 || rootOrSizeCalls != 0 {
		t.Fatal("lightweight inventory did not retain complete metadata without root/size evaluation")
	}
	for index, container := range snapshot.Containers {
		if container.ID != fmt.Sprintf("%064x", index+1) || container.Names != "application" || container.State != "running" || container.Status != "Up 1 hour" || container.Image != "example/application:1" || container.Ports != "127.0.0.1:8080->80/tcp" || container.RunningFor != "1 hour ago" {
			t.Fatal("container metadata changed during lightweight projection")
		}
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || bytes.Contains(encoded, []byte("SECRET")) {
		t.Fatal("unrequested metadata escaped inventory")
	}
}

func TestDockerInventoryDeadlineCannotBecomeEmptySuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	queries := 0
	started := time.Now()
	snapshot := collectDockerInventory(ctx, func(ctx context.Context, _ ...string) ([]byte, error) {
		queries++
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if snapshot.Available || len(snapshot.Containers) != 0 || len(snapshot.Notices) == 0 || queries != 1 || time.Since(started) > time.Second {
		t.Fatal("expired inventory was not a bounded explicit failure")
	}
}
