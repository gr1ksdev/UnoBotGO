export type System = 'updated' | 'legacy'
export interface RankingItem {
 position: number
 key: string
 group_ref?: string
 name: string
 masked_id: string
 score_units: string
 avatar_url: string
 anonymous?: boolean
}
export interface RankingPage {
 month_start: string
 month_name: string
 month_ends_at: string
 server_time: string
 timezone: string
 system: System
 items: RankingItem[]
 group?: RankingItem
 next_cursor?: string
}
export interface UserPrivacy {
 anonymous: boolean
}
export class APIError extends Error {
 constructor(public status: number) { super('Não foi possível carregar o ranking.') }
}
export function authHeaders(): HeadersInit {
 return { Authorization: `tma ${window.Telegram?.WebApp.initData ?? ''}` }
}
export async function rankingPage(path: string, system: System, cursor: string, signal: AbortSignal): Promise<RankingPage> {
 const query = new URLSearchParams({ system, limit: '50' })
 if (cursor) query.set('cursor', cursor)
 const response = await fetch(`/api/v1/rankings/${path}?${query}`, { headers: authHeaders(), signal })
 if (!response.ok) throw new APIError(response.status)
 return response.json() as Promise<RankingPage>
}
export async function getUserPrivacy(signal?: AbortSignal): Promise<UserPrivacy> {
 const response = await fetch('/api/v1/me/privacy', { headers: authHeaders(), signal })
 if (!response.ok) throw new APIError(response.status)
 return response.json() as Promise<UserPrivacy>
}
export async function setUserPrivacy(anonymous: boolean): Promise<UserPrivacy> {
 const response = await fetch('/api/v1/me/privacy', {
  method: 'PUT',
  headers: { ...authHeaders(), 'Content-Type': 'application/json' },
  body: JSON.stringify({ anonymous }),
 })
 if (!response.ok) throw new APIError(response.status)
 return response.json() as Promise<UserPrivacy>
}
