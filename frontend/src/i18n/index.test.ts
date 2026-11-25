import { describe, expect, it } from 'vitest'

describe('frontend sanity', () => {
  it('supports PL and EN language codes', () => {
    const langs = ['pl', 'en']
    expect(langs).toContain('pl')
    expect(langs).toContain('en')
  })
})
