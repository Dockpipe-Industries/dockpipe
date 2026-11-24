"""Black-box file capability checks using the public installed CLI envelope."""
import base64
import hashlib

from contracts import read_json, write_json
from process import MAX_LOG_BYTES


STALE_REVISION_DIAGNOSTIC = "the selected file or directory changed; refresh before retrying"
OUTSIDE_ROOT_DIAGNOSTIC = "workspace paths cannot traverse links, Git metadata, or another root"
NOT_REGULAR_DIAGNOSTIC = "the selected item is not a regular file"


def expected_rejection(output, name, diagnostics):
    logs = output / name
    execution = read_json(logs / "process.json")
    if not isinstance(execution, dict) or type(execution.get("exit_code")) is not int:
        return False
    if execution["exit_code"] != 1 or execution.get("failure") != "":
        return False
    with (logs / "driver.stderr").open("rb") as stream:
        diagnostic = stream.read(MAX_LOG_BYTES + 1)
    if len(diagnostic) > MAX_LOG_BYTES:
        return False
    try:
        message = diagnostic.decode("utf-8").strip()
    except UnicodeDecodeError:
        return False
    return message in diagnostics


def qualify(cli, workspace, environment, output, execute):
    root = output / "file-workspace"
    root.mkdir()
    canary = output / "outside-canary.txt"
    canary.write_bytes(b"outside fixture must remain unchanged\r\n")
    original_canary = canary.read_bytes()
    proof_paths = [canary]

    def invoke(name, action, path, **fields):
        request = output / (name + ".request.json")
        write_json(request, {"root": str(root), "request": {
            "schema": "dockpipe.files/v1", "action": action, "path": path, **fields}})
        completed = execute(cli, ["files", "--request", str(request)], workspace,
                            environment, output, name)
        proof_paths.append(request)
        proof_paths.extend(sorted((output / name).iterdir()))
        response = {}
        if completed:
            response = read_json(output / name / "driver.stdout")
        return completed, response

    initial = b"\xef\xbb\xbfhello file fixture\r\n"
    replacement = b"updated exact bytes\r\n"
    created, _ = invoke("files-create", "create", "original.txt",
                               content=base64.b64encode(initial).decode("ascii"))
    read, contents = invoke("files-read", "read", "original.txt")
    exact_read = read and base64.b64decode(contents.get("content", "")) == initial
    initial_revision = hashlib.sha256(initial).hexdigest()
    revised = contents.get("revision") == initial_revision
    written, _ = invoke("files-write", "write", "original.txt", revision=initial_revision,
                             content=base64.b64encode(replacement).decode("ascii"))
    exact_write = written and (root / "original.txt").read_bytes() == replacement
    stale, _ = invoke("files-stale", "write", "original.txt", revision=initial_revision,
                      content=base64.b64encode(b"must not publish stale edit").decode("ascii"))
    stale_rejected = (not stale and expected_rejection(output, "files-stale", {STALE_REVISION_DIAGNOSTIC})
                      and (root / "original.txt").read_bytes() == replacement)
    revision = hashlib.sha256(replacement).hexdigest()
    renamed, _ = invoke("files-rename", "rename", "original.txt", destination="renamed.txt", revision=revision)
    exact_rename = renamed and not (root / "original.txt").exists() and (root / "renamed.txt").read_bytes() == replacement
    listed, listing = invoke("files-list", "list", ".", limit=150)
    listing_exact = listed and any(item.get("name") == "renamed.txt" for item in listing.get("entries", []))
    traversal, _ = invoke("files-traversal", "read", "../outside-canary.txt")
    (root / "linked.txt").symlink_to(canary)
    linked, _ = invoke("files-symlink", "write", "linked.txt", revision=hashlib.sha256(original_canary).hexdigest(),
                       content=base64.b64encode(b"must not follow link").decode("ascii"))
    deleted, _ = invoke("files-delete", "delete", "renamed.txt", revision=revision)
    exact_delete = deleted and not (root / "renamed.txt").exists()
    traversal_rejected = (not traversal
                          and expected_rejection(output, "files-traversal", {OUTSIDE_ROOT_DIAGNOSTIC}))
    link_rejected = (not linked and expected_rejection(output, "files-symlink",
                                                     {OUTSIDE_ROOT_DIAGNOSTIC, NOT_REGULAR_DIAGNOSTIC}))
    confined = traversal_rejected and link_rejected and canary.read_bytes() == original_canary
    assertions = {"create-read-write-rename-delete": created and exact_read and exact_write
                  and exact_rename and listing_exact and exact_delete,
                  "revision-checks": revised and stale_rejected,
                  "workspace-contained": confined}
    facts = {"exact_byte_read": exact_read, "exact_byte_write": exact_write,
             "stale_revision_rejected": stale_rejected, "traversal_rejected": traversal_rejected,
             "symlink_rejected": link_rejected, "outside_canary_unchanged": canary.read_bytes() == original_canary,
             "renamed_and_deleted": exact_rename and exact_delete}
    return assertions, facts, proof_paths
