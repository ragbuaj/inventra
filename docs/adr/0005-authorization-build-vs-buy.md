# ADR-0005 — Authorization: keep the custom 3-layer model (vs Casbin / OpenFGA)

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #1 |

## Context and problem statement

Inventra's authorization is a **custom 3-layer, data-driven, Redis-cached** model keyed by the caller's
`role_id` (resolved from the JWT):

1. **Action permissions** (`role_permissions`) — boolean keys (`asset.create`, `request.approve`, …).
2. **Data scope** (`data_scope_policies`) — per-row visibility over the **office hierarchy**
   (`global` / `office_subtree` / `office` / `own`), resolvable per module, with `office_subtree`
   expanded from the offices tree.
3. **Field permissions** (`field_permissions`) — per-`(entity, field, role)` view/edit flags,
   default-allow.

The question (backlog #1): should we **replace** part or all of this with a battle-tested authorization
library/service — **Casbin** (policy engine) or **OpenFGA** (Google-Zanzibar-style ReBAC) — per the
project principle of preferring industry standards over hand-rolled solutions?

## Decision drivers

- **Domain fit:** the hard part is **office-subtree data scope** (per-row, hierarchy-aware) and
  **field-level** permissions — both tightly coupled to our schema and query layer.
- **Best practice ≠ "always buy":** the principle is *use mature libraries unless the domain genuinely
  needs custom* — and document the trade-off either way.
- **Migration cost & risk** vs. benefit; the model is already built, tested, and in use.
- Keep authorization **configurable by Superadmin** (data-driven tables), which it already is.

## Considered options

1. **Keep the custom 3-layer model.** Already implemented, data-driven, Redis-cached, enforced in the
   service layer on read **and** write.
2. **Adopt Casbin** for the action-permission layer (RBAC), keep custom scope + field perms. Casbin
   models RBAC well, but per-row office-subtree scope and field-level masking are **not** what Casbin
   does naturally — they'd remain custom anyway, so Casbin would only replace the simplest layer.
3. **Adopt OpenFGA (ReBAC).** Powerful for relationship graphs and could model office-subtree as
   relations, but it's a **separate service** (new infra, latency, ops), and field-level masking +
   query-time row filtering (`office_id IN (subtree)`) still live in our SQL/service layer. High
   integration cost for marginal gain.

## Decision outcome

**Chosen: Option 1 — keep the custom model.** Reasons:

- The **valuable, hard** parts (office-subtree row scoping, field masking) are domain-specific and
  already integrated with sqlc queries and the office tree; no library removes that work.
- Casbin would only replace the **boolean RBAC** layer — the least complex part — while adding a
  dependency and a second source of truth for permissions.
- OpenFGA adds a **new runtime component** and network hop without eliminating the SQL-level row
  filtering or field masking we still must do.
- The current model is **data-driven and Superadmin-configurable** (the main reason teams reach for a
  policy engine) and **Redis-cached** with explicit invalidation.

This is a deliberate **build** decision, recorded so the trade-off is explicit — not a default.

## Consequences

- 👍 No new dependency/service; one coherent, schema-aware model; full control over row-scope + field
  masking; already tested and enforced server-side on read and write.
- 👍 Configurability (the usual reason to adopt a policy engine) is already met via the `*_policies` /
  `*_permissions` tables.
- 👎 We own the authorization code and its correctness (caching, invalidation, scope edge-cases) — must
  keep it covered by tests (ADR-0001), especially IDOR/scope-bypass cases.
- 👎 No external policy-as-code ecosystem (tooling, audit formats) → acceptable at current scope.

## Revisit if

- Authorization needs cross-service sharing (multiple services consuming the same policies), or
- Relationship-based rules grow beyond the office tree (arbitrary resource graphs), or
- A compliance requirement mandates a standardized external policy store/audit.

In any of those cases, reconsider **OpenFGA** for the relationship/scope layer and supersede this ADR.
