import type { System } from '../api/client'

export function formatScore(raw: string, system: System): string {
 if (!/^\d+$/.test(raw)) throw new Error('Invalid integer score')
 const units = BigInt(raw)
 const whole = (units / 100n).toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.')
 return system === 'legacy' ? whole : `${whole},${(units % 100n).toString().padStart(2, '0')}`
}
export function scoreUnit(raw: string, system: System): string {
 return system === 'legacy' && BigInt(raw) === 100n ? 'pt' : 'pts'
}
