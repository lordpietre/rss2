import { useState, useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { IntelligenceLayout } from './components/layout/IntelligenceLayout'
import { ProtectedRoute } from './components/ProtectedRoute'
import { Home } from './pages/Home'
import { News } from './pages/News'
import { Feeds } from './pages/Feeds'
import { Search } from './pages/Search'
import { Login } from './pages/Login'
import { Stats } from './pages/Stats'
import { Favorites } from './pages/Favorites'
import { Account } from './pages/Account'
import { Populares } from './pages/Populares'
import { Analisis } from './pages/Analisis'
import { Alertas } from './pages/Alertas'
import { Noticiario } from './pages/Noticiario'
import { AdminAliases } from './pages/AdminAliases'
import { AdminUsers } from './pages/AdminUsers'
import { AdminSettings } from './pages/AdminSettings'
import { AdminWorkers } from './pages/AdminWorkers'
import { WelcomeWizard } from './pages/WelcomeWizard'
import { Overview } from './pages/Overview'
import { GlobalMonitor } from './pages/GlobalMonitor'
import { ErrorBoundary } from './components/ErrorBoundary'
import { api } from './services/api'

function App() {
  const [showWelcome, setShowWelcome] = useState<boolean | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    checkFirstUser()
  }, [])

  const checkFirstUser = async () => {
    try {
      const res = await api.get('/auth/check-first-user')
      setShowWelcome(res.data.is_first_user)
    } catch {
      setShowWelcome(false)
    } finally {
      setLoading(false)
    }
  }

  const handleWelcomeComplete = () => {
    setShowWelcome(false)
  }

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-rss-bg-main">
        <div className="flex flex-col items-center gap-4">
          <div className="w-12 h-12 rounded-lg bg-rss-green-dim flex items-center justify-center">
            <span className="text-2xl font-mono font-bold text-rss-green-phosphor">R2</span>
          </div>
          <div className="animate-pulse text-rss-text-secondary text-sm font-mono">Loading...</div>
        </div>
      </div>
    )
  }

  if (showWelcome) {
    return <WelcomeWizard onComplete={handleWelcomeComplete} />
  }

  return (
    <ErrorBoundary>
    <Routes>
      {/* Intelligence Center Layout - New dark theme */}
      <Route element={<IntelligenceLayout />}>
        <Route path="/overview" element={<Overview />} />
        <Route path="/" element={<Overview />} />
        <Route path="/map" element={<GlobalMonitor />} />
        <Route path="/search" element={<Search />} />
        <Route path="/stats" element={<Stats />} />
        <Route path="/populares" element={<Populares />} />
        <Route path="/analisis" element={<Analisis />} />
        <Route path="/alertas" element={<Alertas />} />
        <Route path="/noticiario" element={<Noticiario />} />
        <Route path="/feeds" element={<Feeds />} />
        <Route path="/news/:id" element={<News />} />
        <Route path="/favorites" element={<Favorites />} />
        <Route path="/account" element={<Account />} />
        <Route path="/login" element={<Login />} />
        <Route 
          path="admin/workers" 
          element={<ProtectedRoute requireAdmin>{<AdminWorkers />}</ProtectedRoute>} 
        />
        <Route 
          path="admin/users" 
          element={<ProtectedRoute requireAdmin>{<AdminUsers />}</ProtectedRoute>} 
        />
        <Route 
          path="admin/settings" 
          element={<ProtectedRoute requireAdmin>{<AdminSettings />}</ProtectedRoute>} 
        />
        <Route 
          path="admin/aliases" 
          element={<ProtectedRoute requireAdmin>{<AdminAliases />}</ProtectedRoute>} 
        />
      </Route>
    </Routes>
    </ErrorBoundary>
  )
}

export default App