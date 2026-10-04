import { useState } from 'react'
import { Link } from 'react-router-dom'
import { News } from '../../services/api'
import { ChevronDown, ChevronUp, Lightbulb, Plus, X } from 'lucide-react'

interface SuggestedNewsSidebarProps {
  news: News[]
  onAddToList?: (news: News) => void
}

export function SuggestedNewsSidebar({ news, onAddToList }: SuggestedNewsSidebarProps) {
  const [expanded, setExpanded] = useState(false)

  if (news.length === 0) {
    return null
  }

  const displayNews = expanded ? news : news.slice(0, 5)
  const hasMore = news.length > 5

  return (
    <div className="bg-white border border-gray-200 rounded-lg shadow-sm">
      <div className="p-3 border-b border-gray-100 bg-gradient-to-r from-primary-50 to-blue-50">
        <div className="flex items-center gap-2">
          <Lightbulb className="h-4 w-4 text-primary-600" />
          <span className="font-semibold text-sm text-gray-900">Sugerencias para tus listas</span>
        </div>
        <p className="text-xs text-gray-500 mt-0.5">
          Noticias relacionadas con tus listas
        </p>
      </div>
      
      <div className="p-2">
        {displayNews.map(n => (
          <div key={n.id} className="group relative py-1.5 border-b border-gray-50 last:border-0">
            <Link to={`/news/${n.id}`} className="block">
              <p className="text-xs font-medium text-gray-700 hover:text-primary-600 line-clamp-2 leading-tight">
                {n.title_translated || n.titulo}
              </p>
              <p className="text-[10px] text-gray-400 mt-0.5">{n.fuente_nombre}</p>
            </Link>
            {onAddToList && (
              <button
                onClick={(e) => { e.preventDefault(); onAddToList(n); }}
                className="absolute right-0 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100 p-1 bg-primary-100 hover:bg-primary-200 text-primary-600 rounded transition-opacity"
                title="Añadir a lista"
              >
                <Plus className="h-3 w-3" />
              </button>
            )}
          </div>
        ))}
        
        {hasMore && (
          <button
            onClick={() => setExpanded(!expanded)}
            className="w-full mt-2 py-1 text-xs text-primary-600 hover:text-primary-700 flex items-center justify-center gap-1"
          >
            {expanded ? (
              <><ChevronUp className="h-3 w-3" /> Ver menos</>
            ) : (
              <><ChevronDown className="h-3 w-3" /> Ver {news.length - 5} más</>
            )}
          </button>
        )}
      </div>
    </div>
  )
}
