# Fix mirror blob auth

## Goal

Fix `dockerpull` so mirror mode can successfully pull `nginx:latest` end-to-end when manifests resolve but blob downloads currently fail with `aria2c` 401/auth errors.

## What I already know

- User wants the real mirror-mode pull path fixed, not just unit tests.
- Real validation with `nginx:latest` and the provided mirrors currently fails during blob download.
- Manifest fetch, cache directory creation, manifest persistence, and SQLite state DB creation already work.
- Failure happens after `internal/app/run.go` resolves blob download tasks and invokes `downloader.Download`.
- `internal/registry/client.go` currently strips auth headers on cross-host redirects in `followBlobRedirect`.
- `internal/downloader/aria2c.go` only forwards the headers returned by `DownloadLocation.Headers`.
- No archive output file is produced in the failed real run.

## Assumptions (temporary)

- The primary bug is in mirror/blob auth handling rather than archive export.
- Some mirrors redirect blob requests to another host that still needs auth or another re-resolution step.
- Fixing redirect/token handling should be enough to make the mirror path work for the provided mirrors.

## Open Questions

None for MVP.

## Requirements

- Mirror-mode pulls must successfully download config and layer blobs for `nginx:latest`.
- Mirrors that require credentials but have no user-provided credentials available must be ignored/skipped automatically, with fallback continuing to the next mirror or source endpoint.
- Direct/no-mirror pulls should tolerate a small amount of transient request failure through limited request-level retries on manifest/blob resolution.
- The fix must preserve token safety and avoid leaking bearer headers to unrelated hosts.
- Redirect handling must work when mirrors return blob locations on alternate hosts or require another authenticated resolution step.
- The program must still verify blob digests before persisting success state.
- End-to-end validation must use a real image pull, not only synthetic tests.

## Acceptance Criteria

- [ ] `dockerpull nginx:latest` with the provided mirrors completes successfully in mirror mode.
- [ ] Mirrors that require username/password or other missing credentials are skipped automatically instead of aborting the whole pull.
- [ ] Direct/no-mirror pulls do not regress and use the improved fallback/auth path where applicable.
- [ ] Manifest/blob resolution tolerates limited transient request failure through bounded retries.
- [ ] A Docker archive output file is produced and is non-empty.
- [ ] Downloaded blobs are verified and persisted only after successful validation.
- [ ] `go test ./...` passes.
- [ ] `go build ./cmd/dockerpull` passes.

## Definition of Done (team quality bar)

- Tests added/updated where the behavior can be covered deterministically
- Lint / typecheck / CI-equivalent local checks green
- Specs/docs updated if the auth/redirect contract becomes clearer
- Real pull verification completed after code changes

## Technical Approach

- Refine `internal/registry/client.go` so blob URL resolution handles mirror redirects and auth boundaries correctly.
- Keep bearer tokens scoped safely: never blindly forward credentials to unrelated hosts, but allow host-aware re-resolution or re-auth when required.
- Add small bounded retries around manifest/blob resolution for transient request failures without redesigning the downloader.
- Preserve the current download pipeline in `internal/app/run.go` and `internal/downloader/aria2c.go`, changing only the data needed to make auth/fallback reliable.
- Add deterministic tests for the registry client behavior where possible, then validate with a real `nginx:latest` pull.

## Decision (ADR-lite)

**Context**: Mirror-mode blob downloads currently fail with 401/auth errors after manifest resolution succeeds. Direct pulls also show transient network failures in this environment.

**Decision**: Fix the registry auth/redirect path first, and add a small amount of request-level retry for manifest/blob resolution rather than introducing a larger retry framework.

**Consequences**: Scope stays focused on the registry client and current fallback path, while improving resilience for both mirror and direct pulls. We accept that this does not solve every possible network issue or redesign downloader retries.

## Out of Scope

- General Docker Hub network instability unrelated to mirror auth
- New CLI flags or large downloader architecture changes unless required for the fix
- Full retry policy redesign beyond what is needed for this auth failure

## Technical Notes

- Suspect files: `internal/registry/client.go`, `internal/downloader/aria2c.go`, `internal/app/run.go`
- Real failed run showed blob URLs on redirected hosts such as `docker.1ms.run` and `mirror.houlang.cloud`.
- Current redirect logic clears headers when `parsed.Host != endpointURL.Host`, which is safe but may break mirrors that require host-specific re-auth.
- README promises mirror fallback and real blob downloads via `aria2c`, so the current behavior violates the documented feature.
- Retry scope for MVP should stay bounded to manifest/blob resolution rather than download-body retries inside `aria2c`.

