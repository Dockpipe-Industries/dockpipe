#include "tls_trust_policy.h"

#include <assert.h>

int main(void)
{
    assert(DockpipeClassifyTLSRules(NULL, 0) == DockpipeTLSAnchor);
    DockpipeTLSRule root = {true, false, false, 0};
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSAnchor);
    root.resultPresent = true;
    root.result = 2;
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSAnchor);
    root.result = 3;
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSUnsupported);
    root.result = 4;
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSNoDecision);
    root.result = 0;
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSUnsupported);
    root.appliesToSSL = false;
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSNoDecision);
    root.appliesToSSL = true;
    root.result = 1;
    root.restricted = true;
    assert(DockpipeClassifyTLSRules(&root, 1) == DockpipeTLSUnsupported);
    DockpipeTLSRule mixed[] = {{true, false, true, 1}, {true, true, true, 1}};
    assert(DockpipeClassifyTLSRules(mixed, 2) == DockpipeTLSUnsupported);
    assert(DockpipeClassifyTLSRules(NULL, 1) == DockpipeTLSUnsupported);
    assert(DockpipeClassifyTLSRules(mixed, 65) == DockpipeTLSUnsupported);
    return 0;
}
