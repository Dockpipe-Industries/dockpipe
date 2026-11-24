package remote

import (
	"encoding/json"
	"testing"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func TestDesktopAdministratorGrantSeparatesServiceAndControlAuthority(t *testing.T) {
	broker, admin, worker, config, _, _, catalog := desktopFixture(t)
	ctx := t.Context()
	desktop := desktopIntent(t, catalog)
	if _, err := admin.OpenDesktop(ctx, desktop); err != nil {
		t.Fatal(err)
	}
	fingerprint := Hash("administrator-viewer")
	if _, err := admin.SignalDesktop(ctx, contract.DesktopSignal{
		Reference: desktop.Reference(), Sequence: 1, Type: "binding", Fingerprint: fingerprint,
	}); err != nil {
		t.Fatal(err)
	}
	exchange := contract.DesktopExchange{Schema: contract.DesktopVersion, Epoch: catalog.Epoch, Catalog: catalog}
	if _, err := worker.ExchangeDesktop(ctx, exchange); err != nil {
		t.Fatal(err)
	}
	exchange.Reports = []contract.DesktopReport{{Reference: desktop.Reference(), State: "running", Ack: 1}}
	exchangeReply, err := worker.ExchangeDesktop(ctx, exchange)
	if err != nil || len(exchangeReply.Directives) != 1 {
		t.Fatalf("get the authenticated desktop operator: %v", err)
	}

	service, err := remoteidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	sign := func(consent contract.DesktopAdministratorConsent) contract.DesktopAdministratorEnvelope {
		t.Helper()
		payload, err := json.Marshal(consent)
		if err != nil {
			t.Fatal(err)
		}
		signature, err := service.Sign(administratorSignedMessage(contract.DesktopAdministratorConsentVersion, payload))
		if err != nil {
			t.Fatal(err)
		}
		return contract.DesktopAdministratorEnvelope{Payload: payload, Signature: signature}
	}

	consent := contract.DesktopAdministratorConsent{
		Schema: contract.DesktopAdministratorConsentVersion, Reference: desktop.Reference(),
		Challenge: Hash("fresh-service-challenge"), PeerFingerprint: fingerprint,
		Operator:           exchangeReply.Directives[0].Operator,
		ServiceFingerprint: Hash("administrator-service-tls"),
		WindowsUserSID:     "S-1-5-21-1-2-3-1000", WindowsSession: 4,
		Deadline: time.Now().Add(25 * time.Second).UnixMilli(),
	}
	signed := sign(consent)
	if _, err := admin.GrantDesktopAdministratorInput(ctx, signed); err == nil {
		t.Fatal("ordinary control authorized administrator input without an approved service")
	}
	if err := broker.SetDesktopAdministratorService(catalog.Target, config.MachineKey); err == nil {
		t.Fatal("ordinary worker key accepted as independent service identity")
	}
	if err := broker.SetDesktopAdministratorService(catalog.Target, service.PublicKey); err != nil {
		t.Fatal(err)
	}
	forged := signed
	forged.Signature = ""
	if _, err := admin.GrantDesktopAdministratorInput(ctx, forged); err == nil {
		t.Fatal("unsigned service approval authorized administrator input")
	}
	otherPeer := consent
	otherPeer.PeerFingerprint = Hash("other-viewer")
	if _, err := admin.GrantDesktopAdministratorInput(ctx, sign(otherPeer)); err == nil {
		t.Fatal("a different viewer acquired administrator input")
	}

	otherOperator := consent
	otherOperator.Operator = Hash("other-operator")
	if _, err := admin.GrantDesktopAdministratorInput(ctx, sign(otherOperator)); err == nil {
		t.Fatal("a different operator acquired administrator input")
	}

	receipt, err := admin.GrantDesktopAdministratorInput(ctx, signed)
	if err != nil {
		t.Fatal(err)
	}
	var grant contract.DesktopAdministratorGrant
	if err := json.Unmarshal(receipt.Payload, &grant); err != nil {
		t.Fatal(err)
	}
	if !remoteidentity.Verify(broker.state.Identity.PublicKey,
		administratorSignedMessage(contract.DesktopAdministratorGrantVersion, receipt.Payload), receipt.Signature) ||
		grant.Serial == 0 || grant.Schema != contract.DesktopAdministratorGrantVersion ||
		time.Until(time.UnixMilli(grant.LeaseExpires)) <= 0 || time.Until(time.UnixMilli(grant.LeaseExpires)) > contract.DesktopLease {
		t.Fatalf("unexpected grant: %+v", grant)
	}
	changed := consent
	changed.Challenge = Hash("second-challenge")
	if _, err := admin.GrantDesktopAdministratorInput(ctx, sign(changed)); err == nil {
		t.Fatal("the admitted desktop accepted a second administrator-input approval")
	}
	if _, err := admin.CloseDesktop(ctx, desktop.Reference()); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.GrantDesktopAdministratorInput(ctx, signed); err == nil {
		t.Fatal("closed desktop renewed administrator input")
	}
}
