---
name: readability-first-refactor
description: >-
  Refactors application code with emphasis on readable boundaries and thin UI
  shells rather than maximal test decoupling. Use when refactoring features,
  extracting orchestration, or when the user wants a clarity-first structure.
---

# Refactoring: readability and boundaries

## Overall direction

**Readability always comes first.** Everything else is situational—avoid dogma that hurts clarity.

## What matters more

1. **Clear roles and module boundaries** matter more than isolation for tests’ sake. Tests help; they are not the only guide.

2. Keep **input and presentation** **thin**: they invoke APIs and do not duplicate domain rules.

3. **Each behavioral aspect** belongs in its **own class** (or an explicit role split)—not scattered across UI handlers.

4. Keep the **domain scenario** in **one place** (service, session, use case, coordinator).

## Orchestration dependencies

Scenario orchestrators **may depend directly** on DI, global state, and app providers when that is simpler and more faithful than abstractions “for testability.” **Do not** add indirection layers solely to enable mocking in tests unless the user asks for that.

## Duplication

Repeated small steps (one calculation, one drawing primitive) should be **one shared function or helper**, not copy-pasted across components.

## After changes

Run static analysis on touched files; add or run tests for **regression risk**, not as a mandatory step after every refactor.
