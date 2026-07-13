# UX Fixes Batch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship 7 independent UX/correctness fixes: a reusable numeric input, wired profile + verified email change, security-tab password-change-by-email, forgot-password sizing + tiered resend, global-search autofocus, office-map card z-index, and PDF/CSV mojibake fixes.

**Architecture:** Five fixes are frontend-only (Nuxt 4 SPA); two (#2 profile/email, #3 password) add backend endpoints reusing the existing token-store + async-mailer infra; one (#7) is backend PDF/CSV encoding. Companion spec: `docs/superpowers/specs/2026-07-13-ux-fixes-batch-design.md`.

**Tech Stack:** Go 1.25 + Gin + pgx/sqlc + `go-pdf/fpdf` + `xuri/excelize` + Redis; Nuxt 4 + `@nuxt/ui` (U* components) + Vitest/@nuxt/test-utils + Playwright; i18n `id`/`en`.

## Global Constraints

- **Frontend components:** compose `U*` (Nuxt UI) only; extract reusable pieces into `app/components/`; keep pages thin.
- **Theme via tokens:** semantic color props / CSS vars, never literal Tailwind colors.
- **i18n mandatory:** every user-facing string in `i18n/locales/{id,en}.json`, referenced via `t('key')`. Default locale `id`.
- **Lint:** ESLint stylistic — **no trailing commas** (`commaDangle: 'never'`), 1tbs braces. `pnpm lint` + `pnpm typecheck` must pass.
- **API access:** through `runtimeConfig.public.apiBase` / `useApiClient`; never hardcode backend URL.
- **Backend money/numeric columns:** Go `string` (sqlc override). Soft-delete + partial-unique + `set_updated_at` conventions for new columns/tables.
- **Auth self-service endpoints** use `RequireAuth` only (mirror `/auth/password`, `/auth/me`) — no `RequirePermission`.
- **OpenAPI:** hand-maintained `backend/api/openapi.yaml`, Spectral-linted — keep in sync with route changes.
- **sqlc:** never hand-edit `db/sqlc/`; edit `db/queries/*.sql` or migrations and run `sqlc generate`.
- **CI gates:** backend `go build ./... && go vet ./... && go test ./...` + Spectral; frontend `pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
- **Commits:** Conventional Commits with scope; no AI co-author trailers.
- **Update `docs/PROGRESS.md`** when the batch lands.

---

## File Structure

**New files**
- `frontend/app/components/NumberInput.vue` — reusable numeric input.
- `frontend/app/composables/useResendCooldown.ts` — exponential resend cooldown.
- `frontend/app/pages/verify-email.vue` — email-change confirmation landing.
- `frontend/test/nuxt/NumberInput.spec.ts`, `frontend/test/unit/useResendCooldown.spec.ts`, `frontend/test/nuxt/verify-email.spec.ts`.
- `backend/db/migrations/000034_employee_phone.up.sql` / `.down.sql`.
- `backend/internal/auth/emailchange.go` — email-change token store.
- `backend/internal/pdfutil/font.go` + embedded `backend/internal/pdfutil/fonts/DejaVuSans*.ttf` + `LICENSE`.
- `backend/internal/email/templates/email_change_verify.{html,txt}`, `email_changed.{html,txt}`.

**Modified files**
- Frontend form sites (NumberInput rollout): `components/asset/AssetForm.vue`, `components/category/CategoryFormSlideover.vue`, `components/maintenance/RecordSlideover.vue`, `components/maintenance/ScheduleSlideover.vue`, `pages/disposals.vue`, `pages/depreciation.vue`, `pages/master/offices.vue`.
- `frontend/app/utils/format.ts` (host `formatThousands`/`parseThousands`); `frontend/app/constants/categoryMeta.ts` (re-export).
- `frontend/app/pages/account.vue`; `frontend/app/composables/api/useAccount.ts`; `frontend/app/types/index.ts`.
- `frontend/app/pages/forgot-password.vue`; `frontend/app/components/CommandPalette.vue`; `frontend/app/pages/master/map.vue`.
- `frontend/i18n/locales/{id,en}.json`.
- Backend: `internal/identity/{routes,handler,service,dto}.go`; `internal/email/mailer.go`; `db/queries/identity.sql` + `db/queries/employees.sql`; `internal/server/router.go` (only if new deps needed); `internal/report/export.go`; `internal/depreciation/export.go`; `internal/stockopname/report.go`; `internal/asset/barcode.go`; `internal/importer/template.go`; `internal/importer/errreport.go`; `backend/api/openapi.yaml`.

---

# PART A — Frontend-only fixes (independent; parallelizable)

## Task 1: Move thousand-format helpers into `utils/format.ts`

**Files:**
- Modify: `frontend/app/utils/format.ts`
- Modify: `frontend/app/constants/categoryMeta.ts`
- Test: `frontend/test/unit/format.spec.ts` (create or extend)

**Interfaces:**
- Produces: `formatThousands(v: string | number | null | undefined): string`, `parseThousands(v: string | null | undefined): string` (auto-imported from utils).

- [ ] **Step 1: Write failing test** in `frontend/test/unit/format.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { formatThousands, parseThousands } from '~/utils/format'

describe('thousand helpers', () => {
  it('groups digits id-ID', () => {
    expect(formatThousands('1000000')).toBe('1.000.000')
    expect(formatThousands(2500)).toBe('2.500')
  })
  it('strips non-digits before grouping', () => {
    expect(formatThousands('1.0a0b0')).toBe('100')
  })
  it('parses back to raw digits', () => {
    expect(parseThousands('1.000.000')).toBe('1000000')
    expect(parseThousands('')).toBe('')
  })
})
```

- [ ] **Step 2: Run — expect FAIL** (`import` from utils not found): `pnpm test -- format.spec`
- [ ] **Step 3:** Copy the two functions from `constants/categoryMeta.ts` into `utils/format.ts` (append). In `categoryMeta.ts`, replace their bodies with a re-export: `export { formatThousands, parseThousands } from '~/utils/format'`. Keep signatures identical.
- [ ] **Step 4: Run — expect PASS.** Also `pnpm test -- categoryMeta` (if exists) and `pnpm typecheck`.
- [ ] **Step 5: Commit** `refactor(frontend): centralize thousand-format helpers in utils/format`.

---

## Task 2: `NumberInput.vue` component

**Files:**
- Create: `frontend/app/components/NumberInput.vue`
- Test: `frontend/test/nuxt/NumberInput.spec.ts`

**Interfaces:**
- Produces component `<NumberInput>` with `defineModel<string>()` (raw digit-string) and props:
  `allowNegative?: boolean = false`, `thousandSeparator?: boolean = false`, `decimals?: number = 0`,
  `money?: boolean = false`, plus passthrough `min?`, `max?`, `placeholder?`, `disabled?`, `id?`, `dataTestid?: string`.
- Behavior: rejects non-numeric keystrokes; raw value in `modelValue`; formatted display; `money` ⇒ `Rp` leading + `thousandSeparator` on.

- [ ] **Step 1: Write failing runtime test** `frontend/test/nuxt/NumberInput.spec.ts`:

```ts
// @vitest-environment nuxt
import { describe, it, expect } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import NumberInput from '~/components/NumberInput.vue'

describe('NumberInput', () => {
  it('renders raw value with thousand separator', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '1000000', thousandSeparator: true } })
    expect(c.find('input').element.value).toBe('1.000.000')
  })
  it('shows Rp leading in money mode', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '2500', money: true } })
    expect(c.text()).toContain('Rp')
    expect(c.find('input').element.value).toBe('2.500')
  })
  it('emits raw digits on input', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', thousandSeparator: true } })
    const input = c.find('input')
    await input.setValue('1.234.567')
    const emits = c.emitted('update:modelValue')
    expect(emits?.at(-1)?.[0]).toBe('1234567')
  })
  it('strips a minus when allowNegative is false', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', allowNegative: false } })
    await c.find('input').setValue('-42')
    expect(c.emitted('update:modelValue')?.at(-1)?.[0]).toBe('42')
  })
  it('keeps minus and decimals when configured', async () => {
    const c = await mountSuspended(NumberInput, { props: { modelValue: '', allowNegative: true, decimals: 7 } })
    await c.find('input').setValue('-6.2000000')
    expect(c.emitted('update:modelValue')?.at(-1)?.[0]).toBe('-6.2000000')
  })
})
```

- [ ] **Step 2: Run — expect FAIL** (component missing): `pnpm test -- NumberInput`
- [ ] **Step 3: Implement** `frontend/app/components/NumberInput.vue`:

```vue
<script setup lang="ts">
import { formatThousands, parseThousands } from '~/utils/format'

const model = defineModel<string>({ default: '' })
const props = withDefaults(defineProps<{
  allowNegative?: boolean
  thousandSeparator?: boolean
  decimals?: number
  money?: boolean
  min?: number | string
  max?: number | string
  placeholder?: string
  disabled?: boolean
  id?: string
  dataTestid?: string
}>(), {
  allowNegative: false,
  thousandSeparator: false,
  decimals: 0,
  money: false
})

const useThousands = computed(() => props.money || props.thousandSeparator)

// Keep only allowed characters in a raw string: digits, optional leading '-', optional '.'
function sanitizeRaw(input: string): string {
  let s = input.replace(/[^\d.-]/g, '')
  // minus: only when allowed and only at position 0
  const neg = props.allowNegative && s.startsWith('-')
  s = s.replace(/-/g, '')
  if (props.decimals > 0) {
    const parts = s.split('.')
    let dec = parts.slice(1).join('').slice(0, props.decimals)
    s = parts[0] + (parts.length > 1 ? '.' + dec : '')
  } else {
    s = s.replace(/\./g, '')
  }
  return (neg ? '-' : '') + s
}

function toDisplay(raw: string): string {
  if (!raw) return ''
  if (!useThousands.value) return raw
  const neg = raw.startsWith('-')
  const body = neg ? raw.slice(1) : raw
  const [int, dec] = body.split('.')
  const grouped = formatThousands(int || '0')
  return (neg ? '-' : '') + grouped + (dec !== undefined ? '.' + dec : '')
}

const display = ref(toDisplay(model.value))
watch(model, (v) => { display.value = toDisplay(v) })

function onInput(val: string) {
  // when grouping, strip separators first, then sanitize
  const rawInput = useThousands.value ? parseThousandsKeepDecimal(val) : val
  const raw = sanitizeRaw(rawInput)
  model.value = raw
  display.value = toDisplay(raw)
}

// parseThousands strips all non-digits; we must preserve '-' and '.' for decimals
function parseThousandsKeepDecimal(v: string): string {
  const neg = v.trim().startsWith('-')
  const cleaned = v.replace(/[^\d.]/g, '')
  return (neg ? '-' : '') + cleaned
}
</script>

<template>
  <UInput
    :model-value="display"
    inputmode="decimal"
    :placeholder="placeholder"
    :disabled="disabled"
    :id="id"
    :data-testid="dataTestid"
    class="w-full"
    @update:model-value="onInput(String($event))"
  >
    <template v-if="money" #leading>
      <span class="text-muted text-sm">Rp</span>
    </template>
    <template v-if="$slots.trailing" #trailing>
      <slot name="trailing" />
    </template>
  </UInput>
</template>
```

> Note for implementer: verify against `constants/categoryMeta.ts` that `formatThousands('')` returns `''`. The rollout tasks pass `:decimals`, `:allow-negative`, `:money`, `:thousand-separator` as needed. Keep `data-testid` prop name `dataTestid` mapping to `data-testid`.

- [ ] **Step 4: Run — expect PASS.** `pnpm test -- NumberInput` and `pnpm typecheck`, `pnpm lint`.
- [ ] **Step 5: Commit** `feat(frontend): add reusable NumberInput component`.

---

## Task 3: Roll out `NumberInput` to all number forms

**Files (modify):** `components/asset/AssetForm.vue`, `components/category/CategoryFormSlideover.vue`, `components/maintenance/RecordSlideover.vue`, `components/maintenance/ScheduleSlideover.vue`, `pages/disposals.vue`, `pages/depreciation.vue`, `pages/master/offices.vue`. Update affected specs.

**Interfaces:** Consumes `<NumberInput>` from Task 2. All models remain raw digit-strings (existing form state already stores strings), so wiring is drop-in.

Per-site config:

| Site | Field | Replace with |
|------|-------|--------------|
| `AssetForm.vue` | harga (mode new) | `<NumberInput money>` bound to `form.harga` via existing `setField` |
| `CategoryFormSlideover.vue` | useful_life, fiscal_life | `<NumberInput>` + `#trailing` "bln" |
| `CategoryFormSlideover.vue` | salvage_rate | `<NumberInput :max="100">` + `#trailing` "%" |
| `CategoryFormSlideover.vue` | capitalization_threshold | `<NumberInput money>` (drop the manual `formatThousands`/`parseThousands` `@input`) |
| `RecordSlideover.vue` | cost | `<NumberInput money>` (drop `costPayload` strip) |
| `ScheduleSlideover.vue` | intervalMonths | `<NumberInput :min="1">` |
| `disposals.vue` | proceeds | `<NumberInput money>` (remove `proceedsRaw`/`proceedsDisplay`/`onProceedsInput`) |
| `depreciation.vue` | impairRecoverable | `<NumberInput money>` (remove `impairRecoverDisplay`/`onImpairRecoverInput`) |
| `offices.vue` | latitude, longitude | `<NumberInput :decimals="7" allow-negative>` (replace `toCoord`/getter-setter) |

- [ ] **Step 1:** For each site, replace the raw `UInput` with `<NumberInput>` keeping the SAME `v-model`/`data-testid`/`UFormField` wrapper. Preserve existing testids so current specs still pass. Remove now-dead helper code (`onProceedsInput`, `impairRecoverDisplay`, inline `formatThousands` handlers, coord getter/setters) and their now-unused imports.
- [ ] **Step 2: Run existing form specs** to confirm no regression: `pnpm test -- AssetForm CategoryForm RecordSlideover ScheduleSlideover disposals depreciation offices master-offices` (run the ones that exist). Fix any testid mismatches.
- [ ] **Step 3:** Add one assertion per money field to the relevant spec verifying the raw value is submitted unformatted (e.g. disposals proceeds payload is `"1500000"` not `"1.500.000"`). Where a spec doesn't exist for a site, add a minimal runtime test mounting the form and asserting the NumberInput is present with correct props.
- [ ] **Step 4: Run** `pnpm test`, `pnpm typecheck`, `pnpm lint`, `pnpm build` — all green.
- [ ] **Step 5: Commit** `refactor(frontend): use NumberInput across all number forms`.

---

## Task 4: `useResendCooldown` composable

**Files:**
- Create: `frontend/app/composables/useResendCooldown.ts`
- Test: `frontend/test/unit/useResendCooldown.spec.ts`

**Interfaces:**
- Produces: `useResendCooldown(baseSeconds?: number)` → `{ remaining: Ref<number>, canResend: ComputedRef<boolean>, attempts: Ref<number>, start(): void, reset(): void }`.
- Cooldown for attempt n (1-based) = `baseSeconds * 2^(n-1)` → 30, 60, 120…

- [ ] **Step 1: Write failing unit test** (node env, fake timers):

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useResendCooldown } from '~/composables/useResendCooldown'

describe('useResendCooldown', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('exponential backoff 30 -> 60 -> 120', () => {
    const c = useResendCooldown(30)
    expect(c.canResend.value).toBe(true)
    c.start()
    expect(c.attempts.value).toBe(1)
    expect(c.remaining.value).toBe(30)
    expect(c.canResend.value).toBe(false)
    vi.advanceTimersByTime(30000)
    expect(c.remaining.value).toBe(0)
    expect(c.canResend.value).toBe(true)
    c.start()
    expect(c.remaining.value).toBe(60)
    vi.advanceTimersByTime(60000)
    c.start()
    expect(c.remaining.value).toBe(120)
  })

  it('reset clears attempts and timer', () => {
    const c = useResendCooldown(30)
    c.start()
    c.reset()
    expect(c.attempts.value).toBe(0)
    expect(c.remaining.value).toBe(0)
    expect(c.canResend.value).toBe(true)
  })
})
```

- [ ] **Step 2: Run — expect FAIL:** `pnpm test -- useResendCooldown`
- [ ] **Step 3: Implement:**

```ts
export function useResendCooldown(baseSeconds = 30) {
  const remaining = ref(0)
  const attempts = ref(0)
  const canResend = computed(() => remaining.value <= 0)
  let timer: ReturnType<typeof setInterval> | null = null

  function clear() {
    if (timer) { clearInterval(timer); timer = null }
  }

  function start() {
    attempts.value += 1
    remaining.value = baseSeconds * 2 ** (attempts.value - 1)
    clear()
    timer = setInterval(() => {
      remaining.value -= 1
      if (remaining.value <= 0) { remaining.value = 0; clear() }
    }, 1000)
  }

  function reset() {
    attempts.value = 0
    remaining.value = 0
    clear()
  }

  if (getCurrentScope()) onScopeDispose(clear)
  return { remaining, attempts, canResend, start, reset }
}
```

> `getCurrentScope`/`onScopeDispose`/`ref`/`computed` are Nuxt auto-imports. In the unit test (node env) they resolve via the Nuxt vitest alias; if `getCurrentScope` is undefined in that env, guard with `typeof getCurrentScope === 'function' && getCurrentScope()`.

- [ ] **Step 4: Run — expect PASS**, plus `pnpm typecheck`, `pnpm lint`.
- [ ] **Step 5: Commit** `feat(frontend): add useResendCooldown composable (exponential backoff)`.

---

## Task 5: Forgot-password — input sizing + resend

**Files:**
- Modify: `frontend/app/pages/forgot-password.vue`
- Modify: `frontend/i18n/locales/{id,en}.json`
- Test: `frontend/test/nuxt/forgot-password.spec.ts` (extend)

**Interfaces:** Consumes `useResendCooldown` (Task 4).

- [ ] **Step 1: Write failing runtime tests** (extend spec): after submit shows a resend button; button disabled while `remaining > 0`; email input carries `w-full`.

```ts
it('email input is full width', async () => {
  const c = await mountSuspended(ForgotPassword)
  expect(c.find('[data-testid="forgot-email"]').classes()).toContain('w-full')
})
it('shows resend with countdown after sending', async () => {
  // mock account.requestPasswordReset to resolve; submit; then:
  // expect [data-testid="forgot-resend"] present and disabled with text containing 's'
})
```

- [ ] **Step 2: Run — expect FAIL:** `pnpm test -- forgot-password`
- [ ] **Step 3: Implement:**
  - Add `class="w-full"` to the email `UInput` (`data-testid="forgot-email"`).
  - `const cooldown = useResendCooldown(30)`. On first successful `submit()`, call `cooldown.start()`.
  - In the `v-if="sent"` branch, below the `UAlert`, add:

```vue
<UButton
  data-testid="forgot-resend"
  variant="soft" block class="mt-3"
  :disabled="!cooldown.canResend.value || loading"
  :loading="loading"
  @click="resend"
>
  {{ cooldown.canResend.value ? t('auth.forgotResend') : t('auth.forgotResendWait', { s: cooldown.remaining.value }) }}
</UButton>
```
  - `async function resend() { if (!cooldown.canResend.value) return; await doRequest(); cooldown.start() }` where `doRequest` is the extracted request call (share with `submit`). Keep the 429 handling.
- [ ] **Step 4:** i18n keys `auth.forgotResend` ("Kirim ulang tautan" / "Resend link"), `auth.forgotResendWait` ("Kirim ulang dalam {s}s" / "Resend in {s}s"). Add to both locales.
- [ ] **Step 5: Run — expect PASS**, `pnpm lint`, `pnpm typecheck`. **Commit** `feat(frontend): forgot-password full-width input + tiered resend`.

---

## Task 6: Global-search autofocus

**Files:**
- Modify: `frontend/app/components/CommandPalette.vue`
- Test: `frontend/test/nuxt/CommandPalette.spec.ts` (extend)

- [ ] **Step 1: Write failing test:** open palette → the search input is `document.activeElement`.

```ts
it('focuses the search input when opened', async () => {
  const { open } = useCommandPalette()
  const c = await mountSuspended(CommandPalette)
  open()
  await nextTick(); await nextTick()
  const input = c.find('input').element
  expect(document.activeElement).toBe(input)
})
```

- [ ] **Step 2: Run — expect FAIL** (focus relies on flaky `autofocus`): `pnpm test -- CommandPalette`
- [ ] **Step 3: Implement:** add `const inputEl = ref<HTMLInputElement>()`, `ref="inputEl"` on the `<input>`, and:

```ts
watch(isOpen, (v) => {
  if (v) nextTick(() => inputEl.value?.focus())
})
```
  Keep the `autofocus` attribute as fallback.
- [ ] **Step 4: Run — expect PASS**, `pnpm lint`, `pnpm typecheck`.
- [ ] **Step 5: Commit** `fix(frontend): focus global search input on open`.

---

## Task 7: Office-map detail card z-index

**Files:**
- Modify: `frontend/app/pages/master/map.vue`
- Test: `frontend/test/nuxt/master-map.spec.ts` (extend)

- [ ] **Step 1: Write failing test:** when an office is selected, the detail card has a z-index class above Leaflet layers.

```ts
it('detail card sits above the map', async () => {
  // render map.vue with a selected office (mock useOfficeMap selection)
  const card = c.find('[data-testid="office-detail-card"]')
  expect(card.classes().some(k => /^z-\[?1[0-9]{3}\]?$/.test(k) || k === 'z-[1100]')).toBe(true)
})
```

- [ ] **Step 2: Run — expect FAIL:** `pnpm test -- master-map`
- [ ] **Step 3: Implement:** add `z-[1100]` to the detail card element; add `z-[1000]` to the zoom controls, reset-view button, and empty overlay so custom UI consistently sits above Leaflet panes (z 400) and controls (z 800–1000). Do not exceed the command-palette layer (Teleported, separate stacking context).
- [ ] **Step 4: Run — expect PASS**, `pnpm lint`, `pnpm typecheck`, `pnpm build`.
- [ ] **Step 5:** Manually open `pages/master/map.vue`, select an office, confirm the card renders fully over the map (also compare against the mockup). **Commit** `fix(frontend): raise office detail card above Leaflet map`.

---

# PART B — Backend PDF/CSV mojibake (#7, backend-only, independent)

## Task 8: Embed DejaVuSans font util

**Files:**
- Create: `backend/internal/pdfutil/font.go`
- Add binaries: `backend/internal/pdfutil/fonts/DejaVuSans.ttf`, `DejaVuSans-Bold.ttf`, `DejaVuSans-Oblique.ttf`, plus `backend/internal/pdfutil/fonts/LICENSE` (DejaVu license text).
- Test: `backend/internal/pdfutil/font_test.go`

**Interfaces:**
- Produces: `func NewUTF8PDF(orientation, unit, size string) *fpdf.Fpdf` — returns an `*fpdf.Fpdf` with `dejavu` (regular/B/I) registered; and `func RegisterFonts(pdf *fpdf.Fpdf)` for callers that build their own Fpdf. Font family name constant `FontFamily = "dejavu"`.

- [ ] **Step 1:** Obtain the three DejaVuSans TTFs (DejaVu Fonts, permissive license) and place under `backend/internal/pdfutil/fonts/` with the `LICENSE` file. (Download outside the plan; verify sizes ~700KB each.)
- [ ] **Step 2: Write failing test** `font_test.go`:

```go
package pdfutil

import (
	"bytes"
	"strings"
	"testing"
)

func TestUTF8PDFRendersNonASCII(t *testing.T) {
	pdf := NewUTF8PDF("P", "mm", "A4")
	pdf.AddPage()
	pdf.SetFont(FontFamily, "B", 12)
	// middle-dot and em-dash that previously mojibaked, plus accented data
	pdf.MultiCell(0, 6, "Periode · Kantor — José Peña", "", "L", false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("output: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty pdf")
	}
	if strings.Contains(buf.String(), "Helvetica") {
		t.Fatal("expected embedded font, found core Helvetica reference")
	}
}
```

- [ ] **Step 3: Run — expect FAIL** (package missing): `go test ./internal/pdfutil/`
- [ ] **Step 4: Implement** `font.go`:

```go
// Package pdfutil provides a shared Fpdf constructor with an embedded
// Unicode (DejaVuSans) font so PDFs render UTF-8 text (·, —, accents)
// correctly instead of cp1252 mojibake.
package pdfutil

import (
	"embed"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/DejaVuSans.ttf fonts/DejaVuSans-Bold.ttf fonts/DejaVuSans-Oblique.ttf
var fontsFS embed.FS

// FontFamily is the registered UTF-8 font family name.
const FontFamily = "dejavu"

// RegisterFonts registers dejavu regular/bold/italic on an existing Fpdf.
func RegisterFonts(pdf *fpdf.Fpdf) {
	reg, _ := fontsFS.ReadFile("fonts/DejaVuSans.ttf")
	bold, _ := fontsFS.ReadFile("fonts/DejaVuSans-Bold.ttf")
	ital, _ := fontsFS.ReadFile("fonts/DejaVuSans-Oblique.ttf")
	pdf.AddUTF8FontFromBytes(FontFamily, "", reg)
	pdf.AddUTF8FontFromBytes(FontFamily, "B", bold)
	pdf.AddUTF8FontFromBytes(FontFamily, "I", ital)
}

// NewUTF8PDF builds an Fpdf with the dejavu font registered.
func NewUTF8PDF(orientation, unit, size string) *fpdf.Fpdf {
	pdf := fpdf.New(orientation, unit, size, "")
	RegisterFonts(pdf)
	return pdf
}
```

> Confirm the installed `go-pdf/fpdf` exposes `AddUTF8FontFromBytes`; it does in v0.9.0. If not, fall back to writing temp font files + `AddUTF8Font` with `SetFontLocation`.

- [ ] **Step 5: Run — expect PASS:** `go test ./internal/pdfutil/`. **Commit** `feat(report): embed DejaVuSans UTF-8 font util for PDFs`.

---

## Task 9: Switch all PDF generators to the embedded font

**Files (modify):** `internal/report/export.go`, `internal/depreciation/export.go`, `internal/stockopname/report.go`, `internal/asset/barcode.go`.

**Interfaces:** Consumes `pdfutil.NewUTF8PDF` / `pdfutil.RegisterFonts` / `pdfutil.FontFamily` (Task 8).

- [ ] **Step 1: Write a failing test** in `internal/report/export_test.go` (or nearest existing test file) that builds a report PDF containing the `·` subtitle and asserts output is produced and contains no `Helvetica` font reference. If a `Service` is hard to construct, add a focused test on the smallest buildable PDF path. Example assertion helper:

```go
if bytes.Contains(out, []byte("/BaseFont /Helvetica")) {
	t.Fatal("still using core Helvetica")
}
```

- [ ] **Step 2: Run — expect FAIL** (still Helvetica): `go test ./internal/report/`
- [ ] **Step 3: Implement:** in each of the four files, replace `fpdf.New("L"/"P", "mm", "A4", "")` with `pdfutil.NewUTF8PDF(...)` (same args minus the trailing font-dir string), and replace every `pdf.SetFont("Helvetica", style, size)` with `pdf.SetFont(pdfutil.FontFamily, style, size)`. Add the `pdfutil` import; remove now-unused direct `fpdf.New` where fully replaced (keep `fpdf` import if still referencing types). Leave the literal `·`/`—` text as-is — the embedded font renders them correctly now.
- [ ] **Step 4: Run — expect PASS:** `go test ./internal/report/ ./internal/depreciation/ ./internal/stockopname/ ./internal/asset/` and `go build ./... && go vet ./...`.
- [ ] **Step 5:** Manually generate one report PDF and confirm `·`/`—` render correctly. **Commit** `fix(report): render PDFs with embedded Unicode font (no more mojibake)`.

---

## Task 10: CSV UTF-8 BOM

**Files (modify):** `internal/importer/template.go`, `internal/importer/errreport.go`. Test: nearest importer test file.

- [ ] **Step 1: Write failing test** asserting the CSV template bytes start with the UTF-8 BOM `EF BB BF`:

```go
func TestTemplateCSVHasBOM(t *testing.T) {
	body := buildTemplateCSV(/* args matching existing signature */)
	if len(body) < 3 || body[0] != 0xEF || body[1] != 0xBB || body[2] != 0xBF {
		t.Fatalf("expected UTF-8 BOM, got % x", body[:min(3, len(body))])
	}
}
```

- [ ] **Step 2: Run — expect FAIL:** `go test ./internal/importer/`
- [ ] **Step 3: Implement:** prepend `"\xEF\xBB\xBF"` to the CSV byte output in `template.go` (before `strings.Join(...)`) and write the BOM to the `csv.Writer`'s buffer before the writer in `errreport.go` (write `[]byte{0xEF,0xBB,0xBF}` to the buffer first). Leave `Content-Type: text/csv` handlers unchanged.
- [ ] **Step 4: Run — expect PASS**, `go build ./...`, `go vet ./...`.
- [ ] **Step 5: Commit** `fix(import): prepend UTF-8 BOM to CSV exports for Excel`.

---

# PART C — Backend + frontend: Profile, Email change, Password change (#2, #3)

## Task 11: Migration — `employees.phone` — ⚠️ VOID (column already existed)

> **Resolved during execution:** `masterdata.employees.phone` was already added by the earlier
> migration `000019_employee_phone`. A duplicate `000034` was briefly added then removed (commits
> 431cd8f → 5108e41). **No migration is needed** — proceed straight to Task 12 against the existing
> column.

**Files:**
- Create: `backend/db/migrations/000034_employee_phone.up.sql` / `.down.sql`

- [ ] **Step 1:** Write `000034_employee_phone.up.sql`:

```sql
ALTER TABLE masterdata.employees ADD COLUMN phone text;
```
`000034_employee_phone.down.sql`:
```sql
ALTER TABLE masterdata.employees DROP COLUMN phone;
```

- [ ] **Step 2: Run** (dev DB up): `migrate -path db/migrations -database "$DATABASE_URL" up` then `down 1` then `up` — confirm reversible.
- [ ] **Step 3: Commit** `feat(db): add employees.phone column`.

---

## Task 12: sqlc queries — profile read/update

**Files:**
- Modify: `backend/db/queries/identity.sql`, `backend/db/queries/employees.sql`
- Run: `sqlc generate`

**Interfaces:** Produces generated methods used by the service in Task 13. Names below are the contract.

- [ ] **Step 1:** Add queries. In `identity.sql`:

```sql
-- name: UpdateUserName :one
UPDATE identity.users SET name = $2 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserEmail :one
UPDATE identity.users SET email = $2 WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: GetUserProfile :one
SELECT u.id, u.name, u.email, u.role_id, u.office_id, u.employee_id, u.status,
       u.avatar_url, u.google_id, u.created_at,
       e.phone AS employee_phone
FROM identity.users u
LEFT JOIN masterdata.employees e ON e.id = u.employee_id AND e.deleted_at IS NULL
WHERE u.id = $1 AND u.deleted_at IS NULL;
```

In `employees.sql`:

```sql
-- name: UpdateEmployeePhone :exec
UPDATE masterdata.employees SET phone = $2 WHERE id = $1 AND deleted_at IS NULL;
```

- [ ] **Step 2: Run** `sqlc generate` from `backend/`. Confirm `db/sqlc/` compiles: `go build ./...`.
- [ ] **Step 3: Commit** `feat(db): profile read/update queries + employee phone`.

---

## Task 13: Email-change token store

**Files:**
- Create: `backend/internal/auth/emailchange.go`
- Test: `backend/internal/auth/emailchange_test.go` (if a Redis/miniredis test harness exists in package; else a pure hashing test)

**Interfaces:**
- Produces: `GenerateEmailChangeToken() (raw, hash string, err error)`, `HashEmailChangeToken(raw string) string`, and methods on `*TokenStore`: `SaveEmailChange(ctx, hash, userID, newEmail string, ttl time.Duration) error`, `ConsumeEmailChange(ctx, hash string) (userID, newEmail string, err error)`. Sentinel `ErrEmailChangeNotFound`.

- [ ] **Step 1: Write failing test** (mirror any existing `pwreset` test; if tests use miniredis, reuse that setup). Minimal pure test:

```go
func TestEmailChangeTokenRoundtrip(t *testing.T) {
	raw, hash, err := GenerateEmailChangeToken()
	if err != nil || raw == "" || hash == "" {
		t.Fatalf("gen: %v", err)
	}
	if HashEmailChangeToken(raw) != hash {
		t.Fatal("hash mismatch")
	}
}
```

- [ ] **Step 2: Run — expect FAIL:** `go test ./internal/auth/ -run EmailChange`
- [ ] **Step 3: Implement** `emailchange.go` (mirror `pwreset.go`, value is JSON `{userID,newEmail}`):

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const emailChangePrefix = "auth:emailchange:"

// ErrEmailChangeNotFound is returned for unknown/expired/used tokens.
var ErrEmailChangeNotFound = errors.New("email change token not found")

func GenerateEmailChangeToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashEmailChangeToken(raw), nil
}

func HashEmailChangeToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type emailChangePayload struct {
	UserID   string `json:"user_id"`
	NewEmail string `json:"new_email"`
}

func (s *TokenStore) SaveEmailChange(ctx context.Context, hash, userID, newEmail string, ttl time.Duration) error {
	b, err := json.Marshal(emailChangePayload{UserID: userID, NewEmail: newEmail})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, emailChangePrefix+hash, b, ttl).Err()
}

func (s *TokenStore) ConsumeEmailChange(ctx context.Context, hash string) (string, string, error) {
	v, err := s.rdb.GetDel(ctx, emailChangePrefix+hash).Result()
	if err != nil {
		return "", "", ErrEmailChangeNotFound
	}
	var p emailChangePayload
	if err := json.Unmarshal([]byte(v), &p); err != nil {
		return "", "", ErrEmailChangeNotFound
	}
	return p.UserID, p.NewEmail, nil
}
```

- [ ] **Step 4: Run — expect PASS:** `go test ./internal/auth/`.
- [ ] **Step 5: Commit** `feat(auth): email-change token store`.

---

## Task 14: Email templates + mailer methods

**Files:**
- Create: `backend/internal/email/templates/email_change_verify.{html,txt}`, `email_changed.{html,txt}`
- Modify: `backend/internal/email/mailer.go`
- Test: `backend/internal/email/mailer_test.go` (extend if present)

**Interfaces:**
- Produces: `Mailer.SendEmailChangeVerify(ctx, to, name, link string) error`, `Mailer.SendEmailChanged(ctx, to, name, newEmail string) error`.

- [ ] **Step 1:** Create templates mirroring `password_reset.{html,txt}` tone (Indonesian). `email_change_verify` uses `{{.Name}}` + `{{.Link}}`; `email_changed` uses `{{.Name}}` + `{{.NewEmail}}`. Keep them short; the `//go:embed templates/*.html templates/*.txt` glob picks them up automatically.
- [ ] **Step 2: Write failing test** asserting both methods render + call sender (use the existing test Sender/LogSender or a stub capturing args). Assert the verify email body contains the link and the changed email contains the new address.
- [ ] **Step 3: Run — expect FAIL:** `go test ./internal/email/`
- [ ] **Step 4: Implement** in `mailer.go`:

```go
type emailChangeData struct {
	Name string
	Link string
}
type emailChangedData struct {
	Name     string
	NewEmail string
}

func (m *Mailer) SendEmailChangeVerify(ctx context.Context, to, name, link string) error {
	html, text, err := m.render("email_change_verify.html", "email_change_verify.txt", emailChangeData{Name: name, Link: link})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Verifikasi Perubahan Email Inventra", html, text)
}

func (m *Mailer) SendEmailChanged(ctx context.Context, to, name, newEmail string) error {
	html, text, err := m.render("email_changed.html", "email_changed.txt", emailChangedData{Name: name, NewEmail: newEmail})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Email Akun Inventra Diubah", html, text)
}
```

> If the app uses `AsyncMailer`, add matching pass-through methods there too (check `internal/email/async.go` and how `Service` references the mailer interface). Extend the mailer interface used by `identity.Service` to include the two new methods.

- [ ] **Step 5: Run — expect PASS**, `go build ./...`. **Commit** `feat(email): email-change verify + changed templates`.

---

## Task 15: identity service — profile, email change, password change-request

**Files:**
- Modify: `backend/internal/identity/service.go` (+ sentinel errors), `backend/internal/identity/dto.go`
- Test: `backend/internal/identity/service_test.go` (extend)

**Interfaces:**
- Produces service methods:
  - `GetProfile(ctx, userID uuid.UUID) (ProfileView, error)`
  - `UpdateProfile(ctx, userID uuid.UUID, name, phone string) (ProfileView, error)`
  - `RequestEmailChange(ctx, userID uuid.UUID, newEmail, currentPassword string) error`
  - `ConfirmEmailChange(ctx, token string) (sqlc.IdentityUser, error)`
  - `RequestPasswordChange(ctx, userID uuid.UUID, currentPassword string) error`
- New sentinels: `ErrEmailInUse`, `ErrSameEmail`. Reuse `ErrInvalidCredentials`, `ErrInvalidToken`.
- `ProfileView` struct fields: `ID, Name, Email, Phone, RoleID, OfficeID, EmployeeID, Status, AvatarURL, GoogleLinked, JoinedAt`.

- [ ] **Step 1: Write failing tests** for the pure/verifiable branches (use existing service test harness; where it mocks queries + store, follow that):
  - `RequestEmailChange` with wrong password → `ErrInvalidCredentials`.
  - `RequestEmailChange` where new email equals current → `ErrSameEmail`.
  - `RequestEmailChange` where email belongs to another user → `ErrEmailInUse`.
  - `RequestPasswordChange` with wrong password → `ErrInvalidCredentials`; correct → saves reset token + sends mail (assert stub called).
  - `ConfirmEmailChange` with unknown token → `ErrInvalidToken`.

- [ ] **Step 2: Run — expect FAIL:** `go test ./internal/identity/ -run 'Profile|EmailChange|PasswordChange'`
- [ ] **Step 3: Implement** (patterns mirror `RequestPasswordReset`/`ChangePassword` already in `service.go`):

```go
// GetProfile returns the caller's profile incl. employee phone.
func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (ProfileView, error) {
	row, err := s.q.GetUserProfile(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	return profileFromRow(row), nil
}

// UpdateProfile sets the display name and (if linked) the employee phone.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, name, phone string) (ProfileView, error) {
	if strings.TrimSpace(name) == "" {
		return ProfileView{}, ErrInvalidInput
	}
	if _, err := s.q.UpdateUserName(ctx, sqlc.UpdateUserNameParams{ID: userID, Name: name}); err != nil {
		return ProfileView{}, err
	}
	row, err := s.q.GetUserProfile(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	if row.EmployeeID != nil {
		if err := s.q.UpdateEmployeePhone(ctx, sqlc.UpdateEmployeePhoneParams{ID: *row.EmployeeID, Phone: ptrOrNil(phone)}); err != nil {
			return ProfileView{}, err
		}
	}
	return s.GetProfile(ctx, userID)
}

// RequestEmailChange verifies the password, checks the new email is free, and
// emails a verification link to the NEW address.
func (s *Service) RequestEmailChange(ctx context.Context, userID uuid.UUID, newEmail, currentPassword string) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	if strings.EqualFold(newEmail, user.Email) {
		return ErrSameEmail
	}
	if _, err := s.q.GetUserByEmail(ctx, newEmail); err == nil {
		return ErrEmailInUse
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	raw, hash, err := auth.GenerateEmailChangeToken()
	if err != nil {
		return err
	}
	if err := s.store.SaveEmailChange(ctx, hash, user.ID.String(), newEmail, s.resetTTL); err != nil {
		return err
	}
	link := s.frontendURL + "/verify-email?token=" + raw
	return s.mail.SendEmailChangeVerify(ctx, newEmail, user.Name, link)
}

// ConfirmEmailChange consumes the token and updates the email, notifying the old address.
func (s *Service) ConfirmEmailChange(ctx context.Context, token string) (sqlc.IdentityUser, error) {
	userIDStr, newEmail, err := s.store.ConsumeEmailChange(ctx, auth.HashEmailChangeToken(token))
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	// Guard: reject if the target email got taken meanwhile.
	if _, err := s.q.GetUserByEmail(ctx, newEmail); err == nil {
		return sqlc.IdentityUser{}, ErrEmailInUse
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.IdentityUser{}, err
	}
	oldUser, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return sqlc.IdentityUser{}, ErrInvalidToken
	}
	updated, err := s.q.UpdateUserEmail(ctx, sqlc.UpdateUserEmailParams{ID: userID, Email: newEmail})
	if err != nil {
		return sqlc.IdentityUser{}, mapDBError(err)
	}
	_ = s.mail.SendEmailChanged(ctx, oldUser.Email, oldUser.Name, newEmail) // best-effort
	return updated, nil
}

// RequestPasswordChange verifies the current password then emails a reset link.
func (s *Service) RequestPasswordChange(ctx context.Context, userID uuid.UUID, currentPassword string) error {
	user, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	raw, hash, err := auth.GenerateResetToken()
	if err != nil {
		return err
	}
	if err := s.store.SavePasswordReset(ctx, hash, user.ID.String(), s.resetTTL); err != nil {
		return err
	}
	link := s.frontendURL + "/reset-password?token=" + raw
	return s.mail.SendPasswordReset(ctx, user.Email, user.Name, link)
}
```

Add helpers `profileFromRow`, `ptrOrNil(s string) *string`, `ProfileView` struct, and sentinels `ErrEmailInUse`, `ErrSameEmail`, `ErrInvalidInput` (if not present) in `service.go`/`dto.go`. Extend the mailer interface field type used by `Service` to include `SendEmailChangeVerify`/`SendEmailChanged`.

- [ ] **Step 4: Run — expect PASS:** `go test ./internal/identity/`, `go build ./...`, `go vet ./...`.
- [ ] **Step 5: Commit** `feat(identity): profile update, email change, password change-request services`.

---

## Task 16: identity handlers + routes + OpenAPI

**Files:**
- Modify: `backend/internal/identity/handler.go`, `backend/internal/identity/dto.go`, `backend/internal/identity/routes.go`, `backend/api/openapi.yaml`
- Test: `backend/internal/identity/handler_test.go` (extend if present) or rely on service tests + e2e.

**Interfaces:** Consumes Task 15 service methods. New DTOs:
- `updateProfileRequest{ Name string \`binding:"required"\`; Phone string }`
- `emailChangeRequest{ NewEmail string \`binding:"required,email"\`; CurrentPassword string \`binding:"required"\` }`
- `emailConfirmRequest{ Token string \`binding:"required"\` }`
- `passwordChangeRequestRequest{ CurrentPassword string \`binding:"required"\` }`

- [ ] **Step 1:** Add handler methods: `getProfile`, `updateProfile`, `requestEmailChange`, `confirmEmailChange`, `requestPasswordChange`. Each: bind → call service → serialize → `svcError` mapping. Map sentinels: `ErrInvalidCredentials`→401, `ErrEmailInUse`→409, `ErrSameEmail`→409/422, `ErrInvalidToken`→400, `ErrInvalidInput`→422.
- [ ] **Step 2:** Wire routes in `routes.go`:

```go
// public (rate-limited) — confirm works without a session (link from any device)
grp.POST("/email/confirm", middleware.PerIP(limiter, forgotPerMin, "auth_emailconfirm", true), h.confirmEmailChange)

// authed group additions:
authed.GET("/profile", h.getProfile)
authed.PUT("/profile", h.updateProfile)
authed.POST("/email/change-request", h.requestEmailChange)
authed.POST("/password/change-request", h.requestPasswordChange)
```

- [ ] **Step 3:** Update `backend/api/openapi.yaml`: add the 5 paths with request/response schemas, mirroring the existing `/auth/password/*` entries. Run Spectral: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` — expect clean.
- [ ] **Step 4: Run:** `go build ./...`, `go vet ./...`, `go test ./internal/identity/`.
- [ ] **Step 5: Commit** `feat(identity): profile/email/password endpoints + OpenAPI`.

---

## Task 17: Frontend — wire `useAccount.ts`

**Files:**
- Modify: `frontend/app/composables/api/useAccount.ts`, `frontend/app/types/index.ts`
- Test: `frontend/test/nuxt/useAccount.spec.ts` (create/extend) — stub `useApiClient`.

**Interfaces:** Produces on the returned object:
- `getProfile(): Promise<AccountProfile>` → `GET /auth/profile`
- `updateProfile(input: ProfileInput): Promise<AccountProfile>` → `PUT /auth/profile` body `{ name, phone }`
- `requestEmailChange(newEmail: string, currentPassword: string): Promise<void>` → `POST /auth/email/change-request`
- `confirmEmailChange(token: string): Promise<void>` → `POST /auth/email/confirm`
- `requestPasswordChange(currentPassword: string): Promise<void>` → `POST /auth/password/change-request`
- `AccountProfile` gains `hasEmployee: boolean`, `loginMethod`, `joinDate` from the API.

- [ ] **Step 1: Write failing test** stubbing `useApiClient().request` and asserting each function hits the right path/body and maps the response. (Follow the existing wiring-composable test pattern; per memory, stub the client so tests don't hit real `:8080`.)
- [ ] **Step 2: Run — expect FAIL:** `pnpm test -- useAccount`
- [ ] **Step 3: Implement:** replace the mock `getProfile`/`updateProfile`, add the three new functions, map API snake_case → `AccountProfile`. Keep `requestPasswordReset`/`resetPassword` as-is. Remove the old inline `changePassword` (PUT /auth/password) — replaced by `requestPasswordChange`. Update `AccountProfile` type + `ProfileInput` (already `{nama, telepon}` — map to `{name, phone}` at the boundary).
- [ ] **Step 4: Run — expect PASS**, `pnpm typecheck`, `pnpm lint`.
- [ ] **Step 5:** Grep other consumers of the removed `changePassword` and update them (account.vue is handled in Task 18). Ensure full suite green: `pnpm test`. **Commit** `feat(frontend): wire account profile/email/password to backend`.

---

## Task 18: Frontend — account.vue Profil (edit state + email modal)

**Files:**
- Modify: `frontend/app/pages/account.vue`, `frontend/i18n/locales/{id,en}.json`
- Test: `frontend/test/nuxt/account.spec.ts` (create/extend)

**Interfaces:** Consumes Task 17 (`useAccount`) and Task 4 (`useResendCooldown`).

- [ ] **Step 1: Write failing runtime tests:**
  - Profil tab starts read-only; clicking "Edit" enables `nama`/`telepon` inputs; "Batal" reverts.
  - When profile has no employee, telepon input is `disabled` with the hint.
  - "Ubah Email" opens a modal with new-email + current-password fields; submit calls `requestEmailChange`.

- [ ] **Step 2: Run — expect FAIL:** `pnpm test -- account`
- [ ] **Step 3: Implement:**
  - Add `const editing = ref(false)` and a snapshot of `{nama, telepon}` for revert. Fields `:disabled="!editing"`. Header buttons: "Edit" (when `!editing`), "Simpan"+"Batal" (when `editing`). `saveProfil()` calls `updateProfile`, then `editing=false` + toast.
  - Telepon input `:disabled="!editing || !profile?.hasEmployee"`; show hint `account.phoneManagedNote` when `!hasEmployee`.
  - Email card: read-only value + a "Ubah Email" `UButton` (hidden when `isGoogle`). Button opens a `FormModal` (`v-model:open`) with `NumberInput`?? no — a `UInput type=email` for new email + `UInput type=password` for current password. On submit → `requestEmailChange(newEmail, currentPassword)` → switch modal to "sent" state showing target email + a resend `UButton` gated by `useResendCooldown`.
  - i18n keys: `account.edit`, `account.cancel`, `account.changeEmail`, `account.newEmail`, `account.currentPassword`, `account.emailVerifySent`, `account.phoneManagedNote`, resend keys reused from `auth.*` or new `account.resend*`.
- [ ] **Step 4: Run — expect PASS**, `pnpm lint`, `pnpm typecheck`.
- [ ] **Step 5:** Compare Profil tab against its mockup (if one exists in `docs/design`); confirm light + dark. **Commit** `feat(frontend): profile edit state + verified email change`.

---

## Task 19: Frontend — account.vue Keamanan (password modal) + verify-email page

**Files:**
- Modify: `frontend/app/pages/account.vue`, `frontend/i18n/locales/{id,en}.json`
- Create: `frontend/app/pages/verify-email.vue`
- Test: `frontend/test/nuxt/account.spec.ts` (extend), `frontend/test/nuxt/verify-email.spec.ts`

**Interfaces:** Consumes Task 17 (`requestPasswordChange`, `confirmEmailChange`) and Task 4.

- [ ] **Step 1: Write failing tests:**
  - Keamanan tab: NO inline old/new/confirm password inputs; a "Ganti Password" button exists.
  - Clicking it opens a modal with a single current-password field; submit calls `requestPasswordChange`; wrong password (rejected) shows error; success shows "link terkirim" + resend gated by cooldown.
  - `verify-email.vue`: with `?token=abc`, on mount calls `confirmEmailChange('abc')` and renders success; on rejection renders error.

- [ ] **Step 2: Run — expect FAIL:** `pnpm test -- account verify-email`
- [ ] **Step 3: Implement:**
  - Remove the inline password card (old/new/confirm + strength meter) from the security tab main flow. Add a card with description + "Ganti Password" button opening a `FormModal`: one `UInput type=password` (current password) → `requestPasswordChange(currentPassword)` → "sent" state + resend (`useResendCooldown`). Handle 401 → error message.
  - `verify-email.vue` (layout `auth`): read `route.query.token`; `onMounted` → `confirmEmailChange`; states loading/success/error with links to `/login` or `/` (account). If logged in on success, refresh auth/profile.
  - i18n: `account.changePassword`, `account.changePasswordDesc`, `account.pwChangeSent`, `auth.verifyEmailSuccess`, `auth.verifyEmailError`, `auth.verifyEmailLoading`.
- [ ] **Step 4: Run — expect PASS**, `pnpm lint`, `pnpm typecheck`, `pnpm build`.
- [ ] **Step 5:** Compare Keamanan tab + verify-email against mockups/design; light + dark. **Commit** `feat(frontend): password-change-by-email modal + verify-email page`.

---

## Task 20: E2E — email change + password change flows

**Files:**
- Modify/Create: `frontend/e2e/account-security.spec.ts` (new) and update `frontend/e2e/password-reset.spec.ts` if it asserted the old inline change-password flow.

**Interfaces:** Real backend stack + seeded admin. Reuse token-capture approach from existing `password-reset.spec.ts` (how it obtains the reset token — via LogSender output / a test endpoint / DB). Follow memory: unique data per run, assert-after-search, wait-modal-closed, clear cookies+localStorage on mid-test user switch.

- [ ] **Step 1: Write e2e** for password change: login → account → Keamanan → "Ganti Password" → enter current password → assert "link terkirim" state. (If the harness can read the emitted reset link, follow it to `/reset-password`, set a new password, and log in with it.)
- [ ] **Step 2: Write e2e** for email change: login → account → Profil → "Ubah Email" → new email + current password → assert sent state → capture verify token → open `/verify-email?token=...` → assert success → assert `/auth/me` now returns the new email.
- [ ] **Step 3: Run** `pnpm test:e2e` against the up stack (CI `e2e` job runs the full suite). Fix flakiness per memory notes.
- [ ] **Step 4: Commit** `test(e2e): account email + password change flows`.

---

## Task 21: Finalize — PROGRESS.md, full gates, mockup comparisons

**Files:** `docs/PROGRESS.md` (+ any doc ticks).

- [ ] **Step 1:** Run full backend gates: `cd backend && go build ./... && go vet ./... && go test ./...` and Spectral lint. Per memory, if shared signatures changed, run `go test -tags=integration ./...` across all packages.
- [ ] **Step 2:** Run full frontend gates: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`. E2E via the stack.
- [ ] **Step 3:** Side-by-side mockup comparison for `account`, `forgot-password`, `master/map` (light + dark); fix any deviation (do not redesign — ask if a deviation seems warranted, and record approved deviations in PROGRESS.md per memory).
- [ ] **Step 4:** Update `docs/PROGRESS.md`: tick the relevant items with one-line notes + PR number; refresh the "Next session — start here" block.
- [ ] **Step 5: Commit** `docs(progress): UX fixes batch`. Then use `superpowers:finishing-a-development-branch` to open the PR.

---

## Self-Review Notes (coverage map)

- Spec #1 → Tasks 1–3. #2 → Tasks 11–18 (migration, queries, token, templates, service, handlers, wiring, UI) + 20/21. #3 → Tasks 15/16 (backend) + 19 (UI) + 20. #4 → Tasks 4–5. #5 → Task 6. #6 → Task 7. #7 → Tasks 8–10.
- Types consistent across tasks: `ProfileView` (Task 15) ↔ `GetUserProfile` row (Task 12) ↔ `AccountProfile` (Task 17); `SaveEmailChange`/`ConsumeEmailChange` (Task 13) ↔ service (Task 15); `useResendCooldown` shape (Task 4) ↔ consumers (Tasks 5, 18, 19); `pdfutil.FontFamily`/`NewUTF8PDF` (Task 8) ↔ generators (Task 9).
- No placeholders: every code/test step carries real content; font binaries are the only external artifact (Task 8 Step 1).
