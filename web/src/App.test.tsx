import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import type { RankingPage } from './api/client'

const mockPage: RankingPage = {
  month_start: '2026-09-01',
  month_name: 'Setembro',
  month_ends_at: '2026-10-01T03:00:00Z',
  server_time: '2026-09-30T14:00:00Z',
  timezone: 'America/Sao_Paulo',
  system: 'updated',
  items: [
    {
      position: 1,
      key: 'k1',
      group_ref: 'grp1',
      name: 'Grupo Alpha',
      masked_id: 'ID ••••1111',
      score_units: '500000',
      avatar_url: '/api/v1/media/grp1',
    },
    {
      position: 2,
      key: 'k2',
      group_ref: 'grp2',
      name: 'Grupo Beta',
      masked_id: 'ID ••••2222',
      score_units: '300000',
      avatar_url: '/api/v1/media/grp2',
    },
  ],
  group: {
    position: 1,
    key: 'k1',
    name: 'Grupo Alpha',
    masked_id: 'ID ••••1111',
    score_units: '500000',
    avatar_url: '/api/v1/media/grp1',
  },
}

describe('App and Full Routing Flows', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, staleTime: 0 },
      },
    })
    vi.restoreAllMocks()
  })

  it('renders unauthorized banner when Telegram WebApp initData is missing', () => {
    window.Telegram = undefined
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    )

    expect(screen.getByText('Abra o Ranking Global pelo Telegram.')).toBeInTheDocument()
  })

  it('renders ranking page, defaults to Updated & Grupos, and allows switching', async () => {
    window.Telegram = {
      WebApp: {
        initData: 'query_id=test&user=%7B%22id%22%3A123%7D&auth_date=1727700000&hash=mock',
        ready: vi.fn(),
        expand: vi.fn(),
      },
    }

    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
      const url = String(input)
      if (url.includes('/api/v1/rankings/groups') || url.includes('/api/v1/rankings/players')) {
        return {
          ok: true,
          status: 200,
          json: async () => mockPage,
        } as Response
      }
      return { ok: false, status: 404 } as Response
    })

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/']}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    )

    // Wait for items to be loaded
    await waitFor(() => {
      expect(screen.getByText('Grupo Alpha')).toBeInTheDocument()
    })

    // Verify defaults: Updated system and Grupos tab
    const updatedBtn = screen.getByRole('button', { name: 'Atualizado' })
    const legacyBtn = screen.getByRole('button', { name: 'Legado' })
    const gruposBtn = screen.getByRole('button', { name: 'Grupos' })
    const playersBtn = screen.getByRole('button', { name: 'Players' })

    expect(updatedBtn).toHaveAttribute('aria-pressed', 'true')
    expect(legacyBtn).toHaveAttribute('aria-pressed', 'false')
    expect(gruposBtn).toHaveAttribute('aria-pressed', 'true')
    expect(playersBtn).toHaveAttribute('aria-pressed', 'false')

    // Switch system to Legacy
    fireEvent.click(legacyBtn)
    expect(legacyBtn).toHaveAttribute('aria-pressed', 'true')

    // Switch tab to Players
    fireEvent.click(playersBtn)
    expect(playersBtn).toHaveAttribute('aria-pressed', 'true')
  })

  it('navigates to group detail and allows navigating back preserving system and tab', async () => {
    window.Telegram = {
      WebApp: {
        initData: 'query_id=test&user=%7B%22id%22%3A123%7D&auth_date=1727700000&hash=mock',
        ready: vi.fn(),
        expand: vi.fn(),
        BackButton: {
          show: vi.fn(),
          hide: vi.fn(),
          onClick: vi.fn(),
          offClick: vi.fn(),
        },
      },
    }

    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      return {
        ok: true,
        status: 200,
        json: async () => mockPage,
      } as Response
    })

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/?system=updated&tab=groups']}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    )

    await waitFor(() => {
      expect(screen.getByText('Grupo Alpha')).toBeInTheDocument()
    })

    // Click group card link to go to detail
    const groupLink = screen.getByRole('link', { name: /Grupo Alpha/ })
    fireEvent.click(groupLink)

    // Should render detail view
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Ranking do grupo' })).toBeInTheDocument()
      expect(screen.getByText(/Total do grupo no mês de Setembro/)).toBeInTheDocument()
    })

    // Click back button
    const backBtn = screen.getByRole('button', { name: 'Voltar ao Ranking Global' })
    fireEvent.click(backBtn)

    // Back on global list
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: /Ranking Global/ })).toBeInTheDocument()
    })
  })

  it('renders empty state when ranking returns zero items', async () => {
    window.Telegram = {
      WebApp: {
        initData: 'query_id=test&user=%7B%22id%22%3A123%7D&auth_date=1727700000&hash=mock',
        ready: vi.fn(),
        expand: vi.fn(),
      },
    }

    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          ...mockPage,
          items: [],
        }),
      } as Response
    })

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/?system=updated&tab=groups']}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    )

    await waitFor(() => {
      expect(screen.getByText('Nenhum grupo pontuou neste mês.')).toBeInTheDocument()
    })
  })
})
