import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ProfilePage } from './Profile'

function wrap(ui: React.ReactElement) {
  const testClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={testClient}>
      <MemoryRouter initialEntries={['/profile']}>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

describe('ProfilePage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.scrollTo = vi.fn()
    window.Telegram = {
      WebApp: {
        initData: 'query_id=123',
        initDataUnsafe: {
          user: {
            id: 12345,
            first_name: 'Gabriel',
            last_name: 'Silva',
            username: 'gabrielsilva',
            photo_url: 'https://example.com/avatar.jpg',
          },
        },
        ready: vi.fn(),
        expand: vi.fn(),
        close: vi.fn(),
        setHeaderColor: vi.fn(),
        BackButton: { show: vi.fn(), hide: vi.fn(), onClick: vi.fn(), offClick: vi.fn() },
      },
    } as unknown as typeof window.Telegram
  })

  it('renders user name and photo, and converts them to anonymous when toggled', async () => {
    let isAnon = false

    vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, init) => {
      const u = typeof url === 'string' ? url : url.toString()
      if (u.includes('/api/v1/me/privacy')) {
        if (init?.method === 'PUT') {
          const body = JSON.parse(init.body as string)
          isAnon = body.anonymous
          return new Response(JSON.stringify({ anonymous: isAnon }), { status: 200 })
        }
        return new Response(JSON.stringify({ anonymous: isAnon }), { status: 200 })
      }
      return new Response('{}', { status: 404 })
    })

    wrap(<ProfilePage />)

    // Check header
    expect(screen.getByRole('heading', { level: 1, name: 'Meu Perfil' })).toBeInTheDocument()

    // Initially public: shows user's real name, username and photo
    await waitFor(() => {
      const btn = screen.getByRole('switch')
      expect(btn).toBeEnabled()
      expect(btn).toHaveAttribute('aria-checked', 'false')
    })
    expect(screen.getByRole('heading', { level: 2, name: 'Gabriel Silva' })).toBeInTheDocument()
    expect(screen.getByText('@gabrielsilva')).toBeInTheDocument()
    const img = screen.getByRole('img')
    expect(img).toHaveAttribute('src', 'https://example.com/avatar.jpg')
    expect(document.querySelector('.icon-anonymous')).not.toBeInTheDocument()

    // Click toggle to enable anonymous mode
    const switchBtn = screen.getByRole('switch')
    fireEvent.click(switchBtn)

    // Should convert name and photo to anonymous
    await waitFor(() => {
      expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true')
    })
    expect(screen.getByRole('heading', { level: 2, name: 'Anônimo' })).toBeInTheDocument()
    expect(screen.getByText('Modo anônimo ativado • Oculto no ranking')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(document.querySelector('.icon-anonymous')).toBeInTheDocument()
    expect(screen.getByText('✓ Salvo com sucesso')).toBeInTheDocument()

    // Toggle back to public: restores user's real name and photo
    fireEvent.click(switchBtn)
    await waitFor(() => {
      expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'false')
    })
    expect(screen.getByRole('heading', { level: 2, name: 'Gabriel Silva' })).toBeInTheDocument()
    expect(screen.getByRole('img')).toHaveAttribute('src', 'https://example.com/avatar.jpg')

    // Bottom navigation should have Perfil active
    const nav = screen.getByRole('navigation', { name: 'Navegação principal' })
    expect(nav).toHaveAttribute('data-tab', 'profile')
  })

  it('renders initials when user has no photo', async () => {
    window.Telegram = {
      WebApp: {
        initData: 'query_id=123',
        initDataUnsafe: {
          user: {
            first_name: 'Lucas',
            last_name: 'Pereira',
          },
        },
        ready: vi.fn(),
        expand: vi.fn(),
        close: vi.fn(),
        setHeaderColor: vi.fn(),
        BackButton: { show: vi.fn(), hide: vi.fn(), onClick: vi.fn(), offClick: vi.fn() },
      },
    } as unknown as typeof window.Telegram

    vi.spyOn(globalThis, 'fetch').mockImplementation(async (url) => {
      const u = typeof url === 'string' ? url : url.toString()
      if (u.includes('/api/v1/me/privacy')) {
        return new Response(JSON.stringify({ anonymous: false }), { status: 200 })
      }
      return new Response('{}', { status: 404 })
    })

    wrap(<ProfilePage />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: 'Lucas Pereira' })).toBeInTheDocument()
    })
    expect(screen.getByText('LP')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('handles error when saving privacy fails', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (url, init) => {
      const u = typeof url === 'string' ? url : url.toString()
      if (u.includes('/api/v1/me/privacy')) {
        if (init?.method === 'PUT') {
          return new Response('{"error":"busy"}', { status: 500 })
        }
        return new Response(JSON.stringify({ anonymous: false }), { status: 200 })
      }
      return new Response('{}', { status: 404 })
    })

    wrap(<ProfilePage />)

    await waitFor(() => {
      const btn = screen.getByRole('switch')
      expect(btn).toBeEnabled()
      expect(btn).toHaveAttribute('aria-checked', 'false')
    })

    fireEvent.click(screen.getByRole('switch'))

    await waitFor(() => {
      expect(screen.getByText('Não foi possível salvar a alteração.')).toBeInTheDocument()
    })
  })
})
