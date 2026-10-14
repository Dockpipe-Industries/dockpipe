package telemetry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestProcessIdentityRejectsReuseAndMissingStart(t *testing.T) {
	first := ProcessIdentity{PID: 42, StartToken: "boot:100"}
	for _, other := range []ProcessIdentity{{PID: 42}, {PID: 42, StartToken: "boot:101"}, {PID: 43, StartToken: "boot:100"}} {
		if SameProcess(first, other) {
			t.Fatalf("accepted identity: %+v", other)
		}
	}
	if !SameProcess(first, first) {
		t.Fatal("same identity rejected")
	}
}

func TestStoreRetainsBoundedHistoryAndUpdatesLifecycle(t *testing.T) {
	store := Store{Directory: t.TempDir()}
	start := time.Now().UTC()
	for index := 0; index < MaxExecutions+1; index++ {
		record := Execution{ID: fmt.Sprint(index), State: "running", StartedAt: start.Add(time.Duration(index) * time.Second)}
		if err := store.Record(record); err != nil {
			t.Fatal(err)
		}
	}
	records, err := store.Read()
	if err != nil || len(records) != MaxExecutions {
		t.Fatalf("count=%d error=%v", len(records), err)
	}
	record := records[0]
	record.State = "succeeded"
	if err := store.Record(record); err != nil {
		t.Fatal(err)
	}
	records, err = store.Read()
	if err != nil || records[0].State != "succeeded" || records[len(records)-1].ID == "0" {
		t.Fatalf("bad retention: %v", err)
	}
}

func TestStoreConcurrentExecutions(t *testing.T) {
	store := Store{Directory: t.TempDir()}
	var workers sync.WaitGroup
	for index := 0; index < 12; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			record := Execution{ID: fmt.Sprint(index), WorkflowRun: fmt.Sprint(index), StartedAt: time.Now().UTC(), State: "running"}
			if err := store.RecordContext(context.Background(), record); err != nil {
				t.Error(err)
				return
			}
			record.State = "succeeded"
			if err := store.RecordContext(context.Background(), record); err != nil {
				t.Error(err)
			}
		}(index)
	}
	workers.Wait()
	records, err := store.Read()
	if err != nil || len(records) != 12 {
		t.Fatalf("concurrent records: %d %v", len(records), err)
	}
	for _, record := range records {
		if record.State != "succeeded" {
			t.Fatal(record)
		}
	}
}

func TestStoreRejectsOversizeAndLinkedHistory(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "snapshot.json"), make([]byte, MaxSnapshotBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Directory: directory}).Read(); err == nil {
		t.Fatal("accepted oversized history")
	}
	if err := os.Remove(filepath.Join(directory, "snapshot.json")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(target, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory, "snapshot.json")); err != nil {
		t.Skip(err)
	}
	if _, err := (Store{Directory: directory}).Read(); err == nil {
		t.Fatal("accepted linked history")
	}
}

func TestRecorderOverflowFailureAndTeardown(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	recorder, err := NewRecorder(1, func(ctx context.Context, record Execution) error {
		if record.ID == "first" {
			close(entered)
			<-release
		}
		return fmt.Errorf("fixture storage failure")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !recorder.Submit(Execution{ID: "first"}) {
		t.Fatal("submit failed")
	}
	<-entered
	if !recorder.Submit(Execution{ID: "second"}) || recorder.Submit(Execution{ID: "overflow"}) {
		t.Fatal("queue bound failed")
	}
	close(release)
	if err := recorder.Close(context.Background()); err == nil {
		t.Fatal("lost collector failure")
	}
	if recorder.Submit(Execution{}) {
		t.Fatal("submit after close")
	}
	select {
	case <-recorder.done:
	default:
		t.Fatal("worker did not exit")
	}
}

func TestStoreRejectsLinkedAncestorBeforeCreatingFiles(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	store := Store{Directory: filepath.Join(link, "executions")}
	if err := store.Record(Execution{ID: "fixture"}); err == nil {
		t.Fatal("linked ancestor accepted")
	}
	if _, err := os.Stat(filepath.Join(target, "executions")); !os.IsNotExist(err) {
		t.Fatal("wrote through linked ancestor")
	}
}
