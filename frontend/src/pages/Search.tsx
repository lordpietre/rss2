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
  const lang = searchParams.get('lang') || ''
  const categoria = searchParams.get('categoria') || ''
  const pais = searchParams.get('pais') || ''
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [showSaveModal, setShowSaveModal] = useState(false)
  const [showListModal, setShowListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)
  const { favorites, toggleFavorite, isFavorite } = useApiFavorites()
  const { lists, createList, addToList } = useApiLists()
  const { createSearch } = useApiSavedSearches()

  const hasFilters = !!(q || lang || categoria || pais)

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
    if (formData.get('q')) {
      newParams.q = formData.get('q') as string
      if (localStorage.getItem('token')) {
        api.post('/searchlog', { q: formData.get('q') }).catch(() => {})
      }
    }
    if (formData.get('lang')) newParams.lang = formData.get('lang') as string
    if (formData.get('categoria')) newParams.categoria = formData.get('categoria') as string
    if (formData.get('pais')) newParams.pais = formData.get('pais') as string
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
              list="user-search-suggestions"
              className="input w-full"
            />
            <datalist id="user-search-suggestions">
              {suggestions.map((s) => <option key={s} value={s} />)}
            </datalist>
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
          {hasFilters && (
            <button
              type="button"
              onClick={() => setShowSaveModal(true)}
              className="btn-secondary"
              title="Guardar esta búsqueda"
            >
              <Bookmark className="h-5 w-5" />
            </button>
          )}
        </div>
      </form>

      {data && data.total > 0 && (
        <p className="text-gray-600 mb-4">{data.total} resultados encontrados</p>
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
        typeLabel="búsqueda de noticias"
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
