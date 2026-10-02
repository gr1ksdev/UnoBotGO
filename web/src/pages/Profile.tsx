import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router'
import { getUserPrivacy, setUserPrivacy, type System } from '../api/client'
import { BottomNavigation } from '../components/BottomNavigation'
import { useTelegram } from '../lib/telegram'

export function ProfilePage() {
 const [params] = useSearchParams()
 const system: System = params.get('system') === 'legacy' ? 'legacy' : 'updated'
 useTelegram(undefined, { headerColor: '#073b82' })
 const client = useQueryClient()
 const [savedFeedback, setSavedFeedback] = useState(false)
 const [photoError, setPhotoError] = useState(false)

 const user = typeof window !== 'undefined' ? window.Telegram?.WebApp?.initDataUnsafe?.user : undefined
 const realName = [user?.first_name, user?.last_name].filter(Boolean).join(' ') || user?.username || 'Jogador'
 const initials = realName.trim().split(/\s+/u).slice(0, 2).map(p => Array.from(p)[0]).join('').toUpperCase() || 'J'

 useEffect(() => {
  window.scrollTo(0, 0)
 }, [])

 const { data, isPending } = useQuery({
  queryKey: ['me', 'privacy'],
  queryFn: ({ signal }) => getUserPrivacy(signal),
  staleTime: 60_000,
 })

 const mutation = useMutation({
  mutationFn: (anonymous: boolean) => setUserPrivacy(anonymous),
  onSuccess: (updated) => {
   client.setQueryData(['me', 'privacy'], updated)
   void client.invalidateQueries({ queryKey: ['rankings'] })
   setSavedFeedback(true)
   setTimeout(() => setSavedFeedback(false), 2500)
  },
 })

 const isAnonymous = data?.anonymous ?? false
 const displayName = isAnonymous ? 'Anônimo' : realName
 const statusText = isAnonymous
  ? 'Modo anônimo ativado • Oculto no ranking'
  : (user?.username ? `@${user.username}` : 'Visível publicamente no Ranking Global')

 const handleToggle = () => {
  if (isPending || mutation.isPending) return
  mutation.mutate(!isAnonymous)
 }

 return <main className="app-shell global-view profile-view">
  <header className="ranking-header">
   <div className="title-bar">
    <div className="title-bar-left" />
    <h1>Meu Perfil</h1>
    <div className="title-bar-right" />
   </div>
  </header>
  <section className="ranking-panel profile-panel" aria-label="Opções do perfil">
   <div className="profile-card">
    <div className="profile-header-info">
     <span className={`avatar profile-avatar-box ${isAnonymous ? 'avatar-anonymous' : ''}`}>
      {isAnonymous ? (
       <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className="icon-anonymous">
        <path d="M12 2a5 5 0 0 1 5 5v1a5 5 0 0 1-10 0V7a5 5 0 0 1 5-5zm0 13c4.42 0 8 2.24 8 5v2H4v-2c0-2.76 3.58-5 8-5z" opacity="0.75" />
       </svg>
      ) : user?.photo_url && !photoError ? (
       <img src={user.photo_url} alt={realName} onError={() => setPhotoError(true)} />
      ) : (
       <span>{initials}</span>
      )}
     </span>
     <div className="profile-status-text">
      <h2>{displayName}</h2>
      <p>{statusText}</p>
     </div>
    </div>

    <div className="privacy-toggle-row">
     <div className="toggle-label-group">
      <span className="toggle-title" id="anon-label">Aparecer como Anônimo</span>
      <span className="toggle-desc">Ao ativar, seu nome, foto e identificador não serão exibidos no Ranking Global.</span>
     </div>
     <button
      type="button"
      role="switch"
      aria-labelledby="anon-label"
      aria-checked={isAnonymous}
      disabled={isPending || mutation.isPending}
      onClick={handleToggle}
      className={`switch-button ${isAnonymous ? 'is-on' : 'is-off'}`}
     >
      <span className="switch-thumb" />
     </button>
    </div>

    <div className="profile-feedback-bar" aria-live="polite">
     {mutation.isPending && <span className="feedback-saving">Salvando alterações…</span>}
     {savedFeedback && !mutation.isPending && <span className="feedback-saved">✓ Salvo com sucesso</span>}
     {mutation.isError && <span className="feedback-error">Não foi possível salvar a alteração.</span>}
    </div>
   </div>

   <div className="profile-info-box">
    <h3>Como funciona a privacidade?</h3>
    <ul>
     <li><b>Pontuação e posições:</b> O modo anônimo altera apenas a sua exibição pública. Você continua pontuando e disputando colocações normalmente.</li>
     <li><b>Privacidade do grupo:</b> Para alterar a privacidade de um grupo, use o comando <code>/privacidade</code> diretamente no chat do grupo no Telegram.</li>
    </ul>
   </div>
  </section>
  <BottomNavigation system={system} tab="profile" />
 </main>
}
