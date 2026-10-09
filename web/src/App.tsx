import { Route, Routes, useNavigate } from 'react-router'
import { useEffect, useRef } from 'react'
import { RankingsPage } from './pages/Rankings'
import { HomePage } from './pages/Home'
import { InvitePage } from './pages/Invite'
import { GamePage } from './pages/Game'
import { ProfilePage } from './pages/Profile'
import { launchGamePath, useTelegram } from './lib/telegram'

export default function App() {
 useTelegram(undefined, { manageBackButton: false, initialize: true })
 const navigate = useNavigate()
 const launched = useRef(false)
 useEffect(() => {
  if (launched.current) return
  launched.current = true
  const path = launchGamePath()
  if (path && window.Telegram?.WebApp.initData) navigate(path, { replace: true })
 }, [navigate])
 if (!window.Telegram?.WebApp.initData)
  return (
   <main className="app-shell">
    <header className="ranking-header">
     <h1>Ranking Global</h1>
    </header>
    <section className="ranking-panel">
     <div className="state-card">Abra o Ranking Global pelo Telegram.</div>
    </section>
   </main>
  )
 return (
  <Routes>
   <Route path="/" element={<RankingsPage />} />
   <Route path="/groups/:groupRef" element={<RankingsPage />} />
   <Route path="/home" element={<HomePage />} />
   <Route path="/ranking" element={<RankingsPage />} />
   <Route path="/invite/:token" element={<InvitePage />} />
   <Route path="/game/:gameID" element={<GamePage />} />
   <Route path="/profile" element={<ProfilePage />} />
   <Route
    path="*"
    element={<main className="state-card">Página não encontrada.</main>}
   />
  </Routes>
 )
}
