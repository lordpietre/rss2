import { useState, useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { Layout } from './components/layout/Layout'
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
import { AdminAliases } from './pages/AdminAliases'
import { AdminUsers } from './pages/AdminUsers'
import { AdminSettings } from './pages/AdminSettings'
import { AdminWorkers } from './pages/AdminWorkers'
import { WelcomeWizard } from './pages/WelcomeWizard'
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
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
      </div>
    )
  }

  if (showWelcome) {
    return <WelcomeWizard onComplete={handleWelcomeComplete} />
  }

  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Home />} />
        <Route path="news/:id" element={<News />} />
        <Route path="feeds" element={<Feeds />} />
        <Route path="search" element={<Search />} />
        <Route path="populares" element={<Populares />} />
        <Route path="stats" element={<Stats />} />
        <Route path="favorites" element={<Favorites />} />
        <Route path="account" element={<Account />} />
        <Route path="login" element={<Login />} />
        <Route 
          path="admin/aliases" 
          element={<ProtectedRoute requireAdmin>{<AdminAliases />}</ProtectedRoute>} 
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
          path="admin/workers" 
          element={<ProtectedRoute requireAdmin>{<AdminWorkers />}</ProtectedRoute>} 
        />
      </Route>
    </Routes>
  )
}

export default App