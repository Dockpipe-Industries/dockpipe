package remote

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dockpipe/src/lib/domain/invocation"
	contract "dockpipe/src/lib/domain/remote"
)

type featureDisconnectTransport struct {
	http.RoundTripper
	disconnected atomic.Bool
}

func (transport *featureDisconnectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.disconnected.Load() {
		return nil, errors.New("fixture disconnected")
	}
	return transport.RoundTripper.RoundTrip(request)
}

func TestFeatureLeaseStopsOnLockGrantChangeOrDisconnect(t *testing.T) {
	for _, scenario := range []string{"lock", "grants", "disconnect", "operator-expiry"} {
		t.Run(scenario, func(t *testing.T) {
			broker, admin, worker, _, root, permissions, catalog := featureFixture(t)
			transport := &featureDisconnectTransport{RoundTripper: worker.HTTP.Transport}
			worker.HTTP.Transport = transport
			request := fileFeatureRequest(t, catalog, "read")
			if _, err := admin.SubmitFeature(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			var assignment contract.FeatureAssignment
			started := time.Now()
			if err := worker.Call(context.Background(), "/v1/features-next", catalog, &assignment); err != nil {
				t.Fatal(err)
			}
			deadline, err := featureLeaseDeadline(started, assignment.LeaseMillis)
			if err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			done := make(chan contract.FeatureResult, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			go func() {
				done <- executeLeasedFeatureUsing(ctx, worker, root, permissions, permissionFingerprint(permissions), catalog, assignment, deadline,
					func(ctx context.Context, _ FeaturePermissions, _ contract.FeatureRequest, _ invocation.Identity, _ invocation.Authorize) contract.FeatureResult {
						close(entered)
						<-ctx.Done()
						return contract.FeatureResult{Status: "cancelled"}
					})
			}()
			<-entered
			switch scenario {
			case "lock":
				broker.security.Lock()
			case "grants":
				changed := permissions
				changed.Roots = append([]FeatureRootPermission(nil), permissions.Roots...)
				changed.Roots[0].FilesRead = false
				if err := WritePrivate(filepath.Join(root, "features.json"), changed); err != nil {
					t.Fatal(err)
				}
			case "disconnect":
				transport.disconnected.Store(true)
			case "operator-expiry":
				broker.mutex.Lock()
				admission := broker.features.admissions[request.ID]
				admission.deadline = time.Now().Add(-time.Second)
				broker.features.admissions[request.ID] = admission
				broker.mutex.Unlock()
			}
			select {
			case result := <-done:
				if result.Status != "cancelled" || ctx.Err() != nil {
					t.Fatal("feature did not cancel within its supervision interval")
				}
			case <-ctx.Done():
				t.Fatal("feature survived lost authority")
			}
		})
	}
}

func TestFeatureLeaseRejectsExcessiveOrElapsedDuration(t *testing.T) {
	for _, milliseconds := range []int64{-1, 0, contract.FeatureLease.Milliseconds() + 1} {
		if _, err := featureLeaseDeadline(time.Now(), milliseconds); err == nil {
			t.Fatal("invalid lease duration accepted")
		}
	}
	if _, err := featureLeaseDeadline(time.Now().Add(-time.Minute), 1000); err == nil {
		t.Fatal("expired in-transit lease accepted")
	}
}
