package appserversupervisor

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"dorkpipe.orchestrator/providersession"
)

var errPreInitializationCapabilityPlanRejected = errors.New("pre-initialization capability plan was rejected")

// preInitializationCapabilityTarget binds a future confirmation to one exact
// not-yet-started supervisor and its package-facing session. It is deliberately
// package-private and is not a provider-neutral contract.
type preInitializationCapabilityTarget struct {
	Session       providersession.SessionRef
	SupervisorRef string
	PlannerRef    string
}

// preInitializationCapabilityConfirmation is independent evidence supplied
// after the caller has explicitly confirmed one exact capability for the
// target. CatalogRef binds the confirmation to the complete fixture catalog,
// including its private provider mapping.
type preInitializationCapabilityConfirmation struct {
	ConfirmationRef string
	Target          preInitializationCapabilityTarget
	CatalogRef      string
	CapabilityRef   string
}

// preInitializationCapabilityIntent requests exactly one capability. The
// advertisement remains fixture-backed; projection validates it before any
// child is started and no provider-backed model or native-policy selection is
// moved into this lane.
type preInitializationCapabilityIntent struct {
	Target                preInitializationCapabilityTarget
	Advertisement         CapabilityAdvertisement
	RequestedCapabilities []string
	Confirmation          preInitializationCapabilityConfirmation
}

// preInitializationCapabilityPlan is immutable-by-validation planning state.
// It is never attached to Supervisor and therefore cannot affect initialize,
// lifecycle dispatch, request handling, recovery, or a future consumer.
type preInitializationCapabilityPlan struct {
	session            providersession.SessionRef
	supervisorRef      string
	plannerRef         string
	catalogRef         string
	capabilityRef      string
	providerCapability string
	providerEnabled    bool
	confirmationRef    string
	fingerprint        [sha256.Size]byte
}

var preInitializationPlannerReferences atomic.Uint64

// preInitializationCapabilityPlanner consumes at most one confirmation for
// one exact fresh supervisor. A new or recovered supervisor has different
// incarnation references and therefore requires a new target and confirmation.
type preInitializationCapabilityPlanner struct {
	mu         sync.Mutex
	supervisor *Supervisor
	target     preInitializationCapabilityTarget
	consumed   bool
}

func newPreInitializationCapabilityPlanner(s *Supervisor) (*preInitializationCapabilityPlanner, error) {
	if s == nil {
		return nil, errPreInitializationCapabilityPlanRejected
	}
	s.mu.RLock()
	fresh := !s.started && !s.initialized && s.state == providersession.StateReady && s.client == nil
	session, processRef, connectionRef := s.session, s.processRef, s.connectionRef
	s.mu.RUnlock()
	if !fresh || validateSupervisorSession(session) != nil || !validID(processRef) || !validID(connectionRef) {
		return nil, errPreInitializationCapabilityPlanRejected
	}
	target := preInitializationCapabilityTarget{
		Session:       session,
		SupervisorRef: preInitializationSupervisorRef(session, processRef, connectionRef),
		PlannerRef:    fmt.Sprintf("preinit-planner-%d", preInitializationPlannerReferences.Add(1)),
	}
	return &preInitializationCapabilityPlanner{supervisor: s, target: target}, nil
}

func (p *preInitializationCapabilityPlanner) plan(intent preInitializationCapabilityIntent) (preInitializationCapabilityPlan, error) {
	if p == nil {
		return preInitializationCapabilityPlan{}, errPreInitializationCapabilityPlanRejected
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.consumed || !p.supervisorIsFresh() || intent.Target != p.target || len(intent.RequestedCapabilities) != 1 || intent.RequestedCapabilities[0] != requestAttestationCapabilityRef {
		return preInitializationCapabilityPlan{}, errPreInitializationCapabilityPlanRejected
	}

	catalog, reason := projectCapabilityCatalog(intent.Advertisement)
	if reason != "" {
		return preInitializationCapabilityPlan{}, errPreInitializationCapabilityPlanRejected
	}
	option, found := advertisedCapability(catalog, requestAttestationCapabilityRef)
	if !found || option.providerCapability != providerCapabilityAttestation || !option.providerEnabled || !option.Stable || !option.Available || !option.Supported || !option.AuthorityExpanding || option.Experimental {
		return preInitializationCapabilityPlan{}, errPreInitializationCapabilityPlanRejected
	}
	confirmation := intent.Confirmation
	if !validID(confirmation.ConfirmationRef) || confirmation.Target != p.target || confirmation.CatalogRef != catalog.CatalogRef || confirmation.CapabilityRef != requestAttestationCapabilityRef {
		return preInitializationCapabilityPlan{}, errPreInitializationCapabilityPlanRejected
	}

	plan := preInitializationCapabilityPlan{
		session:            p.target.Session,
		supervisorRef:      p.target.SupervisorRef,
		plannerRef:         p.target.PlannerRef,
		catalogRef:         catalog.CatalogRef,
		capabilityRef:      option.CapabilityRef,
		providerCapability: option.providerCapability,
		providerEnabled:    option.providerEnabled,
		confirmationRef:    confirmation.ConfirmationRef,
	}
	plan.fingerprint = plan.fingerprintValue()
	if err := plan.validateFor(p.supervisor); err != nil {
		return preInitializationCapabilityPlan{}, err
	}
	p.consumed = true
	return plan, nil
}

func (p *preInitializationCapabilityPlanner) supervisorIsFresh() bool {
	if p.supervisor == nil {
		return false
	}
	p.supervisor.mu.RLock()
	defer p.supervisor.mu.RUnlock()
	return !p.supervisor.started && !p.supervisor.initialized && p.supervisor.state == providersession.StateReady && p.supervisor.client == nil && p.supervisor.session == p.target.Session && validID(p.target.PlannerRef) && preInitializationSupervisorRef(p.supervisor.session, p.supervisor.processRef, p.supervisor.connectionRef) == p.target.SupervisorRef
}

func (p preInitializationCapabilityPlan) validateFor(s *Supervisor) error {
	if s == nil || validateSupervisorSession(p.session) != nil || !validID(p.supervisorRef) || !validID(p.plannerRef) || !validID(p.catalogRef) || !validID(p.confirmationRef) || p.capabilityRef != requestAttestationCapabilityRef || p.providerCapability != providerCapabilityAttestation || !p.providerEnabled || p.fingerprint != p.fingerprintValue() {
		return errPreInitializationCapabilityPlanRejected
	}
	s.mu.RLock()
	fresh := !s.started && !s.initialized && s.state == providersession.StateReady && s.client == nil
	session, processRef, connectionRef := s.session, s.processRef, s.connectionRef
	s.mu.RUnlock()
	if !fresh || session != p.session || preInitializationSupervisorRef(session, processRef, connectionRef) != p.supervisorRef {
		return errPreInitializationCapabilityPlanRejected
	}
	return nil
}

func (p preInitializationCapabilityPlan) fingerprintValue() [sha256.Size]byte {
	hash := sha256.New()
	for _, value := range []string{p.session.Provider, p.session.SessionID, p.supervisorRef, p.plannerRef, p.catalogRef, p.capabilityRef, p.providerCapability, p.confirmationRef} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	_, _ = hash.Write([]byte{boolByte(p.providerEnabled)})
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func preInitializationSupervisorRef(session providersession.SessionRef, processRef, connectionRef string) string {
	hash := sha256.New()
	for _, value := range []string{session.Provider, session.SessionID, processRef, connectionRef} {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("preinit-target-%x", hash.Sum(nil))
}
