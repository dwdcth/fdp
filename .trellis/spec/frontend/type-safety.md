# Type Safety

> Type safety patterns in this project.

---

## Overview

There is no TypeScript or frontend type layer in this repository today. Type safety comes from Go's static type system and explicit domain structs.

---

## Current Reality

- Frontend-specific type organization does not exist because there is no frontend code.
- Backend/domain types are expressed in Go structs such as `cli.Options`, `reference.ImageReference`, `registry.Platform`, `manifest.Descriptor`, `state.ImageState`, and `state.BlobState`.
- Runtime validation is performed by explicit parsing and validation functions rather than TypeScript schema libraries.

---

## Real Code Examples

1. CLI input is parsed into a typed struct in `internal/cli/options.go` (`Options`).
2. Image references are normalized into `reference.ImageReference` in `internal/reference/reference.go`.
3. Persistent records use explicit structs in `internal/state/db.go` (`ImageState`, `BlobState`).
4. Platform parsing and manifest decoding use typed representations in `internal/manifest/resolve.go`.

---

## Validation Patterns

- Parse external strings into typed values early.
- Keep validation close to the boundary where untrusted input enters.
- Prefer explicit structs and methods over `map[string]any` for stable domain data.

---

## Anti-Patterns

- Do not write TypeScript-specific rules for a repository that does not contain TypeScript.
- Do not replace stable typed structs with untyped maps in Go.
- Do not postpone validation of CLI or registry input until deep inside export or persistence code.

---

## Common Mistakes

- Assuming this template should mention `any`, `unknown`, or Zod when no TS toolchain exists.
- Letting stringly typed values leak across multiple packages before normalization.
