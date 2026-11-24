package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure/remoteidentity"
)

const desktopAdministratorServiceResponse = "dockpipe.remote.desktop-admin-service-response/v1"
const desktopAdministratorApprovalLifetime = 45 * time.Second

func (broker *Broker) desktopAdministratorApproval(raw []byte, admission operatorAdmission) (any, error) {
	var request contract.DesktopAdministratorApprovalRequest
	if Decode(raw, &request) != nil || request.Reference.Validate() != nil {
		return nil, errors.New("invalid desktop administrator approval reference")
	}

	state := broker.desktopState()
	record := state.records[request.Reference.ID]
	if !broker.desktopAdministratorEligible(record, request.Reference) || !desktopOwner(record, admission) ||
		broker.security.CheckAdmission(admission) != nil {
		return nil, errors.New("desktop administrator approval requires the original operator")
	}

	now := time.Now()
	if record.administratorApproval == nil && request.Start {
		expires := minDesktopTime(record.open.Deadline, now.Add(desktopAdministratorApprovalLifetime))
		record.administratorApproval = &contract.DesktopAdministratorApproval{
			Reference: request.Reference, Operator: desktopAdministratorOperator(admission),
			ViewerFingerprint: record.viewer.fingerprint, Expires: expires.UnixMilli(),
			DesktopDeadline: record.open.Deadline.UnixMilli(),
		}
	}
	approval := record.administratorApproval
	if approval == nil || !time.UnixMilli(approval.Expires).After(now) {
		return nil, errors.New("desktop administrator approval is absent or expired")
	}
	copy := *approval
	copy.BrokerKey = broker.state.Identity.PublicKey
	return copy, nil
}

func desktopAdministratorOperator(admission operatorAdmission) string {
	return Hash(fmt.Sprintf("%s:%d", admission.session, admission.generation))
}

func (broker *Broker) desktopAdministratorEligible(record *desktopRecord, reference contract.DesktopReference) bool {
	if record == nil || record.open.Reference() != reference || record.open.Mode != "control" ||
		record.state != "running" || record.viewer.fingerprint == "" || broker.desktopLease(record) <= 0 {
		return false
	}

	service := broker.desktopState().administratorKeys[reference.Target.Node]
	return service.fingerprint == reference.Target.Fingerprint && service.tokenHash != ""
}

func (broker *Broker) serveDesktopAdministratorService(w http.ResponseWriter, path string, raw []byte, token string) {
	state := broker.desktopState()
	var node string
	for id, service := range state.administratorKeys {
		enrollment, enrolled := broker.state.Nodes[id]
		if enrolled && !enrollment.Revoked && remoteidentity.Fingerprint(enrollment.MachineKey) == service.fingerprint &&
			matches(token, service.tokenHash) {
			node = id
			break
		}
	}
	if node == "" {
		http.Error(w, "administrator input service authentication required", http.StatusUnauthorized)
		return
	}

	broker.respond(w, func() (any, error) {
		var result any
		var err error
		if path == contract.DesktopAdministratorRelayTicketPath {
			result, err = broker.desktopAdministratorRelayTicket(raw, false, node, operatorAdmission{})
		} else {
			result, err = broker.desktopAdministratorServiceApproval(raw, node)
		}
		if err != nil {
			return nil, err
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		// Bind the signed response to the exact service request and operation.
		signature, err := broker.state.Identity.Sign(administratorSignedMessage(
			desktopAdministratorServiceResponse+"\n"+path+"\n"+Hash(string(raw)), payload))
		if err != nil {
			return nil, err
		}
		return contract.DesktopAdministratorEnvelope{Payload: payload, Signature: signature}, nil
	})
}

func (broker *Broker) desktopAdministratorServiceApproval(raw []byte, node string) (any, error) {
	var request contract.DesktopAdministratorServiceRequest
	if Decode(raw, &request) != nil || request.Node != node || !validMachineNonce(request.Nonce) ||
		(request.ReplyTo == nil && (request.Consent != nil || request.Denied)) ||
		(request.ReplyTo != nil && (request.Consent != nil) == request.Denied) {
		return nil, errors.New("invalid administrator service approval request")
	}
	now := time.Now()
	state := broker.desktopState()
	if request.ReplyTo == nil {
		if state.administratorSeen == nil {
			state.administratorSeen = map[string]time.Time{}
		}
		state.administratorSeen[node] = now
		for _, record := range state.records {
			approval := record.administratorApproval
			if record.open.Target.Node == node && approval != nil && approval.Consent == nil && !approval.Denied &&
				time.UnixMilli(approval.Expires).After(now) && broker.desktopAdministratorEligible(record, approval.Reference) {
				return *approval, nil
			}
		}
		return nil, nil
	}

	record := state.records[request.ReplyTo.ID]
	if request.ReplyTo.Target.Node != node || !broker.desktopAdministratorEligible(record, *request.ReplyTo) ||
		record.administratorApproval == nil {
		return nil, errors.New("administrator service reply lost its desktop approval")
	}
	approval := record.administratorApproval
	if approval.Consent != nil || approval.Denied || !time.UnixMilli(approval.Expires).After(now) {
		return nil, errors.New("administrator service approval is already resolved or expired")
	}

	if request.Consent != nil {
		var consent contract.DesktopAdministratorConsent
		service := state.administratorKeys[node]
		if Decode(request.Consent.Payload, &consent) != nil || consent.Schema != contract.DesktopAdministratorConsentVersion ||
			consent.Reference != approval.Reference || consent.Operator != approval.Operator || consent.PeerFingerprint != approval.ViewerFingerprint ||
			!remoteidentity.Verify(service.key, administratorSignedMessage(contract.DesktopAdministratorConsentVersion,
				request.Consent.Payload), request.Consent.Signature) {
			return nil, errors.New("administrator service replied with a different approval")
		}
	}
	approval.Denied = request.Denied
	approval.Consent = request.Consent
	return *approval, nil
}

func minDesktopTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
