import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { apiService, api } from '../services/api'
import { Search, Globe, Newspaper, Filter } from 'lucide-react'

export function Home() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = parseInt(searchParams.get('page') || '1')
  const q = searchParams.get('q') || ''
  const categoryId = searchParams.get('category_id') || ''
  const countryId = searchParams.get('country_id') || ''
  const translatedOnly = searchParams.get('translated_only') === 'true'
  const [suggestions, setSuggestions] = useState<string[]>([])

  // Load the user's most frequent search terms (dynamic tags) when logged in
  useEffect(() => {
    if (!localStorage.getItem('token')) return
    api.get('/search/suggestions')
      .then((res) => setSuggestions(res.data.terms || []))
      .catch(() => {})
  }, [])

  const { data: categories } = useQuery({
    queryKey: ['categories'],
    queryFn: () => apiService.getCategories(),
  })

  const { data: countries } = useQuery({
    queryKey: ['countries'],
    queryFn: () => apiService.getCountries(),
  })

  const { data, isLoading, error } = useQuery({
    queryKey: ['news', page, q, categoryId, countryId, translatedOnly],
    queryFn: () => apiService.getNews({ 
      page, 
      q, 
      category_id: categoryId || undefined,
      country_id: countryId || undefined,
      translated_only: translatedOnly ? 'true' : undefined,
    }),
  })

  const handleSearch = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const query = formData.get('q')
    // Record the search for frequency-based suggestions (only when logged in)
    if (localStorage.getItem('token') && query) {
      api.post('/searchlog', { q: String(query) }).catch(() => {})
    }
    setSearchParams({ 
      q: query as string, 
      page: '1',
      category_id: categoryId,
      country_id: countryId,
      translated_only: translatedOnly ? 'true' : '',
    })
  }

  const handleFilterChange = (key: string, value: string) => {
    const newParams = Object.fromEntries(searchParams)
    if (value) {
      newParams[key] = value
    } else {
      delete newParams[key]
    }
    newParams.page = '1'
    setSearchParams(newParams)
  }

  const toggleTranslated = () => {
    const newParams = Object.fromEntries(searchParams)
    if (translatedOnly) {
      delete newParams.translated_only
    } else {
      newParams.translated_only = 'true'
    }
    newParams.page = '1'
    setSearchParams(newParams)
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-4">Noticias del Mundo</h1>
        
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
          <div className="relative md:col-span-2">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
            <form onSubmit={handleSearch} className="flex gap-2">
              <input
                name="q"
                defaultValue={q}
                placeholder="Buscar noticias..."
                list="user-search-suggestions"
                className="input pl-10 w-full"
              />
              <datalist id="user-search-suggestions">
                {suggestions.map((s) => <option key={s} value={s} />)}
              </datalist>
              <button type="submit" className="btn-primary">
                Buscar
              </button>
            </form>
          </div>
          
          <select
            value={categoryId}
            onChange={(e) => handleFilterChange('category_id', e.target.value)}
            className="input"
          >
            <option value="">Todas las categorías</option>
            {categories?.map((cat) => (
              <option key={cat.id} value={cat.id}>{cat.nombre}</option>
            ))}
          </select>

          <select
            value={countryId}
            onChange={(e) => handleFilterChange('country_id', e.target.value)}
            className="input"
          >
            <option value="">Todos los países</option>
            {countries?.map((country) => (
              <option key={country.id} value={country.id}>{country.nombre}</option>
            ))}
          </select>
        </div>

        <div className="flex items-center gap-4">
          <button
            onClick={toggleTranslated}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg border transition-colors ${
              translatedOnly 
                ? 'bg-primary-600 text-white border-primary-600' 
                : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
            }`}
          >
            <Globe className="h-4 w-4" />
            Solo traducidas
          </button>
          
          {(categoryId || countryId || translatedOnly) && (
            <button
              onClick={() => setSearchParams({ page: '1', q })}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Limpiar filtros
            </button>
          )}
        </div>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
        </div>
      ) : error ? (
        <div className="text-center py-12 text-red-600">
          Error al cargar noticias
        </div>
      ) : (
        <>
          <div className="mb-4 text-sm text-gray-600">
            {data?.total} noticias encontradas
            {translatedOnly && <span className="ml-2 text-primary-600">(traducidas)</span>}
          </div>

          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {data?.news?.map((news) => (
              <Link key={news.id} to={`/news/${news.id}`} className="card hover:shadow-md transition-shadow">
                {news.imagen_url && news.imagen_url.trim() && (
                  <img
                    src={news.imagen_url}
                    alt={news.title_translated || news.titulo}
                    className="w-full h-48 object-cover rounded-t-xl"
                  />
                )}
                <div className="p-4">
                  <div className="flex items-center gap-2 text-sm text-gray-500 mb-2">
                    {news.fuente_nombre && (
                      <span className="flex items-center gap-1">
                        <Newspaper className="h-4 w-4" />
                        {news.fuente_nombre}
                      </span>
                    )}
                    {news.fuente_nombre && news.fecha && <span>•</span>}
                    {news.fecha && (
                      <span className="text-xs">
                        {formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
                      </span>
                    )}
                    {news.title_translated && (
                      <span className="ml-auto text-xs bg-primary-100 text-primary-700 px-2 py-0.5 rounded">
                        ES
                      </span>
                    )}
                  </div>
                  <h2 className="text-lg font-semibold text-gray-900 mb-2 line-clamp-2">
                    {news.title_translated || news.titulo}
                  </h2>
                  <p className="text-gray-600 text-sm line-clamp-3">
                    {news.summary_translated || news.resumen}
                  </p>
                </div>
              </Link>
            ))}
          </div>

          {data && data.total_pages > 1 && (
            <div className="flex justify-center gap-2 mt-8">
              <button
                onClick={() => setSearchParams({ ...Object.fromEntries(searchParams), page: String(page - 1) })}
                disabled={page <= 1}
                className="btn-secondary disabled:opacity-50"
              >
                Anterior
              </button>
              <span className="flex items-center px-4 text-gray-600">
                {page} / {data.total_pages}
              </span>
              <button
                onClick={() => setSearchParams({ ...Object.fromEntries(searchParams), page: String(page + 1) })}
                disabled={page >= data.total_pages}
                className="btn-secondary disabled:opacity-50"
              >
                Siguiente
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
