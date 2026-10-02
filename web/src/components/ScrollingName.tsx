import { useEffect, useRef, useState, type CSSProperties } from 'react'

export function ScrollingName({ name }: { name: string }) {
 const viewport = useRef<HTMLSpanElement>(null)
 const text = useRef<HTMLSpanElement>(null)
 const [distance, setDistance] = useState(0)
 useEffect(() => {
   const container = viewport.current
   const content = text.current
   if (!container || !content) return
   let active = true
   const measure = () => {
     if (!active) return
     const overflow = Math.ceil(content.scrollWidth - container.clientWidth)
     setDistance(overflow > 1 ? overflow : 0)
   }
   measure()
   void document.fonts?.ready.then(measure)
   const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(measure) : undefined
   observer?.observe(container)
   observer?.observe(content)
   window.addEventListener('resize', measure)
   return () => {
     active = false
     observer?.disconnect()
     window.removeEventListener('resize', measure)
   }
 }, [name])
 const scrolling = distance > 0
 return <span ref={viewport} className={`scrolling-name${scrolling ? ' is-scrolling' : ''}`} title={name} tabIndex={scrolling ? 0 : undefined}
   style={{ '--name-offset': `${-distance}px`, '--name-duration': `${Math.max(8, distance / 24 * 2 + 4)}s` } as CSSProperties}>
   <span ref={text} className="scrolling-name-text" key={name}>{name}</span>
 </span>
}
