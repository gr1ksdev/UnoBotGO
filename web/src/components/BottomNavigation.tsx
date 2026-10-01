import type { CSSProperties } from 'react'
import { useLiquidGlass } from '../hooks/useLiquidGlass'
import { Link } from 'react-router'
import type { System } from '../api/client'
import { UsersIcon } from './Ranking'

function PlayerIcon() {
 return <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="12" cy="7" r="4" /><path d="M4 21v-3c0-3 3.6-5 8-5s8 2 8 5v3z" /></svg>
}

export function BottomNavigation({ system, tab }: { system: System; tab: 'groups' | 'players' }) {
 const { ref, id, lens } = useLiquidGlass()
 return <nav ref={ref} className="bottom-navigation" data-tab={tab} aria-label="Navegação principal"
   style={lens ? { '--glass-filter': `url(#${id})` } as CSSProperties : undefined}
   onPointerMove={event => {
     const bounds = event.currentTarget.getBoundingClientRect()
     event.currentTarget.style.setProperty('--glass-light-x', `${event.clientX - bounds.left}px`)
     event.currentTarget.style.setProperty('--glass-light-y', `${event.clientY - bounds.top}px`)
   }}
   onPointerLeave={event => {
     event.currentTarget.style.removeProperty('--glass-light-x')
     event.currentTarget.style.removeProperty('--glass-light-y')
   }}>
   {lens && <svg className="glass-definitions" aria-hidden="true" width="0" height="0">
     <defs><filter id={id} x="0" y="0" width="100%" height="100%" colorInterpolationFilters="sRGB">
       <feGaussianBlur in="SourceGraphic" stdDeviation="2.4" result="soft" />
       <feImage href={lens.image} width={lens.width} height={lens.height} preserveAspectRatio="none" result="lens" />
       <feDisplacementMap in="soft" in2="lens" scale="18" xChannelSelector="R" yChannelSelector="G" />
     </filter></defs>
   </svg>}
   <span className="glass-selection" aria-hidden="true" />
   <Link to={`/?system=${system}&tab=groups`} aria-current={tab === 'groups' ? 'page' : undefined} className={`bottom-nav-item nav-groups ${tab === 'groups' ? 'is-active' : ''}`}>
     <span className="bottom-nav-icon" aria-hidden="true"><UsersIcon /></span>
     <span>Grupos</span>
   </Link>
   <Link to={`/?system=${system}&tab=players`} aria-current={tab === 'players' ? 'page' : undefined} className={`bottom-nav-item nav-players ${tab === 'players' ? 'is-active' : ''}`}>
     <span className="bottom-nav-icon" aria-hidden="true"><PlayerIcon /></span>
     <span>Players</span>
   </Link>
 </nav>
}
