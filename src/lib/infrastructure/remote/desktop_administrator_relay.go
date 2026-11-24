package remote

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

func (broker *Broker) desktopAdministratorRelayTicket(raw []byte, viewer bool, node string, admission operatorAdmission) (any, error) {
	var request contract.DesktopRelayRequest
	if Decode(raw, &request) != nil || request.Validate() != nil || viewer != (request.Role == "viewer") {
		return nil, errors.New("invalid administrator input relay request")
	}
	record := broker.desktopState().records[request.Reference.ID]
	if !broker.desktopAdministratorEligible(record, request.Reference) || record.administrator == nil ||
		!record.administrator.until.After(time.Now()) ||
		viewer && !desktopOwner(record, admission) || !viewer && request.Reference.Target.Node != node {
		return nil, errors.New("administrator input relay requires a current separate grant")
	}

	local, peer := record.administrator.viewer, record.administrator.service
	if !viewer {
		local, peer = peer, local
	}
	if request.LocalFingerprint != local || request.PeerFingerprint != peer {
		return nil, errors.New("administrator input relay certificate binding changed")
	}
	if record.administratorRelay == nil {
		ctx, cancel := context.WithDeadline(context.Background(), record.open.Deadline)
		record.administratorRelay = &desktopRelay{context: ctx, cancel: cancel, ready: make(chan struct{}), administrator: true}
	}
	expires := minDesktopTime(record.administrator.until, time.Now().Add(contract.DesktopRelayTicketLifetime))
	return issueDesktopRelayTicket(record, record.administratorRelay, request, expires)
}

func (client *Client) DesktopAdministratorApproval(ctx context.Context, request contract.DesktopAdministratorApprovalRequest) (contract.DesktopAdministratorApproval, error) {
	var approval contract.DesktopAdministratorApproval
	err := client.Call(ctx, contract.DesktopAdministratorApprovalPath, request, &approval)
	if err == nil && (approval.Reference != request.Reference || !validMachineNonce(approval.Operator) ||
		!validMachineNonce(approval.ViewerFingerprint) || remoteidentity.Fingerprint(approval.BrokerKey) == "" || !time.UnixMilli(approval.Expires).After(time.Now())) {
		err = errors.New("administrator input approval changed its original reference")
	}
	return approval, err
}

func (client *Client) desktopAdministratorServiceCall(ctx context.Context, path string, request, result any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var envelope contract.DesktopAdministratorEnvelope
	if err := client.Call(ctx, path, request, &envelope); err != nil {
		return err
	}

	if !remoteidentity.Verify(client.BrokerKey, administratorSignedMessage(
		desktopAdministratorServiceResponse+"\n"+path+"\n"+Hash(string(payload)), envelope.Payload), envelope.Signature) {
		return errors.New("administrator input service reply did not verify")
	}
	return Decode(envelope.Payload, result)
}

func (client *Client) PollDesktopAdministratorService(ctx context.Context, request contract.DesktopAdministratorServiceRequest) (*contract.DesktopAdministratorApproval, error) {
	nonce, err := Secret()
	if err != nil {
		return nil, err
	}
	request.Nonce = nonce
	var approval *contract.DesktopAdministratorApproval
	err = client.desktopAdministratorServiceCall(ctx, contract.DesktopAdministratorServicePath, request, &approval)
	return approval, err
}

func (client *Client) DesktopAdministratorRelayTicket(ctx context.Context, request contract.DesktopRelayRequest) (contract.DesktopRelayTicket, error) {
	var ticket contract.DesktopRelayTicket
	if err := request.Validate(); err != nil {
		return ticket, err
	}
	started := time.Now()
	var err error
	if request.Role == "viewer" {
		err = client.Call(ctx, contract.DesktopAdministratorRelayTicketPath, request, &ticket)
	} else {
		err = client.desktopAdministratorServiceCall(ctx, contract.DesktopAdministratorRelayTicketPath, request, &ticket)
	}
	if err != nil {
		return ticket, err
	}
	if ticket.Request != request || ticket.Provider.Validate() != nil || ticket.Mode != "control" ||
		!validMachineNonce(ticket.Secret) || ticket.ExpiresAt.IsZero() ||
		ticket.LeaseMillis <= 0 || ticket.LeaseMillis > contract.DesktopLease.Milliseconds() {
		return ticket, errors.New("administrator input relay ticket changed its exact binding")
	}
	ticket.LeaseMillis -= time.Since(started).Milliseconds()
	if ticket.LeaseMillis <= 0 {
		return ticket, errors.New("administrator input relay ticket expired in transit")
	}
	return ticket, nil
}

func (client *Client) AttachDesktopAdministratorRelay(ctx context.Context, ticket contract.DesktopRelayTicket) (*DesktopRelay, error) {
	return client.attachDesktopRelay(ctx, contract.DesktopAdministratorRelayPath, ticket)
}
