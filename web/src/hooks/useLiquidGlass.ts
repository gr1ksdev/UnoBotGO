import { useEffect, useId, useRef, useState } from 'react'

// Refract the real backdrop at the capsule rim; regenerate only on resize.
export function useLiquidGlass() {
 const ref = useRef<HTMLElement>(null)
 const id = `glass-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`
 const [lens, setLens] = useState<{ image: string; width: number; height: number }>()
 useEffect(() => {
   const element = ref.current
   if (!element || typeof CSS === 'undefined' || typeof CSS.supports !== 'function' || !CSS.supports('backdrop-filter', `url(#${id})`)) return
   const update = () => {
     const width = Math.round(element.offsetWidth)
     const height = Math.round(element.offsetHeight)
     if (!width || !height) return
     const canvas = document.createElement('canvas')
     canvas.width = width
     canvas.height = height
     const context = canvas.getContext('2d')
     if (!context) return
     const pixels = context.createImageData(width, height)
     const radius = height / 2
     for (let y = 0; y < height; y++) {
       for (let x = 0; x < width; x++) {
         const center = Math.max(radius, Math.min(width - radius, x))
         const dx = x - center
         const dy = y - radius
         const distance = Math.hypot(dx, dy)
         const strength = Math.max(0, Math.min(1, 1 - (radius - distance) / 12)) ** 2
         const index = (y * width + x) * 4
         pixels.data[index] = 128 + 127 * strength * dx / (distance || 1)
         pixels.data[index + 1] = 128 + 127 * strength * dy / (distance || 1)
         pixels.data[index + 2] = 128
         pixels.data[index + 3] = 255
       }
     }
     context.putImageData(pixels, 0, 0)
     setLens({ image: canvas.toDataURL(), width, height })
   }
   update()
   if (typeof ResizeObserver === 'undefined') {
     window.addEventListener('resize', update)
     return () => window.removeEventListener('resize', update)
   }
   const observer = new ResizeObserver(update)
   observer.observe(element)
   return () => observer.disconnect()
 }, [id])
 return { ref, id, lens }
}
