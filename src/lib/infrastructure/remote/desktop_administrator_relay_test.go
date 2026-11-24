package remote

import (
	"bytes"
	"encoding/json"
	"testing"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestDesktopAdministratorApprovalAndDedicatedRelay(t *testing.T) {
	broker, viewer, worker, config, _, _, catalog := desktopFixture(t)
	ctx := t.Context()
	desktop := desktopIntent(t, catalog)
	if _, err := viewer.OpenDesktop(ctx, desktop); err != nil {
		t.Fatal(err)
	}
	viewerPin := Hash("administrator relay viewer")
	servicePin := Hash("administrator relay service")
	if _, err := viewer.SignalDesktop(ctx, contract.DesktopSignal{
		Reference: desktop.Reference(), Sequence: 1, Type: "binding", Fingerprint: viewerPin,
	}); err != nil {
		t.Fatal(err)
	}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: catalog.Epoch, Catalog: catalog}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}
	exchange.Reports = []contract.DesktopReport{{Reference: desktop.Reference(), State: "running", Ack: 1}}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}

	node := catalog.Target.Node
	key, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	token, err := Secret()
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.SetDesktopAdministratorService(catalog.Target, key.PublicKey); err != nil {
		t.Fatal(err)
	}
	broker.mutex.Lock()
	broker.desktops.administratorKeys[node] = desktopAdministratorService{
		fingerprint: catalog.Target.Fingerprint, key: key.PublicKey, tokenHash: Hash(token),
	}
	broker.mutex.Unlock()

	service, err := NewClient(viewer.Endpoint, token)
	if err != nil {
		t.Fatal(err)
	}
	service.BrokerKey = broker.state.Identity.PublicKey
	ordinary, err := NewClient(viewer.Endpoint, config.Token)
	if err != nil {
		t.Fatal(err)
	}
	ordinary.BrokerKey = service.BrokerKey
	if _, err := ordinary.PollDesktopAdministratorService(ctx, contract.DesktopAdministratorServiceRequest{Node: node}); err == nil {
		t.Fatal("ordinary worker credential polled administrator requests")
	}

	catalogBefore, err := viewer.DesktopCatalog(ctx)
	if err != nil || len(catalogBefore) != 1 || catalogBefore[0].AdministratorInput {
		t.Fatalf("unavailable console service advertised a UI capability: %v", err)
	}
	if pending, err := service.PollDesktopAdministratorService(ctx, contract.DesktopAdministratorServiceRequest{Node: node}); err != nil || pending != nil {
		t.Fatalf("service polling without a request: pending=%v error=%v", pending, err)
	}
	catalogAfter, err := viewer.DesktopCatalog(ctx)
	if err != nil || len(catalogAfter) != 1 || !catalogAfter[0].AdministratorInput {
		t.Fatalf("live console service did not advertise its UI capability: %v", err)
	}

	ref := desktop.Reference()
	started, err := viewer.DesktopAdministratorApproval(ctx, contract.DesktopAdministratorApprovalRequest{Reference: ref, Start: true})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := service.PollDesktopAdministratorService(ctx, contract.DesktopAdministratorServiceRequest{Node: node})
	if err != nil || pending == nil || pending.Reference != ref || pending.Operator != started.Operator ||
		pending.ViewerFingerprint != viewerPin {
		t.Fatalf("unexpected console approval request: %+v, %v", pending, err)
	}

	consent := contract.DesktopAdministratorConsent{
		Schema: contract.DesktopAdministratorConsentVersion, Reference: ref, Challenge: Hash("console prompt approval"),
		Operator: pending.Operator, PeerFingerprint: viewerPin, ServiceFingerprint: servicePin,
		WindowsUserSID: "S-1-5-21-1-2-3-1000", WindowsSession: 4, Deadline: desktop.Deadline.UnixMilli(),
	}
	payload, err := json.Marshal(consent)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := key.Sign(administratorSignedMessage(contract.DesktopAdministratorConsentVersion, payload))
	if err != nil {
		t.Fatal(err)
	}
	envelope := contract.DesktopAdministratorEnvelope{Payload: payload, Signature: signature}
	if _, err := viewer.GrantDesktopAdministratorInput(ctx, envelope); err == nil {
		t.Fatal("viewer skipped the protected service approval reply")
	}
	if _, err := service.PollDesktopAdministratorService(ctx, contract.DesktopAdministratorServiceRequest{
		Node: node, ReplyTo: &ref, Consent: &envelope,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := viewer.GrantDesktopAdministratorInput(ctx, envelope); err != nil {
		t.Fatal(err)
	}

	request := contract.DesktopRelayRequest{
		Reference: ref, Role: "worker", LocalFingerprint: servicePin, PeerFingerprint: viewerPin,
	}
	if _, err := ordinary.DesktopAdministratorRelayTicket(ctx, request); err == nil {
		t.Fatal("ordinary worker credential acquired an administrator relay ticket")
	}
	serviceTicket, err := service.DesktopAdministratorRelayTicket(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	viewerTicket, err := viewer.DesktopAdministratorRelayTicket(ctx, contract.DesktopRelayRequest{
		Reference: ref, Role: "viewer", LocalFingerprint: viewerPin, PeerFingerprint: servicePin,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := viewer.AttachDesktopRelay(ctx, viewerTicket); err == nil {
		t.Fatal("ordinary desktop relay accepted an administrator ticket")
	}
	viewerRelay, err := viewer.AttachDesktopAdministratorRelay(ctx, viewerTicket)
	if err != nil {
		t.Fatal(err)
	}
	defer viewerRelay.Close()
	serviceRelay, err := service.AttachDesktopAdministratorRelay(ctx, serviceTicket)
	if err != nil {
		t.Fatal(err)
	}
	defer serviceRelay.Close()

	opaque := []byte("opaque administrator input fixture")
	if err := viewerRelay.Write(ctx, opaque); err != nil {
		t.Fatal(err)
	}
	got, err := serviceRelay.Read(ctx)
	if err != nil || !bytes.Equal(got, opaque) {
		t.Fatalf("dedicated relay did not deliver the opaque payload: %v", err)
	}
	if err := serviceRelay.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := viewer.DesktopStatus(ctx, contract.DesktopRead{Reference: ref})
	if err != nil || view.State != "running" {
		t.Fatalf("administrator relay closure changed the ordinary desktop: %+v, %v", view, err)
	}
}
