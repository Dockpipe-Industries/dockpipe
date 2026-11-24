# Source workflow-control consumer

Select the same five explicit CMake SDK identity variables on Windows and Linux.
The model, case order, assertions, eight-second body, five-second independent
cleanup and default evaluation budget are shared. The native composition supplies
platform facts, admitted paths, clock, bounded raw trace reads and the real SDK
transport. It does not decide expected traces or case outcomes.

The executable contract on both platforms is:

```
workflow_control_probe CLI config root state cache helper traceRoot [config2 root2 state2 cache2 traceRoot2]
```

All paths must be explicit absolute paths. Select an existing root and existing
state/cache child directories. Use existing fresh trace directories. On Linux,
pass an empty string for `helper`; no console signal helper is used. Select
`../../config.yml` for the Linux fixture and `../../windows.yml` for Windows, using their
actual absolute paths. The optional second project selects the fifth relocated
package case; omitting it runs the same first four cases.

Linux links the product harness ConfinedFiles implementation and retains its
Descriptor owners. Windows uses the selected materialized SDK WindowsFiles.
The materialized SDK CMake bridge selects its native client for the actual host.
Source selects `workflow-control.windows.*` or `workflow-control.linux.*` IDs
from the explicit platform fact, and uses a backslash or slash for trace paths.
No workflow fixture or script is copied or rewritten by this consumer.

`SANITIZERS=ON` instruments the generated model, bridge and consumer including
the Linux confined-file adapter through the model's interface flags. Build and
run each actual host separately; changing the Source DTO requires a Windows
rerun as well as Linux qualification. Build or syntax inspection alone is not
runtime acceptance.
