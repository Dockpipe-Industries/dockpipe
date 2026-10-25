# Cockpit qualification inputs

This workflow consumes checksum-pinned published Cockpit artifacts and preserves
their original qualification and host SDK source provenance. It does not rebuild
or replace those immutable release files.

The historical publisher tag was moved by the completed identity-only maintenance.
`publisher.source_sha` still binds the original release record and artifact
metadata. When `tag_commit_sha` and `source_tree_sha` are present together, the
consumer requires that exact current tag target and reads both commit objects
from the publisher API. Both must identify the same explicitly pinned source tree.
Missing pins, different commits, different trees or changed release provenance
fail. Ordinary pins without a reconciliation still require the original exact
source commit. Release and artifact hashes are never relaxed.
