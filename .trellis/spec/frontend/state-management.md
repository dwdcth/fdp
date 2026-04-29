# State Management

> How state is managed in this project.

---

## Overview

There is no frontend state-management layer in this repository. State is managed in Go code through local variables, typed structs, and SQLite-backed persistence for cache metadata.

---

## Current State Categories

- In-memory workflow state lives in function scope inside orchestration code such as `internal/app/run.go`.
- Persistent local state lives in SQLite through `internal/state/db.go`.
- Download/cache validity is derived from filesystem state plus digest verification in `internal/cache/verify.go`.
- Remote registry state is fetched on demand through `internal/registry/client.go` and is not cached in a browser client.

---

## Real Code Examples

1. In-memory orchestration state: `internal/app/run.go` tracks descriptors, pending tasks, and cache-hit decisions within `Run`.
2. Persistent state wrapper: `internal/state/db.go` defines `Store`, `ImageState`, and `BlobState`.
3. Derived validity state: `internal/cache/verify.go` computes whether a cached blob is usable from file metadata plus digest verification.

---

## If Frontend State Is Added Later

- Separate local UI state, server state, and persisted client state explicitly.
- Do not mirror backend persistence rules into a browser store by default.
- Add real conventions only after choosing a frontend stack.

---

## Anti-Patterns

- Do not invent Redux/Zustand/React Query rules before a frontend exists.
- Do not duplicate persistence facts in multiple stores when SQLite already owns local state tracking.
- Do not treat file existence alone as authoritative state; verification is part of the state model.

---

## Common Mistakes

- Confusing backend persistence with frontend state management because both use the word "state".
- Assuming a global UI store exists when the project only has Go runtime state and SQLite metadata.
