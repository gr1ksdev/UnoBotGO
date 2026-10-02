import { Route, Routes } from 'react-router'
import { RankingsPage } from './pages/Rankings'
import { ProfilePage } from './pages/Profile'
import { useTelegram } from './lib/telegram'

export default function App() {
 useTelegram(undefined, { manageBackButton: false, initialize: true })
 if (!window.Telegram?.WebApp.initData) return <main className="app-shell"><header className="ranking-header"><h1>Ranking Global</h1></header><section className="ranking-panel"><div className="state-card">Abra o Ranking Global pelo Telegram.</div></section></main>
 return <Routes>
  <Route path="/" element={<RankingsPage />} />
  <Route path="/groups/:groupRef" element={<RankingsPage />} />
  <Route path="/profile" element={<ProfilePage />} />
  <Route path="*" element={<main className="state-card">Página não encontrada.</main>} />
 </Routes>
}
