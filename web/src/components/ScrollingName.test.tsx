import { act, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ScrollingName } from './ScrollingName'

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('ScrollingName', () => {
 it('moves only overflowing names and recalculates after resize', () => {
   let available = 120
   vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(() => available)
   vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockReturnValue(240)
   const disconnect = vi.fn()
   let resize: () => void = () => {}
   vi.stubGlobal('ResizeObserver', class {
     constructor(callback: () => void) { resize = callback }
     observe() {}
     disconnect = disconnect
   })
   const { unmount } = render(<ScrollingName name="Nome completo muito longo" />)
   const container = screen.getByTitle('Nome completo muito longo')
   expect(container.className).toContain('is-scrolling')
   expect(container.style.getPropertyValue('--name-offset')).toBe('-120px')
   expect(container.tabIndex).toBe(0)
   expect(screen.getAllByText('Nome completo muito longo')).toHaveLength(1)
   available = 300
   act(() => resize())
   expect(container.className).not.toContain('is-scrolling')
   expect(container.hasAttribute('tabindex')).toBe(false)
   unmount()
   expect(disconnect).toHaveBeenCalledOnce()
 })

 it('keeps short names still and remeasures a changed name without ResizeObserver', () => {
   vi.stubGlobal('ResizeObserver', undefined)
   vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(120)
   vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockImplementation(function (this: HTMLElement) {
     return this.textContent === 'Ana' ? 30 : 220
   })
   const { rerender } = render(<ScrollingName name="Ana" />)
   expect(screen.getByTitle('Ana').className).not.toContain('is-scrolling')
   rerender(<ScrollingName name="Nome novo muito longo" />)
   expect(screen.getByTitle('Nome novo muito longo').style.getPropertyValue('--name-offset')).toBe('-100px')
 })
})
