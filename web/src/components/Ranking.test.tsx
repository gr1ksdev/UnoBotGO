import { describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Avatar, RankBadge, Score, RankingCard, Skeleton, ErrorState, Segmented } from './Ranking'
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
})
