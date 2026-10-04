package remote

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQueueAdmissionRetainsIdentityAtCapacityAndAfterRevocation(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	queue := Queue{}
	request := Submission{ID: "benchmark", Node: "mini", Profile: "bench"}
	job, changed, err := queue.Submit(request, true, now)
	if err != nil || !changed || job.Status != "queued" || job.CreatedAt != now {
		t.Fatalf("admission: %+v, changed=%v, err=%v", job, changed, err)
	}
	if len(queue) != 0 {
		t.Fatal("policy mutated the snapshot before durable publication")
	}
	queue[job.ID] = job
	for index := 1; index < MaxJobs; index++ {
		id := fmt.Sprintf("retained-%d", index)
		queue[id] = Job{Submission: Submission{ID: id, Node: "mini", Profile: "bench"}, Status: "success"}
	}
	retry, changed, err := queue.Submit(request, false, now.Add(time.Hour))
	if err != nil || changed || !reflect.DeepEqual(retry, job) {
		t.Fatalf("exact retry changed retained work: %+v, %v, %v", retry, changed, err)
	}
	conflict := request
	conflict.Profile = "different"
	if _, _, err := queue.Submit(conflict, true, now); err == nil {
		t.Fatal("accepted different work under retained identity")
	}
	request.ID = "new-work"
	if _, _, err := queue.Submit(request, true, now); err == nil || len(queue) != MaxJobs {
		t.Fatal("capacity rejection did not preserve retained work")
	}
	if _, _, err := (Queue{}).Submit(request, false, now); err == nil {
		t.Fatal("admitted work without an enrolled node")
	}
	request.ID = "../invalid"
	if _, _, err := queue.Submit(request, true, now); err == nil {
		t.Fatal("admitted invalid identity")
	}
}

func TestQueueClaimsOldestAndNeverRedispatchesUnresolvedWork(t *testing.T) {
	now := time.Now().UTC()
	queue := Queue{
		"later": {Submission: Submission{ID: "later", Node: "mini", Profile: "bench"}, Status: "queued", CreatedAt: now.Add(time.Second)},
		"first": {Submission: Submission{ID: "first", Node: "mini", Profile: "bench"}, Status: "queued", CreatedAt: now},
		"other": {Submission: Submission{ID: "other", Node: "other", Profile: "bench"}, Status: "running"},
	}
	session := strings.Repeat("a", 64)
	job, changed, err := queue.Next("mini", session)
	if err != nil || !changed || job == nil || job.ID != "first" || job.Session != session || job.Status != "running" {
		t.Fatalf("claim: %+v, %v, %v", job, changed, err)
	}
	if queue["first"].Status != "queued" {
		t.Fatal("policy mutated queued work before persistence")
	}
	for _, state := range []string{"running", "unknown"} {
		job.Status = state
		queue[job.ID] = *job
		retry, changed, err := queue.Next("mini", strings.Repeat("b", 64))
		if err != nil || changed || retry == nil || retry.ID != job.ID || retry.Session != session || retry.Status != state {
			t.Fatalf("unresolved work was reassigned: %+v, %v, %v", retry, changed, err)
		}
	}
	if job, changed, err := queue.Next("idle", session); err != nil || changed || job != nil {
		t.Fatalf("idle node: %+v, %v, %v", job, changed, err)
	}
	for _, invalid := range []string{"", "short", strings.Repeat("/", 64)} {
		if _, _, err := queue.Next("mini", invalid); err == nil {
			t.Fatalf("accepted invalid session %q", invalid)
		}
	}
}

func TestQueueRequiresAssignedNodeAndSession(t *testing.T) {
	queue := Queue{"job": {Submission: Submission{ID: "job", Node: "mini"}, Status: "running", Session: "owner"}}
	for _, request := range []struct {
		id      string
		node    string
		session string
	}{
		{"missing", "mini", "owner"},
		{"job", "other", "owner"},
		{"job", "mini", "other"},
		{"job", "mini", ""},
	} {
		if _, err := queue.Assigned(request.id, request.node, request.session); err == nil {
			t.Fatalf("accepted unowned assignment: %+v", request)
		}
	}
	if _, err := queue.Assigned("job", "mini", "owner"); err != nil {
		t.Fatal(err)
	}
}
