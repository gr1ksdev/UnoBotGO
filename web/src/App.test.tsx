import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import type { RankingPage } from './api/client'

function LocationProbe() {
 const location = useLocation()
 return <output data-testid="location">{location.pathname}{location.search}</output>
}

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
    const gruposBtn = screen.getByRole('link', { name: 'Grupos' })
    const playersBtn = screen.getByRole('link', { name: 'Players' })

    expect(updatedBtn).toHaveAttribute('aria-pressed', 'true')
    expect(legacyBtn).toHaveAttribute('aria-pressed', 'false')
    expect(gruposBtn).toHaveAttribute('aria-current', 'page')
    expect(playersBtn).not.toHaveAttribute('aria-current')

    // Switch system to Legacy
    fireEvent.click(legacyBtn)
    expect(legacyBtn).toHaveAttribute('aria-pressed', 'true')
    expect(playersBtn).toHaveAttribute('href', '/?system=legacy&tab=players')

    // Switch tab to Players
    fireEvent.click(playersBtn)
    expect(playersBtn).toHaveAttribute('aria-current', 'page')
    expect(gruposBtn).toHaveAttribute('href', '/?system=legacy&tab=groups')
    expect(screen.queryByRole('group', { name: 'Tipo de ranking' })).not.toBeInTheDocument()
  })

  it('navigates to group detail and allows navigating back preserving system and tab', async () => {
    window.Telegram = {
      WebApp: {
        initData: 'query_id=test&user=%7B%22id%22%3A123%7D&auth_date=1727700000&hash=mock',
        ready: vi.fn(),
        expand: vi.fn(),
        isVersionAtLeast: vi.fn(() => true),
        requestFullscreen: vi.fn(),
        setHeaderColor: vi.fn(),
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
    expect(window.Telegram?.WebApp.requestFullscreen).toHaveBeenCalledTimes(1)
    const groupLink = screen.getByRole('link', { name: /Grupo Alpha/ })
    fireEvent.click(groupLink)

    // Should render detail view
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: 'Ranking do grupo' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Ranking interno do grupo · Setembro' })).toBeInTheDocument()
      expect(screen.queryByText(/Total em/)).not.toBeInTheDocument()
    })

    expect(screen.queryByRole('navigation', { name: 'Navegação principal' })).not.toBeInTheDocument()

    // Click back button
    expect(window.Telegram?.WebApp.setHeaderColor).toHaveBeenLastCalledWith('#99121f')
    const backBtn = screen.getByRole('button', { name: 'Voltar ao Ranking Global' })
    fireEvent.click(backBtn)

    // Back on global list
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1, name: /Ranking Global/ })).toBeInTheDocument()
    })
    expect(window.Telegram?.WebApp.requestFullscreen).toHaveBeenCalledTimes(1)
    expect(window.Telegram?.WebApp.setHeaderColor).toHaveBeenLastCalledWith('#073b82')
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
      expect(screen.getByRole('navigation', { name: 'Navegação principal' })).toBeInTheDocument()
    })
  })

  it('restores a Players/Legacy deep link and changes section without losing the system', async () => {
    window.Telegram = { WebApp: { initData: 'fixture', ready: vi.fn(), expand: vi.fn() } }
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true, status: 200, json: async () => mockPage } as Response)
    render(<QueryClientProvider client={queryClient}><MemoryRouter initialEntries={['/?system=legacy&tab=players']}><App /><LocationProbe /></MemoryRouter></QueryClientProvider>)

    expect(screen.getByRole('link', { name: 'Players' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Legado' })).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/rankings/players?system=legacy'), expect.anything()))
    fireEvent.click(screen.getByRole('link', { name: 'Grupos' }))
    expect(screen.getByTestId('location')).toHaveTextContent('/?system=legacy&tab=groups')
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/rankings/groups?system=legacy'), expect.anything()))
    fireEvent.click(screen.getByRole('button', { name: 'Atualizado' }))
    expect(screen.getByTestId('location')).toHaveTextContent('/?system=updated&tab=groups')
  })

  it('keeps Telegram BackButton active on a direct detail link and returns to Grupos with the same system', async () => {
    const backButton = { show: vi.fn(), hide: vi.fn(), onClick: vi.fn<(fn: () => void) => void>(), offClick: vi.fn() }
    window.Telegram = { WebApp: { initData: 'fixture', ready: vi.fn(), expand: vi.fn(), BackButton: backButton } }
    vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true, status: 200, json: async () => mockPage } as Response)
    render(<QueryClientProvider client={queryClient}><MemoryRouter initialEntries={['/groups/grp1?system=legacy&tab=players']}><App /><LocationProbe /></MemoryRouter></QueryClientProvider>)

    await screen.findByRole('heading', { name: 'Ranking do grupo', level: 1 })
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
    expect(backButton.show).toHaveBeenCalled()
    expect(backButton.hide).not.toHaveBeenCalled()
    const callback = backButton.onClick.mock.calls.at(-1)?.[0]
    expect(callback).toBeDefined()
    act(() => callback?.())
    expect(screen.getByTestId('location')).toHaveTextContent('/?system=legacy&tab=groups')
    expect(screen.getByRole('link', { name: 'Grupos' })).toHaveAttribute('aria-current', 'page')
    expect(backButton.offClick).toHaveBeenCalledWith(callback)
    expect(backButton.hide).toHaveBeenCalled()
  })

  it('keeps navigation available while loading', async () => {
    window.Telegram = { WebApp: { initData: 'fixture', ready: vi.fn(), expand: vi.fn() } }
    let resolvePage!: (response: Response) => void
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise<Response>(resolve => { resolvePage = resolve }))
    render(<QueryClientProvider client={queryClient}><MemoryRouter><App /></MemoryRouter></QueryClientProvider>)
    expect(screen.getByRole('status', { name: 'Carregando ranking' })).toBeInTheDocument()
    expect(screen.getByRole('navigation')).toBeInTheDocument()
    await act(async () => resolvePage({ ok: true, status: 200, json: async () => mockPage } as Response))
  })

  it('allows switching sections from an API error', async () => {
    window.Telegram = { WebApp: { initData: 'fixture', ready: vi.fn(), expand: vi.fn() } }
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async input => String(input).includes('/players?')
      ? { ok: true, status: 200, json: async () => ({ ...mockPage, items: [] }) } as Response
      : { ok: false, status: 404 } as Response)
    render(<QueryClientProvider client={queryClient}><MemoryRouter><App /><LocationProbe /></MemoryRouter></QueryClientProvider>)
    expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível carregar o ranking.')
    fireEvent.click(screen.getByRole('link', { name: 'Players' }))
    expect(screen.getByTestId('location')).toHaveTextContent('/?system=updated&tab=players')
    expect(await screen.findByText('Nenhum jogador pontuou neste mês.')).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/players?system=updated'), expect.anything())
    expect(screen.getByRole('navigation')).toBeInTheDocument()
  })

})
