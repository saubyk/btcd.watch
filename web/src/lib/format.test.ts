import { describe, expect, it } from 'vitest'

import {
  formatBtc,
  formatCompact,
  formatEta,
  formatEtaShort,
  formatFiat,
  formatNumber,
  formatRelative,
  truncateMiddle,
} from './format'

describe('formatBtc', () => {
  it('renders the design examples', () => {
    expect(formatBtc(4_250_000)).toBe('0.0425')
    expect(formatBtc(1_800_000)).toBe('0.0180')
    expect(formatBtc(18_340_000)).toBe('0.1834')
  })
  it('keeps satoshi precision when needed', () => {
    expect(formatBtc(1)).toBe('0.00000001')
    expect(formatBtc(123_456_789)).toBe('1.23456789')
  })
  it('keeps at least four decimals', () => {
    expect(formatBtc(5_000_000_000)).toBe('50.0000')
    expect(formatBtc(0)).toBe('0.0000')
  })
})

describe('formatFiat', () => {
  it('drops cents on large amounts', () => {
    expect(formatFiat(4165)).toBe('$4,165')
    expect(formatFiat(98000)).toBe('$98,000')
  })
  it('keeps cents under $100', () => {
    expect(formatFiat(42.5)).toBe('$42.50')
  })
})

describe('formatNumber / formatCompact', () => {
  it('groups thousands', () => {
    expect(formatNumber(843359)).toBe('843,359')
  })
  it('compacts large counts', () => {
    expect(formatCompact(14231)).toBe('14.2k')
    expect(formatCompact(999)).toBe('999')
    expect(formatCompact(2_000_000)).toBe('2M')
  })
})

describe('truncateMiddle', () => {
  it('matches the design shape', () => {
    expect(truncateMiddle('bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq')).toBe(
      'bc1qar…5mdq',
    )
  })
  it('leaves short strings alone', () => {
    expect(truncateMiddle('abcdef')).toBe('abcdef')
  })
})

describe('formatRelative', () => {
  const now = 1_750_000_000_000 // ms
  it('renders the design examples', () => {
    expect(formatRelative(now / 1000 - 30, now)).toBe('Just now')
    expect(formatRelative(now / 1000 - 3 * 86400, now)).toBe('3 days ago')
    expect(formatRelative(now / 1000 - 3600, now)).toBe('1 hour ago')
    expect(formatRelative(now / 1000 - 120, now)).toBe('2 minutes ago')
  })
})

describe('formatEta', () => {
  it('renders minutes and hours', () => {
    expect(formatEta(45 * 60)).toBe('~45 minutes')
    expect(formatEta(30)).toBe('~1 minute')
    expect(formatEta(2 * 3600)).toBe('~2 hours')
    expect(formatEta(622 * 86400)).toBe('~622 days')
  })
  it('short variant for the live pill', () => {
    expect(formatEtaShort(8 * 60)).toBe('~8 min')
    expect(formatEtaShort(45)).toBe('~1 min')
  })
})
