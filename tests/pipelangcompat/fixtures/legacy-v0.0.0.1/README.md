# Frozen legacy compiler corpus

These 45 sources are byte-identical to TASK-021 acceptance at DockPipe commit
`c511d1c498d3dbd60a056feee61dc0cc30943f0c`. Their logical paths and all four original
source, projection, artifact and evaluation digests remain unchanged.

Seven authored models had evolved after acceptance, before standalone extraction.
The compatibility test now owns its accepted snapshot instead of hashing mutable
product sources. A separate test compiles the current authored models. The two
retired DorkPipe model paths use their accepted fixtures after source extraction.
These files are compiler test inputs and are never embedded or built as packages.
