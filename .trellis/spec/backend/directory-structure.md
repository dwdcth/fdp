# Directory Structure

> How backend code is organized in this project.

---

## Overview

This repository is a Go CLI project. Runtime code is split by responsibility under `internal/`, while `cmd/` only holds the executable entrypoint.

---

## Directory Layout

```text
cmd/
└── dockerpull/
    └── main.go

internal/
├── app/          # top-level orchestration
├── cache/        # cache paths and digest verification
├── cli/          # flag parsing and option validation
├── downloader/   # aria2c integration
├── export/       # docker/oci archive writing
├── manifest/     # manifest decoding and platform resolution
├── reference/    # image reference parsing
├── registry/     # auth, manifest/blob fetching, mirror selection
├── scheduler/    # worker pool
└── state/        # sqlite persistence
```

---

## Module Organization

- Keep `cmd/dockerpull/main.go` minimal; only process exit wiring belongs there.
- Put orchestration in `internal/app/run.go`.
- Keep protocol-specific logic inside `internal/registry/` and `internal/manifest/`.
- Put file-system and digest logic in `internal/cache/` rather than duplicating checks in callers.
- Put export-format-specific code in `internal/export/`.
- Keep persistence isolated in `internal/state/`.

---

## Naming Conventions

- Use short, package-scoped filenames like `client.go`, `resolve.go`, `verify.go`.
- Keep package names singular and lowercase.
- Prefer small structs carrying explicit fields over generic maps.
- Name packages after responsibility, not after call sites.

---

## Real Code Examples

1. Thin entrypoint: `cmd/dockerpull/main.go` only calls `app.Run` and prints one final error.
2. Orchestration layer: `internal/app/run.go` coordinates CLI parsing, registry access, cache validation, persistence, and export.
3. Focused package boundaries:
   - `internal/registry/client.go` handles HTTP/auth/mirror fallback.
   - `internal/cache/verify.go` owns digest verification.
   - `internal/state/db.go` owns schema and upsert logic.

---

## Anti-Patterns

- Do not place business logic in `cmd/dockerpull/main.go`.
- Do not mix registry HTTP, cache, and export code in one package.
- Do not add cross-package utility dumping grounds; put logic in the package that owns the concept.
- Do not duplicate path-building or digest-verification helpers across packages.

---

## Common Mistakes

- Adding new workflow steps directly into `main.go` instead of `internal/app/run.go`.
- Re-implementing manifest or digest logic in callers rather than reusing `manifest` and `cache` packages.
- Putting SQLite access outside `internal/state/`, which weakens ownership boundaries.
