import { Link } from 'react-router-dom'
import { Heart, FolderPlus, Newspaper, Globe } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { News } from '../../services/api'

interface NewsCardProps {
  news: News
  showFavoriteButton?: boolean
  showListButton?: boolean
  isFavorite?: boolean
  onToggleFavorite?: (news: News) => void
  onAddToList?: (news: News) => void
  className?: string
}

export function NewsCard({
  news,
  showFavoriteButton = true,
  showListButton = true,
  isFavorite = false,
  onToggleFavorite,
  onAddToList,
  className = ''
}: NewsCardProps) {

  const handleFavoriteClick = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    onToggleFavorite?.(news)
  }

  const handleListClick = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    onAddToList?.(news)
  }

  return (
    <Link to={`/news/${news.id}`} className={`card hover:shadow-md transition-shadow ${className}`}>
      {news.imagen_url && news.imagen_url.trim() && (
        <img
          src={news.imagen_url}
          alt={news.title_translated || news.titulo}
          className="w-full h-48 object-cover rounded-t-xl"
        />
      )}
      <div className="p-4">
        {/* Action buttons */}
        <div className="flex items-center gap-2 mb-3">
          {showFavoriteButton && onToggleFavorite && (
            <button
              onClick={handleFavoriteClick}
              className={`p-1.5 rounded-full transition-colors ${
                isFavorite
                  ? 'text-red-500 bg-red-50 hover:bg-red-100'
                  : 'text-gray-400 bg-gray-50 hover:text-red-500 hover:bg-red-50'
              }`}
              title={isFavorite ? 'Quitar de favoritos' : 'Añadir a favoritos'}
            >
              <Heart className={`h-5 w-5 ${isFavorite ? 'fill-current' : ''}`} />
            </button>
          )}
          {showListButton && onAddToList && (
            <button
              onClick={handleListClick}
              className="p-1.5 rounded-full text-gray-400 bg-gray-50 hover:text-blue-500 hover:bg-blue-50 transition-colors"
              title="Añadir a lista"
            >
              <FolderPlus className="h-5 w-5" />
            </button>
          )}
          <div className="ml-auto flex items-center gap-2 text-xs text-gray-500">
            {news.title_translated && (
              <span className="flex items-center gap-1 bg-primary-100 text-primary-700 px-2 py-0.5 rounded">
                <Globe className="h-3 w-3" />
                ES
              </span>
            )}
          </div>
        </div>

        {/* Meta */}
        <div className="flex items-center gap-2 text-sm text-gray-500 mb-2">
          {news.fuente_nombre && (
            <span className="flex items-center gap-1">
              <Newspaper className="h-4 w-4" />
              {news.fuente_nombre}
            </span>
          )}
          {news.fecha && (
            <span className="text-xs">
              {formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
            </span>
          )}
        </div>

        {/* Title & Summary */}
        <h2 className="text-lg font-semibold text-gray-900 mb-2 line-clamp-2">
          {news.title_translated || news.titulo}
        </h2>
        <p className="text-gray-600 text-sm line-clamp-3">
          {news.summary_translated || news.resumen}
        </p>
      </div>
    </Link>
  )
}
