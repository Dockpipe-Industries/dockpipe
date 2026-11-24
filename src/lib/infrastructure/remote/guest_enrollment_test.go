package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestDisposableGuestGrantWorkflowAndRevocation(t *testing.T) {
	root, broker, admin, server := testBroker(t)
	ctx := context.Background()
	brokerKey := broker.state.Identity.PublicKey

	newMachine := func(name string, existing bool) (contract.WorkerConfig, *Client) {
		t.Helper()
		identity := remoteidentityMustNew(t)
		token, err := Secret()
		if err != nil {
			t.Fatal(err)
		}
		config := contract.WorkerConfig{
			Endpoint: server.URL, Node: name, Token: token, BrokerKey: brokerKey,
			MachineKey: identity.PublicKey, MachinePrivateKey: identity.PrivateKey,
		}
		if existing {
			broker.mutex.Lock()
			broker.state.Nodes[name] = Enrollment{MachineKey: identity.PublicKey, TokenHash: Hash(token)}
			broker.mutex.Unlock()
		}
		client, err := NewWorkerClient(config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(client.HTTP.CloseIdleConnections)
		return config, client
	}

	_, host := newMachine("host", true)
	_, sibling := newMachine("sibling", true)
	guestConfig, guest := newMachine("run-one-guest", false)
	grantRequest := contract.GuestGrantRequest{Host: "host", Run: "run-one", Guests: 1, LifetimeSeconds: 120}
	unauthenticated, err := NewClient(server.URL, admin.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer unauthenticated.HTTP.CloseIdleConnections()
	if err := unauthenticated.Call(ctx, "/v1/guest-grant", grantRequest, nil); err == nil {
		t.Fatal("grant creation accepted without operator proof")
	}

	var grant, retry contract.GuestGrantReceipt
	if err := admin.Call(ctx, "/v1/guest-grant", grantRequest, &grant); err != nil {
		t.Fatal(err)
	}
	if grant.Guests != grantRequest.Guests {
		t.Fatalf("grant reported %d guest slots, want %d", grant.Guests, grantRequest.Guests)
	}
	if err := admin.Call(ctx, "/v1/guest-grant", grantRequest, &retry); err != nil || retry != grant {
		t.Fatalf("grant retry did not return the original receipt: %v", err)
	}
	changedGrant := grantRequest
	changedGrant.Guests = 2
	if err := admin.Call(ctx, "/v1/guest-grant", changedGrant, nil); err == nil {
		t.Fatal("existing grant widened after an ambiguous retry")
	}

	prepare := func(grantID string, config contract.WorkerConfig) contract.GuestEnrollmentRequest {
		t.Helper()
		binding := contract.GuestEnrollment{
			Grant: grantID, Node: config.Node, PublicKey: config.MachineKey, TokenHash: Hash(config.Token),
		}
		identity := remoteidentity.Identity{PublicKey: config.MachineKey, PrivateKey: config.MachinePrivateKey}
		proof, err := identity.Sign(binding.ProofMessage(brokerKey))
		if err != nil {
			t.Fatal(err)
		}
		return contract.GuestEnrollmentRequest{GuestEnrollment: binding, Proof: proof}
	}

	claim := prepare(grant.ID, guestConfig)
	if err := sibling.Call(ctx, "/v1/guest-enroll", claim, nil); err == nil {
		t.Fatal("another paired host accepted the grant")
	}
	tampered := claim
	tampered.Node = "run-one-changed"
	if err := host.Call(ctx, "/v1/guest-enroll", tampered, nil); err == nil {
		t.Fatal("guest key proof accepted a changed node")
	}
	foreignConfig, _ := newMachine("other-run-guest", false)
	if err := host.Call(ctx, "/v1/guest-enroll", prepare(grant.ID, foreignConfig), nil); err == nil {
		t.Fatal("host enrolled outside the grant's run")
	}

	var enrolled, enrolledAgain contract.GuestEnrollmentReceipt
	if err := host.Call(ctx, "/v1/guest-enroll", claim, &enrolled); err != nil {
		t.Fatal(err)
	}
	if err := host.Call(ctx, "/v1/guest-enroll", claim, &enrolledAgain); err != nil || enrolledAgain != enrolled {
		t.Fatalf("guest retry did not reconcile the original enrollment: %v", err)
	}
	if err := admin.Call(ctx, "/v1/guest-grant", grantRequest, &retry); err != nil || retry != grant {
		t.Fatalf("guest grant receipt changed after consuming a slot: %v", err)
	}
	second, _ := newMachine("run-one-second", false)
	if err := host.Call(ctx, "/v1/guest-enroll", prepare(grant.ID, second), nil); err == nil {
		t.Fatal("grant exceeded its guest limit")
	}
	if err := guest.Call(ctx, "/v1/guest-enroll", prepare(grant.ID, second), nil); err == nil {
		t.Fatal("guest enrolled another machine")
	}
	if err := sibling.Call(ctx, "/v1/guest-release", map[string]string{"id": grant.ID}, nil); err == nil {
		t.Fatal("another paired host released the grant")
	}
	if err := guest.Call(ctx, "/v1/monitor-publish", map[string]any{}, nil); err == nil {
		t.Fatal("workflow-only guest accepted a monitoring call")
	}
	if err := admin.Call(ctx, "/v1/remove", map[string]string{"id": guestConfig.Node}, nil); err == nil {
		t.Fatal("active grant allowed its enrolled identity to be removed")
	}

	instance, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	target := workflowOnlyTarget(guestConfig, instance)
	catalog := contract.FeatureCatalog{Schema: contract.FeatureVersion, Target: target, WorkflowOnly: true}
	expanded := catalog
	expanded.WorkflowOnly = false
	expanded.DockerRead = true
	if err := guest.Call(ctx, "/v1/features-next", expanded, nil); err == nil {
		t.Fatal("disposable guest published Docker access")
	}
	if err := guest.Call(ctx, "/v1/features-next", catalog, nil); err != nil {
		t.Fatal(err)
	}
	peers, err := admin.FeatureCatalog(ctx)
	if err != nil || len(peers) != 1 || peers[0].Target != target || !peers[0].WorkflowOnly {
		t.Fatalf("disposable guest was not discoverable for workflow delivery: %v %v", peers, err)
	}

	bundle := contract.Bundle{
		WorkflowFile: "config.yml",
		Files:        []contract.BundleFile{{Path: "config.yml", Data: []byte("name: guest-proof\nsteps: []\n")}},
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	submit := contract.SubmitRequest{Submission: contract.Submission{
		ID: "guest-proof", Node: guestConfig.Node, BundleHash: digest, Target: target,
	}, Bundle: &bundle}
	if err := admin.Call(ctx, "/v1/submit", submit, nil); err != nil {
		t.Fatal(err)
	}
	session, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	var assignment contract.Job
	if err := guest.Call(ctx, "/v1/next", map[string]string{"session": session}, &assignment); err != nil || assignment.ID != submit.ID {
		t.Fatalf("guest could not claim its job: %v", err)
	}
	jobRequest := map[string]string{"id": assignment.ID, "session": session}
	var delivered contract.Bundle
	if err := guest.Call(ctx, "/v1/bundle", jobRequest, &delivered); err != nil {
		t.Fatal(err)
	}
	if received, err := delivered.Digest(); err != nil || received != digest {
		t.Fatal("bundle digest differs")
	}
	body, err := json.Marshal(jobRequest)
	if err != nil {
		t.Fatal(err)
	}
	guestIdentity := remoteidentity.Identity{PublicKey: guestConfig.MachineKey, PrivateKey: guestConfig.MachinePrivateKey}
	pulseProof := func() (string, string) {
		t.Helper()
		nonce := testMachineNonce(t, broker, guestConfig.Token)
		signature, err := guestIdentity.Sign(machineMessage("request", nonce, "/v1/pulse", 0, body))
		if err != nil {
			t.Fatal(err)
		}
		return nonce, signature
	}
	nonce, signature := pulseProof()
	broker.mutex.Lock()
	activeGrant := broker.state.GuestGrants[grant.ID]
	expiredGrant := activeGrant
	expiredGrant.ExpiresAt = time.Now().Add(-time.Second)
	broker.state.GuestGrants[grant.ID] = expiredGrant
	broker.mutex.Unlock()
	if response := recordedWorkerCall(t, broker, guestConfig.Token, "/v1/pulse", nonce, signature, body); response.Code != http.StatusUnauthorized {
		t.Fatalf("in-flight machine challenge survived grant expiry: %d", response.Code)
	}
	peers, err = admin.FeatureCatalog(ctx)
	if err != nil || len(peers) != 0 {
		t.Fatalf("expired disposable guest remained in the workflow catalog: %v %v", peers, err)
	}
	broker.mutex.Lock()
	broker.state.GuestGrants[grant.ID] = activeGrant
	broker.mutex.Unlock()

	nonce, signature = pulseProof()
	if err := admin.Call(ctx, "/v1/revoke", map[string]string{"id": "host"}, nil); err != nil {
		t.Fatal(err)
	}
	if response := recordedWorkerCall(t, broker, guestConfig.Token, "/v1/pulse", nonce, signature, body); response.Code != http.StatusUnauthorized {
		t.Fatalf("in-flight machine challenge survived host revocation: %d", response.Code)
	}
	submit.ID = "after-revocation"
	if err := admin.Call(ctx, "/v1/submit", submit, nil); err == nil {
		t.Fatal("broker accepted new work for a guest whose host is revoked")
	}

	idleRequest := contract.GuestGrantRequest{Host: "sibling", Run: "run-idle", Guests: 1, LifetimeSeconds: 120}
	var idleGrant contract.GuestGrantReceipt
	if err := admin.Call(ctx, "/v1/guest-grant", idleRequest, &idleGrant); err != nil {
		t.Fatal(err)
	}
	idleGuest, _ := newMachine("run-idle-guest", false)
	if err := sibling.Call(ctx, "/v1/guest-enroll", prepare(idleGrant.ID, idleGuest), nil); err != nil {
		t.Fatal(err)
	}

	broker.mutex.Lock()
	idle := broker.state.GuestGrants[idleGrant.ID]
	idle.ExpiresAt = time.Now().Add(-time.Second)
	broker.state.GuestGrants[idleGrant.ID] = idle
	broker.mutex.Unlock()

	var replacement contract.GuestGrantReceipt
	if err := admin.Call(ctx, "/v1/guest-grant", idleRequest, &replacement); err != nil || replacement.ID == idleGrant.ID {
		t.Fatalf("expired idle guest prevented a fresh grant: %v", err)
	}
	if err := sibling.Call(ctx, "/v1/guest-enroll", prepare(idleGrant.ID, idleGuest), nil); err == nil {
		t.Fatal("reclaimed guest grant accepted the original claim")
	}
	freshGuest, _ := newMachine(idleGuest.Node, false)
	if err := sibling.Call(ctx, "/v1/guest-enroll", prepare(replacement.ID, freshGuest), nil); err != nil {
		t.Fatalf("expired idle guest still reserved its node name: %v", err)
	}

	var release contract.GuestReleaseReceipt
	if err := admin.Call(ctx, "/v1/guest-grant-revoke", map[string]string{"id": grant.ID}, &release); err != nil {
		t.Fatal(err)
	}
	if len(release.Removed) != 0 || len(release.Retained) != 1 || release.Retained[0] != guestConfig.Node {
		t.Fatal("grant revocation forgot a guest with a live assignment")
	}
	server.Close()
	restart, err := NewBroker(root)
	if err != nil {
		t.Fatal(err)
	}
	enrollment, exists := restart.state.Nodes[guestConfig.Node]
	if !exists || restart.enrollmentActive(enrollment, time.Now()) {
		t.Fatal("broker restart lost the retained guest identity or restored its authority")
	}
	enrollment, exists = restart.state.Nodes[freshGuest.Node]
	if !exists || enrollment.GuestGrant != replacement.ID || !restart.enrollmentActive(enrollment, time.Now()) {
		t.Fatal("broker restart lost the replacement enrollment")
	}
}

func TestInactiveGuestAssignmentRequiresOperatorReview(t *testing.T) {
	for _, cause := range []string{"expired", "released", "host revoked"} {
		t.Run(cause, func(t *testing.T) {
			root := privateTestDirectory(t)
			host := remoteidentityMustNew(t)
			state := BrokerState{
				Schema: contract.Version, AdminHash: Hash("operator"),
				Nodes: map[string]Enrollment{"host": {MachineKey: host.PublicKey, TokenHash: Hash("host")}},
				Jobs:  map[string]contract.Job{},
			}
			if err := WritePrivate(filepath.Join(root, "broker.json"), state); err != nil {
				t.Fatal(err)
			}
			broker, err := NewBroker(root)
			if err != nil {
				t.Fatal(err)
			}
			grantRequest := contract.GuestGrantRequest{Host: "host", Run: "run", Guests: 1, LifetimeSeconds: 600}
			response, err := archiveOperator(broker, "/v1/guest-grant", grantRequest)
			if err != nil {
				t.Fatal(err)
			}
			grant := response.(contract.GuestGrantReceipt)
			guest := remoteidentityMustNew(t)
			token := strings.Repeat("e", 64)
			node := "run-guest"
			broker.state.Nodes[node] = Enrollment{MachineKey: guest.PublicKey, TokenHash: Hash(token), GuestGrant: grant.ID}

			bundle := contract.Bundle{
				WorkflowFile: "config.yml",
				Files:        []contract.BundleFile{{Path: "config.yml", Data: []byte("name: fixture\nsteps: []\n")}},
			}
			digest, err := bundle.Digest()
			if err != nil {
				t.Fatal(err)
			}
			submit := contract.SubmitRequest{Submission: contract.Submission{ID: "assigned", Node: node, BundleHash: digest}, Bundle: &bundle}
			if _, err := archiveOperator(broker, "/v1/submit", submit); err != nil {
				t.Fatal(err)
			}
			session := strings.Repeat("d", 64)
			if _, err := broker.machineOperation(node, broker.state.Nodes[node], "/v1/next", marshal(t, map[string]string{"session": session})); err != nil {
				t.Fatal(err)
			}
			request := contract.JobInterruptionRequest{ID: submit.ID, Node: node}
			if _, err := broker.jobInterruption(marshal(t, request), false); err == nil {
				t.Fatal("active guest assignment without a worker result was reviewable")
			}

			body := marshal(t, map[string]any{"id": submit.ID, "session": session, "result": contract.Result{Status: "success"}})
			nonce := testMachineNonce(t, broker, token)
			signature, err := guest.Sign(machineMessage("request", nonce, "/v1/result", 0, body))
			if err != nil {
				t.Fatal(err)
			}
			switch cause {
			case "expired":
				stored := broker.state.GuestGrants[grant.ID]
				stored.ExpiresAt = time.Now().Add(-time.Second)
				broker.state.GuestGrants[grant.ID] = stored
			case "released":
				_, err = archiveOperator(broker, "/v1/guest-grant-revoke", map[string]string{"id": grant.ID})
			case "host revoked":
				_, err = archiveOperator(broker, "/v1/revoke", map[string]string{"id": grant.Host})
			}
			if err != nil {
				t.Fatal(err)
			}
			if recorded := recordedWorkerCall(t, broker, token, "/v1/result", nonce, signature, body); recorded.Code != http.StatusUnauthorized {
				t.Fatalf("inactive guest delivered its final result: HTTP %d", recorded.Code)
			}

			inspected, err := broker.jobInterruption(marshal(t, request), false)
			if err != nil || inspected.Validate(request, false) != nil || inspected.Evidence != contract.InactiveGuestWithoutResult || inspected.Acknowledged {
				t.Fatalf("missing guest result was not identified for review: %+v, %v", inspected, err)
			}
			if broker.state.Jobs[submit.ID].Status != "running" || !broker.nodeHasUnresolvedJobs(node) {
				t.Fatal("inspection changed the assignment")
			}
			if _, err := broker.removeNode(marshal(t, map[string]string{"id": node})); err == nil {
				t.Fatal("unreviewed guest assignment allowed removal")
			}

			request.ReviewToken = inspected.ReviewToken
			if cause == "released" {
				// The worker may already have stopped before the broker restarted.
				broker, err = NewBroker(root)
				if err != nil {
					t.Fatal(err)
				}
			} else if cause == "expired" {
				// A result can reach durable storage before its metadata digest.
				// Even a valid review token must not discard that worker evidence.
				path, err := broker.resultPath(submit.ID)
				if err != nil {
					t.Fatal(err)
				}
				if err := WritePrivate(path, contract.Result{Status: "success"}); err != nil {
					t.Fatal(err)
				}
				if _, err := broker.jobInterruption(marshal(t, request), true); err == nil {
					t.Fatal("missing-result acknowledgement accepted an existing worker result")
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			acknowledged, err := broker.jobInterruption(marshal(t, request), true)
			if err != nil || acknowledged.Validate(request, true) != nil || acknowledged.ResultHash != "" {
				t.Fatalf("failed to acknowledge the exact absence of worker evidence: %+v, %v", acknowledged, err)
			}
			broker, err = NewBroker(root)
			if err != nil {
				t.Fatal(err)
			}
			if broker.nodeHasUnresolvedJobs(node) || broker.enrollmentActive(broker.state.Nodes[node], time.Now()) {
				t.Fatal("restart restored the hold or reauthorized the guest")
			}
			if _, err := broker.worker(node, "/v1/result", body); err == nil {
				t.Fatal("closed assignment accepted a later result")
			}
			if cause == "host revoked" {
				_, err = broker.removeNode(marshal(t, map[string]string{"id": node}))
			} else {
				_, err = archiveOperator(broker, "/v1/guest-grant", grantRequest)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := broker.state.Nodes[node]; exists {
				t.Fatal("resolved guest was not reclaimed")
			}
			retried, err := broker.jobInterruption(marshal(t, request), true)
			if err != nil || retried != acknowledged {
				t.Fatalf("reclamation changed the review receipt: %+v, %v", retried, err)
			}
			history := broker.state.Jobs[submit.ID]
			if history.Status != "unknown" || history.Session != session || history.Result != nil || history.ResultHash != "" {
				t.Fatal("operator recovery changed the assignment or fabricated a worker result")
			}
			if cause == "released" {
				if err := broker.storeArchivedJob(history); err != nil {
					t.Fatal(err)
				}
				archived, err := broker.readArchivedJob(history.ID)
				if err != nil || !archived.HasAcknowledgedInterruption() || archived.ResultHash != "" || archived.Session != session {
					t.Fatalf("archive lost the reviewed absence of a worker result: %+v, %v", archived, err)
				}
			}
		})
	}
}
