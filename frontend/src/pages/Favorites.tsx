import { useState, useEffect } from 'react'
import { News } from '../services/api'
import { Heart, Trash2, ExternalLink, Calendar, Newspaper, Search, Play, Edit2, Check, X, FolderPlus, Folder, Plus, Minus, Globe, Lightbulb, Sparkles } from 'lucide-react'
import { useNavigate, Link } from 'react-router-dom'
import { CreateListModal } from '../components/CreateListModal'
import { NewsCard } from '../components/ui/NewsCard'
import { TagCloud } from '../components/ui/TagCloud'
import { useApiFavorites, useApiLists, useApiSavedSearches } from '../hooks/useApiFavorites'

type Tab = 'news' | 'lists' | 'searches'

const TIPO_LABELS: Record<string, string> = {
  persona: '👤 Persona',
  organizacion: '🏢 Organización',
  lugar: '📍 Lugar',
  tema: '📰 Tema',
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleDateString('es', { day: '2-digit', month: 'short', year: 'numeric' })
}

// --- SavedSearchCard ---
function SavedSearchCard({ search, onRemove, onUpdateLabel }: {
  search: { id: number; type: string; label: string; params: Record<string, any>; created_at: string }
  onRemove: (id: number) => void
  onUpdateLabel: (id: number, label: string) => void
}) {
  const navigate = useNavigate()
  const [editing, setEditing] = useState(false)
  const [editLabel, setEditLabel] = useState(search.label)

  const isNews = search.type === 'news'
  const tipoIcon = isNews ? '📰' : TIPO_LABELS[search.params.tipo as string] || '📰'

  const handleExecute = () => {
    if (isNews) {
      const params = new URLSearchParams()
      if (search.params.q) params.set('q', String(search.params.q))
      if (search.params.lang) params.set('lang', String(search.params.lang))
      if (search.params.categoria) params.set('categoria', String(search.params.categoria))
      if (search.params.pais) params.set('pais', String(search.params.pais))
      navigate(`/search?${params.toString()}`)
    } else {
      const params = new URLSearchParams()
      params.set('tipo', String(search.params.tipo || 'persona'))
      if (search.params.countryId) params.set('countryId', String(search.params.countryId))
      if (search.params.categoryId) params.set('categoryId', String(search.params.categoryId))
      if (search.params.q) params.set('q', String(search.params.q))
      navigate(`/populares?${params.toString()}`)
    }
  }

  const handleSaveEdit = () => {
    if (editLabel.trim()) onUpdateLabel(search.id, editLabel.trim())
    setEditing(false)
  }

  const paramsDisplay = isNews
    ? Object.entries(search.params).filter(([, v]) => v).map(([k, v]) => `${k}: ${v}`).join(' • ')
    : `${search.params.tipo || ''} ${search.params.q || ''}`.trim()

  return (
    <div className="card p-4 hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-lg">{tipoIcon}</span>
            {editing ? (
              <div className="flex items-center gap-1 flex-1">
                <input type="text" value={editLabel} onChange={e => setEditLabel(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter') handleSaveEdit(); if (e.key === 'Escape') setEditing(false) }}
                  className="input py-1 px-2 text-sm flex-1" autoFocus />
                <button onClick={handleSaveEdit} className="p-1 text-green-600"><Check className="h-4 w-4" /></button>
                <button onClick={() => setEditing(false)} className="p-1 text-gray-500"><X className="h-4 w-4" /></button>
              </div>
            ) : (
              <>
                <span className="font-semibold text-gray-900">{search.label}</span>
                <button onClick={() => setEditing(true)} className="p-1 text-gray-400 hover:text-gray-600" title="Editar">
                  <Edit2 className="h-3 w-3" />
                </button>
              </>
            )}
          </div>
          {paramsDisplay && <p className="text-xs text-gray-500 mb-2">{paramsDisplay}</p>}
          <div className="flex items-center gap-3 mt-2">
            <button onClick={handleExecute} className="inline-flex items-center gap-1 text-sm text-primary-600 hover:text-primary-700 font-medium">
              <Play className="h-4 w-4" /> Ejecutar
            </button>
            <span className="text-xs text-gray-400">Guardado {formatDate(search.created_at)}</span>
          </div>
        </div>
        <button onClick={() => onRemove(search.id)} className="text-red-500 hover:text-red-600 p-1" title="Eliminar">
          <Trash2 className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}

// --- FavoriteCard (simple news card for favorites tab) ---
function FavoriteCard({ news, onRemove, onAddToList }: {
  news: News
  onRemove: (id: string) => void
  onAddToList: (news: News) => void
}) {
  return (
    <div className="card p-6 hover:shadow-md transition-shadow">
      <div className="flex gap-4">
        {news.imagen_url && (
          <img src={news.imagen_url} alt={news.titulo} className="w-32 h-24 object-cover rounded-lg flex-shrink-0" />
        )}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 line-clamp-2">
                {news.title_translated || news.titulo}
              </h3>
              <p className="text-sm text-gray-600 mt-1 line-clamp-2">
                {news.summary_translated || news.resumen}
              </p>
            </div>
            <div className="flex items-center gap-1">
              <button onClick={() => onAddToList(news)} className="text-blue-500 hover:text-blue-600 p-1" title="Añadir a lista">
                <FolderPlus className="h-5 w-5" />
              </button>
              <button onClick={() => onRemove(news.id)} className="text-red-500 hover:text-red-600 p-1" title="Quitar de favoritos">
                <Heart className="h-5 w-5 fill-current" />
              </button>
            </div>
          </div>
          <div className="flex items-center gap-4 mt-3 text-sm text-gray-500">
            <span className="flex items-center gap-1"><Newspaper className="h-4 w-4" />{news.fuente_nombre}</span>
            <span className="flex items-center gap-1"><Calendar className="h-4 w-4" />{formatDate(news.fecha)}</span>
          </div>
          <a href={news.url} target="_blank" rel="noopener noreferrer"
            className="text-primary-600 hover:underline text-sm mt-2 inline-flex items-center gap-1">
            <ExternalLink className="h-3 w-3" /> Ver original
          </a>
        </div>
      </div>
    </div>
  )
}

// --- ListCard ---
function ListCard({ list, onOpen, onDelete }: {
  list: { id: number; name: string; keywords?: string; item_count: number }
  onOpen: (id: number, name: string, keywords?: string) => void
  onDelete: (id: number) => void
}) {
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState(list.name)

  return (
    <div className="card p-4 hover:shadow-md transition-shadow">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <Folder className="h-6 w-6 text-blue-500 flex-shrink-0" />
          {editing ? (
            <div className="flex items-center gap-1 flex-1">
              <input type="text" value={editName} onChange={e => setEditName(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') { onOpen(list.id, editName, list.keywords); setEditing(false) }
                  if (e.key === 'Escape') setEditing(false)
                }}
                className="input py-1 px-2 text-sm flex-1" autoFocus />
              <button onClick={() => { onOpen(list.id, editName, list.keywords); setEditing(false) }} className="p-1 text-green-600"><Check className="h-4 w-4" /></button>
              <button onClick={() => setEditing(false)} className="p-1 text-gray-500"><X className="h-4 w-4" /></button>
            </div>
          ) : (
            <button onClick={() => onOpen(list.id, list.name, list.keywords)} className="flex-1 text-left">
              <span className="font-semibold text-gray-900 hover:text-primary-600">{list.name}</span>
              <span className="text-sm text-gray-500 ml-2">{list.item_count} noticias</span>
              {list.keywords && (
                <span className="ml-2 text-xs text-primary-600 bg-primary-50 px-2 py-0.5 rounded-full">
                  ✨
                </span>
              )}
            </button>
          )}
        </div>
        <div className="flex items-center gap-1">
          <button onClick={() => setEditing(true)} className="p-1 text-gray-400 hover:text-gray-600" title="Renombrar">
            <Edit2 className="h-4 w-4" />
          </button>
          <button onClick={() => onDelete(list.id)} className="p-1 text-red-500 hover:text-red-600" title="Eliminar lista">
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>
      {list.keywords && !editing && (
        <p className="text-xs text-gray-500 mt-2 truncate">{list.keywords}</p>
      )}
    </div>
  )
}

// --- ListDetailView ---
interface ListTag {
  valor: string
  tipo: string
  count: number
}

function ListDetailView({ 
  listId, 
  listName, 
  keywords,
  news, 
  onClose, 
  onRemoveNews, 
  onRename,
  onUpdateKeywords,
  tags = [],
  relatedNews = [],
  onAddRelatedNews,
}: {
  listId: number
  listName: string
  keywords: string
  news: News[]
  onClose: () => void
  onRemoveNews: (newsId: string) => void
  onRename: (name: string) => void
  onUpdateKeywords: (keywords: string) => void
  tags?: ListTag[]
  relatedNews?: News[]
  onAddRelatedNews?: (news: News) => void
}) {
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState(listName)
  const [editingKeywords, setEditingKeywords] = useState(false)
  const [editKeywords, setEditKeywords] = useState(keywords)
  const [showRelated, setShowRelated] = useState(false)

  const handleSaveKeywords = () => {
    onUpdateKeywords(editKeywords)
    setEditingKeywords(false)
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <button onClick={onClose} className="btn-secondary">← Volver</button>
          <Folder className="h-6 w-6 text-blue-500" />
          {editing ? (
            <div className="flex items-center gap-2">
              <input type="text" value={editName} onChange={e => setEditName(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') { onRename(editName); setEditing(false) }; if (e.key === 'Escape') setEditing(false) }}
                className="input py-1 px-2 text-lg font-bold" autoFocus />
              <button onClick={() => { onRename(editName); setEditing(false) }} className="p-1 text-green-600"><Check className="h-5 w-5" /></button>
              <button onClick={() => setEditing(false)} className="p-1 text-gray-500"><X className="h-5 w-5" /></button>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-bold text-gray-900">{listName}</h1>
              <button onClick={() => setEditing(true)} className="p-1 text-gray-400 hover:text-gray-600">
                <Edit2 className="h-4 w-4" />
              </button>
            </div>
          )}
        </div>
        <span className="text-gray-500">{news.length} noticias</span>
      </div>

      {/* Keywords Section */}
      <div className="card p-4 mb-6 bg-gradient-to-r from-primary-50 to-blue-50">
        <div className="flex items-center gap-2 mb-3">
          <Sparkles className="h-5 w-5 text-primary-600" />
          <span className="font-semibold text-gray-900">Palabras clave</span>
          {editingKeywords ? (
            <div className="flex items-center gap-2 ml-auto">
              <input
                type="text"
                value={editKeywords}
                onChange={e => setEditKeywords(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') handleSaveKeywords(); if (e.key === 'Escape') setEditingKeywords(false) }}
                placeholder=" palabra1, palabra2, palabra3"
                className="input py-1 px-2 text-sm flex-1"
                autoFocus
              />
              <button onClick={handleSaveKeywords} className="p-1 text-green-600"><Check className="h-4 w-4" /></button>
              <button onClick={() => setEditingKeywords(false)} className="p-1 text-gray-500"><X className="h-4 w-4" /></button>
            </div>
          ) : (
            <>
              <button onClick={() => { setEditKeywords(keywords); setEditingKeywords(true) }} className="ml-auto p-1 text-gray-400 hover:text-gray-600">
                <Edit2 className="h-4 w-4" />
              </button>
            </>
          )}
        </div>
        {keywords ? (
          <p className="text-sm text-gray-600">{keywords}</p>
        ) : (
          <p className="text-sm text-gray-400 italic">No hay palabras clave configuradas</p>
        )}
        <p className="text-xs text-gray-500 mt-1">El sistema busca noticias relacionadas automáticamente</p>
      </div>

      {/* Tags Cloud Section */}
      {tags.length > 0 && (
        <div className="card p-4 mb-6">
          <h3 className="font-semibold text-gray-900 mb-3 flex items-center gap-2">
            <span>🏷️</span> Etiquetas de la lista
          </h3>
          <TagCloud tags={tags} />
        </div>
      )}

      {/* Related News Section */}
      {relatedNews.length > 0 && (
        <div className="card p-4 mb-6 border-2 border-dashed border-primary-200 bg-primary-50/30">
          <button
            onClick={() => setShowRelated(!showRelated)}
            className="flex items-center gap-2 w-full text-left"
          >
            <Lightbulb className="h-5 w-5 text-primary-600" />
            <span className="font-semibold text-gray-900">
              Noticias relacionadas encontradas ({relatedNews.length})
            </span>
            <span className="ml-auto text-gray-400">{showRelated ? '▲' : '▼'}</span>
          </button>
          
          {showRelated && (
            <div className="mt-4 space-y-3">
              {relatedNews.map(n => (
                <div key={n.id} className="flex gap-3 p-2 bg-white rounded-lg hover:bg-gray-50">
                  {n.imagen_url && (
                    <img src={n.imagen_url} alt={n.titulo} className="w-16 h-16 object-cover rounded flex-shrink-0" />
                  )}
                  <div className="flex-1 min-w-0">
                    <Link to={`/news/${n.id}`} className="text-sm font-medium text-gray-900 hover:text-primary-600 line-clamp-2">
                      {n.title_translated || n.titulo}
                    </Link>
                    <p className="text-xs text-gray-500 mt-0.5">{n.fuente_nombre}</p>
                    {onAddRelatedNews && (
                      <button
                        onClick={() => onAddRelatedNews(n)}
                        className="mt-1 text-xs text-primary-600 hover:text-primary-700 flex items-center gap-1"
                      >
                        <Plus className="h-3 w-3" /> Añadir a lista
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* News Grid */}
      {news.length === 0 ? (
        <div className="card p-12 text-center">
          <Folder className="h-16 w-16 text-gray-300 mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-gray-700 mb-2">Lista vacía</h2>
          <p className="text-gray-500">Añade noticias desde el carrusel de noticias</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {news.map(n => (
            <div key={n.id} className="relative">
              <Link to={`/news/${n.id}`} className="card hover:shadow-md transition-shadow block">
                {n.imagen_url && (
                  <img src={n.imagen_url} alt={n.titulo} className="w-full h-36 object-cover rounded-t-xl" />
                )}
                <div className="p-3">
                  <div className="flex items-center gap-2 text-xs text-gray-500 mb-1">
                    <Newspaper className="h-3 w-3" />
                    <span className="truncate">{n.fuente_nombre}</span>
                    {n.title_translated && (
                      <span className="flex items-center gap-1 bg-primary-100 text-primary-700 px-1.5 py-0.5 rounded text-xs">
                        <Globe className="h-3 w-3" />ES
                      </span>
                    )}
                  </div>
                  <h3 className="text-sm font-semibold text-gray-900 line-clamp-2 mb-1">
                    {n.title_translated || n.titulo}
                  </h3>
                  <p className="text-xs text-gray-600 line-clamp-2">
                    {n.summary_translated || n.resumen}
                  </p>
                </div>
              </Link>
              <button
                onClick={() => onRemoveNews(n.id)}
                className="absolute top-2 right-2 bg-white/90 hover:bg-red-50 text-red-500 p-1.5 rounded-full shadow"
                title="Quitar de lista"
              >
                <Minus className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// --- Main Favorites component ---
export function Favorites() {
  const [tab, setTab] = useState<Tab>('news')
  const [showCreateListModal, setShowCreateListModal] = useState(false)
  const [showAddToListModal, setShowAddToListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)
  const [activeListId, setActiveListId] = useState<number | null>(null)
  const [activeListName, setActiveListName] = useState('')
  const [activeListKeywords, setActiveListKeywords] = useState('')
  const [activeListNews, setActiveListNews] = useState<News[]>([])
  const [activeListTags, setActiveListTags] = useState<ListTag[]>([])
  const [activeListRelatedNews, setActiveListRelatedNews] = useState<News[]>([])

  const { favorites, removeFavorite, refresh: refreshFavorites } = useApiFavorites()
  const { lists, createList, deleteList, updateList, addToList, removeFromList, getListItems, getListTags, getListRelatedNews, refresh: refreshLists } = useApiLists()
  const { searches, deleteSearch, updateSearch, refresh: refreshSearches } = useApiSavedSearches()

  const handleAddToList = (news: News) => {
    setSelectedNewsForList(news)
    setShowAddToListModal(true)
  }

  const handleSelectList = async (listId: number) => {
    if (selectedNewsForList) {
      await addToList(listId, selectedNewsForList.id)
      setShowAddToListModal(false)
      setSelectedNewsForList(null)
    }
  }

  const handleCreateAndAdd = async (name: string, keywords?: string) => {
    const newListId = await createList(name, keywords)
    if (selectedNewsForList) {
      await addToList(newListId, selectedNewsForList.id)
    }
    setShowAddToListModal(false)
    setSelectedNewsForList(null)
  }

  const handleOpenList = async (id: number, name: string, keywords: string = '') => {
    setActiveListId(id)
    setActiveListName(name)
    setActiveListKeywords(keywords)
    const items = await getListItems(id)
    setActiveListNews(items)
    
    // Load tags and related news
    const [tags, related] = await Promise.all([
      getListTags(id),
      getListRelatedNews(id),
    ])
    setActiveListTags(tags)
    setActiveListRelatedNews(related)
  }

  const handleCloseList = () => {
    setActiveListId(null)
    setActiveListName('')
    setActiveListKeywords('')
    setActiveListNews([])
    setActiveListTags([])
    setActiveListRelatedNews([])
  }

  const handleRemoveNewsFromList = async (newsId: string) => {
    if (activeListId) {
      await removeFromList(activeListId, newsId)
      const items = await getListItems(activeListId)
      setActiveListNews(items)
    }
  }

  const handleRenameList = async (name: string) => {
    if (activeListId) {
      await updateList(activeListId, name)
      setActiveListName(name)
      await refreshLists()
    }
  }

  const handleUpdateKeywords = async (keywords: string) => {
    if (activeListId) {
      await updateList(activeListId, undefined, keywords)
      setActiveListKeywords(keywords)
      await refreshLists()
      
      // Reload related news with new keywords
      const related = await getListRelatedNews(activeListId)
      setActiveListRelatedNews(related)
    }
  }

  const handleAddRelatedNews = async (news: News) => {
    if (activeListId) {
      await addToList(activeListId, news.id)
      const items = await getListItems(activeListId)
      setActiveListNews(items)
    }
  }

  // If viewing a list detail
  if (activeListId !== null) {
    return (
      <ListDetailView
        listId={activeListId}
        listName={activeListName}
        keywords={activeListKeywords}
        news={activeListNews}
        onClose={handleCloseList}
        onRemoveNews={handleRemoveNewsFromList}
        onRename={handleRenameList}
        onUpdateKeywords={handleUpdateKeywords}
        tags={activeListTags}
        relatedNews={activeListRelatedNews}
        onAddRelatedNews={handleAddRelatedNews}
      />
    )
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold text-gray-900">Favoritos</h1>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 mb-6">
        <button onClick={() => setTab('news')}
          className={`px-4 py-2 font-medium border-b-2 transition-colors ${tab === 'news' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
          <span className="flex items-center gap-2">
            <Heart className="h-4 w-4" />
            Noticias guardadas
            <span className="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded-full">{favorites.length}</span>
          </span>
        </button>
        <button onClick={() => setTab('lists')}
          className={`px-4 py-2 font-medium border-b-2 transition-colors ${tab === 'lists' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
          <span className="flex items-center gap-2">
            <Folder className="h-4 w-4" />
            Listas
            <span className="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded-full">{lists.length}</span>
          </span>
        </button>
        <button onClick={() => setTab('searches')}
          className={`px-4 py-2 font-medium border-b-2 transition-colors ${tab === 'searches' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
          <span className="flex items-center gap-2">
            <Search className="h-4 w-4" />
            Búsquedas guardadas
            <span className="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded-full">{searches.length}</span>
          </span>
        </button>
      </div>

      {/* News Tab */}
      {tab === 'news' && (
        favorites.length === 0 ? (
          <div className="card p-12 text-center">
            <Heart className="h-16 w-16 text-gray-300 mx-auto mb-4" />
            <h2 className="text-xl font-semibold text-gray-700 mb-2">No hay noticias guardadas</h2>
            <p className="text-gray-500">Guarda noticias en favoritos haciendo clic en el corazón de cualquier noticia.</p>
          </div>
        ) : (
          <div className="space-y-4">
            {favorites.map(news => (
              <FavoriteCard key={news.id} news={news} onRemove={removeFavorite} onAddToList={handleAddToList} />
            ))}
          </div>
        )
      )}

      {/* Lists Tab */}
      {tab === 'lists' && (
        <>
          <div className="flex justify-end mb-4">
            <button onClick={() => setShowCreateListModal(true)} className="btn-primary flex items-center gap-2">
              <Plus className="h-4 w-4" />
              Nueva lista
            </button>
          </div>
          {lists.length === 0 ? (
            <div className="card p-12 text-center">
              <Folder className="h-16 w-16 text-gray-300 mx-auto mb-4" />
              <h2 className="text-xl font-semibold text-gray-700 mb-2">No hay listas creadas</h2>
              <p className="text-gray-500">Crea listas para agrupar noticias por tema.</p>
            </div>
          ) : (
            <div className="space-y-4">
              {lists.map(list => (
                <ListCard key={list.id} list={list} onOpen={handleOpenList} onDelete={deleteList} />
              ))}
            </div>
          )}
        </>
      )}

      {/* Searches Tab */}
      {tab === 'searches' && (
        searches.length === 0 ? (
          <div className="card p-12 text-center">
            <Search className="h-16 w-16 text-gray-300 mx-auto mb-4" />
            <h2 className="text-xl font-semibold text-gray-700 mb-2">No hay búsquedas guardadas</h2>
            <p className="text-gray-500">Guarda tus búsquedas haciendo clic en el botón "💾" en Search o Populares.</p>
          </div>
        ) : (
          <div className="space-y-4">
            {searches.map(search => (
              <SavedSearchCard key={search.id} search={search} onRemove={deleteSearch} onUpdateLabel={updateSearch} />
            ))}
          </div>
        )
      )}

      {/* Create List Modal */}
      <CreateListModal
        isOpen={showCreateListModal}
        onClose={() => setShowCreateListModal(false)}
        onCreate={async (name: string, keywords?: string) => { await createList(name, keywords) }}
        mode="create"
      />

      {/* Add to List Modal */}
      <CreateListModal
        isOpen={showAddToListModal}
        onClose={() => { setShowAddToListModal(false); setSelectedNewsForList(null) }}
        onCreate={handleCreateAndAdd}
        mode="add-to"
        existingLists={lists.map((l: { id: number; name: string }) => ({ id: l.id, name: l.name }))}
        onSelectList={handleSelectList}
      />
    </div>
  )
}
