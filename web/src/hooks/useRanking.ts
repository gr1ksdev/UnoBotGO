import { useEffect, useMemo } from 'react'
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import { APIError, rankingPage, type System } from '../api/client'

export function useRanking(path: string, system: System) {
 const client = useQueryClient()
 const key = useMemo(() => ['rankings', path, system], [path, system])
 const query = useInfiniteQuery({
   queryKey: key, initialPageParam: '',
   queryFn: ({ pageParam, signal }) => rankingPage(path, system, pageParam, signal),
   getNextPageParam: page => page.next_cursor || undefined,
   staleTime: 30_000,
   retry: (count, error) => !(error instanceof APIError && [400, 401, 404, 409].includes(error.status)) && count < 2,
 })
 const pages = query.data?.pages
 const first = pages?.[0]
 useEffect(() => {
   if (!pages) return
   const keys = new Set<string>()
   let position = 0
   for (const page of pages) {
     if (page.month_start !== pages[0].month_start || page.items.some(item => {
       position++
       if (keys.has(item.key) || item.position !== position) return true
       keys.add(item.key); return false
     })) { void client.resetQueries({ queryKey: key, exact: true }); break }
   }
 }, [pages, client, key])
 useEffect(() => {
   if (query.error instanceof APIError && query.error.status === 409 && path.indexOf('groups/') !== 0) void client.resetQueries({ queryKey: key, exact: true })
 }, [query.error, client, key, path])
 useEffect(() => {
   if (!first) return
   // The interval comes from backend timestamps, not the browser's timezone.
   const delay = Math.max(1000, Math.min(2_147_000_000, Date.parse(first.month_ends_at) - Date.parse(first.server_time) + 100))
   const timer = setTimeout(() => { void client.resetQueries({ queryKey: key, exact: true }) }, delay)
   return () => clearTimeout(timer)
 }, [first, client, key])
 return { ...query, first, items: pages?.flatMap(page => page.items) ?? [] }
}
