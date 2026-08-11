import { useState, useEffect, useRef } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../services/api'
import { WikiTooltip } from '../components/ui/WikiTooltip'

interface Entity {
  valor: string
  tipo: string
  count: number
  wiki_summary?: string
  wiki_url?: string
  image_path?: string
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
  { value: 'persona', label: '👤 Persona', color: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' },
  { value: 'organizacion', label: '🏢 Organización', color: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200' },
  { value: 'lugar', label: '📍 Lugar', color: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
  { value: 'tema', label: '📰 Tema', color: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' },
]

const TIMELINE_WINDOWS: { key: string; label: string; title: string; offset: number }[] = [
  { key: '0', label: 'Hoy', title: 'hoy', offset: 0 },
  { key: '1', label: '1d', title: 'hace 1 día', offset: 1 },
  { key: '2', label: '2d', title: 'hace 2 días', offset: 2 },
  { key: '3', label: '3d', title: 'hace 3 días', offset: 3 },
  { key: '4', label: '4d', title: 'hace 4 días', offset: 4 },
  { key: '5', label: '5d', title: 'hace 5 días', offset: 5 },
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
  const [activeSearch, setActiveSearch] = useState('')
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(false)
  const [configModal, setConfigModal] = useState<ConfigModal | null>(null)
  const [saving, setSaving] = useState(false)
  const [successMsg, setSuccessMsg] = useState('')

  // News-by-entity modal state
  const [newsModal, setNewsModal] = useState<Entity | null>(null)
  const [newsModalMode, setNewsModalMode] = useState<'news' | 'timeline'>('news')
  const [timelineLabel, setTimelineLabel] = useState('últimas 24 horas')
  const [entityNews, setEntityNews] = useState<any[]>([])
  const [newsLoading, setNewsLoading] = useState(false)
  const [newsTotal, setNewsTotal] = useState(0)

  // Form state for configure modal
  const [newTipo, setNewTipo] = useState('')
  const [aliasInput, setAliasInput] = useState('')
  const [canonicalInput, setCanonicalInput] = useState('')
  const [activeTab, setActiveTab] = useState<'tipo' | 'alias'>('tipo')

  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    api.get('/categories').then(res => setCategories(res.data)).catch(console.error)
    api.get('/countries').then(res => setCountries(res.data)).catch(console.error)
  }, [])

  useEffect(() => {
    setPage(1)
  }, [tipo, countryId, categoryId, activeSearch])

  useEffect(() => {
    fetchEntities()
  }, [tipo, countryId, categoryId, activeSearch, page])

  const fetchEntities = async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams()
      params.append('tipo', tipo)
      if (countryId) params.append('country_id', countryId)
      if (categoryId) params.append('category_id', categoryId)
      if (activeSearch) params.append('q', activeSearch)
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
    setNewsModalMode('news')
    setNewsModal(entity)
    setNewsLoading(true)
    setEntityNews([])
    setNewsTotal(0)
    try {
      const params = new URLSearchParams()
      params.append('valor', entity.valor)
      params.append('tipo', entity.tipo)
      params.append('per_page', '30')
      const res = await api.get(`/entities/news?${params}`)
      setEntityNews(res.data.news || [])
      setNewsTotal(res.data.total || 0)
    } catch (err) {
      console.error(err)
    } finally {
      setNewsLoading(false)
    }
  }

  const openTimeline = async (entity: Entity, win: typeof TIMELINE_WINDOWS[number]) => {
    setTimelineLabel(win.title)
    setNewsModalMode('timeline')
    setNewsModal(entity)
    setNewsLoading(true)
    setEntityNews([])
    setNewsTotal(0)
    try {
      const params = new URLSearchParams()
      params.append('valor', entity.valor)
      params.append('tipo', entity.tipo)
      params.append('day_offset', String(win.offset))
      params.append('per_page', '50')
      const res = await api.get(`/entities/news?${params}`)
      setEntityNews(res.data.news || [])
      setNewsTotal(res.data.total || 0)
    } catch (err) {
      console.error(err)
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
      setSuccessMsg(`✅ Tipo cambiado a "${getTipoInfo(newTipo).label}"`)
      fetchEntities()
    } catch (err: any) {
      setSuccessMsg(`❌ Error: ${err?.response?.data?.error || 'Error desconocido'}`)
    } finally {
      setSaving(false)
    }
  }

  const handleSaveAlias = async () => {
    if (!canonicalInput.trim() || !aliasInput.trim()) return
    setSaving(true)
    
    // Split input by newlines or commas, trim spaces, and remove empty entries
    const aliasesList = aliasInput
      .split(/[\n,]+/)
      .map(a => a.trim())
      .filter(a => a.length > 0)

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
      setSuccessMsg(`✅ ${aliasesList.length} alias creados y fusionados en "${canonicalInput}"`)
      setAliasInput('')
      fetchEntities() // Refresh popular entity metrics
    } catch (err: any) {
      setSuccessMsg(`❌ Error: ${err?.response?.data?.error || 'Ya existe este alias'}`)
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
      const res = await api.post('/admin/aliases/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      })
      alert(`✅ Importados: ${res.data.inserted}, Omitidos: ${res.data.skipped}`)
      e.target.value = ''
    } catch { alert('❌ Error al importar CSV') }
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
      alert('❌ Error al generar backup. Verifica que pg_dump esté disponible.')
    }
  }

  const selectedCountry = countries.find(c => c.id.toString() === countryId)

  return (
    <div className="p-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center mb-6 gap-3">
        <h1 className="text-2xl font-bold">
          🔥 Populares {selectedCountry ? `en ${selectedCountry.nombre}` : 'Global'}
        </h1>
        <div className="flex flex-wrap gap-2">
          <button
            onClick={handleExportAliases}
            className="px-3 py-1.5 text-sm border rounded hover:bg-gray-50 dark:hover:bg-gray-700 dark:border-gray-600"
          >
            📤 Exportar aliases
          </button>
          <label className="px-3 py-1.5 text-sm border rounded hover:bg-gray-50 dark:hover:bg-gray-700 dark:border-gray-600 cursor-pointer">
            📥 Importar aliases
            <input
              ref={fileInputRef}
              type="file"
              accept=".csv"
              onChange={handleImportAliases}
              className="hidden"
            />
          </label>
          <button
            onClick={handleBackupDB}
            className="px-3 py-1.5 text-sm bg-amber-500 hover:bg-amber-600 text-white rounded font-medium"
          >
            💾 Backup BBDD
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div>
            <label className="block text-sm font-medium mb-1">Tipo</label>
            <select
              value={tipo}
              onChange={(e) => setTipo(e.target.value)}
              className="w-full p-2 border rounded dark:bg-gray-700 dark:border-gray-600"
            >
              {TIPOS.map(t => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">País</label>
            <select
              value={countryId}
              onChange={(e) => setCountryId(e.target.value)}
              className="w-full p-2 border rounded dark:bg-gray-700 dark:border-gray-600"
            >
              <option value="">🌍 Global</option>
              {countries.map(c => (
                <option key={c.id} value={c.id}>{c.nombre}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Categoría</label>
            <select
              value={categoryId}
              onChange={(e) => setCategoryId(e.target.value)}
              className="w-full p-2 border rounded dark:bg-gray-700 dark:border-gray-600"
            >
              <option value="">Todas las categorías</option>
              {categories.map(c => (
                <option key={c.id} value={c.id}>{c.nombre}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">Buscar</label>
            <div className="flex gap-2">
              <input
                type="text"
                value={searchQuery}
                onChange={e => setSearchQuery(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && setActiveSearch(searchQuery)}
                placeholder="Término o alias..."
                className="w-full p-2 border rounded dark:bg-gray-700 dark:border-gray-600"
              />
              <button
                onClick={() => setActiveSearch(searchQuery)}
                className="px-3 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
              >
                🔍
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Table */}
      {loading ? (
        <div className="text-center py-12 text-gray-400">Cargando...</div>
      ) : entities.length === 0 ? (
        <div className="text-center py-12 text-gray-400">No se encontraron entidades</div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider w-10">#</th>
                <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">Entidad</th>
                <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider hidden md:table-cell">Tipo</th>
                <th className="px-4 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wider">Menciones</th>
                <th className="px-4 py-3 text-center text-xs font-semibold text-gray-500 uppercase tracking-wider w-24">Config</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
              {entities.map((entity, idx) => {
                const tipoInfo = getTipoInfo(entity.tipo)
                return (
                  <tr key={entity.valor + idx} className="hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors">
                    <td className="px-4 py-3 text-gray-400 text-sm">{(page - 1) * 50 + idx + 1}</td>
                    <td className="px-4 py-3 font-medium">
                      <WikiTooltip
                        name={entity.valor}
                        summary={entity.wiki_summary}
                        imagePath={entity.image_path}
                        wikiUrl={entity.wiki_url}
                      >
                        <div className="flex items-center gap-3">
                          {entity.image_path && (
                            <img 
                              src={entity.image_path} 
                              alt="" 
                              className="w-24 h-24 rounded-full object-cover border-2 border-white shadow-md mx-auto"
                              onError={(e) => (e.currentTarget.style.display = 'none')}
                            />
                          )}
                          <span>{entity.valor}</span>
                        </div>
                      </WikiTooltip>
                    </td>
                    <td className="px-4 py-3 hidden md:table-cell">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${tipoInfo.color}`}>
                        {tipoInfo.label}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-sm font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                        {entity.count}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-center">
                      <div className="flex flex-col items-center gap-1">
                        <div className="flex items-center justify-center gap-1">
                          <button
                            onClick={() => openNews(entity)}
                            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                            title="Ver noticias donde se menciona"
                          >
                            🔍 Buscar
                          </button>
                          <button
                            onClick={() => openConfig(entity)}
                            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                            title="Configurar entidad"
                          >
                            ⚙️ Configurar
                          </button>
                        </div>
                        <div className="flex flex-wrap items-center justify-center gap-1">
                          {TIMELINE_WINDOWS.map((win) => (
                            <button
                              key={win.key}
                              onClick={() => openTimeline(entity, win)}
                              className="inline-flex items-center px-1.5 py-0.5 text-[10px] rounded bg-blue-600 text-white hover:bg-blue-700 transition-colors"
                              title={win.title}
                            >
                              {win.label}
                            </button>
                          ))}
                        </div>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination Controls */}
      {!loading && entities.length > 0 && totalPages > 1 && (
        <div className="mt-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-sm text-gray-600 dark:text-gray-400 bg-white dark:bg-gray-800 p-4 rounded-lg shadow">
          <div>
            Página <span className="font-semibold text-gray-900 dark:text-white">{page}</span> de <span className="font-semibold text-gray-900 dark:text-white">{totalPages}</span>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="px-4 py-2 border rounded-lg hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed dark:border-gray-600 dark:hover:bg-gray-700 transition-colors"
            >
              Anterior
            </button>
            <button
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="px-4 py-2 border rounded-lg hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed dark:border-gray-600 dark:hover:bg-gray-700 transition-colors"
            >
              Siguiente
            </button>
          </div>
        </div>
      )}

      {/* Configure Modal */}
      {configModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={closeConfig}>
          <div
            className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md"
            onClick={e => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b dark:border-gray-700">
              <div>
                <h2 className="text-lg font-bold">Configurar entidad</h2>
                <p className="text-sm text-gray-500 mt-0.5">"{configModal.entity.valor}"</p>
              </div>
              <button
                onClick={closeConfig}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-xl leading-none"
              >
                ✕
              </button>
            </div>

            {/* Tabs */}
            <div className="flex border-b dark:border-gray-700">
              <button
                onClick={() => { setActiveTab('tipo'); setSuccessMsg('') }}
                className={`flex-1 py-3 text-sm font-medium transition-colors ${
                  activeTab === 'tipo'
                    ? 'border-b-2 border-blue-500 text-blue-600'
                    : 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
                }`}
              >
                🔄 Cambiar tipo
              </button>
              <button
                onClick={() => { setActiveTab('alias'); setSuccessMsg('') }}
                className={`flex-1 py-3 text-sm font-medium transition-colors ${
                  activeTab === 'alias'
                    ? 'border-b-2 border-blue-500 text-blue-600'
                    : 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
                }`}
              >
                🔁 Crear alias
              </button>
            </div>

            {/* Tab Content */}
            <div className="p-6">
              {activeTab === 'tipo' && (
                <div>
                  <p className="text-sm text-gray-500 mb-4">
                    Mover "<strong>{configModal.entity.valor}</strong>" a otra categoría. Afecta a todos los tags con este valor en la base de datos.
                  </p>
                  <div className="grid grid-cols-2 gap-2 mb-4">
                    {TIPOS.map(t => (
                      <button
                        key={t.value}
                        onClick={() => setNewTipo(t.value)}
                        className={`px-3 py-2.5 rounded-lg border-2 text-sm font-medium transition-all ${
                          newTipo === t.value
                            ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                            : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'
                        }`}
                      >
                        {t.label}
                        {t.value === configModal.entity.tipo && (
                          <span className="ml-1 text-xs text-gray-400">(actual)</span>
                        )}
                      </button>
                    ))}
                  </div>
                  {successMsg && (
                    <p className="text-sm mb-3 py-2 px-3 rounded bg-gray-50 dark:bg-gray-700">{successMsg}</p>
                  )}
                  <button
                    onClick={handleSaveTipo}
                    disabled={saving || newTipo === configModal.entity.tipo}
                    className="w-full py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg font-medium text-sm transition-colors"
                  >
                    {saving ? 'Guardando...' : 'Cambiar tipo'}
                  </button>
                </div>
              )}

              {activeTab === 'alias' && (
                <div>
                  <p className="text-sm text-gray-500 mb-4">
                    Crea un alias para normalizar variantes del mismo nombre. El alias se mapeará al nombre canónico que definas.
                  </p>
                  <div className="mb-3">
                    <label className="block text-sm font-medium mb-1">
                      Aliases <span className="text-gray-400 font-normal">(separados por comas o saltos de línea)</span>
                    </label>
                    <textarea
                      value={aliasInput}
                      onChange={e => setAliasInput(e.target.value)}
                      className="w-full p-2 border rounded dark:bg-gray-700 dark:border-gray-600 text-sm min-h-[80px] resize-y"
                      placeholder="ej: donald trump, el presidente, dictador&#10;trump"
                    />
                  </div>
                  <div className="mb-4">
                    <label className="block text-sm font-medium mb-1">
                      Nombre canónico <span className="text-gray-400 font-normal">(nombre correcto y normalizado)</span>
                    </label>
                    <input
                      type="text"
                      value={canonicalInput}
                      onChange={e => setCanonicalInput(e.target.value)}
                      className="w-full p-2 border rounded dark:bg-gray-700 dark:border-gray-600 text-sm"
                      placeholder="ej: Donald Trump"
                    />
                  </div>
                  {successMsg && (
                    <p className="text-sm mb-3 py-2 px-3 rounded bg-gray-50 dark:bg-gray-700">{successMsg}</p>
                  )}
                  <button
                    onClick={handleSaveAlias}
                    disabled={saving || !aliasInput.trim() || !canonicalInput.trim()}
                    className="w-full py-2 bg-green-600 hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg font-medium text-sm transition-colors"
                  >
                    {saving ? 'Creando...' : 'Crear alias'}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* News-by-entity Modal */}
      {newsModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setNewsModal(null)}>
          <div
            className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-2xl max-h-[85vh] overflow-hidden flex flex-col"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-6 py-4 border-b dark:border-gray-700">
              <div>
                <h2 className="text-lg font-bold">
                  {newsModalMode === 'timeline'
                    ? <>📈 Timeline de &quot;{newsModal.valor}&quot;</>
                    : <>Noticias sobre &quot;{newsModal.valor}&quot;</>}
                </h2>
                <p className="text-sm text-gray-500 mt-0.5">
                  {newsModalMode === 'timeline'
                    ? `Últimas ${Math.min(entityNews.length, 50)} noticias · ${timelineLabel} (${getTipoInfo(newsModal.tipo).label})`
                    : `${newsTotal} noticias encontradas (${getTipoInfo(newsModal.tipo).label})`}
                </p>
              </div>
              <button
                onClick={() => setNewsModal(null)}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-xl leading-none"
              >
                ✕
              </button>
            </div>

            <div className="p-4 overflow-y-auto">
              {newsLoading ? (
                <div className="text-center py-10 text-gray-400">Cargando...</div>
              ) : entityNews.length === 0 ? (
                <div className="text-center py-10 text-gray-400">
                  {newsModalMode === 'timeline'
                    ? `No hay noticias de esta entidad para ${timelineLabel}.`
                    : 'No hay noticias que mencionen a esta entidad.'}
                </div>
              ) : newsModalMode === 'timeline' ? (
                <ol className="relative border-l-2 border-blue-200 dark:border-blue-800 ml-2 space-y-4">
                  {entityNews.map((n) => (
                    <li key={n.id} className="ml-4 relative">
                      <span className="absolute -left-[21px] top-2 h-3 w-3 rounded-full bg-blue-500 ring-2 ring-blue-100 dark:ring-blue-900" />
                      <div className="flex items-baseline gap-2">
                        {n.fecha && (
                          <span className="text-[11px] font-semibold text-blue-600 dark:text-blue-400 shrink-0 tabular-nums">
                            {new Date(n.fecha).toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })}
                          </span>
                        )}
                        {n.fecha && (
                          <span className="text-[10px] text-gray-400 shrink-0 tabular-nums">
                            {new Date(n.fecha).toLocaleDateString('es-ES', { day: '2-digit', month: '2-digit', year: '2-digit' })}
                          </span>
                        )}
                      </div>
                      <Link
                        to={`/news/${n.id}`}
                        onClick={() => setNewsModal(null)}
                        className="block p-3 mt-1 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors border border-gray-100 dark:border-gray-700"
                      >
                        <div className="flex items-start gap-3">
                          {n.imagen_url && (
                            <img
                              src={n.imagen_url}
                              alt=""
                              className="w-20 h-14 object-cover rounded-md shrink-0"
                              onError={(e) => (e.currentTarget.style.display = 'none')}
                            />
                          )}
                          <div className="min-w-0">
                            <div className="font-medium text-sm line-clamp-2">
                              {n.title_translated || n.titulo}
                            </div>
                            <p className="text-xs text-gray-500 mt-1 line-clamp-2">
                              {n.summary_translated || n.resumen}
                            </p>
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
                      <Link
                        to={`/news/${n.id}`}
                        onClick={() => setNewsModal(null)}
                        className="block p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors border border-gray-100 dark:border-gray-700"
                      >
                        <div className="flex items-start gap-3">
                          {n.imagen_url && (
                            <img
                              src={n.imagen_url}
                              alt=""
                              className="w-20 h-14 object-cover rounded-md shrink-0"
                              onError={(e) => (e.currentTarget.style.display = 'none')}
                            />
                          )}
                          <div className="min-w-0">
                            <div className="font-medium text-sm line-clamp-2">
                              {n.title_translated || n.titulo}
                            </div>
                            <p className="text-xs text-gray-500 mt-1 line-clamp-2">
                              {n.summary_translated || n.resumen}
                            </p>
                            {n.fecha && (
                              <span className="text-[11px] text-gray-400 mt-1 block">
                                {new Date(n.fecha).toLocaleDateString('es-ES')}
                              </span>
                            )}
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
    </div>
  )
}
