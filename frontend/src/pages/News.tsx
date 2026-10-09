import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { apiService } from '../services/api'
import { Globe, ArrowLeft, ExternalLink, Newspaper, Heart, MapPin } from 'lucide-react'
import { useState, useEffect } from 'react'
import { WikiTooltip } from '../components/ui/WikiTooltip'
import { ShareButtons } from '../components/ui/ShareButtons'
import { useApiFavorites } from '../hooks/useApiFavorites'

export function News() {
  const { id } = useParams<{ id: string }>()
  
  const { data: news, isLoading, error } = useQuery({
    queryKey: ['news', id],
    queryFn: () => apiService.getNewsById(id!),
    enabled: !!id,
  })

  const { data: countries } = useQuery({
    queryKey: ['countries'],
    queryFn: apiService.getCountries,
  })

  const { isFavorite, toggleFavorite, refresh: refreshFavorites } = useApiFavorites()

  // Helper to get country name by ID
  const getCountryName = (countryId: number | undefined) => {
    if (!countryId || !countries) return undefined
    const country = countries.find(c => c.id === countryId)
    return country?.nombre
  }

  const countryName = news ? getCountryName(news.pais_id) : undefined

  const handleToggleFavorite = () => {
    if (!id) return
    // Convert apiService format to our News format
    const newsForToggle = {
      id: news!.id,
      titulo: news!.titulo || news!.title_translated || '',
      resumen: news!.resumen || news!.summary_translated || '',
      url: news!.url,
      fecha: news!.fecha || '',
      imagen_url: news!.imagen_url,
      fuente_nombre: news!.fuente_nombre || '',
    }
    toggleFavorite(newsForToggle)
  }

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
      </div>
    )
  }

  if (error || !news) {
    return (
      <div className="text-center py-12">
        <p className="text-red-600">Error al cargar la noticia</p>
        <Link to="/" className="text-primary-600 hover:underline mt-4 inline-block">
          Volver al inicio
        </Link>
      </div>
    )
  }

  return (
    <div>
      <Link to="/" className="inline-flex items-center gap-2 text-gray-600 hover:text-primary-600 mb-6">
        <ArrowLeft className="h-4 w-4" />
        Volver
      </Link>

      <article className="card p-8">
        {news.imagen_url && news.imagen_url.trim() && (
          <img src={news.imagen_url} alt={news.titulo} className="w-full h-96 object-cover rounded-xl mb-8" />
        )}
        
        <div className="flex items-center gap-4 text-sm text-gray-500 mb-4 flex-wrap">
          {news.fuente_nombre && (
            <span className="flex items-center gap-1">
              <Newspaper className="h-4 w-4" />
              {news.fuente_nombre}
            </span>
          )}
          <span>•</span>
          <span>
            {news.fecha && formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
          </span>
          {countryName && (
            <>
              <span>•</span>
              <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
                <MapPin className="h-4 w-4" />
                {countryName}
              </span>
            </>
          )}
        </div>

        <h1 className="text-3xl font-bold text-gray-900 mb-4">
          {news.title_translated || news.titulo}
        </h1>

        <p className="text-lg text-gray-600 mb-6">
          {news.summary_translated || news.resumen}
        </p>

        {(news.content_translated || news.contenido) && (
          <p className="text-gray-700 whitespace-pre-wrap leading-relaxed mt-4">
            {news.content_translated || news.contenido}
          </p>
        )}

        <div className="flex flex-wrap gap-4 items-center">
          <a
            href={news.url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 text-primary-600 hover:underline"
          >
            <ExternalLink className="h-4 w-4" />
            View Original
          </a>
          <button
            onClick={handleToggleFavorite}
            className={`inline-flex items-center gap-2 ${id && isFavorite(id) ? 'text-red-500' : 'text-gray-500 hover:text-red-500'}`}
          >
            <Heart className={`h-4 w-4 ${id && isFavorite(id) ? 'fill-current' : ''}`} />
            {id && isFavorite(id) ? 'Saved' : 'Save'}
          </button>
          <ShareButtons news={news} size="md" />
        </div>

        {news.entities && news.entities.length > 0 && (
          <div className="mt-8 pt-8 border-t border-gray-100">
            <h3 className="text-xl font-bold tracking-tight text-gray-900 mb-4">Entidades Mencionadas</h3>
            <div className="flex flex-wrap gap-2">
              {news.entities.map((ent: any) => (
                <WikiTooltip 
                  key={ent.valor + ent.tipo} 
                  name={ent.valor}
                  summary={ent.wiki_summary}
                  imagePath={ent.image_path}
                  wikiUrl={ent.wiki_url}
                >
                  <span className="inline-flex items-center gap-4 px-3 py-2 rounded-xl text-base bg-gray-50 border border-gray-200 hover:border-primary-300 hover:bg-primary-50 transition-colors cursor-pointer group-hover:border-primary-300 group-hover:bg-primary-50">
                    {ent.image_path && (
                      <img 
                        src={ent.image_path} 
                        alt="" 
                        className="w-20 h-20 rounded-full object-cover border-2 border-white shadow-md"
                        onError={(e) => (e.currentTarget.style.display = 'none')}
                      />
                    )}
                    <span className="font-semibold text-gray-800 ml-1">{ent.valor}</span>
                    <span className="text-[10px] text-gray-500 font-medium uppercase tracking-wider bg-white px-1.5 py-0.5 rounded-md border border-gray-200">
                      {ent.tipo}
                    </span>
                  </span>
                </WikiTooltip>
              ))}
            </div>
          </div>
        )}
      </article>
    </div>
  )
}
