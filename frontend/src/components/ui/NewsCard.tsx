import { Link } from 'react-router-dom'
import { Heart, FolderPlus, Newspaper, Globe, MapPin } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { News } from '../../services/api'
import { ShareButtons } from './ShareButtons'

interface NewsCardProps {
  news: News
  showFavoriteButton?: boolean
  showListButton?: boolean
  showShareButtons?: boolean
  isFavorite?: boolean
  countryName?: string
  onToggleFavorite?: (news: News) => void
  onAddToList?: (news: News) => void
  className?: string
}

export function NewsCard({
  news,
  showFavoriteButton = true,
  showListButton = true,
  showShareButtons = false,
  isFavorite = false,
  countryName,
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
    <Link to={`/news/${news.id}`} className={`card hover:shadow-md transition-shadow flex gap-3 p-3 ${className}`}>
      {/* Image - smaller and on the left */}
      {news.imagen_url && news.imagen_url.trim() && (
        <img
          src={news.imagen_url}
          alt=""
          className="w-24 h-24 object-cover rounded-lg shrink-0"
        />
      )}

      {/* Content - elongated vertical layout */}
      <div className="flex-1 min-w-0 flex flex-col">
        {/* Meta row - small text */}
        <div className="flex items-center gap-2 text-xs text-gray-500 mb-1 flex-wrap">
          {news.fuente_nombre && (
            <span className="truncate max-w-[120px]">{news.fuente_nombre}</span>
          )}
          {news.fecha && (
            <span className="shrink-0">
              {formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
            </span>
          )}
          {countryName && (
            <span className="flex items-center gap-0.5 shrink-0 text-emerald-600 dark:text-emerald-400">
              <MapPin className="h-3 w-3" />
              {countryName}
            </span>
          )}
        </div>

        {/* Title - smaller, more lines */}
        <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-1 line-clamp-2 leading-snug">
          {news.title_translated || news.titulo}
        </h3>

        {/* Summary - smaller, more lines visible */}
        <p className="text-xs text-gray-600 dark:text-gray-400 line-clamp-3 leading-relaxed flex-1">
          {news.summary_translated || news.resumen}
        </p>

        {/* Action buttons - minimal */}
        <div className="flex items-center gap-1 mt-2">
          {showFavoriteButton && onToggleFavorite && (
            <button
              onClick={handleFavoriteClick}
              className={`p-1 rounded-full transition-colors ${
                isFavorite
                  ? 'text-red-500 bg-red-50 hover:bg-red-100'
                  : 'text-gray-400 hover:text-red-500 hover:bg-red-50'
              }`}
              title={isFavorite ? 'Quitar de favoritos' : 'Añadir a favoritos'}
            >
              <Heart className={`h-3.5 w-3.5 ${isFavorite ? 'fill-current' : ''}`} />
            </button>
          )}
          {showListButton && onAddToList && (
            <button
              onClick={handleListClick}
              className="p-1 rounded-full text-gray-400 hover:text-blue-500 hover:bg-blue-50 transition-colors"
              title="Añadir a lista"
            >
              <FolderPlus className="h-3.5 w-3.5" />
            </button>
          )}
          {showShareButtons && (
            <ShareButtons news={news} size="sm" />
          )}
        </div>
      </div>
    </Link>
  )
}
