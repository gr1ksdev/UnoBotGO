import { describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Avatar, RankBadge, Score, RankingCard, Skeleton, ErrorState, Segmented, displayName } from './Ranking'
import type { RankingItem } from '../api/client'

const testQueryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

function wrap(ui: React.ReactElement) {
  return render(
    <QueryClientProvider client={testQueryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

describe('Ranking Components', () => {
  const dummyItem: RankingItem = {
    position: 1,
    key: 'k1',
    name: 'Grupo dos Amigos',
    masked_id: 'ID ••••1234',
    score_units: '284000',
    avatar_url: '/api/v1/media/ref1',
    group_ref: 'group_ref_123',
  }

  const dummyPlayerItem: RankingItem = {
    position: 4,
    key: 'k2',
    name: 'Carlos Silva',
    masked_id: 'ID ••••5678',
    score_units: '100',
    avatar_url: '/api/v1/media/ref2',
  }

  describe('Avatar', () => {
    it('renders initials as fallback when image is not yet loaded', () => {
      wrap(<Avatar item={dummyItem} />)
      expect(screen.getByText('Gd')).toBeInTheDocument()
    })
  })

  describe('RankBadge', () => {
    it('renders medals for positions 1, 2, and 3', () => {
      const { rerender } = wrap(<RankBadge position={1} />)
      expect(screen.getByLabelText('1º lugar')).toBeInTheDocument()

      rerender(<RankBadge position={2} />)
      expect(screen.getByLabelText('2º lugar')).toBeInTheDocument()

      rerender(<RankBadge position={3} />)
      expect(screen.getByLabelText('3º lugar')).toBeInTheDocument()
    })

    it('renders clean number for position > 3', () => {
      wrap(<RankBadge position={4} />)
      expect(screen.getByText('4')).toBeInTheDocument()
    })
  })

  describe('Score', () => {
    it('renders updated score with pts', () => {
      wrap(<Score item={dummyItem} system="updated" />)
      expect(screen.getByText(/2\.840,00/)).toBeInTheDocument()
      expect(screen.getByText('pts')).toBeInTheDocument()
    })

    it('renders legacy score with pt for 100 units', () => {
      wrap(<Score item={dummyPlayerItem} system="legacy" />)
      expect(screen.getByText(/1/)).toBeInTheDocument()
      expect(screen.getByText('pt')).toBeInTheDocument()
    })
  })

  describe('displayName', () => {
    it.each(['.', ' ... ', '', '   ', '\u200b'])('uses a neutral player fallback for %j without changing the item', name => {
      const item = { ...dummyPlayerItem, name }
      expect(displayName(item)).toBe('Jogador')
      expect(item.name).toBe(name)
    })

    it.each(['João', '李', '🦊', 'é', '123'])('preserves useful Unicode names: %s', name => {
      expect(displayName({ ...dummyPlayerItem, name })).toBe(name)
    })

    it('preserves group titles and uses the fallback in the player row and avatar', () => {
      expect(displayName({ ...dummyItem, name: '.' })).toBe('.')
      wrap(<RankingCard item={{ ...dummyPlayerItem, name: '.' }} system="updated" tab="players" />)
      expect(screen.getByText('Jogador')).toBeInTheDocument()
      expect(screen.getByText('J')).toBeInTheDocument()
      expect(screen.getByText('ID ••••5678')).toBeInTheDocument()
    })
  })

  describe('RankingCard', () => {
    it('renders clickable Link when group_ref is present', () => {
      wrap(<RankingCard item={dummyItem} system="updated" tab="groups" />)
      const link = screen.getByRole('link')
      expect(link).toHaveAttribute('href', '/groups/group_ref_123?system=updated&tab=groups')
      expect(screen.getByText('Grupo dos Amigos')).toBeInTheDocument()
      expect(screen.getByText('ID ••••1234')).toBeInTheDocument()
    })

    it('renders non-clickable element when group_ref is absent (player item)', () => {
      wrap(<RankingCard item={dummyPlayerItem} system="legacy" tab="players" />)
      expect(screen.queryByRole('link')).not.toBeInTheDocument()
      expect(screen.getByText('Carlos Silva')).toBeInTheDocument()
      expect(screen.getByText('ID ••••5678')).toBeInTheDocument()
    })
  })

  describe('Skeleton', () => {
    it('renders loading status skeleton', () => {
      wrap(<Skeleton />)
      expect(screen.getByRole('status', { name: 'Carregando ranking' })).toBeInTheDocument()
    })
  })

  describe('ErrorState', () => {
    it('renders error message and retry button', () => {
      const retry = vi.fn()
      wrap(<ErrorState retry={retry} />)
      expect(screen.getByRole('alert')).toHaveTextContent('Não foi possível carregar o ranking.')
      const button = screen.getByRole('button', { name: 'Tentar novamente' })
      fireEvent.click(button)
      expect(retry).toHaveBeenCalledTimes(1)
    })

    it('renders unauthorized message without retry button when unauthorized', () => {
      wrap(<ErrorState retry={vi.fn()} unauthorized />)
      expect(screen.getByRole('alert')).toHaveTextContent('Abra o Ranking Global pelo Telegram.')
      expect(screen.queryByRole('button')).not.toBeInTheDocument()
    })
  })

  describe('Segmented', () => {
    it('toggles value on button click', () => {
      const onChange = vi.fn()
      wrap(
        <Segmented
          label="Sistema"
          options={[
            { value: 'updated', label: 'Atualizado' },
            { value: 'legacy', label: 'Legado' },
          ]}
          value="updated"
          onChange={onChange}
        />
      )
      const legacyBtn = screen.getByRole('button', { name: 'Legado' })
      fireEvent.click(legacyBtn)
      expect(onChange).toHaveBeenCalledWith('legacy')
    })
  })

  describe('Anonymous Mode presentation', () => {
    it('renders neutral anonymous avatar without initials for anonymous item', () => {
      const anonItem: RankingItem = {
        position: 1,
        key: 'anon1',
        name: 'Anônimo',
        masked_id: '',
        score_units: '500',
        avatar_url: '',
        anonymous: true,
      }
      const { container } = wrap(<Avatar item={anonItem} />)
      expect(container.querySelector('.avatar-anonymous')).toBeInTheDocument()
      expect(container.querySelector('.icon-anonymous')).toBeInTheDocument()
      expect(screen.queryByText('An')).not.toBeInTheDocument()
    })

    it('omits masked_id when empty in RankingCard', () => {
      const anonItem: RankingItem = {
        position: 1,
        key: 'anon1',
        name: 'Anônimo',
        masked_id: '',
        score_units: '500',
        avatar_url: '',
        anonymous: true,
      }
      const { container } = wrap(<RankingCard item={anonItem} system="updated" tab="players" />)
      expect(container.querySelector('.masked-id')).not.toBeInTheDocument()
      expect(screen.getByText('Anônimo')).toBeInTheDocument()
    })
  })
})
