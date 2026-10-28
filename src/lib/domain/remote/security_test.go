package remote

import "testing"

func TestRemoteSecurityPolicyBoundsAndWeakening(t *testing.T) {
	baseline := DefaultRemoteSecurityPolicy()
	for _, idle := range []int{-1, 0, 59, 3601} {
		policy := baseline
		policy.IdleSeconds = idle
		if policy.Validate() == nil {
			t.Fatalf("accepted unsupported inactivity duration %d", idle)
		}
	}
	for _, idle := range []int{60, 300, 3600} {
		policy := baseline
		policy.IdleSeconds = idle
		if err := policy.Validate(); err != nil {
			t.Fatal(err)
		}
		if policy.Weakens(baseline) != (idle > baseline.IdleSeconds) {
			t.Fatal("incorrect policy weakening classification")
		}
	}
	policy := baseline
	policy.LockOnScreenLock = false
	if !policy.Weakens(baseline) {
		t.Fatal("disabling lock integration did not require authentication")
	}
	policy = baseline
	policy.Schema = "unknown"
	if policy.Validate() == nil {
		t.Fatal("unknown security policy accepted")
	}
}
