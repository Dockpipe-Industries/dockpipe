# Repository tooling

The repository builds, tests, packages and installs through Dockpipe workflows.
Workflow-specific scripts stay beside their workflow; this directory holds shared
maintainer adapters and editor provenance.

Run the CLI build directly from the repository root:

```sh
python3 workflows/shared/launch.py workflows/build/cli/config.yml -- --output src/bin/dockpipe.bin --alias src/bin/dockpipe
```

The optional Make adapter delegates to the same workflows:

```sh
make -f tooling/Makefile build
make -f tooling/Makefile test
make -f tooling/Makefile dev-install
```

It also works from this directory as `make build`. Recursive targets keep the
same adapter and repository root. CI calls the workflows directly.

`go.mod`/`go.sum` define the module root; `go.work`/`go.work.sum` define the workspace.
They remain at the repository root. Go source lives in `src/`. The generic container
entrypoint belongs to `src/lib/runtimeassets/containers/`, where the runtime can
materialize it without depending on checkout tooling.
