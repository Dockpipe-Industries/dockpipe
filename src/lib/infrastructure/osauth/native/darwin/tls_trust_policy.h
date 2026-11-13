#ifndef DOCKPIPE_TLS_TRUST_POLICY_H
#define DOCKPIPE_TLS_TRUST_POLICY_H

#include <stdbool.h>
#include <stddef.h>

// Values mirror the public SecTrustSettingsResult constants. Keep classification
// independent of Foundation so the fail-closed policy can be tested on any host.
typedef enum {
    DockpipeTLSNoDecision,
    DockpipeTLSAnchor,
    DockpipeTLSUnsupported
} DockpipeTLSDisposition;

typedef struct {
    bool appliesToSSL;
    bool restricted;
    bool resultPresent;
    int result;
} DockpipeTLSRule;

static DockpipeTLSDisposition DockpipeClassifyTLSRules(const DockpipeTLSRule *rules, size_t count)
{
    if (count > 64 || (count != 0 && rules == NULL))
        return DockpipeTLSUnsupported;
    // An existing, empty settings array means unrestricted TrustRoot. Missing
    // settings must be handled by the caller and never passed as an empty array.
    if (count == 0)
        return DockpipeTLSAnchor;
    bool anchor = false;
    for (size_t index = 0; index < count; index++) {
        const DockpipeTLSRule rule = rules[index];
        if (!rule.appliesToSSL)
            continue;
        if (rule.restricted)
            return DockpipeTLSUnsupported;
        const int result = rule.resultPresent ? rule.result : 1;
        if (result == 1 || result == 2)
            anchor = true;
        else if (result != 4)
            return DockpipeTLSUnsupported;
    }
    return anchor ? DockpipeTLSAnchor : DockpipeTLSNoDecision;
}

#endif
