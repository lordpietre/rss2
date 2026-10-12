import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../services/api'
import { WikiTooltip } from '../components/ui/WikiTooltip'
import { SaveSearchModal } from '../components/SaveSearchModal'
import { useApiSavedSearches } from '../hooks/useApiFavorites'
import { Bookmark, Network, ChevronDown, ChevronUp } from 'lucide-react'

interface Entity {
  valor: string
  tipo: string
  count: number
  wiki_summary?: string
  wiki_url?: string
  image_path?: string
  last_date?: string
}

interface Category {
  id: number
  nombre: string
}

interface Country {
  id: number
  nombre: string
}

interface ConfigModal {
  entity: Entity
}

const TIPOS = [
  { value: 'persona', label: '👤 Person', color: 'bg-blue-900/30 text-blue-400 border border-blue-500/30' },
  { value: 'organizacion', label: '🏢 Organization', color: 'bg-purple-900/30 text-purple-400 border border-purple-500/30' },
  { value: 'lugar', label: '📍 Place', color: 'bg-green-900/30 text-green-400 border border-green-500/30' },
  { value: 'tema', label: '📰 Topic', color: 'bg-amber-900/30 text-amber-400 border border-amber-500/30' },
]

const TIMELINE_WINDOWS: { key: string; label: string; title: string; days: number }[] = [
  { key: '1', label: '24h', title: 'last 24 hours', days: 1 },
  { key: '2', label: '2d', title: 'last 2 days', days: 2 },
  { key: '3', label: '3d', title: 'last 3 days', days: 3 },
  { key: '7', label: '7d', title: 'last 7 days', days: 7 },
]

function getTipoInfo(tipo: string) {
  return TIPOS.find(t => t.value === tipo) || TIPOS[3]
}

export function Populares() {
  const [entities, setEntities] = useState<Entity[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [countries, setCountries] = useState<Country[]>([])
  const [tipo, setTipo] = useState('persona')
  const [countryId, setCountryId] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const [sortBy, setSortBy] = useState('count')
  const [activeSearch, setActiveSearch] = useState('')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(false)
  const [configModal, setConfigModal] = useState<ConfigModal | null>(null)
  const [saving, setSaving] = useState(false)
  const [successMsg, setSuccessMsg] = useState('')
  const [expandedEntities, setExpandedEntities] = useState<Set<string>>(new Set())
  const [allExpanded, setAllExpanded] = useState(false)

  const [newsModal, setNewsModal] = useState<Entity | null>(null)
  const [newsModalMode, setNewsModalMode] = useState<'news' | 'timeline'>('news')
  const [timelineLabel, setTimelineLabel] = useState('last 24 hours')
  const [entityNews, setEntityNews] = useState<any[]>([])
  const [newsLoading, setNewsLoading] = useState(false)
  const [newsTotal, setNewsTotal] = useState(0)
  const [newsError, setNewsError] = useState<string | null>(null)
  const newsAbortRef = useRef<AbortController | null>(null)

  const [newTipo, setNewTipo] = useState('')
  const [aliasInput, setAliasInput] = useState('')
  const [canonicalInput, setCanonicalInput] = useState('')
  const [activeTab, setActiveTab] = useState<'tipo' | 'alias'>('tipo')

  const fileInputRef = useRef<HTMLInputElement>(null)
  const [showSaveModal, setShowSaveModal] = useState(false)
  const { createSearch } = useApiSavedSearches()

  const hasEntityFilters = !!(countryId || categoryId || activeSearch)

  useEffect(() => {
    api.get('/categories').then(res => setCategories(res.data)).catch(console.error)
    api.get('/countries').then(res => setCountries(res.data)).catch(console.error)
  }, [])

  useEffect(() => {
    setPage(1)
  }, [tipo, countryId, categoryId, activeSearch, sortBy])

  useEffect(() => {
    fetchEntities()
  }, [tipo, countryId, categoryId, activeSearch, page, sortBy])

  const fetchEntities = async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams()
      params.append('tipo', tipo)
      if (countryId) params.append('country_id', countryId)
      if (categoryId) params.append('category_id', categoryId)
      if (activeSearch) params.append('q', activeSearch)
      params.append('sort_by', sortBy)
      params.append('page', page.toString())
      params.append('per_page', '50')
      const res = await api.get(`/entities?${params}`)
      setEntities(res.data.entities || [])
      setTotalPages(res.data.total_pages || 1)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const openConfig = (entity: Entity) => {
    setConfigModal({ entity })
    setNewTipo(entity.tipo)
    setAliasInput(entity.valor)
    setCanonicalInput(entity.valor)
    setActiveTab('tipo')
    setSuccessMsg('')
  }

  const closeConfig = () => {
    setConfigModal(null)
    setSaving(false)
    setSuccessMsg('')
  }

  const openNews = async (entity: Entity) => {
    // Cancel any pending request
    if (newsAbortRef.current) {
      newsAbortRef.current.abort()
    }
    newsAbortRef.current = new AbortController()
    
    setNewsModalMode('news')
    setNewsModal(entity)
    setNewsLoading(true)
    setEntityNews([])
    setNewsTotal(0)
    setNewsError(null)
    
    const timeout = setTimeout(() => {
      if (newsAbortRef.current) {
        newsAbortRef.current.abort()
        setNewsError('Request timed out. This entity may have too much data.')
        setNewsLoading(false)
      }
    }, 60000)
    
    try {
      const params = new URLSearchParams()
      params.append('valor', entity.valor)
      params.append('tipo', entity.tipo)
      params.append('per_page', '30')
      const res = await api.get(`/entities/news?${params}`, { signal: newsAbortRef.current.signal })
      clearTimeout(timeout)
      setEntityNews(res.data.news || [])
      setNewsTotal(res.data.total || 0)
    } catch (err: any) {
      clearTimeout(timeout)
      if (err.name === 'AbortError' || err.name === 'CanceledError') {
        return
      }
      console.error(err)
      setNewsError('Failed to load news. Please try again.')
    } finally {
      setNewsLoading(false)
    }
  }

  const openTimeline = async (entity: Entity, win: typeof TIMELINE_WINDOWS[number]) => {
    // Cancel any pending request
    if (newsAbortRef.current) {
      newsAbortRef.current.abort()
    }
    newsAbortRef.current = new AbortController()
    
    setTimelineLabel(win.title)
    setNewsModalMode('timeline')
    setNewsModal(entity)
    setNewsLoading(true)
    setEntityNews([])
    setNewsTotal(0)
    setNewsError(null)
    
    const timeout = setTimeout(() => {
      if (newsAbortRef.current) {
        newsAbortRef.current.abort()
        setNewsError('Request timed out. This entity may have too much data.')
        setNewsLoading(false)
      }
    }, 60000)
    
    try {
      const params = new URLSearchParams()
      params.append('valor', entity.valor)
      params.append('tipo', entity.tipo)
      params.append('days', String(win.days))
      params.append('per_page', '50')
      const res = await api.get(`/entities/news?${params}`, { signal: newsAbortRef.current.signal })
      clearTimeout(timeout)
      setEntityNews(res.data.news || [])
      setNewsTotal(res.data.total || 0)
    } catch (err: any) {
      clearTimeout(timeout)
      if (err.name === 'AbortError' || err.name === 'CanceledError') {
        return
      }
      console.error(err)
      setNewsError('Failed to load news. Please try again.')
    } finally {
      setNewsLoading(false)
    }
  }

  const handleSaveTipo = async () => {
    if (!configModal || newTipo === configModal.entity.tipo) return
    setSaving(true)
    try {
      await api.post('/admin/entities/retype', {
        valor: configModal.entity.valor,
        new_tipo: newTipo,
      })
      setSuccessMsg(`✅ Type changed to "${getTipoInfo(newTipo).label}"`)
      setConfigModal(null)
      fetchEntities()
    } catch (err: any) {
      setSuccessMsg(`❌ Error: ${err?.response?.data?.error || 'Unknown error'}`)
    } finally {
      setSaving(false)
    }
  }

  const handleSaveAlias = async () => {
    if (!canonicalInput.trim() || !aliasInput.trim()) return
    setSaving(true)
    const aliasesList = aliasInput.split(/[\n,]+/).map(a => a.trim()).filter(a => a.length > 0)
    if (aliasesList.length === 0) {
      setSaving(false)
      return
    }
    try {
      await api.post('/admin/aliases', {
        aliases: aliasesList,
        canonical_name: canonicalInput.trim(),
        tipo: configModal?.entity.tipo || newTipo,
      })
      setSuccessMsg(`✅ ${aliasesList.length} aliases created and merged into "${canonicalInput}"`)
      setAliasInput('')
      fetchEntities()
    } catch (err: any) {
      setSuccessMsg(`❌ Error: ${err?.response?.data?.error || 'Alias already exists'}`)
    } finally {
      setSaving(false)
    }
  }

  const handleExportAliases = async () => {
    try {
      const res = await api.get('/admin/aliases/export', { responseType: 'blob' })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', 'aliases.csv')
      document.body.appendChild(link)
      link.click()
      link.remove()
    } catch (err) { console.error(err) }
  }

  const handleImportAliases = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const formData = new FormData()
    formData.append('file', file)
    try {
      const res = await api.post('/admin/aliases/import', formData, { headers: { 'Content-Type': 'multipart/form-data' } })
      alert(`✅ Imported: ${res.data.inserted}, Skipped: ${res.data.skipped}`)
      e.target.value = ''
    } catch { alert('❌ Error importing CSV') }
  }

  const handleBackupDB = async () => {
    try {
      const res = await api.get('/admin/backup', { responseType: 'blob' })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const link = document.createElement('a')
      link.href = url
      const now = new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')
      link.setAttribute('download', `backup_${now}.sql`)
      document.body.appendChild(link)
      link.click()
      link.remove()
    } catch (err: any) {
      alert('❌ Error generating backup. Check that pg_dump is available.')
    }
  }

  const selectedCountry = countries.find(c => c.id.toString() === countryId)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">Entity Network</h1>
          <p className="text-sm text-gray-400 mt-1">Explore entities, people, places and organizations</p>
        </div>
      </div>

      {/* Header Actions */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
        <div className="flex gap-2">
          <button
            onClick={() => setAllExpanded(p => !p)}
            className="px-3 py-1.5 text-sm border border-gray-700 rounded hover:bg-gray-800 transition-colors"
          >
            {allExpanded ? <><ChevronUp className="w-4 h-4 inline" /> Collapse all</> : <><ChevronDown className="w-4 h-4 inline" /> Expand all</>}
          </button>
        </div>
        <div className="flex flex-wrap gap-2">
          <button onClick={handleExportAliases} className="px-3 py-1.5 text-sm border border-gray-700 rounded hover:bg-gray-800">
            📤 Export aliases
          </button>
          <label className="px-3 py-1.5 text-sm border border-gray-700 rounded hover:bg-gray-800 cursor-pointer">
            📥 Import aliases
            <input ref={fileInputRef} type="file" accept=".csv" onChange={handleImportAliases} className="hidden" />
          </label>
          <button onClick={handleBackupDB} className="px-3 py-1.5 text-sm bg-amber-600 hover:bg-amber-500 text-white rounded font-medium">
            💾 Backup DB
          </button>
          {hasEntityFilters && (
            <button onClick={() => setShowSaveModal(true)} className="px-3 py-1.5 text-sm border border-gray-700 rounded hover:bg-gray-800">
              <Bookmark className="h-4 w-4 inline mr-1" /> Save
            </button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="panel">
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Type</label>
            <select value={tipo} onChange={(e) => setTipo(e.target.value)} className="input">
              {TIPOS.map(t => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Country</label>
            <select value={countryId} onChange={(e) => setCountryId(e.target.value)} className="input">
              <option value="">🌍 Global</option>
              {countries.map(c => (
                <option key={c.id} value={c.id}>{c.nombre}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Category</label>
            <select value={categoryId} onChange={(e) => setCategoryId(e.target.value)} className="input">
              <option value="">All categories</option>
              {categories.map(c => (
                <option key={c.id} value={c.id}>{c.nombre}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Sort by</label>
            <select value={sortBy} onChange={(e) => setSortBy(e.target.value)} className="input">
              <option value="count">Most mentions</option>
              <option value="name">Name A-Z</option>
              <option value="recent">Most recent</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Search</label>
            <div className="flex gap-2">
              <input
                type="text"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && setActiveSearch(searchQuery)}
                placeholder="Term or alias..."
                className="input flex-1"
              />
              <button
                onClick={() => setActiveSearch(searchQuery)}
                className="btn-secondary px-3"
              >
                🔍
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Entity List */}
      {loading ? (
        <div className="panel p-16 flex justify-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-50" />
        </div>
      ) : entities.length === 0 ? (
        <div className="panel p-16 text-center text-gray-400">
          <Network className="w-12 h-12 mx-auto mb-4 text-gray-600" />
          No entities found
        </div>
      ) : (
        <div className="panel overflow-hidden">
          <div className="p-4 border-b border-gray-700">
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-400 font-mono">{entities.length} entities</span>
            </div>
          </div>
          <div className="p-4 space-y-2">
            {entities.map((entity) => {
              const isExpanded = allExpanded || expandedEntities.has(entity.valor)
              const tipoInfo = getTipoInfo(entity.tipo)
              return (
                <div key={entity.valor} className={`rounded-lg ${isExpanded ? 'bg-gray-800/50' : ''} transition-colors`}>
                  <div 
                    className="flex items-center justify-between px-2 py-2 cursor-pointer hover:bg-gray-800 transition-colors"
                    onClick={() => setExpandedEntities(prev => {
                      const next = new Set(prev)
                      if (next.has(entity.valor)) next.delete(entity.valor)
                      else next.add(entity.valor)
                      return next
                    })}
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      {entity.image_path && (
                        <img src={entity.image_path} alt={entity.valor} className="w-20 h-20 object-cover rounded-xl shrink-0 bg-gray-800 border border-gray-700" onError={(e) => e.currentTarget.style.display = 'none'} />
                      )}
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <WikiTooltip name={entity.valor} summary={entity.wiki_summary} imagePath={entity.image_path} wikiUrl={entity.wiki_url}>
                            <span className="font-semibold text-lg text-primary-50 hover:text-primary-300 transition-colors">{entity.valor}</span>
                          </WikiTooltip>
                          <span className="text-sm text-gray-500 shrink-0 font-mono">#{entity.count}</span>
                        </div>
                        {entity.wiki_summary && (
                          <p className="text-sm text-gray-400 mt-1 line-clamp-2 leading-relaxed">{entity.wiki_summary}</p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className={`inline-flex items-center px-3 py-1 rounded-lg text-sm font-semibold ${tipoInfo.color}`}>
                        {tipoInfo.label}
                      </span>
                      <span className="text-gray-500">{entity.count}</span>
                    </div>
                  </div>
                  {isExpanded && (
                    <div className="p-3 pt-0">
                      <div className="flex items-center gap-2 mb-2">
                        <button onClick={() => openNews(entity)} className="px-2 py-1 text-xs rounded border border-gray-700 hover:bg-gray-800 transition-colors">
                          🔍 View news
                        </button>
                        <button onClick={() => openConfig(entity)} className="px-2 py-1 text-xs rounded border border-gray-700 hover:bg-gray-800 transition-colors">
                          ⚙️ Configure
                        </button>
                      </div>
                      <div className="flex flex-wrap gap-1">
                        {TIMELINE_WINDOWS.map((win) => (
                          <button key={win.key} onClick={() => openTimeline(entity, win)} className="inline-flex items-center px-2 py-0.5 text-xs rounded bg-primary-900/30 text-primary-50 hover:bg-primary-900/50 border border-primary-500/30 transition-colors" title={win.title}>
                            {win.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Pagination */}
      {!loading && entities.length > 0 && totalPages > 1 && (
        <div className="flex items-center justify-between gap-4 bg-gray-900 p-4 rounded-lg border border-gray-700">
          <div className="text-gray-400 text-sm">
            Page <span className="font-semibold text-gray-50">{page}</span> of <span className="font-semibold text-gray-50">{totalPages}</span>
          </div>
          <div className="flex gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page <= 1} className="btn-secondary disabled:opacity-50">Previous</button>
            <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page >= totalPages} className="btn-secondary disabled:opacity-50">Next</button>
          </div>
        </div>
      )}

      {/* Configure Modal */}
      {configModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={closeConfig}>
          <div className="bg-gray-900 rounded-xl shadow-2xl w-full max-w-md border border-gray-700" onClick={e => e.stopPropagation()}>
            <div className="flex items-center gap-4 px-6 py-4 border-b border-gray-700">
              {configModal.entity.image_path && (
                <img src={configModal.entity.image_path} alt={configModal.entity.valor} className="w-24 h-24 object-cover rounded-xl border-2 border-gray-600" />
              )}
              <div>
                <h2 className="text-xl font-bold text-gray-50">{configModal.entity.valor}</h2>
                <p className="text-sm text-gray-400 mt-1">Configure entity</p>
              </div>
              <button onClick={closeConfig} className="text-gray-400 hover:text-gray-200 text-xl leading-none">✕</button>
            </div>
            <div className="flex border-b border-gray-700">
              <button onClick={() => { setActiveTab('tipo'); setSuccessMsg('') }} className={`flex-1 py-3 text-sm font-medium transition-colors ${activeTab === 'tipo' ? 'border-b-2 border-primary-50 text-primary-50' : 'text-gray-400 hover:text-gray-200'}`}>
                🔄 Change type
              </button>
              <button onClick={() => { setActiveTab('alias'); setSuccessMsg('') }} className={`flex-1 py-3 text-sm font-medium transition-colors ${activeTab === 'alias' ? 'border-b-2 border-primary-50 text-primary-50' : 'text-gray-400 hover:text-gray-200'}`}>
                🔁 Create alias
              </button>
            </div>
            <div className="p-6">
              {activeTab === 'tipo' && (
                <div>
                  <p className="text-sm text-gray-400 mb-4">
                    Move "<strong>{configModal.entity.valor}</strong>" to another category. Affects all tags with this value in the database.
                  </p>
                  <div className="grid grid-cols-2 gap-2 mb-4">
                    {TIPOS.map(t => (
                      <button key={t.value} onClick={() => setNewTipo(t.value)} className={`px-4 py-3 rounded-xl border-2 text-base font-semibold transition-all ${newTipo === t.value ? 'border-primary-50 bg-primary-900/30 text-primary-50' : 'border-gray-700 hover:border-gray-600'}`}>
                        {t.label}
                        {t.value === configModal.entity.tipo && <span className="ml-1 text-xs text-gray-500">(current)</span>}
                      </button>
                    ))}
                  </div>
                  {successMsg && <p className="text-sm mb-3 py-2 px-3 rounded bg-gray-800">{successMsg}</p>}
                  <button onClick={handleSaveTipo} disabled={saving || newTipo === configModal.entity.tipo} className="w-full py-2 bg-primary-600 hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg font-medium text-sm transition-colors">
                    {saving ? 'Saving...' : 'Change type'}
                  </button>
                </div>
              )}
              {activeTab === 'alias' && (
                <div>
                  <p className="text-sm text-gray-400 mb-4">
                    Create an alias to normalize name variants. The alias will map to the canonical name you define.
                  </p>
                  <div className="mb-3">
                    <label className="block text-sm font-medium mb-1">Aliases <span className="text-gray-500 font-normal">(separated by commas or newlines)</span></label>
                    <textarea value={aliasInput} onChange={e => setAliasInput(e.target.value)} className="input min-h-[80px] resize-y" placeholder="e.g.: donald trump, the president&#10;trump" />
                  </div>
                  <div className="mb-4">
                    <label className="block text-sm font-medium mb-1">Canonical name <span className="text-gray-500 font-normal">(correct normalized name)</span></label>
                    <input type="text" value={canonicalInput} onChange={e => setCanonicalInput(e.target.value)} className="input" placeholder="e.g.: Donald Trump" />
                  </div>
                  {successMsg && <p className="text-sm mb-3 py-2 px-3 rounded bg-gray-800">{successMsg}</p>}
                  <button onClick={handleSaveAlias} disabled={saving || !aliasInput.trim() || !canonicalInput.trim()} className="w-full py-2 bg-green-600 hover:bg-green-500 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg font-medium text-sm transition-colors">
                    {saving ? 'Creating...' : 'Create alias'}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* News Modal */}
      {newsModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setNewsModal(null)}>
          <div className="bg-gray-900 rounded-xl shadow-2xl w-full max-w-2xl max-h-[85vh] overflow-hidden flex flex-col border border-gray-700" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700">
              <div>
                <h2 className="text-lg font-bold text-gray-50">
                  {newsModalMode === 'timeline' ? <>📈 Timeline of &quot;{newsModal.valor}&quot;</> : <>News about &quot;{newsModal.valor}&quot;</>}
                </h2>
                <p className="text-sm text-gray-400 mt-0.5">
                  {newsModalMode === 'timeline' ? `Latest ${Math.min(entityNews.length, 50)} news · ${timelineLabel} (${getTipoInfo(newsModal.tipo).label})` : `${newsTotal} news found (${getTipoInfo(newsModal.tipo).label})`}
                </p>
              </div>
              <button onClick={() => setNewsModal(null)} className="text-gray-400 hover:text-gray-200 text-xl leading-none">✕</button>
            </div>
            <div className="p-4 overflow-y-auto">
              {newsError ? (
                <div className="text-center py-10">
                  <p className="text-rss-red mb-3">{newsError}</p>
                  <button 
                    onClick={() => newsModal && openNews(newsModal)} 
                    className="btn-secondary text-sm"
                  >
                    Retry
                  </button>
                </div>
              ) : newsLoading ? (
                <div className="text-center py-10 flex flex-col items-center">
                  <div className="w-8 h-8 border-2 border-gray-700 border-t-primary-50 rounded-full animate-spin mb-3" />
                  <span className="text-gray-400">Loading news...</span>
                </div>
              ) : entityNews.length === 0 ? (
                <div className="text-center py-10 text-gray-400">
                  {newsModalMode === 'timeline' ? `No news for this entity in ${timelineLabel}.` : 'No news mentions this entity.'}
                </div>
              ) : newsModalMode === 'timeline' ? (
                <ol className="relative border-l-2 border-primary-500/30 ml-2 space-y-4">
                  {entityNews.map((n) => (
                    <li key={n.id} className="ml-4 relative">
                      <span className="absolute -left-[21px] top-2 h-3 w-3 rounded-full bg-primary-500 ring-2 ring-primary-900" />
                      <div className="flex items-baseline gap-2">
                        {n.fecha && (
                          <span className="text-[11px] font-semibold text-primary-400 shrink-0 tabular-nums">
                            {new Date(n.fecha).toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })}
                          </span>
                        )}
                        {n.fecha && (
                          <span className="text-[10px] text-gray-500 shrink-0 tabular-nums">
                            {new Date(n.fecha).toLocaleDateString('es-ES', { day: '2-digit', month: '2-digit', year: '2-digit' })}
                          </span>
                        )}
                      </div>
                      <Link to={`/news/${n.id}`} onClick={() => setNewsModal(null)} className="block p-3 mt-1 rounded-lg hover:bg-gray-800 transition-colors border border-gray-700">
                        <div className="flex items-start gap-3">
                          {n.imagen_url && <img src={n.imagen_url} alt="" className="w-20 h-14 object-cover rounded-md shrink-0" onError={(e) => e.currentTarget.style.display = 'none'} />}
                          <div className="min-w-0">
                            <div className="font-medium text-sm text-gray-50 line-clamp-2">{n.title_translated || n.titulo}</div>
                            <p className="text-xs text-gray-400 mt-1 line-clamp-2">{n.summary_translated || n.resumen}</p>
                          </div>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ol>
              ) : (
                <ul className="space-y-2">
                  {entityNews.map((n) => (
                    <li key={n.id}>
                      <Link to={`/news/${n.id}`} onClick={() => setNewsModal(null)} className="block p-3 rounded-lg hover:bg-gray-800 transition-colors border border-gray-700">
                        <div className="flex items-start gap-3">
                          {n.imagen_url && <img src={n.imagen_url} alt="" className="w-20 h-14 object-cover rounded-md shrink-0" onError={(e) => e.currentTarget.style.display = 'none'} />}
                          <div className="min-w-0">
                            <div className="font-medium text-sm text-gray-50 line-clamp-2">{n.title_translated || n.titulo}</div>
                            <p className="text-xs text-gray-400 mt-1 line-clamp-2">{n.summary_translated || n.resumen}</p>
                            {n.fecha && <span className="text-[11px] text-gray-500 mt-1 block font-mono">{new Date(n.fecha).toLocaleDateString('es-ES')}</span>}
                          </div>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>
      )}

      <SaveSearchModal
        isOpen={showSaveModal}
        onClose={() => setShowSaveModal(false)}
        onSave={(label) => {
          const params: Record<string, string> = { tipo }
          if (countryId) params.countryId = countryId
          if (categoryId) params.categoryId = categoryId
          if (activeSearch) params.q = activeSearch
          createSearch('entity', label, params)
        }}
        typeLabel={`search of ${getTipoInfo(tipo).label}`}
      />
    </div>
  )
}
