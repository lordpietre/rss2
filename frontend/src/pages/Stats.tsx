import { useQuery } from '@tanstack/react-query'
import { apiService } from '../services/api'
import { Newspaper, Rss, TrendingUp, Globe, Languages, Clock, CheckCircle, XCircle, AlertCircle } from 'lucide-react'

export function Stats() {
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['stats'],
    queryFn: apiService.getStats,
    refetchInterval: 30000, // Refetch cada 30 segundos
  })

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
      </div>
    )
  }

  const stats = [
    { label: 'Total Noticias', value: data?.total_news || 0, icon: Newspaper, color: 'text-blue-600' },
    { label: 'Total Feeds', value: data?.total_feeds || 0, icon: Rss, color: 'text-green-600' },
    { label: 'Traducidas', value: data?.total_translated || 0, icon: Globe, color: 'text-indigo-600' },
    { label: 'Noticias Hoy', value: data?.news_today || 0, icon: TrendingUp, color: 'text-orange-600' },
    { label: 'Esta Semana', value: data?.news_this_week || 0, icon: TrendingUp, color: 'text-purple-600' },
    { label: 'Este Mes', value: data?.news_this_month || 0, icon: TrendingUp, color: 'text-red-600' },
  ]

  const translationStats = [
    { label: 'Pendientes', value: data?.translations_pending || 0, icon: AlertCircle, color: 'text-yellow-600' },
    { label: 'Completadas', value: data?.translations_done || 0, icon: CheckCircle, color: 'text-green-600' },
    { label: 'Con Error', value: data?.translations_error || 0, icon: XCircle, color: 'text-red-600' },
  ]

  // Idiomas originales (top 10)
  const languages = data?.top_languages || []

  // Stats de traducción por hora (últimas 12h)
  const hourlyStats = data?.translation_stats_12h || []

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">Estadísticas</h1>
      
      {/* Stats generales */}
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3 mb-8">
        {stats.map((stat) => (
          <div key={stat.label} className="card p-6">
            <div className="flex items-center gap-4">
              <div className={`p-3 rounded-lg bg-gray-100 ${stat.color}`}>
                <stat.icon className="h-6 w-6" />
              </div>
              <div>
                <p className="text-sm text-gray-500">{stat.label}</p>
                <p className="text-2xl font-bold text-gray-900">{stat.value.toLocaleString()}</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Stats de traducción */}
      <h2 className="text-xl font-bold text-gray-900 mb-4">Estado de Traducciones</h2>
      <div className="grid gap-6 md:grid-cols-3 mb-8">
        {translationStats.map((stat) => (
          <div key={stat.label} className="card p-6">
            <div className="flex items-center gap-4">
              <div className={`p-3 rounded-lg bg-gray-100 ${stat.color}`}>
                <stat.icon className="h-6 w-6" />
              </div>
              <div>
                <p className="text-sm text-gray-500">{stat.label}</p>
                <p className="text-2xl font-bold text-gray-900">{stat.value.toLocaleString()}</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Idiomas originales */}
      <h2 className="text-xl font-bold text-gray-900 mb-4">Idiomas Originales de Noticias</h2>
      <div className="card p-6 mb-8">
        {languages.length === 0 ? (
          <p className="text-gray-500">No hay datos de idiomas</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 px-3 text-sm font-medium text-gray-500">Idioma</th>
                  <th className="text-right py-2 px-3 text-sm font-medium text-gray-500">Noticias</th>
                  <th className="text-right py-2 px-3 text-sm font-medium text-gray-500">Días Activo</th>
                </tr>
              </thead>
              <tbody>
                {languages.map((lang: { lang: string; count: number; dias_activos: number }) => (
                  <tr key={lang.lang} className="border-b last:border-0">
                    <td className="py-2 px-3">
                      <span className="font-medium">{lang.lang}</span>
                    </td>
                    <td className="text-right py-2 px-3">{lang.count.toLocaleString()}</td>
                    <td className="text-right py-2 px-3 text-gray-500">{lang.dias_activos}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Traducciones por hora (últimas 12h) */}
      <h2 className="text-xl font-bold text-gray-900 mb-4">Traducciones por Hora (últimas 12h)</h2>
      <div className="card p-6">
        {hourlyStats.length === 0 ? (
          <p className="text-gray-500">No hay datos de traducciones por hora</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 px-3 text-sm font-medium text-gray-500">Hora</th>
                  <th className="text-right py-2 px-3 text-sm font-medium text-gray-500">Batches</th>
                  <th className="text-right py-2 px-3 text-sm font-medium text-gray-500">Items Traducidos</th>
                </tr>
              </thead>
              <tbody>
                {hourlyStats.map((hour: { hour: string; total: number; items: number }) => (
                  <tr key={hour.hour} className="border-b last:border-0">
                    <td className="py-2 px-3">
                      <span className="font-medium">{hour.hour}</span>
                    </td>
                    <td className="text-right py-2 px-3">{hour.total}</td>
                    <td className="text-right py-2 px-3 font-medium text-indigo-600">{hour.items.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
