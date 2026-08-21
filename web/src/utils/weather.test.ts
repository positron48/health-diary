import { describe, expect, it } from 'vitest'
import { isSignificantPressureChange, pressureChangeArrow, signedPressureChange } from './weather'

describe('weather pressure presentation', () => {
  it('applies the five hPa threshold symmetrically to unrounded values', () => {
    expect(isSignificantPressureChange(5)).toBe(true)
    expect(isSignificantPressureChange(-5)).toBe(true)
    expect(isSignificantPressureChange(4.6)).toBe(false)
    expect(isSignificantPressureChange(-4.6)).toBe(false)
  })

  it('keeps one decimal so sub-threshold values do not look like five', () => {
    expect(pressureChangeArrow(-5)).toBe('↓5,0')
    expect(signedPressureChange(-4.6)).toBe('-4,6 гПа за 24 ч')
    expect(signedPressureChange(5)).toBe('+5,0 гПа за 24 ч')
  })
})
