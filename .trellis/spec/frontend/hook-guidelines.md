# Hook Guidelines

> How hooks are used in this project.

---

## Overview

There is currently no frontend hook system in this repository because there is no frontend framework in use.

The only "hooks" present today are Go function calls and package boundaries, not React-style custom hooks.

---

## Current Reality

- No `use*` functions exist.
- No React Query, SWR, Zustand hooks, or framework-specific data-fetch abstractions exist.
- Stateful behavior is implemented through explicit Go structs and functions, such as `state.Store`, `registry.Client`, and `scheduler.Run`.

---

## If Frontend Hooks Are Added Later

- Introduce them only after a real frontend layer exists.
- Define naming rules, side-effect boundaries, and data-fetch ownership from actual code.
- Add examples from real `use*` functions rather than copying framework defaults.

---

## Real Repository Evidence

1. `internal/state/db.go` models persistent state through Go types and methods, not hooks.
2. `internal/registry/client.go` handles remote fetch behavior through a typed client.
3. `internal/app/run.go` composes stateful workflow directly in Go orchestration code.

---

## Anti-Patterns

- Do not document React-style hook conventions that the repository does not use.
- Do not confuse Go helper functions with frontend hooks.
- Do not add data-fetching libraries solely to match this empty template.

---

## Common Mistakes

- Assuming every Trellis frontend template implies actual hook usage.
- Describing server-state hooks when the codebase only has CLI-driven Go execution.
