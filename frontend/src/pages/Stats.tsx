import { useQuery } from '@tanstack/react-query'
import { apiService } from '../services/api'
import { Newspaper, Rss, TrendingUp, Globe, Languages, Clock, CheckCircle, XCircle, AlertCircle } from 'lucide-react'

export function Stats() {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['stats'],
    queryFn: apiService.getStats,
    refetchInterval: 30000,
  })

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-50"></div>
      </div>
    )
  }

  const stats = [
    { label: 'Total News', value: data?.total_news || 0, icon: Newspaper, color: 'text-blue-400' },
    { label: 'Total Feeds', value: data?.total_feeds || 0, icon: Rss, color: 'text-primary-50' },
    { label: 'Translated', value: data?.total_translated || 0, icon: Globe, color: 'text-purple-400' },
    { label: 'News Today', value: data?.news_today || 0, icon: TrendingUp, color: 'text-amber-400' },
    { label: 'This Week', value: data?.news_this_week || 0, icon: TrendingUp, color: 'text-amber-400' },
    { label: 'This Month', value: data?.news_this_month || 0, icon: TrendingUp, color: 'text-red-400' },
  ]

  const translationStats = [
    { label: 'Pending', value: data?.translations_pending || 0, icon: AlertCircle, color: 'text-amber-400' },
    { label: 'Completed', value: data?.translations_done || 0, icon: CheckCircle, color: 'text-primary-50' },
    { label: 'With Error', value: data?.translations_error || 0, icon: XCircle, color: 'text-red-400' },
  ]

  const languages = data?.top_languages || []
  const hourlyStats = data?.translation_stats_12h || []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">Statistics</h1>
          <p className="text-sm text-gray-400 mt-1">System performance and news metrics</p>
        </div>
      </div>
      
      {/* Stats generales */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {stats.map((stat) => (
          <div key={stat.label} className="panel">
            <div className="flex items-center gap-4">
              <div className={`p-3 rounded-lg bg-gray-800 ${stat.color}`}>
                <stat.icon className="h-6 w-6" />
              </div>
              <div>
                <p className="text-sm text-gray-400">{stat.label}</p>
                <p className="text-2xl font-bold text-gray-50">{stat.value.toLocaleString()}</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Stats de traducción */}
      <div className="panel">
        <div className="panel-header">
          <h2 className="panel-title flex items-center gap-2">
            <Globe className="w-4 h-4" />
            Translation Status
          </h2>
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {translationStats.map((stat) => (
            <div key={stat.label} className="flex items-center gap-3">
              <div className={`p-2 rounded-lg bg-gray-800 ${stat.color}`}>
                <stat.icon className="h-5 w-5" />
              </div>
              <div>
                <p className="text-sm text-gray-400">{stat.label}</p>
                <p className="text-xl font-bold text-gray-50">{stat.value.toLocaleString()}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Idiomas originales */}
      <div className="panel">
        <div className="panel-header">
          <h2 className="panel-title flex items-center gap-2">
            <Languages className="w-4 h-4" />
            Original News Languages
          </h2>
        </div>
        {languages.length === 0 ? (
          <p className="text-gray-400">No language data available</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-gray-700">
                  <th className="text-left py-3 px-4 text-xs font-mono uppercase text-gray-400">Language</th>
                  <th className="text-right py-3 px-4 text-xs font-mono uppercase text-gray-400">News</th>
                  <th className="text-right py-3 px-4 text-xs font-mono uppercase text-gray-400">Active Days</th>
                </tr>
              </thead>
              <tbody>
                {languages.map((lang: { lang: string; count: number; dias_activos: number }) => (
                  <tr key={lang.lang} className="border-b border-gray-800 last:border-0">
                    <td className="py-3 px-4">
                      <span className="font-mono font-medium text-gray-50">{lang.lang}</span>
                    </td>
                    <td className="text-right py-3 px-4 text-gray-300">{lang.count.toLocaleString()}</td>
                    <td className="text-right py-3 px-4 text-gray-400">{lang.dias_activos}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Traducciones por hora (últimas 12h) */}
      <div className="panel">
        <div className="panel-header">
          <h2 className="panel-title flex items-center gap-2">
            <Clock className="w-4 h-4" />
            Translation Activity (12h)
          </h2>
        </div>
        {hourlyStats.length === 0 ? (
          <p className="text-gray-400">No hourly translation data</p>
        ) : (
          <div className="space-y-3">
            {(hourlyStats as { hour: string; total: number; items: number }[]).map((hour) => {
              const maxItems = Math.max(...(hourlyStats as { hour: string; total: number; items: number }[]).map(h => h.items), 1)
              const pct = (hour.items / maxItems) * 100
              return (
                <div key={hour.hour} className="flex items-center gap-3">
                  <span className="text-sm font-mono text-gray-400 w-16 shrink-0">{hour.hour}</span>
                  <div className="flex-1 bg-gray-800 rounded-full h-6 overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-primary-500 to-primary-300 rounded-full flex items-center justify-end pr-3 transition-all duration-500"
                      style={{ width: `${Math.max(pct, 2)}%` }}
                    >
                      <span className="text-xs font-mono font-semibold text-gray-900 whitespace-nowrap">{hour.items.toLocaleString()}</span>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
