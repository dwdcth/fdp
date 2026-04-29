# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

This project uses SQLite only for local state tracking. The driver must be pure Go and must not require CGO.

Current implementation uses `modernc.org/sqlite` in `internal/state/db.go`.

---

## Query Patterns

- Open the database through a thin wrapper (`state.Open`) instead of scattering `sql.Open` calls.
- Keep schema creation in code via a migration/bootstrap function because this is a single-binary CLI tool.
- Prefer explicit `Upsert*` helper methods for writes.
- Track image-level and blob-level state separately.
- Keep callers working with typed structs (`ImageState`, `BlobState`) instead of raw SQL rows.

---

## Migrations

- Migrations are currently bootstrap SQL executed in `(*Store).migrate`.
- Schema changes should remain backward-compatible where possible because users may reuse an existing local cache directory.
- If schema changes become complex, add versioned migration steps rather than ad-hoc SQL in multiple places.

---

## Naming Conventions

- Tables use snake_case: `image_state`, `blob_state`.
- Columns use snake_case and describe persisted facts, e.g. `image_manifest_digest`, `platform_arch`.
- Keep uniqueness constraints aligned to the natural lookup key, e.g. image state is unique by registry/repository/reference/platform.

---

## Real Code Examples

1. Database bootstrap lives in one place: `internal/state/db.go` in `(*Store).migrate`.
2. Read path uses a dedicated query method: `internal/state/db.go` in `(*Store).GetImageState`.
3. Write paths use explicit upserts:
   - `internal/state/db.go` in `(*Store).UpsertImageState`
   - `internal/state/db.go` in `(*Store).UpsertBlobState`
4. Callers consume the store through a wrapper instead of raw SQL: `internal/app/run.go` opens the store with `state.Open` and persists only after verification succeeds.

---

## Anti-Patterns

- Do not introduce `mattn/go-sqlite3`; that would add a CGO dependency and break the no-CGO requirement.
- Do not treat file existence as cache validity; persist only after digest verification succeeds.
- Do not write SQL inline across unrelated packages; keep persistence logic under `internal/state/`.
- Do not update blob or image success state before download verification finishes.

---

## Common Mistakes

- Bypassing `state.Open` and forgetting parent-directory creation or migration.
- Encoding transient runtime concerns into schema instead of persisting stable facts.
- Letting schema and query ownership leak into `app`, `cache`, or `registry` packages.
