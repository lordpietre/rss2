import { useQuery } from '@tanstack/react-query'
import { apiService, Stats, News } from '../services/api'
import { Link } from 'react-router-dom'
import { 
  Rss, Globe, Users, Network, TrendingUp, AlertTriangle, Clock, 
  Activity, Newspaper, ArrowUp, ArrowDown, Minus, Map, Zap
} from 'lucide-react'
import { LineChart } from '../components/LineChart'

interface MetricCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon: React.ComponentType<{ className?: string }>
  trend?: 'up' | 'down' | 'stable'
  trendValue?: string
  color?: 'green' | 'amber' | 'red' | 'blue'
}

function MetricCard({ title, value, subtitle, icon: Icon, trend, trendValue, color = 'green' }: MetricCardProps) {
  const colorClasses = {
    green: 'text-primary-50',
    amber: 'text-amber-400',
    red: 'text-red-400',
    blue: 'text-blue-400'
  }

  return (
    <div className="panel">
      <div className="flex items-start justify-between mb-3">
        <div className="p-2 rounded-lg bg-gray-800">
          <Icon className={`w-5 h-5 ${colorClasses[color]}`} />
        </div>
        {trend && (
          <div className={`flex items-center gap-1 text-xs font-mono ${
            trend === 'up' ? 'text-primary-50' : trend === 'down' ? 'text-red-400' : 'text-gray-400'
          }`}>
            {trend === 'up' && <ArrowUp className="w-3 h-3" />}
            {trend === 'down' && <ArrowDown className="w-3 h-3" />}
            {trend === 'stable' && <Minus className="w-3 h-3" />}
            {trendValue && <span>{trendValue}</span>}
          </div>
        )}
      </div>
      <div className="metric text-3xl">{value}</div>
      <div className="metric-label mt-1">{title}</div>
      {subtitle && (
        <p className="text-xs text-gray-400 mt-1">{subtitle}</p>
      )}
    </div>
  )
}

interface Alert {
  id: number
  valor: string
  tipo: string
  hits: number
  ratio: number
  created_at: string
}

export function Overview() {
  const { data: stats, isLoading: statsLoading } = useQuery<Stats>({
    queryKey: ['stats'],
    queryFn: () => apiService.getStats(),
    refetchInterval: 60000,
  })

  const { data: allCountries } = useQuery({
    queryKey: ['countries-all'],
    queryFn: () => apiService.getCountries(),
  })

  const { data: alerts } = useQuery<{ alertas: Alert[]; total: number; nuevas: number }>({
    queryKey: ['alerts-overview'],
    queryFn: () => apiService.getAlertas({ limit: 5 }),
    refetchInterval: 30000,
  })

  const { data: newsData } = useQuery({
    queryKey: ['news-overview'],
    queryFn: () => apiService.getNews({ per_page: 10 }),
  })

  const totalCountries = allCountries?.length || 0
  const activeCountriesWithNews = stats?.top_countries?.length || 0

  const recentNews: News[] = newsData?.news || []

  // Process hourly data from translation_stats_24h
  const hourlyData = stats?.translation_stats_24h?.map((val) => ({
    fecha: val.hour,
    count: val.total
  })) || []

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">Intelligence Overview</h1>
          <p className="text-sm text-gray-300 mt-1">
            Real-time monitoring of global news activity
          </p>
        </div>
        <div className="flex items-center gap-2 text-xs font-mono text-gray-400">
          <Clock className="w-4 h-4" />
          <span>Last updated: {new Date().toLocaleTimeString('es-ES')}</span>
        </div>
      </div>

      {/* Metrics Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Total News"
          value={stats?.total_news?.toLocaleString() || '—'}
          subtitle="All time"
          icon={Newspaper}
          color="green"
        />
        <MetricCard
          title="News Today"
          value={stats?.news_today?.toLocaleString() || '—'}
          subtitle="Last 24h"
          icon={Activity}
          color="green"
        />
        <MetricCard
          title="Active Sources"
          value={stats?.total_feeds?.toLocaleString() || '—'}
          subtitle="Configured feeds"
          icon={Rss}
          color="blue"
        />
        <MetricCard
          title="Countries"
          value={totalCountries || '—'}
          subtitle={`${activeCountriesWithNews} with news`}
          icon={Globe}
          color="green"
        />
      </div>

      {/* Secondary Metrics */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Translations"
          value={stats?.translations_done?.toLocaleString() || '—'}
          subtitle="Completed"
          icon={Network}
          color="green"
        />
        <MetricCard
          title="Pending"
          value={stats?.translations_pending?.toLocaleString() || '—'}
          subtitle="In queue"
          icon={Clock}
          color="amber"
        />
        <MetricCard
          title="Active Alerts"
          value={alerts?.total || 0}
          subtitle={`${alerts?.nuevas || 0} new`}
          icon={AlertTriangle}
          color="red"
        />
        <MetricCard
          title="Languages"
          value={stats?.top_languages?.length || '—'}
          subtitle="Detected"
          icon={Users}
          color="blue"
        />
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Activity Chart */}
        <div className="lg:col-span-2 panel">
          <div className="panel-header">
            <h2 className="panel-title flex items-center gap-2">
              <TrendingUp className="w-4 h-4" />
              News Activity (24h)
            </h2>
            <Link to="/stats" className="text-xs text-primary-300 hover:text-primary-50">
              View details →
            </Link>
          </div>
          <div className="h-64">
            <LineChart 
              series={[{
                valor: 'News',
                data: hourlyData.map(d => ({ fecha: d.fecha, count: d.count })),
                count: hourlyData.reduce((sum, d) => sum + d.count, 0)
              }]}
            />
          </div>
        </div>

        {/* Top Countries */}
        <div className="panel">
          <div className="panel-header">
            <h2 className="panel-title flex items-center gap-2">
              <Globe className="w-4 h-4" />
              Top Countries
            </h2>
          </div>
          <div className="space-y-3">
            {stats?.top_countries?.slice(0, 6).map((country, idx) => (
              <div key={country.pais_id} className="flex items-center gap-3">
                <span className="w-6 text-xs font-mono text-gray-400">{idx + 1}</span>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-50 truncate">{country.pais_nombre}</p>
                  <div className="mt-1 h-1.5 bg-gray-800 rounded-full overflow-hidden">
                    <div 
                      className="h-full bg-primary-50 rounded-full"
                      style={{ width: `${Math.min((country.count / (stats?.top_countries?.[0]?.count || 1)) * 100, 100)}%` }}
                    />
                  </div>
                </div>
                <span className="text-xs font-mono text-gray-400">{country.count}</span>
              </div>
            ))}
            {!stats?.top_countries?.length && (
              <p className="text-sm text-gray-400 text-center py-4">No data available</p>
            )}
          </div>
        </div>
      </div>

      {/* Bottom Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Recent Alerts */}
        <div className="panel">
          <div className="panel-header">
            <h2 className="panel-title flex items-center gap-2">
              <AlertTriangle className="w-4 h-4" />
              Recent Alerts
            </h2>
            <Link to="/alertas" className="text-xs text-primary-300 hover:text-primary-50">
              View all →
            </Link>
          </div>
          <div className="space-y-3">
            {alerts?.alertas?.slice(0, 5).map((alert) => (
              <Link 
                key={alert.id}
                to={`/analisis?entity=${encodeURIComponent(alert.valor)}`}
                className="flex items-start gap-3 p-3 rounded-lg bg-gray-800 hover:bg-primary-900/20 transition-colors"
              >
                <div className={`
                  w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0
                  ${alert.tipo === 'persona' ? 'bg-primary-900/30' : 
                    alert.tipo === 'lugar' ? 'bg-blue-900/30' : 
                    alert.tipo === 'organizacion' ? 'bg-amber-900/30' : 
                    'bg-gray-900'}
                `}>
                  {alert.tipo === 'persona' && <Users className="w-4 h-4 text-primary-50" />}
                  {alert.tipo === 'lugar' && <Globe className="w-4 h-4 text-blue-400" />}
                  {alert.tipo === 'organizacion' && <Network className="w-4 h-4 text-amber-400" />}
                  {alert.tipo === 'tema' && <TrendingUp className="w-4 h-4 text-gray-400" />}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-50 truncate">{alert.valor}</p>
                  <div className="flex items-center gap-3 mt-1">
                    <span className="text-xs font-mono text-gray-400">{alert.hits} hits</span>
                    <span className="text-xs font-mono text-amber-400">{(alert.ratio || 0).toFixed(1)}x</span>
                  </div>
                </div>
                <span className="text-[10px] font-mono text-gray-400 whitespace-nowrap">
                  {new Date(alert.created_at).toLocaleDateString('es-ES', { day: '2-digit', month: 'short' })}
                </span>
              </Link>
            ))}
            {!alerts?.alertas?.length && (
              <p className="text-sm text-gray-400 text-center py-4">No active alerts</p>
            )}
          </div>
        </div>

        {/* Recent News */}
        <div className="panel">
          <div className="panel-header">
            <h2 className="panel-title flex items-center gap-2">
              <Newspaper className="w-4 h-4" />
              Latest News
            </h2>
            <Link to="/" className="text-xs text-primary-300 hover:text-primary-50">
              View all →
            </Link>
          </div>
          <div className="space-y-2">
            {recentNews.slice(0, 6).map((news) => (
              <Link
                key={news.id}
                to={`/news/${news.id}`}
                className="flex items-start gap-3 p-2 rounded-lg hover:bg-gray-800 transition-colors group"
              >
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-gray-50 line-clamp-2 group-hover:text-primary-300 transition-colors">
                    {news.title_translated || news.titulo}
                  </p>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="text-[10px] font-mono text-gray-400">{news.fuente_nombre}</span>
                    <span className="text-[10px] text-gray-600">•</span>
                    <span className="text-[10px] font-mono text-gray-400 uppercase">{news.lang_translated || 'ES'}</span>
                  </div>
                </div>
                <span className="text-[10px] font-mono text-gray-400 whitespace-nowrap">
                  {new Date(news.fecha).toLocaleDateString('es-ES', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })}
                </span>
              </Link>
            ))}
            {!recentNews.length && (
              <p className="text-sm text-gray-400 text-center py-4">No recent news</p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
