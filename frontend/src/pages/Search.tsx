import { useState } from 'react'
import { useSearchParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { apiService, News, Category, Country } from '../services/api'
import { Search as SearchIcon, Filter } from 'lucide-react'

export function Search() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') || ''
  const lang = searchParams.get('lang') || ''
  const categoria = searchParams.get('categoria') || ''
  const pais = searchParams.get('pais') || ''

  const { data: categorias } = useQuery({
    queryKey: ['categories'],
    queryFn: apiService.getCategories,
  })

  const { data: paises } = useQuery({
    queryKey: ['countries'],
    queryFn: apiService.getCountries,
  })

  const { data, isLoading } = useQuery({
    queryKey: ['search', q, lang, categoria, pais],
    queryFn: () => apiService.search({ 
      q, 
      lang: lang || undefined,
    }),
    enabled: !!q || !!lang,
  })

  const handleSearch = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const newParams: Record<string, string> = {}
    if (formData.get('q')) newParams.q = formData.get('q') as string
    if (formData.get('lang')) newParams.lang = formData.get('lang') as string
    if (formData.get('categoria')) newParams.categoria = formData.get('categoria') as string
    if (formData.get('pais')) newParams.pais = formData.get('pais') as string
    setSearchParams(newParams)
  }

  const clearFilters = () => {
    const q = searchParams.get('q') || ''
    setSearchParams(q ? { q } : {})
  }

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">Buscar Noticias</h1>
      
      <form onSubmit={handleSearch} className="card p-4 mb-6">
        <div className="flex flex-wrap gap-4 items-end">
          <div className="flex-1 min-w-[200px]">
            <label className="block text-sm font-medium text-gray-700 mb-1">Buscar</label>
            <input
              name="q"
              defaultValue={q}
              placeholder="Palabras clave..."
              className="input w-full"
            />
          </div>
          
          <div className="w-32">
            <label className="block text-sm font-medium text-gray-700 mb-1">Idioma</label>
            <select name="lang" defaultValue={lang} className="input w-full">
              <option value="">Todos</option>
              <option value="es">Español</option>
              <option value="en">Inglés</option>
              <option value="fr">Francés</option>
              <option value="pt">Portugués</option>
              <option value="de">Alemán</option>
              <option value="it">Italiano</option>
              <option value="ru">Ruso</option>
              <option value="zh">Chino</option>
              <option value="ja">Japonés</option>
              <option value="ar">Árabe</option>
            </select>
          </div>

          <div className="w-40">
            <label className="block text-sm font-medium text-gray-700 mb-1">Categoría</label>
            <select name="categoria" defaultValue={categoria} className="input w-full">
              <option value="">Todas</option>
              {categorias?.map((cat) => (
                <option key={cat.id} value={cat.id}>{cat.nombre}</option>
              ))}
            </select>
          </div>

          <div className="w-40">
            <label className="block text-sm font-medium text-gray-700 mb-1">País</label>
            <select name="pais" defaultValue={pais} className="input w-full">
              <option value="">Todos</option>
              {paises?.map((p) => (
                <option key={p.id} value={p.id}>{p.nombre}</option>
              ))}
            </select>
          </div>

          <button type="submit" className="btn-primary">
            <SearchIcon className="h-5 w-5" />
          </button>
          
          {(categoria || pais || lang) && (
            <button type="button" onClick={clearFilters} className="btn-secondary">
              Limpiar
            </button>
          )}
        </div>
      </form>

      {data && data.total > 0 && (
        <p className="text-gray-600 mb-4">{data.total} resultados encontrados</p>
      )}

      {isLoading && (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
        </div>
      )}

      {data?.news && data.news.length > 0 && (
        <div className="space-y-4">
          {data.news.map((news: News) => (
            <Link key={news.id} to={`/news/${news.id}`} className="card p-4 block hover:shadow-md transition-shadow">
              <h3 className="font-semibold text-gray-900">{news.title_translated || news.titulo}</h3>
              <p className="text-gray-600 text-sm mt-1">{news.summary_translated || news.resumen}</p>
              <div className="flex gap-4 mt-2 text-xs text-gray-500">
                <span>{news.lang_translated || news.fuente_nombre}</span>
                <span>{new Date(news.fecha).toLocaleDateString()}</span>
              </div>
            </Link>
          ))}
        </div>
      )}

      {(!q && !lang && !categoria && !pais) && (
        <p className="text-center text-gray-500 mt-8">Introduce una búsqueda o selecciona filtros</p>
      )}

      {q && !isLoading && data?.news?.length === 0 && (
        <p className="text-center text-gray-500 mt-8">No se encontraron resultados</p>
      )}
    </div>
  )
}
