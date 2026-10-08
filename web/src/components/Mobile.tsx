import { Link, useLocation } from 'react-router'
import { Avatar, displayName, ErrorState } from './Ranking'
import type { RankingItem, System } from '../api/client'
import { formatScore } from '../lib/score'
export function Icon({ name }: { name: string }) {
 return (
  <span
   className="icon"
   style={{ maskImage: `url(/assets/icons/${name}.svg)` }}
   aria-hidden="true"
  />
 )
}
export function Brand() {
 return (
  <div className="brand">
   <span className="brand-mark">
    <Icon name="cards" />
   </span>
   UnoBotGO
  </div>
 )
}
export function BottomNav() {
 const { pathname } = useLocation()
 return (
  <nav className="bottom-nav" aria-label="Navegação principal">
   {[
    { to: '/home', name: 'Jogar', icon: 'home' },
    { to: '/ranking', name: 'Ranking', icon: 'trophy' },
    { to: '/profile', name: 'Perfil', icon: 'user' },
   ].map((item) => (
    <Link
     key={item.to}
     className={
      pathname === item.to || (item.to === '/ranking' && pathname === '/')
       ? 'active'
       : ''
     }
     to={item.to}
    >
     <Icon name={item.icon} />
     <span>{item.name}</span>
    </Link>
   ))}
  </nav>
 )
}
export function RankingRow({
 item,
 system,
 self = false,
}: {
 item: RankingItem
 system: System
 self?: boolean
}) {
 const content = (
  <>
   <span className="rank">{item.position}</span>
   <Avatar item={item} />
   <div className="who">
    <strong title={displayName(item)}>{displayName(item)}</strong>
    <small>{self ? 'Sua posição neste mês' : item.masked_id}</small>
   </div>
   <div className="points">
    {formatScore(item.score_units, system)}
    <small>pontos</small>
   </div>
  </>
 )
 return item.group_ref ? (
  <Link
   className={`row ${self ? 'self' : ''}`}
   to={`/groups/${encodeURIComponent(item.group_ref)}?system=${system}`}
  >
   {content}
  </Link>
 ) : (
  <div className={`row ${self ? 'self' : ''}`}>{content}</div>
 )
}
export function Podium({
 items,
 system,
}: {
 items: RankingItem[]
 system: System
}) {
 if (items.length < 3)
  return (
   <div className="list few-results">
    {items.map((item) => (
     <RankingRow key={item.key} item={item} system={system} />
    ))}
   </div>
  )
 return (
  <div className="podium" aria-label="Pódio">
   {[items[1], items[0], items[2]].map((item, i) => (
    <div
     key={item.key}
     className={`podium-item ${i === 1 ? 'first' : i === 2 ? 'third' : ''}`}
    >
     <div className="crown">{i === 1 && <Icon name="trophy" />}</div>
     <Avatar item={item} />
     <div className="podium-name" title={displayName(item)}>
      {item.group_ref ? (
       <Link
        to={`/groups/${encodeURIComponent(item.group_ref)}?system=${system}`}
       >
        {displayName(item)}
       </Link>
      ) : (
       displayName(item)
      )}
     </div>
     <div className="podium-score">
      {formatScore(item.score_units, system)} pts
     </div>
     <div className="podium-base">{item.position}</div>
    </div>
   ))}
  </div>
 )
}
export function MyPosition({
 entry,
 system,
}: {
 entry: RankingItem | null
 system: System
}) {
 return (
  <div className="self-rank">
   {entry ? (
    <RankingRow item={entry} system={system} self />
   ) : (
    <p className="notice">
     Jogue uma partida elegível para aparecer neste ranking mensal.
    </p>
   )}
  </div>
 )
}
export function Loading({ label = 'Carregando ranking' }: { label?: string }) {
 return (
  <div role="status" aria-label={label}>
   <div className="skeleton loading-hero" />
   {[0, 1, 2].map((n) => (
    <div className="skeleton" key={n} />
   ))}
  </div>
 )
}
export function Empty({
 title,
 children,
}: {
 title: string
 children?: React.ReactNode
}) {
 return (
  <div className="empty">
   <Icon name="cards" />
   <h2>{title}</h2>
   {children}
  </div>
 )
}
export { ErrorState }
export function PolicyPicker({
 system,
 change,
}: {
 system: System
 change: (s: System) => void
}) {
 return (
  <div className="policy-picker" role="group" aria-label="Sistema de ranking">
   {(['updated', 'legacy'] as const).map((s) => (
    <button key={s} aria-pressed={s === system} onClick={() => change(s)}>
     {s === 'updated' ? 'Atualizado' : 'Legado'}
    </button>
   ))}
  </div>
 )
}

export function RankingTabs({
 category,
 change,
}: {
 category: 'players' | 'groups'
 change: (category: 'players' | 'groups') => void
}) {
 return (
  <div className="segmented" role="tablist" aria-label="Categorias de ranking">
   {(['players', 'groups'] as const).map((t) => (
    <button
     role="tab"
     key={t}
     aria-selected={t === category}
     onClick={() => change(t)}
    >
     <Icon name={t === 'players' ? 'user' : 'users'} />
     {t === 'players' ? 'Jogadores' : 'Grupos'}
    </button>
   ))}
  </div>
 )
}
