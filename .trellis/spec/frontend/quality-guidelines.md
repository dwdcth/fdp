# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

There is currently no frontend implementation in this repository, so there are no frontend-specific lint, test, accessibility, or component review rules in active use.

The quality rule for this layer today is: do not pretend a frontend exists.

---

## Current Quality Standard

- Document the absence of frontend code accurately.
- Keep frontend templates aligned with repository reality until a UI is introduced.
- When adding a frontend in the future, create real tooling and examples first, then update this guide.

---

## Real Repository Evidence

1. `README.md` documents only CLI usage.
2. `go.mod` is the active build manifest and contains no frontend package ecosystem.
3. No `package.json`, `tsconfig.json`, `src/`, or JSX/TSX files exist in the repository.

---

## Forbidden Patterns

- Do not copy generic frontend standards from unrelated projects.
- Do not claim Jest/Vitest/Playwright/ESLint rules that are not configured here.
- Do not mark frontend accessibility checks as passing when there is no rendered UI to assess.

---

## Required Patterns If Frontend Work Starts Later

- Add the frontend toolchain and document it explicitly.
- Add real examples for component, state, and type conventions.
- Define lint/test/accessibility gates from the chosen stack, not from Trellis templates.

---

## Common Mistakes

- Treating an empty template as proof of an existing frontend architecture.
- Writing requirements for nonexistent components, hooks, and browser tests.
