# Logging Guidelines

> How logging is done in this project.

---

## Overview

The current CLI does not use a dedicated logging library yet. Output is intentionally minimal: the binary prints a single final error line on failure, while `aria2c` streams its own progress output.

---

## Output Strategy

- Default command output should stay quiet unless the external downloader is running.
- Fatal or user-actionable failures should go to stderr.
- Detailed operational tracing should only be added behind an explicit verbose/debug mode.

---

## Log Levels

- Use plain stderr errors for user-visible failures.
- Treat `aria2c` stdout/stderr as operational progress output, not as application-level structured logs.
- If structured logging is introduced later, keep default output concise and make verbose/debug output opt-in.

---

## Structured Logging

- No structured logger is in use today.
- If added later, include fields such as image reference, mirror endpoint, digest, platform, and stage (`manifest`, `blob`, `export`).

---

## Real Code Examples

1. Final failure output is centralized in `cmd/dockerpull/main.go`.
2. External tool progress is streamed directly in `internal/downloader/aria2c.go` via `cmd.Stdout = os.Stdout` and `cmd.Stderr = os.Stderr`.
3. Registry and cache layers currently return rich errors instead of logging internally:
   - `internal/registry/client.go`
   - `internal/cache/verify.go`

---

## What to Log

- Final command failure with enough context to debug.
- Mirror fallback decisions when verbose logging is added.
- Export mode and target path when adding informational logs.
- Cache hit/miss summaries if future observability is added.

---

## What NOT to Log

- Bearer tokens or authorization headers.
- Full signed blob redirect URLs if they contain temporary credentials.
- Repetitive per-chunk download noise beyond what `aria2c` already emits.
- Duplicate errors from both leaf packages and the main package.

---

## Anti-Patterns

- Do not introduce always-on debug spam in a CLI meant for piping and scripting.
- Do not log and also return the same error.
- Do not log secrets from registry auth or redirected download URLs.
