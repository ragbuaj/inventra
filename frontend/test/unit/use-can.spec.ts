import { describe, it, expect, beforeAll, beforeEach, vi } from 'vitest'

// ---------------------------------------------------------------------------
// Exercises the real `can` route middleware (app/middleware/can.ts) and its
// OR semantics for permission?: string | string[]. The middleware auto-imports
// useCan (mocked here to a controllable predicate) plus the real Nuxt
// defineNuxtRouteMiddleware / abortNavigation — the latter throws a 403 nuxt
// error on denial, which is exactly what we assert.
// ---------------------------------------------------------------------------

// Permissions the fake caller currently holds; mutated per test.
const { granted } = vi.hoisted(() => ({ granted: new Set<string>() }))

vi.mock('~/composables/useCan', () => ({
  useCan: () => (p: string) => granted.has('*') || granted.has(p)
}))

interface ToStub { meta: { permission?: string | string[] } }
type Middleware = (to: ToStub) => unknown

let canMiddleware: Middleware

beforeAll(async () => {
  canMiddleware = (await import('~/middleware/can')).default as Middleware
})

beforeEach(() => {
  granted.clear()
})

function run(permission?: string | string[]): unknown {
  return canMiddleware({ meta: { permission } })
}

/** Asserts the middleware denied navigation by throwing a 403 nuxt error. */
function expectDenied(permission: string | string[]) {
  let error: unknown
  try {
    run(permission)
  } catch (e) {
    error = e
  }
  expect(error).toBeDefined()
  const err = error as { statusCode?: number, message?: string }
  expect(err.statusCode).toBe(403)
  expect(String(err.message)).toContain('Akses ditolak')
}

describe('can middleware — no permission required', () => {
  it('allows navigation when the route declares no permission', () => {
    expect(run(undefined)).toBeUndefined()
  })
})

describe('can middleware — string permission', () => {
  it('allows when the caller holds the single required key', () => {
    granted.add('asset.view')
    expect(run('asset.view')).toBeUndefined()
  })

  it('denies with 403 when the caller lacks the key', () => {
    expectDenied('asset.view')
  })

  it('allows a wildcard caller regardless of the required key', () => {
    granted.add('*')
    expect(run('scope.manage')).toBeUndefined()
  })
})

describe('can middleware — array permission (OR)', () => {
  it('allows when the caller holds ANY one of the keys', () => {
    granted.add('request.create')
    // maintenance guard: ['maintenance.view', 'request.create']
    expect(run(['maintenance.view', 'request.create'])).toBeUndefined()
  })

  it('allows when the caller holds the other key', () => {
    granted.add('maintenance.view')
    expect(run(['maintenance.view', 'request.create'])).toBeUndefined()
  })

  it('denies with 403 when the caller holds NONE of the keys', () => {
    granted.add('asset.view')
    expectDenied(['masterdata.employee.manage', 'masterdata.office.manage', 'masterdata.global.manage'])
  })
})
