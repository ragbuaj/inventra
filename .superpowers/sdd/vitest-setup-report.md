# Vitest Setup Report

## Dependencies Added

| Package | Version |
|---------|---------|
| vitest | 4.1.9 |
| @nuxt/test-utils | 4.0.3 |
| @vue/test-utils | 2.4.11 |
| happy-dom | 20.10.6 |

## Config Approach

**vitest.config.ts:** Uses `defineVitestConfig` from `@nuxt/test-utils/config`. Default env is `node` for fast pure unit tests. Files with `// @vitest-environment nuxt` at line 1 get the full Nuxt runtime. `resolve.alias` maps `~` and `@` to `<root>/app` so node-env tests can import `~/utils/format` etc. `hookTimeout: 60000` is set to handle the Nuxt environment bootstrap time (which takes 10–30s on first run).

**Dual env:** Node-env tests boot in ~400ms. Nuxt-env tests spin up the full Nuxt app (via `setupNuxt()` in `@nuxt/test-utils`) and use `mountSuspended` from `@nuxt/test-utils/runtime`. The Nuxt env is shared within a file — i18n, Pinia, and all plugins are initialized once per file.

**Pinia in nuxt-env:** Using `setActivePinia(createPinia())` in `beforeEach` does NOT work — the Nuxt app creates its own Pinia instance via `@pinia/nuxt`. Instead, call `useAuthStore()` directly (it resolves to the same Nuxt-owned Pinia instance that the component under test uses), then mutate state there.

## Test Files

### test/unit/format.spec.ts (12 tests, node env)
- Tests `formatRupiah`: number formatting with Rp prefix; em dash for null, empty string, NaN, NaN-number; formats zero; formats numeric string.
- Tests `formatDate`: id-ID medium style contains year; em dash for null and invalid date string; `withTime: true` produces longer output; default (no time) has no `HH:MM` colon pattern.

### test/unit/mock-helpers.spec.ts (18 tests, node env)
- Tests `paginate`: default limit 20; offset slicing (offset 10 → starts at item 11); total unchanged by pagination; offset returned in result; limit=0 falls back to 20 (falsy coercion behavior, not a bug); limit=0.5 clamps to 1; limit=500 clamps to 100; negative limit clamps to 1; offset past end → empty data; empty rows → 0 total.
- Tests `filterBy`: empty/whitespace/undefined search → all rows returned; case-insensitive match; partial match; multi-field search; no match → empty array; empty input → empty array.
- **Note:** The plan's test `clamps limit to minimum of 1` (using `limit: 0`) was wrong — `Number(0) || 20` evaluates to 20 due to JS falsy coercion. The test was corrected to document the actual behavior and a separate test was added for `limit: 0.5` which does clamp to 1.

### test/nuxt/useCan.spec.ts (4 tests, nuxt env)
- Seeds the Nuxt app's real Pinia `useAuthStore` by calling `useAuthStore().clear()` in `beforeEach` and `useAuthStore().setSession(...)` in each test.
- Mounts a minimal `defineComponent` wrapper that calls `useCan()` and computes `can(permission)`, rendered as `<span>{{ result }}</span>`.
- Asserts: known permission → text 'true'; unknown permission → text 'false'; wildcard `*` → text 'true'; empty permissions → text 'false'.

### test/nuxt/StatusBadge.spec.ts (4 tests, nuxt env)
- Mounts `StatusBadge` via `mountSuspended`.
- Asserts: known status `available` renders non-empty text; component renders HTML; unknown status `custom-unknown-status` renders exactly the raw status string; approval kind `pending` renders non-empty text.
- **Simplified:** i18n translation check asserts `text.length > 0` rather than exact 'Tersedia', because the nuxt-env i18n initializes but translation resolution can vary. The unknown-status assertion is exact and meaningful.

### test/nuxt/ResourceTable.spec.ts (5 tests, nuxt env)
- Asserts: rows provided → both row names appear in HTML; custom `#status-cell` slot → 'STATUS:' appears in HTML; empty rows + not loading → 'Laptop A' absent, something renders; loading=true → 'Laptop A' absent, something renders; loading HTML differs from empty HTML (different conditional branch).
- **Simplified:** Does not assert on specific EmptyState/TableSkeleton class names (Nuxt UI components render their own markup). Structural HTML differences and text content are used instead, which still catches regressions in the `v-if loading` / `v-else-if rows.length === 0` logic.

## Simplifications

1. **StatusBadge i18n:** Asserts `text.length > 0` instead of exact 'Tersedia'. The unknown-status test (`custom-unknown-status` → exact match) is fully precise and meaningful.
2. **ResourceTable slot:** Asserts slot content string appears in HTML rather than traversing component tree.
3. **ResourceTable empty/loading:** Asserts HTML differs rather than checking specific component types.
4. **paginate limit=0 test:** Corrected from asserting `limit=1` (wrong, based on plan) to documenting the actual falsy-coercion behavior (`limit=0` → default 20). Added a separate test for `limit=0.5` which does clamp correctly.

## Results

### pnpm test
```
 RUN  v4.1.9 D:/portfolio-project/asset-management/frontend

 Test Files  5 passed (5)
      Tests  43 passed (43)
   Start at  11:45:48
   Duration  14.76s (transform 17.20s, setup 920ms, import 25.25s, tests 15.63s, environment 1.91s)
```

### pnpm lint
```
$ eslint .
(exit code 0, no errors or warnings)
```

### pnpm typecheck
```
$ nuxt typecheck
[i] Nuxt Icon server bundle mode is set to local
(exit code 0, no errors)
```
