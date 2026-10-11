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

  const getCountryName = (countryId: number | undefined) => {
    if (!countryId || !countries) return undefined
    const country = countries.find(c => c.id === countryId)
    return country?.nombre
  }

  const countryName = news ? getCountryName(news.pais_id) : undefined

  const handleToggleFavorite = () => {
    if (!id) return
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
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-50"></div>
      </div>
    )
  }

  if (error || !news) {
    return (
      <div className="text-center py-12">
        <p className="text-rss-red">Error loading news</p>
        <Link to="/" className="text-primary-300 hover:text-primary-50 mt-4 inline-block">
          Back to home
        </Link>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <Link to="/" className="inline-flex items-center gap-2 text-gray-400 hover:text-primary-50">
        <ArrowLeft className="h-4 w-4" />
        Back
      </Link>

      <article className="panel p-8">
        {news.imagen_url && news.imagen_url.trim() && (
          <img src={news.imagen_url} alt={news.titulo} className="w-full h-96 object-cover rounded-xl mb-8" />
        )}
        
        <div className="flex items-center gap-4 text-sm text-gray-400 mb-4 flex-wrap">
          {news.fuente_nombre && (
            <span className="flex items-center gap-1">
              <Newspaper className="h-4 w-4" />
              {news.fuente_nombre}
            </span>
          )}
          <span className="text-gray-600">•</span>
          <span>
            {news.fecha && formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
          </span>
          {countryName && (
            <>
              <span className="text-gray-600">•</span>
              <span className="flex items-center gap-1 text-primary-400">
                <MapPin className="h-4 w-4" />
                {countryName}
              </span>
            </>
          )}
        </div>

        <h1 className="text-3xl font-bold text-gray-50 mb-4">
          {news.title_translated || news.titulo}
        </h1>

        <p className="text-lg text-gray-300 mb-6">
          {news.summary_translated || news.resumen}
        </p>

        {(news.content_translated || news.contenido) && (
          <p className="text-gray-300 whitespace-pre-wrap leading-relaxed mt-4">
            {news.content_translated || news.contenido}
          </p>
        )}

        <div className="flex flex-wrap gap-4 items-center mt-6 pt-6 border-t border-gray-700">
          <a
            href={news.url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 text-primary-300 hover:text-primary-50"
          >
            <ExternalLink className="h-4 w-4" />
            View Original
          </a>
          <button
            onClick={handleToggleFavorite}
            className={`inline-flex items-center gap-2 ${id && isFavorite(id) ? 'text-rss-red' : 'text-gray-400 hover:text-rss-red'}`}
          >
            <Heart className={`h-4 w-4 ${id && isFavorite(id) ? 'fill-current' : ''}`} />
            {id && isFavorite(id) ? 'Saved' : 'Save'}
          </button>
          <ShareButtons news={news} size="md" />
        </div>

        {news.entities && news.entities.length > 0 && (
          <div className="mt-8 pt-8 border-t border-gray-700">
            <h3 className="text-xl font-bold tracking-tight text-gray-50 mb-4">Mentioned Entities</h3>
            <div className="flex flex-wrap gap-2">
              {news.entities.map((ent: any) => (
                <WikiTooltip 
                  key={ent.valor + ent.tipo} 
                  name={ent.valor}
                  summary={ent.wiki_summary}
                  imagePath={ent.image_path}
                  wikiUrl={ent.wiki_url}
                >
                  <span className="inline-flex items-center gap-2 px-3 py-2 rounded-xl text-base bg-gray-800 border border-gray-700 hover:border-primary-500/50 transition-colors cursor-pointer">
                    {ent.image_path && (
                      <img 
                        src={ent.image_path} 
                        alt="" 
                        className="w-8 h-8 rounded-full object-cover border border-gray-600"
                        onError={(e) => e.currentTarget.style.display = 'none'}
                      />
                    )}
                    <span className="font-semibold text-gray-200">{ent.valor}</span>
                    <span className="text-[10px] text-gray-400 font-medium uppercase tracking-wider bg-gray-900 px-1.5 py-0.5 rounded border border-gray-700">
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
