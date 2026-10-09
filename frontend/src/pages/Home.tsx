import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { apiService, api } from '../services/api'
import { News } from '../services/api'
import { Search, Globe, X, Sparkles, Bookmark } from 'lucide-react'
import { SkeletonNewsList, NoResultsFound, CompactNewsCard, ExpandedNewsModal } from '../components/ui'
import { NewsCard } from '../components/ui/NewsCard'
import { CreateListModal } from '../components/CreateListModal'
import { SaveSearchModal } from '../components/SaveSearchModal'
import { useApiFavorites, useApiLists, useApiSavedSearches } from '../hooks/useApiFavorites'
import { useSearchHistory } from '../hooks/useSearchHistory'

export function Home() {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = parseInt(searchParams.get('page') || '1')
  const q = searchParams.get('q') || ''
  const categoryId = searchParams.get('category_id') || ''
  const countryId = searchParams.get('country_id') || ''
  const translatedOnly = searchParams.get('translated_only') === 'true'
  const semantic = searchParams.get('semantic') === 'true'
  
  // Local state for search input
  const [localQuery, setLocalQuery] = useState(q)
  
  // List modal state
  const [showListModal, setShowListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)
  const { favorites, toggleFavorite, isFavorite } = useApiFavorites()
  const { lists, createList, addToList } = useApiLists()
  
  // Search history
  const { history, addToHistory, clearHistory, removeFromHistory } = useSearchHistory()
  const [showHistory, setShowHistory] = useState(false)
  const historyRef = useRef<HTMLDivElement>(null)
  
  // Expanded modal for hover/touch
  const [expandedNews, setExpandedNews] = useState<News | null>(null)

  // Save search modal state
  const [showSaveModal, setShowSaveModal] = useState(false)
  const { createSearch } = useApiSavedSearches()

  // Detect if we're in search mode (for grid layout)
  const isSearchMode = !!q || !!categoryId || !!countryId

  // Use compact cards only when there's actual text/category/country search, not just translated filter
  const useCompactCards = !!q || !!categoryId || !!countryId

  // Load the user's most frequent search terms (dynamic tags) when logged in
  useEffect(() => {
    if (!localStorage.getItem('token')) return
    api.get('/search/suggestions')
      .then((res) => {
        // Merge with local history
      })
      .catch(() => {})
  }, [])

  // Close history dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (historyRef.current && !historyRef.current.contains(e.target as Node)) {
        setShowHistory(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const { data: categories } = useQuery({
    queryKey: ['categories'],
    queryFn: () => apiService.getCategories(),
  })

  const { data: countries } = useQuery({
    queryKey: ['countries'],
    queryFn: apiService.getCountries,
  })

  // Helper to get country name by ID
  const getCountryName = (countryId: number | undefined) => {
    if (!countryId || !countries) return undefined
    const country = countries.find(c => c.id === countryId)
    return country?.nombre
  }

  const { data, isLoading, error } = useQuery({
    queryKey: ['news', page, q, categoryId, countryId, translatedOnly, semantic],
    queryFn: () => apiService.getNews({ 
      page, 
      q, 
      category_id: categoryId || undefined,
      country_id: countryId || undefined,
      translated_only: translatedOnly ? 'true' : undefined,
      semantic: semantic ? 'true' : undefined,
    }),
  })

  const handleSearch = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const query = formData.get('q') as string
    
    // Add to history if not empty
    if (query?.trim()) {
      addToHistory(query.trim())
    }
    
    // Log search if logged in
    if (localStorage.getItem('token') && query) {
      api.post('/searchlog', { q: String(query) }).catch(() => {})
    }
    
    setSearchParams({ 
      q: query || '',
      page: '1',
      category_id: categoryId,
      country_id: countryId,
      translated_only: translatedOnly ? 'true' : '',
      semantic: semantic ? 'true' : '',
    })
    setShowHistory(false)
  }

  const handleClearSearch = () => {
    setLocalQuery('')
    setSearchParams({ 
      page: '1',
      category_id: categoryId,
      country_id: countryId,
      translated_only: translatedOnly ? 'true' : '',
      semantic: semantic ? 'true' : '',
    })
  }

  const handleHistoryClick = (term: string) => {
    setLocalQuery(term)
    setSearchParams({ 
      q: term,
      page: '1',
      category_id: categoryId,
      country_id: countryId,
      translated_only: translatedOnly ? 'true' : '',
      semantic: semantic ? 'true' : '',
    })
    setShowHistory(false)
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

  const toggleSemantic = () => {
    const newParams = Object.fromEntries(searchParams)
    if (semantic) {
      delete newParams.semantic
    } else {
      newParams.semantic = 'true'
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
    const newListId = await createList(name)
    if (selectedNewsForList) {
      await addToList(newListId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
  }

  // Update localQuery when URL changes
  useEffect(() => {
    setLocalQuery(q)
  }, [q])

  return (
    <div>
      {/* Header - Only show count, not title */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-4">
          <div className="text-sm text-gray-500">
            {data?.total || 0} noticias
            {isSearchMode && q && (
              <span className="ml-1">para "<strong>{q}</strong>"</span>
            )}
          </div>
        </div>
        
        {/* Search Box with History */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
          <div className="relative md:col-span-2" ref={historyRef}>
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-gray-400" />
            <form onSubmit={handleSearch} className="flex gap-2">
              <div className="relative flex-1">
                <input
                  name="q"
                  value={localQuery}
                  onChange={(e) => setLocalQuery(e.target.value)}
                  onFocus={() => setShowHistory(true)}
                  placeholder="Buscar noticias..."
                  className="input pl-10 pr-10 w-full"
                />
                {localQuery && (
                  <button
                    type="button"
                    onClick={handleClearSearch}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors p-1"
                  >
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>
              <button type="submit" className="btn-primary">
                Buscar
              </button>
            </form>
            
            {/* History Dropdown */}
            {showHistory && history.length > 0 && (
              <div className="absolute z-50 w-full mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg overflow-hidden">
                <div className="flex justify-between items-center px-3 py-2 border-b border-gray-200 dark:border-gray-700">
                  <span className="text-xs text-gray-500">Búsquedas recientes</span>
                  <button 
                    onClick={clearHistory} 
                    className="text-xs text-red-500 hover:underline"
                  >
                    Limpiar todo
                  </button>
                </div>
                {history.map((term) => (
                  <div
                    key={term}
                    className="flex items-center justify-between px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer transition-colors"
                    onClick={() => handleHistoryClick(term)}
                  >
                    <div className="flex items-center gap-2">
                      <Search className="h-4 w-4 text-gray-400" />
                      <span className="text-sm text-gray-700 dark:text-gray-300">{term}</span>
                    </div>
                    <button
                      onClick={(e) => {
                        e.stopPropagation()
                        removeFromHistory(term)
                      }}
                      className="p-1 text-gray-400 hover:text-red-500 transition-colors"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                ))}
              </div>
            )}
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

          <button
            onClick={toggleSemantic}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg border transition-colors ${
              semantic 
                ? 'bg-purple-600 text-white border-purple-600' 
                : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
            }`}
          >
            <Sparkles className="h-4 w-4" />
            Búsqueda semántica
          </button>
          
          {isSearchMode && (
            <button
              onClick={() => setSearchParams({ page: '1' })}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Limpiar filtros
            </button>
          )}

          {isSearchMode && (
            <button
              onClick={() => setShowSaveModal(true)}
              className="flex items-center gap-1 text-sm text-primary-600 hover:text-primary-700 font-medium"
              title="Guardar esta búsqueda"
            >
              <Bookmark className="h-4 w-4" />
              Guardar
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
          onClearFilters={() => setSearchParams({ page: '1' })}
        />
      ) : (
        <>
          {/* News Grid */}
          <div className='grid gap-4 md:grid-cols-2'>
            {data?.news?.map((news) => 
              useCompactCards ? (
                <CompactNewsCard
                  key={news.id}
                  news={news}
                  isFavorite={isFavorite(news.id)}
                  countryName={getCountryName(news.pais_id)}
                  onToggleFavorite={toggleFavorite}
                  onAddToList={handleAddToList}
                  onClick={() => setExpandedNews(news)}
                />
              ) : (
                <NewsCard
                  key={news.id}
                  news={news}
                  isFavorite={isFavorite(news.id)}
                  countryName={getCountryName(news.pais_id)}
                  onToggleFavorite={toggleFavorite}
                  onAddToList={handleAddToList}
                />
              )
            )}
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

      {/* Expanded News Modal for hover/touch */}
      <ExpandedNewsModal
        news={expandedNews}
        isFavorite={expandedNews ? isFavorite(expandedNews.id) : false}
        countryName={expandedNews ? getCountryName(expandedNews.pais_id) : undefined}
        onClose={() => setExpandedNews(null)}
        onToggleFavorite={toggleFavorite}
        onAddToList={handleAddToList}
      />

      {/* Save Search Modal */}
      <SaveSearchModal
        isOpen={showSaveModal}
        onClose={() => setShowSaveModal(false)}
        onSave={(label) => {
          const params: Record<string, any> = {}
          if (q) params.q = q
          if (categoryId) params.categoria = categoryId
          if (countryId) params.pais = countryId
          if (translatedOnly) params.lang = 'es'
          createSearch('news', label, params)
          setShowSaveModal(false)
        }}
        typeLabel="búsqueda"
      />
    </div>
  )
}
