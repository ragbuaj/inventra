import { describe, it, expect } from 'vitest'
import { createStore, generateId } from '~/mock/helpers'

interface Row { id: string, name: string }

describe('generateId', () => {
  it('returns unique non-empty strings', () => {
    const a = generateId()
    const b = generateId()
    expect(a).not.toBe('')
    expect(a).not.toBe(b)
  })
})

describe('createStore', () => {
  it('returns all seeded rows', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    expect(store.all()).toHaveLength(1)
  })

  it('finds a row by id', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    expect(store.find('1')?.name).toBe('A')
    expect(store.find('nope')).toBeUndefined()
  })

  it('inserts a row at the front', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    store.insert({ id: '2', name: 'B' })
    expect(store.all()[0].id).toBe('2')
    expect(store.all()).toHaveLength(2)
  })

  it('patches an existing row and returns it', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    const updated = store.patch('1', { name: 'Z' })
    expect(updated?.name).toBe('Z')
    expect(store.find('1')?.name).toBe('Z')
  })

  it('returns undefined when patching a missing row', () => {
    const store = createStore<Row>([])
    expect(store.patch('x', { name: 'Z' })).toBeUndefined()
  })

  it('removes a row and reports success', () => {
    const store = createStore<Row>([{ id: '1', name: 'A' }])
    expect(store.remove('1')).toBe(true)
    expect(store.all()).toHaveLength(0)
    expect(store.remove('1')).toBe(false)
  })
})
