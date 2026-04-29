# Component Guidelines

> How components are built in this project.

---

## Overview

This repository currently has no frontend component layer. There are no React, Vue, Svelte, or other UI component files in the codebase.

This file exists to prevent assistants from hallucinating component patterns that are not present.

---

## Current Reality

- No component directories such as `src/components/`, `app/components/`, or `ui/components/` exist.
- No JSX/TSX files exist in the repository.
- User interaction is currently through CLI flags and stdout/stderr, not reusable UI components.

---

## If Components Are Introduced Later

- Define a dedicated frontend root first.
- Standardize component file structure, props typing, styling approach, and accessibility rules only after real component code exists.
- Update this guide with examples from actual component files instead of hypothetical rules.

---

## Real Repository Evidence

1. `README.md` only documents CLI usage and flags.
2. `go.mod` shows a pure Go module with no frontend package ecosystem.
3. `cmd/dockerpull/main.go` exposes the current interface as a command-line entrypoint rather than a rendered UI.

---

## Anti-Patterns

- Do not add placeholder component standards copied from another project.
- Do not claim prop, composition, or styling conventions before a UI exists.
- Do not treat CLI option parsing as a substitute for component architecture.

---

## Common Mistakes

- Writing generic React guidance into a backend-only repository.
- Adding frontend terms like props/hooks/state here without code examples to justify them.
