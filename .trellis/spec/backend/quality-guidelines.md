# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

This repository is a Go CLI codebase. Quality gates are currently based on formatting, buildability, and package tests.

---

## Required Patterns

- Run `gofmt` on all Go files.
- Keep orchestration in `internal/app/run.go` and isolate concerns by package.
- Reuse helpers for cache path generation and manifest/media-type parsing.
- Validate downloaded blobs with size/digest checks before persisting success.
- Keep the executable entrypoint thin and side-effect orchestration explicit.

---

## Forbidden Patterns

- No CGO-based SQLite drivers.
- No business logic inside `cmd/dockerpull/main.go`.
- No duplicated registry URL construction or digest verification logic across packages.
- No silent fallback that hides a failed mirror or failed verification.
- No package-level god objects that combine CLI parsing, downloading, persistence, and export.

---

## Testing Requirements

- At minimum, `go test ./...` and `go build ./cmd/dockerpull` must pass.
- For behavior that depends on registries or mirrors, prefer small unit tests around pure functions plus manual end-to-end verification against public registries.
- Add tests first for parsing-heavy packages (`cli`, `reference`, `manifest`) when extending functionality.

---

## Real Code Examples

1. Separation of concerns:
   - `internal/cli/options.go` parses and validates flags.
   - `internal/reference/reference.go` normalizes image references.
   - `internal/manifest/resolve.go` handles media type and platform selection.
2. Reuse over duplication:
   - `internal/app/run.go` relies on `cache.BlobPath`, `cache.ExistsAndValid`, and `cache.VerifyDigest`.
   - `internal/registry/client.go` centralizes registry URL construction and mirror fallback.
3. Persistence only after verification: `internal/app/run.go` verifies each downloaded blob before calling `store.UpsertBlobState`.

---

## Code Review Checklist

- Does the change preserve the no-CGO requirement?
- Are mirrors merged and ordered correctly between CLI flags and `DOCKER_MIRRORS`?
- Are credential-required mirrors skipped safely when no user credentials are available?
- Is manifest/blob resolution retry bounded and kept inside `internal/registry/`?
- Is manifest media type handling robust to `Content-Type` parameters?
- Are config blobs and layer blobs both handled?
- Does export code only run after cache validation succeeds?
- Is the new logic placed in the package that owns the responsibility?

---

## Common Mistakes

- Adding a shortcut implementation in `app.Run` instead of extending a focused package.
- Repeating string parsing or URL normalization logic that already exists.
- Accepting successful downloads without digest validation.
