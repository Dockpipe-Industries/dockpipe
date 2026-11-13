# Read-only TLS trust material

The signed OS-authentication helper accepts `--tls-trust` as a separate read-only
operation. Existing authentication, activity snapshot and account-directory
commands are unchanged. It emits only the schema and sorted, deduplicated,
canonical base64 DER public certificates. It does not enumerate keys or labels,
export keychains, modify trust, or allow certificate errors.

The native source reads public Security framework trust settings. Only explicit
user and administrator additional server-SSL anchors are candidates; the system
domain is also inspected for conflicting restrictions on those certificates.
Default platform roots are deliberately excluded: native trust-asset restrictions
cannot be flattened into a DER-only array. Chromium retains its normal public
roots. This is an additional SSL anchor snapshot, not a full macOS trust store.

User, administrator and system domains are inspected in precedence order,
accepting unrestricted or server-SSL TrustRoot/TrustAsRoot settings in the user or
administrator domain. Existing empty settings arrays and missing result fields
mean TrustRoot; missing settings do not confer trust. Every selected additional
anchor also requires positive current-account native server-SSL evaluation with
network fetching disabled. This verifies existing trust and never promotes a
certificate merely because it was obtained from a server. It may conservatively
refuse a CA that cannot be evaluated by itself as an SSL certificate.

The row-level `kSecTrustSettingsPolicyName` value `sslServer` is recognized only
alongside a matching server-SSL policy. It is the policy identifier metadata that
Apple's trust-settings API copies separately from the hostname constraints. Other
SSL policy-name values or types fail closed. `kSecTrustSettingsPolicyString` and
`SecPolicyCopyProperties`' actual `kSecPolicyName` remain rejected as hostname
constraints; recognizing the row metadata does not allow either restriction.

Any SSL-applicable deny, application/hostname/key-usage restriction, allowed error,
unknown constraint, explicit domain-wide default or unreadable setting makes the
entire operation unavailable (exit 2 with no JSON). Lower-priority unsupported
restrictions are conservatively rejected even when a higher domain might override
them. Explicit unrelated policies and client-only SSL rules do not grant server
SSL trust. This deliberately refuses some valid native configurations rather than
flattening them into broader anchors. It is an additive trust snapshot, not a claim
of full cross-platform trust equivalence or automatic distrust subtraction.

Resource contract: at most 512 unique candidates/output certificates, 16 KiB DER
per certificate, 2 MiB aggregate DER, 64 policy rows per certificate/domain and
3 MiB complete JSON. Exhaustion fails without truncation. The owning Go adapter
supplies the command deadline and cancellation; no output is written until the
complete snapshot has passed validation. Trust-state changes during collection
are not an atomic OS snapshot; the authenticated worker generation and refresh
policy own freshness.

Failure writes one bounded stderr line with a compiled stage/reason code, numeric
OS status or trust-evaluation error, domain number and certificate/DER/rule counts.
It never prints policy values, certificate contents, labels or keychain paths.
In particular `read-settings` identifies an OS API failure, `default-rule-present`
an explicit domain-wide default, `settings.*` an unsupported rule category and
`ssl-evaluation` a failed native positive trust check. Native fixtures verify
reason codes without reading actual trust stores. Diagnosing a failure does not
relax it or produce a partial certificate array.

Portable normalized policy fixtures:

```sh
cc -std=c11 -Wall -Wextra -Werror tls_trust_policy_test.c -o /tmp/tls-trust-policy-test
/tmp/tls-trust-policy-test
```

Native parser fixtures (no actual trust-store reads or changes):

```sh
clang -fobjc-arc -fblocks -O2 -Wall -Wextra -framework Foundation -framework Security \
  tls_trust_test.m -o /tmp/tls-trust-test
/tmp/tls-trust-test
```

The normal macOS bundle preparation links Security into the signed helper.
Native compilation and a qualified read-only export remain required acceptance
checks on macOS; Linux policy fixtures alone cannot establish those outcomes.
