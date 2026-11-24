package remote

import (
	"context"
	"strings"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
)

func TestDesktopOriginalGenerationViewCleanupUsesExistingWorkerAPI(t *testing.T) {
	_, operator, worker, config, _, permissions, original := desktopFixture(t)
	ctx := context.Background()
	intent := desktopIntent(t, original)
	intent.Mode = "view"
	intent.ID, _ = intent.Digest()
	if _, err := operator.OpenDesktop(ctx, intent); err != nil {
		t.Fatal(err)
	}
	if _, err := operator.SignalDesktop(ctx, contract.DesktopSignal{Reference: intent.Reference(), Sequence: 1, Type: "binding", Fingerprint: strings.Repeat("d", 64)}); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.ExchangeDesktop(ctx, contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: original.Epoch, Catalog: original,
		Reports: []contract.DesktopReport{{Reference: intent.Reference(), State: "running", Ack: 1}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := operator.CloseDesktop(ctx, intent.Reference()); err != nil {
		t.Fatal(err)
	}
	instance, _ := Secret()
	newer, err := NewDesktopCatalog(config, permissions, instance, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if newer.Target.Generation == original.Target.Generation {
		t.Fatal("fixture did not advance provider lifetime")
	}
	if _, err := worker.ExchangeDesktop(ctx, contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: original.Epoch, Catalog: newer}); err != nil {
		t.Fatal(err)
	}
	cleanup := original
	cleanup.View, cleanup.Input = false, false
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: original.Epoch, Catalog: cleanup}
	response, err := worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(response.Directives) != 1 {
		t.Fatal("old generation could not obtain its exact revoked directive", err)
	}
	directive := response.Directives[0]
	if directive.Open.Reference() != intent.Reference() || directive.Open.Mode != "view" || directive.Open.Provider != intent.Provider || !directive.Close || directive.LeaseMillis != 0 || len(directive.Signals) != 0 {
		t.Fatal("cleanup reacquired media, signaling, or another operation")
	}
	// An in-range but invented ACK must not release the original reservation.
	exchange.Reports = []contract.DesktopReport{{Reference: intent.Reference(), State: "closed", Ack: 63}}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err == nil {
		t.Fatal("forged cleanup acknowledgment was accepted")
	}
	status, err := operator.DesktopStatus(ctx, contract.DesktopRead{Reference: intent.Reference(), MetadataOnly: true})
	if err != nil || status.State == "closed" || status.Ack != 1 || status.LeaseMillis != 0 {
		t.Fatal("invalid acknowledgment cleared unknown authority", err)
	}
	exchange.Reports[0].Ack = status.Ack
	response, err = worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(response.Forget) != 1 || response.Forget[0] != intent.Reference() {
		t.Fatal("exact native proof did not receive original Forget", err)
	}
	status, err = operator.DesktopStatus(ctx, contract.DesktopRead{Reference: intent.Reference(), MetadataOnly: true})
	if err != nil || status.State != "closed" || status.LeaseMillis != 0 || len(status.Signals) != 0 {
		t.Fatal("authenticated metadata status lost exact closure", err)
	}
}
