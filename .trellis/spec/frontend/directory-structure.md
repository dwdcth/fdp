# Directory Structure

> How frontend code is organized in this project.

---

## Overview

There is currently no frontend code in this repository. The project is a single Go CLI application, so this frontend layer is intentionally empty for now.

Documenting this absence is important: AI should not invent React/Vue/TypeScript structure that does not exist.

---

## Current Reality

- No `package.json`, `tsconfig.json`, or `src/` tree exists in the repository root.
- Runtime code lives under Go directories such as `cmd/` and `internal/`.
- The only documented product surface today is the CLI described in `README.md`.

---

## If Frontend Code Is Added Later

- Create a dedicated top-level directory such as `web/`, `ui/`, or `frontend/` instead of mixing frontend files into `internal/`.
- Keep build tooling (`package.json`, lockfile, bundler config) at that frontend root.
- Add a real directory layout section here once frontend files exist.

---

## Real Repository Evidence

1. `README.md` describes `dockerpull` as a Go + `aria2c` CLI.
2. `go.mod` defines a Go module and contains no frontend toolchain dependencies.
3. `cmd/dockerpull/main.go` is the executable entrypoint, confirming the current product surface is the CLI.

---

## Anti-Patterns

- Do not create fictional frontend conventions just to fill this file.
- Do not place browser UI code under `internal/` alongside backend packages.
- Do not add a frontend toolchain without also updating this spec and the project README.

---

## Common Mistakes

- Assuming the presence of React components or hooks because Trellis generated a frontend layer template.
- Mixing documentation for a possible future UI with the current backend-only repository reality.
