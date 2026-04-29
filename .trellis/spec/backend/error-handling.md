# Error Handling

> How errors are handled in this project.

---

## Overview

This CLI propagates errors upward with context and prints a single final error in `cmd/dockerpull/main.go`.

---

## Error Types

- Prefer wrapped errors with `fmt.Errorf("context: %w", err)`.
- Add dedicated custom error types only when the caller must branch on behavior.
- Include operation context such as mirror endpoint, digest, image reference, or export stage.
- Keep user-visible output short by formatting once at the process boundary.

---

## Error Handling Patterns

- Validate CLI input early in `internal/cli/options.go`.
- Return errors from lower layers; do not log and swallow them.
- Keep retry/fallback logic inside the registry layer for mirror handling instead of duplicating it in callers.
- Treat mirrors that return auth-required responses without usable user credentials as skippable endpoints; continue to the next mirror or source endpoint instead of aborting the whole pull.
- Keep bounded request-level retries for transient manifest/blob resolution failures inside the registry layer, not in `app` or `aria2c` orchestration.
- After external side effects like downloads, verify the result before persisting success state.
- Let orchestrators compose steps and bubble failures upward rather than partially continuing after critical errors.

---

## User-Facing Error Behavior

This is a CLI, not an HTTP service. User-facing errors should be short and emitted once by the main package.

---

## Real Code Examples

1. Single exit point for user-visible failures: `cmd/dockerpull/main.go` prints `error:` once and exits non-zero.
2. Early validation returns clear errors:
   - `internal/cli/options.go` rejects missing `-o`, invalid worker counts, and mutually exclusive `--docker/--oci`.
   - `internal/reference/reference.go` rejects empty or unsupported image references.
3. Wrapped external-command failure: `internal/downloader/aria2c.go` returns `fmt.Errorf("aria2c download %s failed: %w", task.Digest, err)`.
4. Context-rich HTTP failures: `internal/registry/client.go` includes endpoint and response status when manifest/blob requests fail.

---

## Anti-Patterns

- Do not ignore digest verification failures after download.
- Do not hide which mirror or registry endpoint failed when wrapping HTTP/download errors.
- Do not partially mark blobs as successful before validation and state write complete.
- Do not print the same failure in lower layers and again in `main`.

---

## Common Mistakes

- Returning bare external errors without enough mirror/digest context.
- Mixing retry policy into `app` instead of keeping it in `registry`.
- Continuing export after a failed cache validation or failed state update.
