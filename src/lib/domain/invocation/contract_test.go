package invocation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func fixtureDefinition() Definition {
	return Definition{
		ID:     ID{Capability: "fixture.document", Action: "read", Version: "1"},
		Target: Target{Kind: Local, ID: "current-host"},
	}
}

func fixtureCall(definition Definition) Call {
	return Call{Contract: definition.ID, Target: definition.Target,
		Identity: Identity{Issuer: "native-process", Subject: "current-user"}}
}

func TestContractRejectsUnknownAndUnavailableAuthority(t *testing.T) {
	prepare := func(context.Context, int) (Prepared[int], error) {
		t.Fatal("invalid admission reached preparation")
		return Prepared[int]{}, nil
	}
	definition := fixtureDefinition()
	for _, invalid := range []Definition{
		{},
		{ID: definition.ID, Target: Target{Kind: TargetKind(99), ID: "host"}},
		{ID: definition.ID, Target: Target{Kind: Remote, ID: "host"}},
		{ID: definition.ID, Target: Target{Kind: Local, ID: "host", Generation: "remote-generation"}},
	} {
		if _, err := New(invalid, prepare, nil); !errors.Is(err, ErrInvalidContract) {
			t.Fatalf("invalid definition accepted: %v", err)
		}
	}
	definition.RequireAuthorization = true
	if _, err := New(definition, prepare, nil); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("privileged local invocation accepted without authority: %v", err)
	}
	definition.RequireAuthorization = false
	definition.Target = Target{Kind: Remote, ID: "worker", Fingerprint: strings.Repeat("a", 64), Generation: "worker-1"}
	if _, err := New(definition, prepare, nil); !errors.Is(err, ErrAuthorizationUnavailable) {
		t.Fatalf("remote invocation accepted without authority: %v", err)
	}
	var zero Contract[int, int]
	if _, err := zero.Invoke(context.Background(), fixtureCall(definition), 1); !errors.Is(err, ErrInvalidContract) {
		t.Fatalf("zero contract accepted: %v", err)
	}
}

func TestContractRejectsChangedBindingBeforePreparing(t *testing.T) {
	definition := fixtureDefinition()
	definition.Target = Target{Kind: Remote, ID: "worker", Fingerprint: strings.Repeat("a", 64), Generation: "worker-1"}
	contract, err := New(definition, func(context.Context, int) (Prepared[int], error) {
		t.Fatal("changed binding reached preparation")
		return Prepared[int]{}, nil
	}, func(context.Context, Binding) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*Call){
		"capability":      func(call *Call) { call.Contract.Capability = "other" },
		"action":          func(call *Call) { call.Contract.Action = "write" },
		"version":         func(call *Call) { call.Contract.Version = "2" },
		"host":            func(call *Call) { call.Target.ID = "other-worker" },
		"key":             func(call *Call) { call.Target.Fingerprint = strings.Repeat("b", 64) },
		"generation":      func(call *Call) { call.Target.Generation = "worker-2" },
		"local downgrade": func(call *Call) { call.Target = Target{Kind: Local, ID: "current-host"} },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			call := fixtureCall(definition)
			change(&call)
			if _, err := contract.Invoke(context.Background(), call, 1); !errors.Is(err, ErrContractMismatch) {
				t.Fatalf("changed binding accepted: %v", err)
			}
		})
	}
	call := fixtureCall(definition)
	call.Identity = Identity{}
	if _, err := contract.Invoke(context.Background(), call, 1); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("missing identity accepted: %v", err)
	}
}

func TestDeniedInvocationNeverExecutes(t *testing.T) {
	definition := fixtureDefinition()
	definition.RequireAuthorization = true
	denied := errors.New("existing policy denied")
	digest := strings.Repeat("a", 64)
	contract, err := New(definition, func(context.Context, int) (Prepared[int], error) {
		return Prepared[int]{Digest: digest, Revision: "review-1", Execute: func(context.Context) (int, error) {
			t.Fatal("denied invocation executed")
			return 0, nil
		}}, nil
	}, func(_ context.Context, binding Binding) error {
		if binding.Call != fixtureCall(definition) || binding.Digest != digest || binding.Revision != "review-1" {
			t.Fatalf("authorization received wrong binding: %+v", binding)
		}
		return denied
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contract.Invoke(context.Background(), fixtureCall(definition), 1); !errors.Is(err, denied) {
		t.Fatalf("authorization failure lost: %v", err)
	}
}

func TestDomainPreparationOwnsValidatedInput(t *testing.T) {
	definition := fixtureDefinition()
	definition.RequireAuthorization = true
	input := []byte("reviewed")
	contract, err := New(definition, func(_ context.Context, source []byte) (Prepared[string], error) {
		owned := string(source)
		digest := sha256.Sum256([]byte(owned))
		return Prepared[string]{Digest: hex.EncodeToString(digest[:]), Execute: func(context.Context) (string, error) {
			return owned, nil
		}}, nil
	}, func(_ context.Context, binding Binding) error {
		input[0] = 'X'
		expected := sha256.Sum256([]byte("reviewed"))
		if binding.Digest != hex.EncodeToString(expected[:]) {
			t.Fatal("authorization did not bind the owned input")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := contract.Invoke(context.Background(), fixtureCall(definition), input)
	if err != nil || result != "reviewed" {
		t.Fatalf("execution did not use the domain-owned snapshot: %q, %v", result, err)
	}
}

func TestRequiredDigestRejectsInvalidPreparation(t *testing.T) {
	definition := fixtureDefinition()
	definition.RequireAuthorization = true
	for _, digest := range []string{"", "a", strings.Repeat("g", 64), strings.Repeat("A", 64), strings.Repeat("a", 65)} {
		contract, err := New(definition, func(context.Context, int) (Prepared[int], error) {
			return Prepared[int]{Digest: digest, Execute: func(context.Context) (int, error) {
				t.Fatal("malformed binding executed")
				return 0, nil
			}}, nil
		}, func(context.Context, Binding) error {
			t.Fatal("malformed binding reached authorization")
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := contract.Invoke(context.Background(), fixtureCall(definition), 1); !errors.Is(err, ErrInvalidPreparation) {
			t.Fatalf("invalid digest accepted: %v", err)
		}
	}
}

func TestRemoteInvocationRequiresExactInputBinding(t *testing.T) {
	definition := fixtureDefinition()
	definition.Target = Target{Kind: Remote, ID: "worker", Fingerprint: strings.Repeat("a", 64), Generation: "worker-1"}
	for _, digest := range []string{"", strings.Repeat("b", 64)} {
		authorized := false
		contract, err := New(definition, func(context.Context, int) (Prepared[int], error) {
			return Prepared[int]{Digest: digest, Execute: func(context.Context) (int, error) {
				if !authorized {
					t.Fatal("remote execution preceded authorization")
				}
				return 42, nil
			}}, nil
		}, func(_ context.Context, binding Binding) error {
			if binding.Call != fixtureCall(definition) || binding.Digest != digest {
				t.Fatal("remote authority received a different invocation")
			}
			authorized = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := contract.Invoke(context.Background(), fixtureCall(definition), 1)
		if digest == "" {
			if !errors.Is(err, ErrInvalidPreparation) || authorized {
				t.Fatalf("unbound remote input admitted: %v", err)
			}
		} else if err != nil || result != 42 {
			t.Fatalf("authorized remote invocation failed: %d, %v", result, err)
		}
	}
}

func TestCancellationAtAdmissionBoundaries(t *testing.T) {
	for _, stage := range []string{"before", "prepare", "authorize", "execute"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			definition := fixtureDefinition()
			contract, err := New(definition, func(context.Context, int) (Prepared[int], error) {
				if stage == "before" {
					t.Fatal("cancelled call prepared")
				}
				if stage == "prepare" {
					cancel()
				}
				return Prepared[int]{Execute: func(ctx context.Context) (int, error) {
					if stage != "execute" {
						t.Fatal("cancelled call executed")
					}
					cancel()
					return 0, ctx.Err()
				}}, nil
			}, func(context.Context, Binding) error {
				if stage == "prepare" {
					t.Fatal("cancelled preparation reached authorization")
				}
				if stage == "authorize" {
					cancel()
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if stage == "before" {
				cancel()
			}
			if _, err := contract.Invoke(ctx, fixtureCall(definition), 1); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}

func TestDomainFailureAndSuccessfulLocalFastPath(t *testing.T) {
	definition := fixtureDefinition()
	domainFailure := errors.New("revision changed")
	contract, err := New(definition, func(_ context.Context, value int) (Prepared[int], error) {
		if value < 0 {
			return Prepared[int]{}, domainFailure
		}
		return Prepared[int]{Execute: func(context.Context) (int, error) {
			if value == 0 {
				return 0, domainFailure
			}
			return value, nil
		}}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []int{-1, 0} {
		if _, err := contract.Invoke(context.Background(), fixtureCall(definition), value); !errors.Is(err, domainFailure) {
			t.Fatalf("domain failure lost: %v", err)
		}
	}
	result, err := contract.Invoke(context.Background(), fixtureCall(definition), 7)
	if err != nil || result != 7 {
		t.Fatalf("local invocation failed without services: %d, %v", result, err)
	}
}
