import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { X, Heart, FolderPlus, Globe, ExternalLink, MapPin } from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { News } from '../../services/api'
import { ShareButtons } from './ShareButtons'

interface ExpandedNewsModalProps {
  news: News | null
  isFavorite: boolean
  countryName?: string
  onClose: () => void
  onToggleFavorite: (news: News) => void
  onAddToList: (news: News) => void
}

export function ExpandedNewsModal({
  news,
  isFavorite,
  countryName,
  onClose,
  onToggleFavorite,
  onAddToList,
}: ExpandedNewsModalProps) {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    if (news) {
      // Delay to allow animation
      requestAnimationFrame(() => setVisible(true))
    } else {
      setVisible(false)
    }
  }, [news])

  // Close on escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [onClose])

  // Prevent body scroll when modal is open
  useEffect(() => {
    if (news) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [news])

  const handleBackdropClick = useCallback(() => {
    onClose()
  }, [onClose])

  if (!news) return null

  return (
    <div
      className={`fixed inset-0 z-50 flex items-center justify-center p-4 transition-opacity duration-200 ${
        visible ? 'opacity-100' : 'opacity-0 pointer-events-none'
      }`}
      onClick={handleBackdropClick}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
      
      {/* Modal */}
      <div
        className={`relative bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-full max-w-3xl max-h-[85vh] overflow-hidden transition-transform duration-200 ${
          visible ? 'scale-100 translate-y-0' : 'scale-95 translate-y-4'
        }`}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header con imagen */}
        <div className="relative h-64">
          {news.imagen_url && news.imagen_url.trim() ? (
            <img
              src={news.imagen_url}
              alt=""
              className="w-full h-full object-cover"
            />
          ) : (
            <div className="w-full h-full bg-gradient-to-br from-primary-500 to-primary-700" />
          )}
          <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/20 to-transparent" />
          
          {/* Close button */}
          <button
            onClick={onClose}
            className="absolute top-4 right-4 p-2 bg-white/20 hover:bg-white/40 rounded-full text-white transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
          
          {/* Title overlay */}
          <div className="absolute bottom-0 left-0 right-0 p-6">
            <h2 className="text-2xl font-bold text-white mb-2 line-clamp-2">
              {news.title_translated || news.titulo}
            </h2>
            <div className="flex items-center gap-3 text-sm text-white/80 flex-wrap">
              {news.fuente_nombre && <span>{news.fuente_nombre}</span>}
              {news.fecha && (
                <span>
                  {formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
                </span>
              )}
              {countryName && (
                <span className="flex items-center gap-1 bg-white/20 px-2 py-0.5 rounded">
                  <MapPin className="h-3 w-3" /> {countryName}
                </span>
              )}
              {news.title_translated && (
                <span className="flex items-center gap-1 bg-white/20 px-2 py-0.5 rounded">
                  <Globe className="h-3 w-3" /> Traducido
                </span>
              )}
            </div>
          </div>
        </div>
        
        {/* Content */}
        <div className="p-6 overflow-y-auto max-h-[calc(85vh-16rem)]">
          <p className="text-gray-600 dark:text-gray-300 leading-relaxed mb-6">
            {news.summary_translated || news.resumen}
          </p>
          
          {/* Actions */}
          <div className="flex flex-wrap items-center gap-3">
            <a
              href={news.url}
              target="_blank"
              rel="noopener noreferrer"
              className="btn-secondary flex items-center gap-2"
            >
              <ExternalLink className="h-4 w-4" />
              Ver original
            </a>
            <button
              onClick={() => onToggleFavorite(news)}
              className={`btn-secondary flex items-center gap-2 ${
                isFavorite ? 'text-red-500 border-red-200 dark:border-red-800' : ''
              }`}
            >
              <Heart className={`h-4 w-4 ${isFavorite ? 'fill-current' : ''}`} />
              {isFavorite ? 'Quitar de favoritos' : 'Favorito'}
            </button>
            <button
              onClick={() => onAddToList(news)}
              className="btn-secondary flex items-center gap-2"
            >
              <FolderPlus className="h-4 w-4" />
              Añadir a lista
            </button>
            <ShareButtons news={news} size="md" />
            <Link
              to={`/news/${news.id}`}
              className="btn-primary flex items-center gap-2 ml-auto"
              onClick={onClose}
            >
              Ver detalle →
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}
