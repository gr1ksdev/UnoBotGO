import { describe, expect, it } from 'vitest'
import { formatScore, scoreUnit } from './score'

describe('formatScore', () => {
  it('formats updated score with two decimal places and thousand separators', () => {
    expect(formatScore('284000', 'updated')).toBe('2.840,00')
    expect(formatScore('12345678', 'updated')).toBe('123.456,78')
    expect(formatScore('0', 'updated')).toBe('0,00')
    expect(formatScore('50', 'updated')).toBe('0,50')
  })

  it('formats legacy score with whole units and thousand separators', () => {
    expect(formatScore('284000', 'legacy')).toBe('2.840')
    expect(formatScore('100', 'legacy')).toBe('1')
    expect(formatScore('0', 'legacy')).toBe('0')
  })

  it('handles large BigInt values above 2^53 without precision loss', () => {
    // 2^53 is 9007199254740992; units = 900719925474099300
    const raw = '900719925474099300'
    expect(formatScore(raw, 'updated')).toBe('9.007.199.254.740.993,00')
    expect(formatScore(raw, 'legacy')).toBe('9.007.199.254.740.993')
  })

  it('throws on invalid integer score string', () => {
    expect(() => formatScore('abc', 'updated')).toThrow('Invalid integer score')
    expect(() => formatScore('12.34', 'updated')).toThrow('Invalid integer score')
    expect(() => formatScore('-100', 'updated')).toThrow('Invalid integer score')
  })
})

describe('scoreUnit', () => {
  it('returns pt only for legacy 100 units (1 pt)', () => {
    expect(scoreUnit('100', 'legacy')).toBe('pt')
    expect(scoreUnit('200', 'legacy')).toBe('pts')
    expect(scoreUnit('0', 'legacy')).toBe('pts')
    expect(scoreUnit('100', 'updated')).toBe('pts')
  })
})
