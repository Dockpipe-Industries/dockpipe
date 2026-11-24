//go:build windows

package remote

import (
	"context"
	"testing"
	"time"
)

func TestWindowsFeatureWorkerPublishesFilesCatalog(t *testing.T) {
	_, admin, worker, config, root, _, initial := featureFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stopped := make(chan error, 1)
	go func() { stopped <- RunFeatureWorker(ctx, root, config, worker) }()

	poll := time.NewTicker(50 * time.Millisecond)
	defer poll.Stop()

	for {
		select {
		case err := <-stopped:
			t.Fatalf("feature worker stopped before publication: %v", err)
		case <-ctx.Done():
			<-stopped
			t.Fatal("feature worker did not publish its Files catalog")
		case <-poll.C:
			catalogs, err := admin.FeatureCatalog(ctx)
			if err != nil {
				cancel()
				<-stopped
				t.Fatal(err)
			}

			for _, catalog := range catalogs {
				if catalog.Target.Node != config.Node || catalog.Target.Generation == initial.Target.Generation {
					continue
				}

				cancel()
				<-stopped
				if len(catalog.Roots) != 1 || !catalog.Roots[0].FilesWrite || catalog.Roots[0].Terminal {
					t.Fatalf("unexpected worker grants: %+v", catalog)
				}

				return
			}
		}
	}
}
