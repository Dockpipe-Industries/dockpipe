package invocation

import "context"

// Definition is chosen by trusted composition, not deserialized from a caller.
// Installation, discovery and the chosen interface do not grant authority.
type Definition struct {
	ID                   ID
	Target               Target
	RequireAuthorization bool
}

// Prepared is returned only by trusted domain preparation. Prepare must perform
// memory-only validation and capture an owned input snapshot: no filesystem,
// process, network or other external effects may precede authorization. It must
// not retain mutable caller slices, maps or pointers in Execute.
//
// Execute owns filesystem preconditions, race-aware revision checks, policy
// enforcement, resource bounds and effects. Preparation is not proof that files
// or other external state remain unchanged. No prepared permit escapes Invoke.
type Prepared[Output any] struct {
	Digest   string
	Revision string
	Execute  func(context.Context) (Output, error)
}

type Prepare[Input, Output any] func(context.Context, Input) (Prepared[Output], error)

// Authorize adapts the existing domain authority to the exact binding. The
// adapter must check all applicable policy, approval and identity requirements.
// It must not mutate the owned input captured by preparation.
type Authorize func(context.Context, Binding) error

// Contract is an immutable native composition, not a registry entry or security
// sandbox. Its zero value rejects calls. Trusted callback code remains trusted;
// this boundary cannot constrain a malicious native implementation.
type Contract[Input, Output any] struct {
	definition Definition
	prepare    Prepare[Input, Output]
	authorize  Authorize
}

func New[Input, Output any](definition Definition, prepare Prepare[Input, Output], authorize Authorize) (Contract[Input, Output], error) {
	if !definition.ID.valid() || !definition.Target.valid() || prepare == nil {
		return Contract[Input, Output]{}, ErrInvalidContract
	}
	if (definition.RequireAuthorization || definition.Target.Kind == Remote) && authorize == nil {
		return Contract[Input, Output]{}, ErrAuthorizationUnavailable
	}
	return Contract[Input, Output]{definition: definition, prepare: prepare, authorize: authorize}, nil
}

// Invoke validates, prepares, authorizes and executes one typed operation. It
// never returns a reusable permit, retries an effect, changes targets, or starts
// an authorization service. Ordinary local calls need only their domain validator.
func (contract Contract[Input, Output]) Invoke(ctx context.Context, call Call, input Input) (Output, error) {
	var zero Output
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if contract.prepare == nil {
		return zero, ErrInvalidContract
	}
	if !call.Contract.valid() || !call.Target.valid() || call.Identity.Issuer == "" || call.Identity.Subject == "" {
		return zero, ErrInvalidCall
	}
	if call.Contract != contract.definition.ID || call.Target != contract.definition.Target {
		return zero, ErrContractMismatch
	}
	prepared, err := contract.prepare(ctx, input)
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	requiresAuthorization := contract.definition.RequireAuthorization || call.Target.Kind == Remote
	if prepared.Execute == nil {
		return zero, ErrInvalidPreparation
	}
	if prepared.Digest != "" && !validDigest(prepared.Digest) {
		return zero, ErrInvalidPreparation
	}
	if requiresAuthorization && prepared.Digest == "" {
		return zero, ErrInvalidPreparation
	}
	if requiresAuthorization && contract.authorize == nil {
		return zero, ErrAuthorizationUnavailable
	}
	if contract.authorize != nil {
		binding := Binding{Call: call, Digest: prepared.Digest, Revision: prepared.Revision}
		if err := contract.authorize(ctx, binding); err != nil {
			return zero, err
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return prepared.Execute(ctx)
}
