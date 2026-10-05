package remote

import (
	"reflect"
	"strings"
	"testing"
)

func TestJobRecoveryAndCancellationPreserveAssignment(t *testing.T) {
	for _, status := range []string{"queued", "running", "unknown", "success", "failure", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			job := Job{Submission: Submission{ID: "job", Node: "mini"}, Status: status, Session: "owner"}
			recovered := job.Recover()
			wantStatus := status
			if status == "running" {
				wantStatus = "unknown"
			}
			if recovered.Status != wantStatus || recovered.Session != job.Session || recovered.Submission != job.Submission {
				t.Fatalf("recovery changed identity: %+v", recovered)
			}
			cancelled, changed := recovered.Cancel()
			if wantStatus == "queued" {
				wantStatus = "cancelled"
			}
			if !changed || !cancelled.CancelRequested || cancelled.Status != wantStatus || cancelled.Session != job.Session {
				t.Fatalf("cancellation changed assignment: %+v", cancelled)
			}
			job.ResultHash = strings.Repeat("a", 64)
			completed, changed := job.Cancel()
			if changed || !reflect.DeepEqual(completed, job) {
				t.Fatal("cancellation changed completed work")
			}
		})
	}
}

func TestJobCompletionRequiresAssignmentAndPreservesExactRetries(t *testing.T) {
	digest := strings.Repeat("a", 64)
	result := &Result{Status: "success", Log: "finished"}
	for _, status := range []string{"running", "unknown"} {
		job := Job{Submission: Submission{ID: "job", Node: "mini"}, Status: status, Session: "owner", CancelRequested: true}
		completed, changed, err := job.Complete(result, digest)
		if err != nil || !changed || completed.Status != "success" || completed.ResultHash != digest || completed.Result != nil {
			t.Fatalf("completion: %+v, %v, %v", completed, changed, err)
		}
		if completed.Session != job.Session || !completed.CancelRequested || job.Status != status || job.ResultHash != "" {
			t.Fatal("completion mutated original work or assignment identity")
		}
		retry, changed, err := completed.Complete(result, digest)
		if err != nil || changed || !reflect.DeepEqual(retry, completed) {
			t.Fatalf("exact delivery retry changed work: %+v, %v, %v", retry, changed, err)
		}
		if _, _, err := completed.Complete(result, strings.Repeat("b", 64)); err == nil {
			t.Fatal("conflicting delivery accepted")
		}
		if _, _, err := job.Complete(nil, digest); err == nil {
			t.Fatal("missing result accepted")
		}
		if _, _, err := job.Complete(&Result{Status: "success", ExitCode: 1}, digest); err == nil {
			t.Fatal("invalid result accepted")
		}
	}
	for _, status := range []string{"queued", "cancelled", "success", "failure"} {
		if _, _, err := (Job{Status: status}).Complete(result, digest); err == nil {
			t.Fatalf("accepted result without an assignment in state %s", status)
		}
	}
}
