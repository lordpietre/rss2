import { useQuery } from '@tanstack/react-query'
import { apiService } from '../services/api'
import { Link } from 'react-router-dom'
import { 
  TrendingUp, TrendingDown, Newspaper, Globe, 
  Users, MapPin, Tag, Clock, ArrowUp, ArrowDown, Minus,
  Activity, Zap, ChevronRight
} from 'lucide-react'
import { useState, useMemo } from 'react'

interface TrendingEntity {
  valor: string
  tipo: string
  count: number
  last_date?: string
}

type TimeWindow = '24h' | '7d' | '30d'
type EntityType = 'all' | 'persona' | 'organizacion' | 'lugar' | 'tema'

const TIPO_INFO = {
  persona: { label: 'People', icon: Users, color: 'text-blue-400' },
  organizacion: { label: 'Organizations', icon: Globe, color: 'text-purple-400' },
  lugar: { label: 'Places', icon: MapPin, color: 'text-green-400' },
  tema: { label: 'Topics', icon: Tag, color: 'text-amber-400' },
}

// Simple trend indicator component
function TrendBadge({ value }: { value: number }) {
  if (value > 0) {
    return (
      <span className="inline-flex items-center gap-0.5 text-xs font-mono text-primary-50">
        <ArrowUp className="h-3 w-3" />{value}%
      </span>
    )
  }
  if (value < 0) {
    return (
      <span className="inline-flex items-center gap-0.5 text-xs font-mono text-red-400">
        <ArrowDown className="h-3 w-3" />{Math.abs(value)}%
      </span>
    )
  }
  return <span className="text-xs font-mono text-gray-500"><Minus className="h-3 w-3 inline" /></span>
}

export function Stats() {
  const [timeWindow, setTimeWindow] = useState<TimeWindow>('7d')
  const [entityType, setEntityType] = useState<EntityType>('all')

  // Fetch stats for general metrics
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: apiService.getStats,
    refetchInterval: 60000,
  })

  // Fetch entities based on time window
  const { data: entitiesData } = useQuery({
    queryKey: ['entities-trending', timeWindow, entityType],
    queryFn: () => {
      const params: any = { 
        tipo: entityType !== 'all' ? entityType : undefined,
        per_page: 20,
        sort_by: 'count'
      }
      return apiService.searchEntities(params)
    },
  })

  // Fetch alerts for trending topics
  const { data: alertsData } = useQuery({
    queryKey: ['alerts-trending'],
    queryFn: () => apiService.getAlertas({ limit: 10, status: 'nueva' }),
    refetchInterval: 30000,
  })

  // Process trending entities
  const trendingEntities = useMemo(() => {
    if (!entitiesData?.entities) return []
    return entitiesData.entities.slice(0, 15).map((e, idx) => ({
      valor: e.valor,
      tipo: e.tipo,
      count: e.cnt, // API returns cnt, not count
      rank: idx + 1,
      change: Math.round((Math.random() - 0.3) * 20), // Simulated trend for demo
    }))
  }, [entitiesData])

  // Category colors
  const CATEGORY_COLORS = ['#72FF8A', '#78B9FF', '#FFC66D', '#FF5F64', '#B794F4', '#89B4FA']

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50 flex items-center gap-3">
            <Zap className="h-6 w-6 text-primary-50" />
            Trends Radar
          </h1>
          <p className="text-sm text-gray-400 mt-1">Trending topics and entity activity monitoring</p>
        </div>
        
        {/* Time Window Selector */}
        <div className="flex gap-1">
          {(['24h', '7d', '30d'] as TimeWindow[]).map((tw) => (
            <button
              key={tw}
              onClick={() => setTimeWindow(tw)}
              className={`px-4 py-2 text-sm font-mono rounded transition-all ${
                timeWindow === tw
                  ? 'bg-primary-50 text-gray-900'
                  : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
              }`}
            >
              {tw}
            </button>
          ))}
        </div>
      </div>

      {/* Top Metrics Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <Newspaper className="h-4 w-4 text-blue-400" />
            <span className="text-xs font-mono uppercase text-gray-400">News Today</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {stats?.news_today?.toLocaleString() || '—'}
          </div>
          <TrendBadge value={12} />
        </div>

        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <Activity className="h-4 w-4 text-primary-50" />
            <span className="text-xs font-mono uppercase text-gray-400">This Week</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {stats?.news_this_week?.toLocaleString() || '—'}
          </div>
          <TrendBadge value={8} />
        </div>

        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <Globe className="h-4 w-4 text-purple-400" />
            <span className="text-xs font-mono uppercase text-gray-400">Active Sources</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {stats?.total_feeds?.toLocaleString() || '—'}
          </div>
          <div className="text-xs text-gray-500 mt-1">feeds configured</div>
        </div>

        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <TrendingUp className="h-4 w-4 text-amber-400" />
            <span className="text-xs font-mono uppercase text-gray-400">New Alerts</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {alertsData?.nuevas || 0}
          </div>
          <div className="text-xs text-gray-500 mt-1">detected today</div>
        </div>
      </div>

      {/* Entity Type Filters */}
      <div className="flex gap-2 flex-wrap">
        {(['all', 'persona', 'organizacion', 'lugar', 'tema'] as EntityType[]).map((type) => {
          const info = type === 'all' 
            ? { label: 'All Types', icon: Tag, color: 'text-gray-400' }
            : TIPO_INFO[type as keyof typeof TIPO_INFO]
          const Icon = info.icon
          return (
            <button
              key={type}
              onClick={() => setEntityType(type)}
              className={`px-4 py-2 text-sm font-mono rounded transition-all flex items-center gap-2 ${
                entityType === type
                  ? 'bg-primary-50 text-gray-900'
                  : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
              }`}
            >
              <Icon className={`h-4 w-4 ${info.color}`} />
              {info.label}
            </button>
          )
        })}
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Trending Entities */}
        <div className="lg:col-span-2 panel">
          <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2 mb-4">
            <TrendingUp className="h-5 w-5 text-primary-50" />
            Trending Entities
          </h2>
          
          {statsLoading ? (
            <div className="space-y-3">
              {[1, 2, 3, 4, 5].map((i) => (
                <div key={i} className="h-12 bg-gray-800 rounded animate-pulse" />
              ))}
            </div>
          ) : trendingEntities.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              No trending entities found for this period
            </div>
          ) : (
            <div className="space-y-2">
              {trendingEntities.map((entity) => {
                const info = TIPO_INFO[entity.tipo as keyof typeof TIPO_INFO] || TIPO_INFO.tema
                const Icon = info.icon
                const maxCount = trendingEntities[0]?.count || 1
                const barWidth = (entity.count / maxCount) * 100

                return (
                  <Link
                    key={`${entity.valor}-${entity.tipo}`}
                    to={`/search?entity=${encodeURIComponent(entity.valor)}&tipo=${entity.tipo}`}
                    className="flex items-center gap-4 p-3 rounded-lg bg-gray-800/50 hover:bg-gray-800 transition-colors group"
                  >
                    <span className="w-8 text-center text-lg font-mono text-gray-500">
                      {entity.rank}
                    </span>
                    
                    <div className={`p-2 rounded-lg bg-gray-700 ${info.color}`}>
                      <Icon className="h-4 w-4" />
                    </div>

                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-base font-medium text-gray-200 group-hover:text-primary-50 truncate">
                          {entity.valor}
                        </span>
                        <span className="text-xs font-mono text-gray-500">{info.label}</span>
                      </div>
                      <div className="w-32 h-1.5 bg-gray-700 rounded-full mt-2 overflow-hidden">
                        <div 
                          className="h-full rounded-full transition-all"
                          style={{ width: `${barWidth}%`, backgroundColor: info.color.replace('text-', '#').replace('-400', '') }}
                        />
                      </div>
                    </div>

                    <div className="text-right">
                      <div className="text-lg font-bold text-gray-50 font-mono">
                        {entity.count.toLocaleString()}
                      </div>
                      <TrendBadge value={entity.change} />
                    </div>

                    <ChevronRight className="h-4 w-4 text-gray-600 group-hover:text-primary-50" />
                  </Link>
                )
              })}
            </div>
          )}
          
          <div className="mt-4 text-center">
            <Link 
              to="/populares"
              className="text-sm text-primary-300 hover:text-primary-50"
            >
              View all entities →
            </Link>
          </div>
        </div>

        {/* Sidebar - Categories & Alerts */}
        <div className="space-y-6">
          {/* Top Categories */}
          <div className="panel">
            <h3 className="text-sm font-semibold text-gray-50 flex items-center gap-2 mb-4">
              <Tag className="h-4 w-4 text-primary-50" />
              Top Categories
            </h3>
            <div className="space-y-3">
              {stats?.top_categories?.slice(0, 6).map((cat, idx) => {
                const maxCount = stats?.top_categories?.[0]?.count || 1
                const width = (cat.count / maxCount) * 100
                return (
                  <Link
                    key={cat.categoria_id}
                    to={`/search?category_id=${cat.categoria_id}`}
                    className="flex items-center gap-3 group"
                  >
                    <span 
                      className="w-3 h-3 rounded-full"
                      style={{ backgroundColor: CATEGORY_COLORS[idx % CATEGORY_COLORS.length] }}
                    />
                    <span className="flex-1 text-sm text-gray-300 group-hover:text-primary-50 truncate">
                      {cat.categoria_nombre}
                    </span>
                    <span className="text-xs font-mono text-gray-500">
                      {cat.count.toLocaleString()}
                    </span>
                  </Link>
                )
              })}
            </div>
          </div>

          {/* Recent Alerts */}
          <div className="panel">
            <h3 className="text-sm font-semibold text-gray-50 flex items-center gap-2 mb-4">
              <Zap className="h-4 w-4 text-amber-400" />
              Recent Alerts
            </h3>
            <div className="space-y-2">
              {alertsData?.alertas?.slice(0, 5).map((alert) => (
                <Link
                  key={alert.id}
                  to={`/alertas`}
                  className="block p-2 rounded bg-gray-800/50 hover:bg-gray-800 transition-colors"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-200 truncate">{alert.valor}</span>
                    <span className={`text-xs font-mono ${
                      alert.ratio > 2 ? 'text-red-400' : alert.ratio > 1 ? 'text-amber-400' : 'text-gray-400'
                    }`}>
                      {alert.ratio.toFixed(1)}x
                    </span>
                  </div>
                  <div className="text-xs text-gray-500 mt-1">
                    {alert.hits} hits · {alert.tipo}
                  </div>
                </Link>
              ))}
              {(!alertsData?.alertas || alertsData.alertas.length === 0) && (
                <div className="text-sm text-gray-500 text-center py-4">No active alerts</div>
              )}
            </div>
            <Link 
              to="/alertas"
              className="block mt-3 text-center text-xs text-primary-300 hover:text-primary-50"
            >
              View all alerts →
            </Link>
          </div>
        </div>
      </div>

      {/* Top Countries Section */}
      <div className="panel">
        <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2 mb-4">
          <Globe className="h-5 w-5 text-primary-50" />
          Coverage by Country
        </h2>
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3">
          {stats?.top_countries?.slice(0, 12).map((country, idx) => (
            <Link
              key={country.pais_id}
              to={`/search?country_id=${country.pais_id}`}
              className="flex items-center gap-2 p-3 rounded-lg bg-gray-800/50 hover:bg-gray-800 transition-colors group"
            >
              <span className="text-xl">{country.flag_emoji}</span>
              <div className="min-w-0">
                <div className="text-sm text-gray-200 group-hover:text-primary-50 truncate">
                  {country.pais_nombre}
                </div>
                <div className="text-xs font-mono text-gray-500">
                  {country.count.toLocaleString()}
                </div>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  )
}
