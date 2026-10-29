import { Outlet, Link, useNavigate, useLocation } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { 
  Search, Rss, BarChart3, Home as HomeIcon, Heart, User, Flame, Settings, 
  Users, Tags, Database, Server, TrendingUp, Bell, Menu, X, Globe, Zap,
  Activity, Map, Network, AlertTriangle, Radio, Monitor, Mic
} from 'lucide-react'
import { apiService } from '../../services/api'

interface NavItem {
  name: string
  path: string
  icon: React.ComponentType<{ className?: string }>
  badge?: number
}

export function IntelligenceLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [username, setUsername] = useState('')
  const [isAdmin, setIsAdmin] = useState(false)
  const [nuevasAlertas, setNuevasAlertas] = useState(0)
  const [showSidebar, setShowSidebar] = useState(true)
  const [currentTime, setCurrentTime] = useState(new Date())
  const [apiStatus, setApiStatus] = useState<'online' | 'offline' | 'checking'>('checking')

  // Update clock
  useEffect(() => {
    const timer = setInterval(() => setCurrentTime(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  // Check API health - use /api/stats as health check
  useEffect(() => {
    const checkApi = async () => {
      try {
        const response = await fetch('/api/stats')
        setApiStatus(response.ok ? 'online' : 'offline')
      } catch {
        setApiStatus('offline')
      }
    }
    checkApi()
    const interval = setInterval(checkApi, 30000)
    return () => clearInterval(interval)
  }, [])

  // Auth check
  useEffect(() => {
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
      }
    } else {
      setIsLoggedIn(false)
    }
  }, [location.pathname])

  // Load alerts count
  useEffect(() => {
    const loadAlertas = async () => {
      try {
        const r = await apiService.getAlertas({ status: 'nueva', limit: 1 })
        setNuevasAlertas(r.nuevas)
      } catch {
        setNuevasAlertas(0)
      }
    }
    if (isLoggedIn) {
      loadAlertas()
      const id = setInterval(loadAlertas, 60000)
      return () => clearInterval(id)
    }
  }, [isLoggedIn])

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    setIsLoggedIn(false)
    setUsername('')
    navigate('/login')
  }

  const navItems: NavItem[] = [
    { name: 'Overview', path: '/overview', icon: Monitor },
    { name: 'Global Monitor', path: '/map', icon: Map },
    { name: 'Intelligence', path: '/search', icon: Globe },
    { name: 'Trends', path: '/stats', icon: TrendingUp },
    { name: 'Entities', path: '/populares', icon: Network },
    { name: 'Evolution', path: '/analisis', icon: Activity },
    { name: 'Alerts', path: '/alertas', icon: AlertTriangle, badge: nuevasAlertas },
    { name: 'Noticiario', path: '/noticiario', icon: Mic },
    { name: 'Sources', path: '/feeds', icon: Radio },
  ]

  const adminItems = isAdmin ? [
    { name: 'System', path: '/admin/workers', icon: Server },
    { name: 'Users', path: '/admin/users', icon: Users },
    { name: 'Config', path: '/admin/settings', icon: Settings },
  ] : []

  const isActive = (path: string) => {
    if (path === '/overview') return location.pathname === '/overview' || location.pathname === '/'
    return location.pathname.startsWith(path)
  }

  // Detailed descriptions for navigation tooltips
  const navDescriptions: Record<string, { title: string; description: string }> = {
    '/overview': {
      title: 'Intelligence Overview',
      description: 'Dashboard principal con métricas globales del sistema: noticias totales, feeds activos, traducciones completadas, alertas activas y actividad de las últimas 24 horas.'
    },
    '/map': {
      title: 'Global Monitor',
      description: 'Mapa mundial interactivo que muestra la distribución geográfica de las fuentes de noticias. Permite filtrar por región y explorar la cobertura informativa por país.'
    },
    '/search': {
      title: 'News Intelligence',
      description: 'Explorador avanzado de noticias con filtros por país, idioma, categoría y entidad. Incluye búsqueda semántica y traducción de contenidos.'
    },
    '/stats': {
      title: 'Trends & Statistics',
      description: 'Análisis de tendencias informative con métricas de traducción, idiomas originales detectados y actividad por hora. Visualiza el rendimiento del sistema.'
    },
    '/populares': {
      title: 'Entity Network',
      description: 'Red de entidades extraídas mediante NER: personas, lugares, organizaciones y temas. Explora las relaciones y co-ocurrencias entre entidades.'
    },
    '/analisis': {
      title: 'News Evolution',
      description: 'Análisis temporal del ciclo de vida de las noticias. Seguimiento de temas desde su primera detección hasta su maduración en el sistema.'
    },
    '/alertas': {
      title: 'Alert Center',
      description: 'Consola de alertas de actividad informativa anómala. Detecta incrementos de cobertura sobre entidades, temas o lugares específicos.'
    },
    '/noticiario': {
      title: 'Noticiario por voz',
      description: 'Busca noticias, elige las que forman el informativo y genera la locución en español. Cada audio pasa por aprobación editorial antes de poder usarse.'
    },
    '/feeds': {
      title: 'Sources Management',
      description: 'Supervisión y gestión de fuentes RSS activas.监控 fuentes por país, idioma, categoría y estado de salud.'
    },
    '/admin/workers': {
      title: 'System Status',
      description: 'Panel de observabilidad del sistema: estado de workers, colas de procesamiento, ingesta de noticias y métricas de rendimiento.'
    },
    '/admin/users': {
      title: 'User Management',
      description: 'Administración de usuarios del sistema: creación, permisos, autenticación y gestión de acceso.'
    },
    '/admin/settings': {
      title: 'Configuration',
      description: 'Configuración global del sistema RSS2: parámetros de procesamiento, límites y opciones de funcionamiento.'
    },
  }

  return (
    <div className="min-h-screen bg-rss-bg-main flex dark">
      {/* Sidebar */}
      <aside 
        className={`
          fixed lg:static inset-y-0 left-0 z-50
          w-64 bg-rss-bg-panel border-r border-rss-border
          transform transition-transform duration-300 ease-in-out
          ${showSidebar ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
          flex flex-col
        `}
      >
        {/* Logo & Branding */}
        <div className="h-16 flex items-center justify-between px-4 border-b border-rss-border">
          <Link to="/" className="flex items-center gap-3">
            <div className="w-8 h-8 rounded bg-rss-green-dim flex items-center justify-center">
              <Zap className="w-5 h-5 text-rss-green-phosphor" />
            </div>
            <div className="flex flex-col">
              <span className="font-mono font-bold text-rss-green-phosphor text-sm tracking-wider">RSS2</span>
              <span className="font-mono text-[10px] text-rss-text-secondary tracking-widest">INTELLIGENCE</span>
            </div>
          </Link>
          <button 
            onClick={() => setShowSidebar(false)}
            className="lg:hidden p-1 text-rss-text-secondary hover:text-rss-text-primary"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto py-4 px-3">
          <div className="space-y-1">
            {navItems.map((item) => {
              const Icon = item.icon
              const active = isActive(item.path)
              const info = navDescriptions[item.path]
              return (
                <div key={item.path} className="relative group">
                  <Link
                    to={item.path}
                    className={`
                      flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium
                      transition-all duration-150
                      ${active 
                        ? 'bg-rss-green-dim/20 text-rss-green-phosphor border-l-2 border-rss-green-phosphor -ml-[2px]' 
                        : 'text-rss-text-secondary hover:bg-rss-bg-secondary hover:text-rss-text-primary'
                      }
                    `}
                  >
                    <Icon className={`w-4 h-4 ${active ? 'text-rss-green-phosphor' : ''}`} />
                    <span>{item.name}</span>
                    {item.badge && item.badge > 0 && (
                      <span className="ml-auto px-2 py-0.5 text-xs font-bold bg-rss-red text-white rounded-full">
                        {item.badge > 99 ? '99+' : item.badge}
                      </span>
                    )}
                  </Link>
                  {/* Tooltip */}
                  {info && (
                    <div className="absolute left-full top-0 ml-2 w-72 p-3 rounded-lg bg-rss-bg-panel border border-rss-border 
                                    opacity-0 invisible group-hover:opacity-100 group-hover:visible 
                                    transition-all duration-200 z-50 shadow-xl
                                    pointer-events-none">
                      <div className="text-xs font-mono font-bold text-rss-green-phosphor mb-1">
                        {info.title}
                      </div>
                      <div className="text-xs text-gray-300 leading-relaxed">
                        {info.description}
                      </div>
                      {/* Flecha del tooltip */}
                      <div className="absolute -left-2 top-4 w-0 h-0 
                                    border-t-8 border-t-transparent 
                                    border-b-8 border-b-transparent 
                                    border-r-8 border-rss-border"></div>
                      <div className="absolute -left-1 top-4 w-0 h-0 
                                    border-t-6 border-t-transparent 
                                    border-b-6 border-b-transparent 
                                    border-r-6 border-r-rss-bg-panel"></div>
                    </div>
                  )}
                </div>
              )
            })}
          </div>

          {adminItems.length > 0 && (
            <div className="mt-6 pt-4 border-t border-rss-border">
              <p className="px-3 mb-2 text-[10px] font-mono uppercase tracking-wider text-rss-text-secondary">
                Administration
              </p>
              <div className="space-y-1">
                {adminItems.map((item) => {
                  const Icon = item.icon
                  const active = isActive(item.path)
                  return (
                    <Link
                      key={item.path}
                      to={item.path}
                      className={`
                        flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium
                        transition-all duration-150
                        ${active 
                          ? 'bg-rss-green-dim/20 text-rss-green-phosphor' 
                          : 'text-rss-text-secondary hover:bg-rss-bg-secondary hover:text-rss-text-primary'
                        }
                      `}
                    >
                      <Icon className="w-4 h-4" />
                      <span>{item.name}</span>
                    </Link>
                  )
                })}
              </div>
            </div>
          )}
        </nav>

        {/* Sidebar Footer */}
        <div className="p-4 border-t border-rss-border">
          {isLoggedIn ? (
            <div className="space-y-3">
              <div className="flex items-center gap-3 px-2">
                <div className="w-8 h-8 rounded-full bg-rss-green-dim flex items-center justify-center">
                  <User className="w-4 h-4 text-rss-green-phosphor" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-rss-text-primary truncate">{username}</p>
                  <p className="text-[10px] text-rss-text-secondary">Logged in</p>
                </div>
              </div>
              <div className="flex gap-2">
                <Link to="/account" className="flex-1 btn-ghost text-center py-1.5 text-xs">
                  Account
                </Link>
                <button 
                  onClick={handleLogout}
                  className="flex-1 btn-ghost text-center py-1.5 text-xs text-rss-red hover:bg-rss-red/10"
                >
                  Logout
                </button>
              </div>
            </div>
          ) : (
            <Link to="/login" className="btn-primary w-full text-center py-2">
              Login
            </Link>
          )}
        </div>
      </aside>

      {/* Mobile sidebar overlay */}
      {showSidebar && (
        <div 
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={() => setShowSidebar(false)}
        />
      )}

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Bar */}
        <header className="h-16 bg-rss-bg-panel border-b border-rss-border flex items-center justify-between px-4 lg:px-6">
          <div className="flex items-center gap-4">
            <button 
              onClick={() => setShowSidebar(true)}
              className="lg:hidden p-2 text-rss-text-secondary hover:text-rss-text-primary"
            >
              <Menu className="w-5 h-5" />
            </button>
            
            {/* Breadcrumb / Page Title area */}
            <div className="hidden sm:block">
              <p className="text-xs font-mono text-rss-text-secondary uppercase tracking-wider">
                {navItems.find(n => isActive(n.path))?.name || 'Dashboard'}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-4 lg:gap-6">
            {/* API Status */}
            <div className="flex items-center gap-2">
              <div className={`
                w-2 h-2 rounded-full
                ${apiStatus === 'online' ? 'bg-rss-green-phosphor animate-pulse' : 
                  apiStatus === 'checking' ? 'bg-rss-amber' : 'bg-rss-red'}
              `} />
              <span className="hidden md:inline text-xs font-mono text-rss-text-secondary">
                API {apiStatus === 'online' ? 'ONLINE' : apiStatus === 'checking' ? 'CHECKING' : 'OFFLINE'}
              </span>
            </div>

            {/* Clock */}
            <div className="hidden md:flex flex-col items-end">
              <span className="text-sm font-mono text-rss-text-primary">
                {currentTime.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
              </span>
              <span className="text-[10px] font-mono text-rss-text-secondary">
                {currentTime.toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })}
              </span>
            </div>

            {/* Quick Search */}
            <Link 
              to="/search"
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-rss-bg-secondary border border-rss-border text-rss-text-secondary hover:text-rss-text-primary hover:border-rss-green-dim transition-colors"
            >
              <Search className="w-4 h-4" />
              <span className="hidden lg:inline text-sm">Search</span>
              <kbd className="hidden lg:inline text-[10px] px-1.5 py-0.5 rounded bg-rss-bg-main border border-rss-border font-mono">
                /
              </kbd>
            </Link>
          </div>
        </header>

        {/* Page Content */}
        <main className="flex-1 overflow-auto p-4 lg:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
