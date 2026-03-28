import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiService, News } from '../services/api'
import { Heart, Trash2, ExternalLink, Calendar, Newspaper } from 'lucide-react'

export function Favorites() {
  const queryClient = useQueryClient()
  const [favorites, setFavorites] = useState<News[]>([])

  useEffect(() => {
    const stored = localStorage.getItem('favorites')
    if (stored) {
      setFavorites(JSON.parse(stored))
    }
  }, [])

  const removeFavorite = (id: string) => {
    const updated = favorites.filter(n => n.id !== id)
    setFavorites(updated)
    localStorage.setItem('favorites', JSON.stringify(updated))
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-'
    const date = new Date(dateStr)
    return date.toLocaleDateString('es', { day: '2-digit', month: 'short', year: 'numeric' })
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Favorites</h1>
        <span className="text-gray-500">{favorites.length} news saved</span>
      </div>

      {favorites.length === 0 ? (
        <div className="card p-12 text-center">
          <Heart className="h-16 w-16 text-gray-300 mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-gray-700 mb-2">No favorites yet</h2>
          <p className="text-gray-500">
            Save news to your favorites by clicking the heart icon on any news item.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {favorites.map((news) => (
            <div key={news.id} className="card p-6 hover:shadow-md transition-shadow">
              <div className="flex gap-4">
                {news.imagen_url && (
                  <img
                    src={news.imagen_url}
                    alt={news.titulo}
                    className="w-32 h-24 object-cover rounded-lg flex-shrink-0"
                  />
                )}
                <div className="flex-1 min-w-0">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h3 className="text-lg font-semibold text-gray-900 line-clamp-2">
                        {news.title_translated || news.titulo}
                      </h3>
                      {news.summary_translated && (
                        <p className="text-sm text-gray-600 mt-1 line-clamp-2">
                          {news.summary_translated}
                        </p>
                      )}
                      {news.summary_translated === null && news.resumen && (
                        <p className="text-sm text-gray-600 mt-1 line-clamp-2">
                          {news.resumen}
                        </p>
                      )}
                    </div>
                    <button
                      onClick={() => removeFavorite(news.id)}
                      className="text-red-500 hover:text-red-600 p-1"
                      title="Remove from favorites"
                    >
                      <Heart className="h-5 w-5 fill-current" />
                    </button>
                  </div>
                  <div className="flex items-center gap-4 mt-3 text-sm text-gray-500">
                    <span className="flex items-center gap-1">
                      <Newspaper className="h-4 w-4" />
                      {news.fuente_nombre}
                    </span>
                    <span className="flex items-center gap-1">
                      <Calendar className="h-4 w-4" />
                      {formatDate(news.fecha)}
                    </span>
                  </div>
                  <a
                    href={news.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-primary-600 hover:underline text-sm mt-2 inline-flex items-center gap-1"
                  >
                    <ExternalLink className="h-3 w-3" />
                    View Original
                  </a>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
