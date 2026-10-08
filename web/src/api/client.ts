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
 constructor(public status: number) {
  super('Não foi possível carregar o ranking.')
 }
}
export function authHeaders(): HeadersInit {
 return { Authorization: `tma ${window.Telegram?.WebApp.initData ?? ''}` }
}
export async function rankingPage(
 path: string,
 system: System,
 cursor: string,
 signal: AbortSignal,
): Promise<RankingPage> {
 const query = new URLSearchParams({ system, limit: '50' })
 if (cursor) query.set('cursor', cursor)
 const response = await fetch(`/api/v1/rankings/${path}?${query}`, {
  headers: authHeaders(),
  signal,
 })
 if (!response.ok) throw new APIError(response.status)
 return response.json() as Promise<RankingPage>
}
export async function getUserPrivacy(
 signal?: AbortSignal,
): Promise<UserPrivacy> {
 const response = await fetch('/api/v1/me/privacy', {
  headers: authHeaders(),
  signal,
 })
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

export async function apiGet<T>(
 path: string,
 signal?: AbortSignal,
): Promise<T> {
 const response = await fetch(`/api/v1/${path}`, {
  headers: authHeaders(),
  signal,
 })
 if (!response.ok) throw new APIError(response.status)
 return response.json() as Promise<T>
}
export interface ProfileStats {
 system: System
 score_units: string
 games: number
 wins: number
}
export interface HistoryEntry {
 id: string
 group: string
 origin: 'inline' | 'webapp'
 system: System
 position: number
 score_units?: string
 status: string
 finished_at: string
}
export interface PlayerProfile {
 identity?: RankingItem
 stats: ProfileStats[]
 history: HistoryEntry[]
 next_cursor?: string
}
export interface PositionResponse {
 entry: RankingItem | null
 month_start: string
 month_name: string
 system: System
}
export interface Room {
 game_id: string
 group: string
 phase: number
}
export interface Card {
 ID: string
 Color: number
 Rank: number
}
export interface Seat {
 key: string
 name: string
 count: number
 me: boolean
 current: boolean
 active: boolean
 position?: number
}
export interface GameView {
 awards?: { key: string; score_units: string | null; position: number }[]
 game_id: string
 revision: number
 phase: number
 group: string
 mode: string
 system: System
 players: Seat[]
 hand: { Card: Card; Playable: boolean }[]
 top: Card | null
 active_color: number
 direction: number
 closed: boolean
 close_reason?: string
 owner: boolean
 my_turn: boolean
 drawn_card_id: string
 can_bluff: boolean
 deadline: string | null
 server_time: string
 result: HistoryEntry | null
}
export interface GameCommand {
 game_id: string
 request_id: string
 expected_revision: number
 action: string
 card_id?: string
 color?: number
 target?: string
}
