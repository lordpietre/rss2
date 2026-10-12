import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { apiService, api, News } from '../services/api'
import { Search as SearchIcon, Bookmark } from 'lucide-react'
import { SkeletonNewsList, NoResultsFound, NoSearchResults } from '../components/ui'
import { SaveSearchModal } from '../components/SaveSearchModal'
import { NewsCard } from '../components/ui/NewsCard'
import { CreateListModal } from '../components/CreateListModal'
import { useApiFavorites, useApiLists, useApiSavedSearches } from '../hooks/useApiFavorites'

export function Search() {
  const [searchParams, setSearchParams] = useSearchParams()
  const q = searchParams.get('q') || ''
  const entity = searchParams.get('entity') || ''
  const tipo = searchParams.get('tipo') || ''
  const lang = searchParams.get('lang') || ''
  const categoria = searchParams.get('categoria_id') || ''
  const pais = searchParams.get('pais_id') || ''
  const page = parseInt(searchParams.get('page') || '1')
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [showSaveModal, setShowSaveModal] = useState(false)
  const [showListModal, setShowListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)
  const { favorites, toggleFavorite, isFavorite } = useApiFavorites()
  const { lists, createList, addToList } = useApiLists()
  const { createSearch } = useApiSavedSearches()

  const hasFilters = !!(q || lang || categoria || pais || entity)

  useEffect(() => {
    if (!localStorage.getItem('token')) return
    api.get('/search/suggestions')
      .then((res) => setSuggestions(res.data.terms || []))
      .catch(() => {})
  }, [])

  const { data: categorias } = useQuery({
    queryKey: ['categories'],
    queryFn: apiService.getCategories,
  })

  const { data: paises } = useQuery({
    queryKey: ['countries'],
    queryFn: apiService.getCountries,
  })

  // Entity search query (when clicking on entity from Trends)
  const { data: entityData, isLoading: entityLoading } = useQuery({
    queryKey: ['entity-news', entity, tipo, page],
    queryFn: () => apiService.getEntityNews({
      valor: entity,
      tipo: tipo || undefined,
      page,
      per_page: 30,
    }),
    enabled: !!entity,
  })

  // Keyword search query
  const { data: searchData, isLoading: searchLoading } = useQuery({
    queryKey: ['search', q, lang, categoria, pais, page],
    queryFn: () => apiService.search({
      q,
      lang: lang || undefined,
      categoria_id: categoria || undefined,
      pais_id: pais || undefined,
      page,
    }),
    enabled: !!q && !entity,
  })

  const data = entity ? entityData : searchData
  const isLoading = entity ? entityLoading : searchLoading

  const handleSearch = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    const newParams: Record<string, string> = {}
    if (formData.get('q')) {
      newParams.q = formData.get('q') as string
      if (localStorage.getItem('token')) {
        api.post('/searchlog', { q: formData.get('q') }).catch(() => {})
      }
    }
    if (formData.get('lang')) newParams.lang = formData.get('lang') as string
    if (formData.get('categoria_id')) newParams.categoria_id = formData.get('categoria_id') as string
    if (formData.get('pais_id')) newParams.pais_id = formData.get('pais_id') as string
    newParams.page = '1'
    setSearchParams(newParams)
  }

  const clearFilters = () => {
    const q = searchParams.get('q') || ''
    setSearchParams(q ? { q } : {})
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">News Intelligence</h1>
          <p className="text-sm text-gray-400 mt-1">Search and explore news across all sources</p>
        </div>
      </div>

      <form onSubmit={handleSearch} className="panel">
        <div className="flex flex-wrap gap-4 items-end">
          <div className="flex-1 min-w-[200px]">
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Search</label>
            <input
              name="q"
              defaultValue={q}
              placeholder="Keywords..."
              list="user-search-suggestions"
              className="input w-full"
            />
            <datalist id="user-search-suggestions">
              {suggestions.map((s) => <option key={s} value={s} />)}
            </datalist>
          </div>
          
          <div className="w-32">
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Language</label>
            <select name="lang" defaultValue={lang} className="input w-full">
              <option value="">All</option>
              <option value="es">Español</option>
              <option value="en">English</option>
              <option value="fr">Français</option>
              <option value="pt">Português</option>
              <option value="de">Deutsch</option>
              <option value="it">Italiano</option>
              <option value="ru">Русский</option>
              <option value="zh">中文</option>
              <option value="ja">日本語</option>
              <option value="ar">العربية</option>
            </select>
          </div>

          <div className="w-40">
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Category</label>
            <select name="categoria_id" defaultValue={categoria} className="input w-full">
              <option value="">All</option>
              {categorias?.map((cat) => (
                <option key={cat.id} value={cat.id}>{cat.nombre}</option>
              ))}
            </select>
          </div>

          <div className="w-40">
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Country</label>
            <select name="pais_id" defaultValue={pais} className="input w-full">
              <option value="">All</option>
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
              Clear
            </button>
          )}
          {hasFilters && (
            <button
              type="button"
              onClick={() => setShowSaveModal(true)}
              className="btn-secondary"
              title="Save this search"
            >
              <Bookmark className="h-5 w-5" />
            </button>
          )}
        </div>
      </form>

      {data && data.total > 0 && (
        <p className="text-sm text-gray-400">{data.total.toLocaleString()} results found</p>
      )}

      {isLoading ? (
        <SkeletonNewsList count={6} />
      ) : data?.news?.length === 0 ? (
        <NoResultsFound
          query={q}
          onClearFilters={clearFilters}
        />
      ) : data?.news && data.news.length > 0 ? (
        <div className="grid gap-6 md:grid-cols-2">
          {data.news.map((news: News) => (
            <NewsCard
              key={news.id}
              news={news}
              isFavorite={isFavorite(news.id)}
              onToggleFavorite={toggleFavorite}
              onAddToList={handleAddToList}
            />
          ))}
        </div>
      ) : (
        <NoSearchResults query={q} onSearch={clearFilters} />
      )}

      {/* Pagination */}
      {data && data.total_pages > 1 && (
        <div className="flex justify-center gap-2 mt-8">
          <button
            onClick={() => setSearchParams({ ...Object.fromEntries(searchParams), page: String(page - 1) })}
            disabled={page <= 1}
            className="btn-secondary disabled:opacity-50"
          >
            Previous
          </button>
          <span className="flex items-center px-4 text-gray-400 font-mono text-sm">
            {page} / {data.total_pages}
          </span>
          <button
            onClick={() => setSearchParams({ ...Object.fromEntries(searchParams), page: String(page + 1) })}
            disabled={page >= data.total_pages}
            className="btn-secondary disabled:opacity-50"
          >
            Next
          </button>
        </div>
      )}

      <SaveSearchModal
        isOpen={showSaveModal}
        onClose={() => setShowSaveModal(false)}
        onSave={(label) => {
          const params: Record<string, string> = {}
          if (q) params.q = q
          if (lang) params.lang = lang
          if (categoria) params.categoria = categoria
          if (pais) params.pais = pais
          createSearch('news', label, params)
        }}
        typeLabel="news search"
      />

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
