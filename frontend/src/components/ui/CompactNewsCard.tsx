import { Link } from 'react-router-dom'
import { Heart, Globe, MapPin } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { useState, useRef } from 'react'
import { News } from '../../services/api'
import { ShareButtons } from './ShareButtons'

interface CompactNewsCardProps {
  news: News
  isFavorite?: boolean
  countryName?: string
  onToggleFavorite?: (news: News) => void
  onAddToList?: (news: News) => void
  onClick?: () => void
}

export function CompactNewsCard({
  news,
  isFavorite = false,
  countryName,
  onToggleFavorite,
  onAddToList,
  onClick,
}: CompactNewsCardProps) {
  const [showHover, setShowHover] = useState(false)
  const hoverTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleMouseEnter = () => {
    hoverTimeout.current = setTimeout(() => {
      setShowHover(true)
    }, 1000)
  }

  const handleMouseLeave = () => {
    if (hoverTimeout.current) {
      clearTimeout(hoverTimeout.current)
      hoverTimeout.current = null
    }
    setShowHover(false)
  }

  const handleFavoriteClick = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    onToggleFavorite?.(news)
  }

  return (
    <div className="relative">
      <Link
        to={`/news/${news.id}`}
        onClick={onClick}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className="card hover:shadow-md transition-all flex gap-3 p-3"
      >
        {news.imagen_url && news.imagen_url.trim() && (
          <img
            src={news.imagen_url}
            alt=""
            className="w-20 h-20 object-cover rounded-lg shrink-0"
          />
        )}
        <div className="min-w-0 flex-1">
          <h3 className="font-medium text-sm line-clamp-2 mb-1 text-gray-900 dark:text-white">
            {news.title_translated || news.titulo}
          </h3>
          <div className="flex items-center gap-2 text-xs text-gray-500 mb-1 flex-wrap">
            {news.fuente_nombre && <span className="truncate">{news.fuente_nombre}</span>}
            {news.fecha && (
              <span>
                {formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
              </span>
            )}
            {countryName && (
              <span className="flex items-center gap-0.5 text-emerald-600 dark:text-emerald-400">
                <MapPin className="h-3 w-3" />
                {countryName}
              </span>
            )}
          </div>
          <p className="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
            {news.summary_translated || news.resumen}
          </p>
        </div>
        <div className="flex flex-col gap-1 shrink-0">
          {news.title_translated && (
            <span className="text-xs bg-primary-100 text-primary-700 px-1.5 py-0.5 rounded flex items-center gap-1">
              <Globe className="h-3 w-3" />
            </span>
          )}
          {onToggleFavorite && (
            <button
              onClick={handleFavoriteClick}
              className={`p-1 rounded-full transition-colors ${
                isFavorite
                  ? 'text-red-500 bg-red-50'
                  : 'text-gray-400 hover:text-red-500 hover:bg-red-50'
              }`}
            >
              <Heart className={`h-4 w-4 ${isFavorite ? 'fill-current' : ''}`} />
            </button>
          )}
          <ShareButtons news={news} size="sm" />
        </div>
      </Link>

      {/* Hover tooltip with full content */}
      {showHover && (
        <div 
          className="absolute z-50 left-0 top-full mt-2 w-[500px] bg-white/50 dark:bg-gray-800/50 backdrop-blur-sm rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 p-5"
          onMouseEnter={handleMouseEnter}
          onMouseLeave={handleMouseLeave}
        >
          <h3 className="font-bold text-lg text-gray-900 dark:text-white mb-3">
            {news.title_translated || news.titulo}
          </h3>
          <p className="text-sm text-gray-700 dark:text-gray-300 mb-4 max-h-40 overflow-y-auto leading-relaxed">
            {news.summary_translated || news.resumen}
          </p>
          {news.content_translated || news.contenido ? (
            <p className="text-sm text-gray-600 dark:text-gray-400 mb-4 max-h-32 overflow-y-auto leading-relaxed">
              {news.content_translated || news.contenido}
            </p>
          ) : null}
          <div className="flex items-center gap-4 text-sm text-gray-600 dark:text-gray-400">
            {news.fuente_nombre && <span className="font-medium">{news.fuente_nombre}</span>}
            {news.fecha && (
              <span>{formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}</span>
            )}
            {countryName && (
              <span className="flex items-center gap-1 text-emerald-600">
                <MapPin className="h-4 w-4" />
                {countryName}
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
