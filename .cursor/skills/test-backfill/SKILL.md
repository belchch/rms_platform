---
name: test-backfill
description: >-
  Guides writing tests for existing code in a project without prior design stage.
  Covers how to pick candidates by invariant and bug cost, when architectural changes
  are allowed (and how to register them as ADR), and how to exit the transitional mode.
  Use when covering existing code with tests, picking test candidates, adding tests to
  auth or sync handlers, writing backfill tests, or when the user mentions "обложить
  тестами", "покрытие", "backfill", "seam", "инвариант", "кандидат для теста".
---

# Test backfill skill

Applies during the transitional phase documented in `docs/adr/0003-test-backfill-transitional.md`.
Baseline metrics: `docs/planning/testing-baseline.md`.

## Picking a candidate

Accept a candidate if ALL hold:

1. The invariant fits in one sentence.
2. Hot path only: `internal/handler/auth`, `internal/handler/sync`, `internal/handler/photos`, `internal/middleware`, `internal/jwtutil`.
3. There is a pure function (unit test, no mocks) OR an explicit seam (`db.Querier`, `storage.Provider`).
4. Not on the stop-list: `internal/db/*`, thin mappers, plain DTOs, single-line delegate wrappers, trivial pure functions whose contract is obvious from reading the code (one `if`-branch, fixed error message, no non-trivial logic).

## Writing the test

1. Read the function **signature only** first. Write 2–3 expected behaviours before reading the body.
2. Open the body. Any mismatch between your expectation and the implementation is a bug or a gap in the invariant.
3. Write tests against the invariant, not the implementation details.
4. Sanity-check: would the test catch a plausible mutation in the implementation?

## Architecture is not constant

If a test is impossible without an architectural change (fused I/O + decision logic per `pure-vs-io.mdc`, missing seam), the fix is allowed under these conditions:

- Extract a pure function OR introduce a seam (`db.Querier`-style interface).
- Open a separate ADR for the change before merging.
- A structural change without an ADR is a hack under `no-hacks.mdc`.
- Split the refactor and the test into separate PRs.

## Exit condition (transitional mode is done when)

- `auth`: refresh token rotation + hash storage covered; access claims expiry → 401; workspace isolation.
- `sync`: LWW by `clientUpdatedAt`; `sync_cursor` monotonicity; soft-delete tombstone in pull; workspace isolation in pull/push.
- Endpoint coverage >= 50% (3 of 6 routes from `testing-baseline.md`).

When done: close ADR-0003, remove the Transitional mode paragraph from `testing-goals.mdc`, close Linear project — one commit.
