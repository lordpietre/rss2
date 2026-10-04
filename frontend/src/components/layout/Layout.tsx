import { Outlet, Link, useNavigate, useLocation } from 'react-router-dom'
import { Search, Rss, BarChart3, Home as HomeIcon, Heart, User, Flame, Settings, Users, Tags, Database, Server, TrendingUp, Bell, X, Menu } from 'lucide-react'
import { useEffect, useState } from 'react'
import { apiService } from '../../services/api'
import { useSuggestedNews } from '../../hooks/useApiFavorites'
import { SuggestedNewsSidebar } from '../ui/SuggestedNewsSidebar'
import { CreateListModal } from '../CreateListModal'
import { News } from '../../services/api'
import { useApiLists } from '../../hooks/useApiFavorites'

export function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [username, setUsername] = useState('')
  const [isAdmin, setIsAdmin] = useState(false)
  const [showAdminMenu, setShowAdminMenu] = useState(false)
  const [nuevasAlertas, setNuevasAlertas] = useState(0)
  const [showListModal, setShowListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)

  const { suggestedNews } = useSuggestedNews()
  const { lists, createList, addToList } = useApiLists()

  const isHomePage = location.pathname === '/'

  const checkAuth = () => {
    const token = localStorage.getItem('token')
    const userData = localStorage.getItem('user')
    if (token && userData) {
      try {
        const user = JSON.parse(userData)
        setIsLoggedIn(true)
        setUsername(user.username || '')
        setIsAdmin(user.is_admin === true)
      } catch {
        setIsLoggedIn(false)
        setUsername('')
        setIsAdmin(false)
      }
    } else {
      setIsLoggedIn(false)
      setUsername('')
      setIsAdmin(false)
    }
  }

  useEffect(() => {
    checkAuth()
    // Listen for storage changes (when user logs in/out in another tab)
    window.addEventListener('storage', checkAuth)
    return () => window.removeEventListener('storage', checkAuth)
  }, [])

  useEffect(() => {
    const loadAlertas = async () => {
      try {
        const r = await apiService.getAlertas({ status: 'nueva', limit: 1 })
        setNuevasAlertas(r.nuevas)
      } catch {
        setNuevasAlertas(0)
      }
    }
    loadAlertas()
    const id = setInterval(loadAlertas, 60000)
    return () => clearInterval(id)
  }, [])

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    setIsLoggedIn(false)
    setUsername('')
    navigate('/login')
  }

  const handleAddToList = (news: News) => {
    setSelectedNewsForList(news)
    setShowListModal(true)
  }

  const handleSelectList = async (listId: number) => {
    if (selectedNewsForList) {
      await addToList(listId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
  }

  const handleCreateAndAdd = async (name: string, keywords?: string) => {
    const newListId = await createList(name, keywords)
    if (selectedNewsForList) {
      await addToList(newListId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white border-b border-gray-200 sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <Link to="/" className="flex items-center gap-2">
                <Rss className="h-8 w-8 text-primary-600" />
                <span className="text-xl font-bold text-gray-900">RSS2</span>
              </Link>
              <div className="hidden sm:flex ml-10 space-x-8">
                <Link to="/" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <HomeIcon className="h-4 w-4" />
                  Home
                </Link>
                <Link to="/feeds" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <Rss className="h-4 w-4" />
                  Feeds
                </Link>
                <Link to="/search" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <Search className="h-4 w-4" />
                  Search
                </Link>
                <Link to="/populares" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <Flame className="h-4 w-4" />
                  Popular
                </Link>
                <Link to="/analisis" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <TrendingUp className="h-4 w-4" />
                  Evolución
                </Link>
                <Link to="/alertas" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors relative">
                  <Bell className="h-4 w-4" />
                  Alertas
                  {nuevasAlertas > 0 && (
                    <span className="absolute -top-1.5 -right-3 bg-red-600 text-white text-[10px] font-bold min-w-[16px] h-4 px-1 rounded-full flex items-center justify-center">
                      {nuevasAlertas > 99 ? '99+' : nuevasAlertas}
                    </span>
                  )}
                </Link>
                <Link to="/stats" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <BarChart3 className="h-4 w-4" />
                  Stats
                </Link>
                {isAdmin && (
                  <div className="relative">
                    <button 
                      onClick={() => setShowAdminMenu(!showAdminMenu)}
                      className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors"
                    >
                      <Settings className="h-4 w-4" />
                      Admin
                    </button>
                    {showAdminMenu && (
                      <div className="absolute left-0 mt-2 w-48 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg py-1 z-50">
                        <Link 
                          to="/admin/workers" 
                          className="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                          onClick={() => setShowAdminMenu(false)}
                        >
                          <Server className="h-4 w-4" />
                          Workers
                        </Link>
                        <Link 
                          to="/populares" 
                          className="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                          onClick={() => setShowAdminMenu(false)}
                        >
                          <Tags className="h-4 w-4" />
                          Alias / Tipos (Popular)
                        </Link>
                        <Link 
                          to="/admin/users" 
                          className="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                          onClick={() => setShowAdminMenu(false)}
                        >
                          <Users className="h-4 w-4" />
                          Usuarios
                        </Link>
                        <Link 
                          to="/admin/settings" 
                          className="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
                          onClick={() => setShowAdminMenu(false)}
                        >
                          <Database className="h-4 w-4" />
                          Configuración
                        </Link>
                      </div>
                    )}
                  </div>
                )}
                <Link to="/favorites" className="flex items-center gap-2 text-gray-600 hover:text-primary-600 transition-colors">
                  <Heart className="h-4 w-4" />
                  Favorites
                </Link>
              </div>
            </div>
            <div className="flex items-center gap-4">
              {isLoggedIn ? (
                <>
                  <Link to="/account" className="flex items-center gap-2 text-gray-600 hover:text-primary-600">
                    <User className="h-4 w-4" />
                    {username}
                  </Link>
                  <button onClick={handleLogout} className="btn-secondary">
                    Logout
                  </button>
                </>
              ) : (
                <Link to="/login" className="btn-secondary">
                  Login
                </Link>
              )}
            </div>
          </div>
        </div>
      </nav>
      
      {/* Main content with sidebars */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex gap-8">
          {/* Left Sidebar - Hidden on mobile */}
          {isLoggedIn && isHomePage && suggestedNews.length > 0 && (
            <aside className="hidden lg:block w-64 flex-shrink-0">
              <div className="sticky top-24">
                <SuggestedNewsSidebar news={suggestedNews} onAddToList={handleAddToList} />
              </div>
            </aside>
          )}
          
          {/* Main Content */}
          <main className="flex-1 min-w-0">
            <Outlet />
          </main>
          
          {/* Right Sidebar - Hidden on mobile */}
          {isLoggedIn && isHomePage && suggestedNews.length > 0 && (
            <aside className="hidden lg:block w-64 flex-shrink-0">
              <div className="sticky top-24">
                <div className="bg-white border border-gray-200 rounded-lg shadow-sm p-3">
                  <div className="p-2 border-b border-gray-100 bg-gradient-to-r from-blue-50 to-primary-50 rounded-t-lg">
                    <div className="flex items-center gap-2">
                      <Rss className="h-4 w-4 text-blue-600" />
                      <span className="font-semibold text-sm text-gray-900">Novedades</span>
                    </div>
                    <p className="text-xs text-gray-500 mt-0.5">
                      Últimas noticias
                    </p>
                  </div>
                  <div className="p-2 space-y-1">
                    {suggestedNews.slice(0, 3).map(n => (
                      <Link key={n.id} to={`/news/${n.id}`} className="block py-1.5 px-2 rounded hover:bg-gray-50">
                        <p className="text-xs font-medium text-gray-700 hover:text-primary-600 line-clamp-2 leading-tight">
                          {n.title_translated || n.titulo}
                        </p>
                        <p className="text-[10px] text-gray-400 mt-0.5">{n.fuente_nombre}</p>
                      </Link>
                    ))}
                  </div>
                </div>
              </div>
            </aside>
          )}
        </div>
      </div>

      {/* Create List Modal */}
      <CreateListModal
        isOpen={showListModal}
        onClose={() => { setShowListModal(false); setSelectedNewsForList(null) }}
        onCreate={handleCreateAndAdd}
        mode="add-to"
        existingLists={lists.map((l: { id: number; name: string }) => ({ id: l.id, name: l.name }))}
        onSelectList={handleSelectList}
      />
    </div>
  )
}
