import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import { ScrollingName } from './ScrollingName'
import { useQuery } from '@tanstack/react-query'
import { authHeaders, type RankingItem, type System } from '../api/client'
import { formatScore, scoreUnit } from '../lib/score'

// Presentation only: persisted Telegram names and ranking identities stay intact.
export function displayName(item: RankingItem) {
 return !item.group_ref && !/[^\p{P}\p{Z}\p{C}\s]/u.test(item.name) ? 'Jogador' : item.name
}

function AnonymousIcon() {
 return <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className="icon-anonymous"><path d="M12 2a5 5 0 0 1 5 5v1a5 5 0 0 1-10 0V7a5 5 0 0 1 5-5zm0 13c4.42 0 8 2.24 8 5v2H4v-2c0-2.76 3.58-5 8-5z" opacity="0.75" /></svg>
}

export function Avatar({ item, large = false }: { item: RankingItem; large?: boolean }) {
 const ref = useRef<HTMLSpanElement>(null)
 const [visible, setVisible] = useState(false)
 useEffect(() => {
  if (!ref.current || !('IntersectionObserver' in window)) return
  const observer = new IntersectionObserver(entries => { if (entries.some(e => e.isIntersecting)) { setVisible(true); observer.disconnect() } }, { rootMargin: '100px' })
  observer.observe(ref.current); return () => observer.disconnect()
 }, [])
 const photo = useQuery({
  queryKey: ['avatar', item.key], enabled: visible && !item.anonymous && !!item.avatar_url,
  staleTime: 300_000, gcTime: 600_000, retry: 2,
  queryFn: async ({ signal }) => {
   const response = await fetch(item.avatar_url, { headers: authHeaders(), signal })
   if (response.status === 202) throw new Error('Photo pending')
   if (!response.ok || response.status === 204) return null
   return response.blob()
  }, retryDelay: 2000,
 })
 const [url, setUrl] = useState<string>()
 useEffect(() => {
  if (!photo.data) return
  const next = URL.createObjectURL(photo.data); setUrl(next)
  return () => URL.revokeObjectURL(next)
 }, [photo.data])
 if (item.anonymous) {
  return <span ref={ref} className={`avatar ${large ? 'avatar-large' : ''} avatar-anonymous`} aria-hidden="true"><AnonymousIcon /></span>
 }
 const initials = displayName(item).trim().split(/\s+/u).slice(0, 2).map(part => Array.from(part)[0]).join('')
 return <span ref={ref} className={`avatar ${large ? 'avatar-large' : ''}`} aria-hidden="true">{url ? <img src={url} loading="lazy" alt="" onError={() => setUrl(undefined)} /> : initials}</span>
}

export function RankBadge({ position }: { position: number }) {
 if (position > 3) return <span className="rank-number">{position}</span>
 return <span className={`medal medal-${position}`} aria-label={`${position}º lugar`}>
  <svg viewBox="0 0 36 46" aria-hidden="true"><path className="ribbon" d="m9 27-2 18 10-5 10 5-1-19z"/><path className="rim" d="m18 1 5 3 5 1 2 5 4 5-1 6-1 5-5 3-4 4-6-1-6-1-3-5-4-4 1-6 1-5 5-4z"/><circle className="coin" cx="18" cy="17" r="12"/><circle className="shine" cx="18" cy="17" r="10"/><text x="18" y="23" textAnchor="middle">{position}</text></svg>
 </span>
}
export function Score({ item, system }: { item: RankingItem; system: System }) {
 return <span className="score">{formatScore(item.score_units, system)} <small>{scoreUnit(item.score_units, system)}</small></span>
}
export function RankingCard({ item, system, tab }: { item: RankingItem; system: System; tab: string }) {
 const content = <><RankBadge position={item.position} /><Avatar item={item} /><span className="identity"><span className="player-name"><ScrollingName name={displayName(item)} /></span>{item.masked_id && <span className="masked-id">{item.masked_id}</span>}</span><Score item={item} system={system} /></>
 const className = `ranking-card place-${item.position}`
 return <li>{item.group_ref ? <Link className={className} to={`/groups/${encodeURIComponent(item.group_ref)}?system=${system}&tab=${tab}`}>{content}</Link> : <div className={className}>{content}</div>}</li>
}
export function Skeleton() { return <div role="status" aria-label="Carregando ranking" className="skeleton-list">{Array.from({ length: 7 }, (_, i) => <div key={i} className="ranking-card skeleton"><span className="skeleton-medal"/><span className="skeleton-avatar"/><span className="skeleton-name"/><span className="skeleton-score"/></div>)}</div> }
export function ErrorState({ retry, unauthorized = false }: { retry: () => void; unauthorized?: boolean }) {
 return <div role="alert" className="state-card"><p>{unauthorized ? 'Abra o Ranking Global pelo Telegram.' : 'Não foi possível carregar o ranking.'}</p>{!unauthorized && <button onClick={retry}>Tentar novamente</button>}</div>
}
export function Segmented({ label, options, value, onChange, className = '' }: { label: string; options: { value: string; label: string }[]; value: string; onChange: (value: string) => void; className?: string }) {
 return <div className={`segmented ${className}`} role="group" aria-label={label}>{options.map(option => <button key={option.value} aria-pressed={option.value === value} className={`segmented-button ${option.value === value ? `selected selected-${option.value} ${value}` : ''}`} onClick={() => onChange(option.value)}>{option.label}</button>)}</div>
}
export function Arrow() {return <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true"><path d="m12 4-8 8 8 8M4 12h17"/></svg>}
export function Calendar() {return <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><rect x="4" y="5" width="16" height="16" rx="2"/><path d="M8 2v6m8-6v6M4 10h16m-12 4h2m4 0h2m-8 3h2m4 0h2"/></svg>}
export function UsersIcon() {return <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 3s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z"/></svg>}
