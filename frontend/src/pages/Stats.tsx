import { useQuery } from '@tanstack/react-query'
import { apiService } from '../services/api'
import { Newspaper, Rss, TrendingUp, Globe } from 'lucide-react'

export function Stats() {
  const { data, isLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: apiService.getStats,
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

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">Estadísticas</h1>
      
      <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
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
    </div>
  )
}
