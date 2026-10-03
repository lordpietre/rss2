import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { apiService, api } from '../services/api'
import { News } from '../services/api'
import { Search, Globe } from 'lucide-react'
import { SkeletonNewsList, NoResultsFound } from '../components/ui'
import { NewsCard } from '../components/ui/NewsCard'
import { CreateListModal } from '../components/CreateListModal'
import { useApiFavorites, useApiLists } from '../hooks/useApiFavorites'

export function Home() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = parseInt(searchParams.get('page') || '1')
  const q = searchParams.get('q') || ''
  const categoryId = searchParams.get('category_id') || ''
  const countryId = searchParams.get('country_id') || ''
  const translatedOnly = searchParams.get('translated_only') === 'true'
  const [suggestions, setSuggestions] = useState<string[]>([])

  // List modal state
  const [showListModal, setShowListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)
  const { favorites, toggleFavorite, isFavorite } = useApiFavorites()
  const { lists, createList, addToList } = useApiLists()

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
    queryFn: apiService.getCountries,
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

  const handleAddToList = (news: News) => {
    setSelectedNewsForList(news)
    setShowListModal(true)
  }

  const handleSelectList = async (listId: number) => {
    if (selectedNewsForList) {
      await addToList(listId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
  }

  const handleCreateAndAdd = async (name: string) => {
    console.log('Home: handleCreateAndAdd called with:', name)
    const newListId = await createList(name)
    console.log('Home: createList returned:', newListId)
    if (selectedNewsForList) {
      console.log('Home: adding to list:', newListId, selectedNewsForList.id)
      await addToList(newListId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
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
        <SkeletonNewsList count={6} />
      ) : error ? (
        <div className="text-center py-12 text-red-600">
          Error al cargar noticias
          <button
            onClick={() => window.location.reload()}
            className="btn-primary mt-4"
          >
            Reintentar
          </button>
        </div>
      ) : data?.news?.length === 0 ? (
        <NoResultsFound
          query={q}
          onClearFilters={() => setSearchParams({ page: '1', q })}
        />
      ) : (
        <>
          <div className="mb-4 text-sm text-gray-600">
            {data?.total} noticias encontradas
            {translatedOnly && <span className="ml-2 text-primary-600">(traducidas)</span>}
          </div>

          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {data?.news?.map((news) => (
              <NewsCard
                key={news.id}
                news={news}
                isFavorite={isFavorite(news.id)}
                onToggleFavorite={toggleFavorite}
                onAddToList={handleAddToList}
              />
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

      <CreateListModal
        isOpen={showListModal}
        onClose={() => { setShowListModal(false); setSelectedNewsForList(null) }}
        onCreate={handleCreateAndAdd}
        mode="add-to"
        existingLists={lists.map((l) => ({ id: l.id, name: l.name }))}
        onSelectList={handleSelectList}
      />
    </div>
  )
}
