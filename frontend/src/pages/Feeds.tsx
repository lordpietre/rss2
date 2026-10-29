import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiService, Feed, Category, Country } from '../services/api'
import { Plus, Trash2, ExternalLink, Upload, Download, RefreshCw, Power, PowerOff, AlertCircle, Trash, Languages, CheckSquare, Square } from 'lucide-react'

export function Feeds() {
  const queryClient = useQueryClient()
  const [showAddForm, setShowAddForm] = useState(false)
  const [newFeed, setNewFeed] = useState({ nombre: '', url: '', descripcion: '', categoria_id: '', pais_id: '', idioma: '' })
  const [filtroActivo, setFiltroActivo] = useState('')
  const [filtroCategoria, setFiltroCategoria] = useState('')
  const [filtroPais, setFiltroPais] = useState('')
  const [page, setPage] = useState(1)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number; failed: number; message: string } | null>(null)
  // Seleccion multiple para montar la lista de canales ya en espanol.
  const [selected, setSelected] = useState<number[]>([])
  // Busqueda simple por nombre/URL de la fuente.
  const [busqueda, setBusqueda] = useState('')

  const { data: feedsData, isLoading } = useQuery({
    queryKey: ['feeds', filtroActivo, filtroCategoria, filtroPais, page, busqueda],
    queryFn: () => apiService.getFeeds({
      activo: filtroActivo || undefined,
      categoria_id: filtroCategoria || undefined,
      pais_id: filtroPais || undefined,
      q: busqueda || undefined,
      page,
      per_page: 50,
    }),
  })

  const { data: categorias } = useQuery({
    queryKey: ['categories'],
    queryFn: apiService.getCategories,
  })

  const { data: paises } = useQuery({
    queryKey: ['countries'],
    queryFn: apiService.getCountries,
  })

  const createFeed = useMutation({
    mutationFn: apiService.createFeed,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
      setShowAddForm(false)
      setNewFeed({ nombre: '', url: '', descripcion: '', categoria_id: '', pais_id: '', idioma: '' })
    },
  })

  const deleteFeed = useMutation({
    mutationFn: apiService.deleteFeed,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
  })

  const toggleFeed = useMutation({
    mutationFn: apiService.toggleFeed,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
  })

  const reactivateFeed = useMutation({
    mutationFn: apiService.reactivateFeed,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
  })

  const setFeedTraducido = useMutation({
    mutationFn: ({ id, traducido }: { id: number; traducido: boolean }) =>
      apiService.setFeedTraducido(id, traducido),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
  })

  const bulkTraducido = useMutation({
    mutationFn: ({ ids, traducido }: { ids: number[]; traducido: boolean }) =>
      apiService.bulkSetFeedsTraducido(ids, traducido),
    onSuccess: () => {
      setSelected([])
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
  })

  const toggleSelected = (id: number) => {
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  const bulkDelete = useMutation({
    mutationFn: (params: { ids?: number[]; q?: string }) => apiService.bulkDeleteFeeds(params),
    onSuccess: (data) => {
      setSelected([])
      setBusqueda('')
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
      queryClient.invalidateQueries({ queryKey: ['news'] })
      alert(
        `Se han eliminado ${data.feeds_deleted} fuentes y ${data.noticias_deleted} noticias.`
      )
    },
  })

  const importFeeds = useMutation({
    mutationFn: apiService.importFeeds,
    onSuccess: (data) => {
      setImportResult(data)
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
    onError: (error: any) => {
      const message = error?.response?.status === 401 
        ? 'You must be logged in to import files' 
        : error?.response?.data?.error || 'Error importing file'
      setImportResult({ imported: 0, skipped: 0, failed: 0, message })
    },
  })

  const deleteInactiveFeeds = useMutation({
    mutationFn: async ({ paisId }: { paisId?: string }) => {
      return apiService.deleteInactiveFeeds(paisId)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
  })

  const handleExport = async () => {
    try {
      const blob = await apiService.exportFeeds({
        activo: filtroActivo || undefined,
        categoria_id: filtroCategoria || undefined,
        pais_id: filtroPais || undefined,
      })
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'feeds_export.csv'
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } catch (error) {
      console.error('Export failed:', error)
    }
  }

  const handleImport = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      importFeeds.mutate(file)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    createFeed.mutate({
      nombre: newFeed.nombre,
      url: newFeed.url,
      descripcion: newFeed.descripcion || undefined,
      categoria_id: newFeed.categoria_id ? parseInt(newFeed.categoria_id) : undefined,
      pais_id: newFeed.pais_id ? parseInt(newFeed.pais_id) : undefined,
      idioma: newFeed.idioma || undefined,
    })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">Sources Management</h1>
          <p className="text-sm text-gray-400 mt-1">RSS feeds monitoring and configuration</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <button onClick={() => fileInputRef.current?.click()} className="btn-secondary flex items-center gap-2">
            <Upload className="h-4 w-4" />
            Import CSV
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".csv"
            onChange={handleImport}
            className="hidden"
          />
          <button onClick={handleExport} className="btn-secondary flex items-center gap-2">
            <Download className="h-4 w-4" />
            Export
          </button>
          <button onClick={() => setShowAddForm(!showAddForm)} className="btn-primary flex items-center gap-2">
            <Plus className="h-4 w-4" />
            Add Feed
          </button>
        </div>
      </div>

      {importResult && (
        <div className={`panel ${importResult.failed > 0 || importResult.message.includes('Error') || importResult.message.includes('logged') ? 'border-rss-red/50' : 'border-primary-50/50'}`}>
          <p className={importResult.failed > 0 || importResult.message.includes('Error') || importResult.message.includes('logged') ? 'text-rss-red' : 'text-primary-50'}>
            {importResult.message}
          </p>
          {importResult.imported > 0 && (
            <p className="text-sm text-gray-400 mt-1">
              Imported: {importResult.imported}, Skipped: {importResult.skipped}, Failed: {importResult.failed}
            </p>
          )}
          <button onClick={() => setImportResult(null)} className="text-sm text-gray-400 underline mt-2">
            Close
          </button>
        </div>
      )}

      <div className="panel">
        <div className="flex flex-wrap gap-4 items-center">
          <div className="flex-1 min-w-[240px]">
            <input
              type="search"
              value={busqueda}
              onChange={(e) => { setBusqueda(e.target.value); setPage(1) }}
              placeholder="Buscar fuente por nombre o URL (p.ej. wikipedia)"
              className="input w-full"
              title="Busqueda simple sobre el nombre, la descripcion y la URL de la fuente"
            />
          </div>
          <select
            value={filtroActivo}
            onChange={(e) => { setFiltroActivo(e.target.value); setPage(1) }}
            className="input w-auto"
          >
            <option value="">All</option>
            <option value="true">Active</option>
            <option value="false">Inactive</option>
          </select>
          <select
            value={filtroCategoria}
            onChange={(e) => { setFiltroCategoria(e.target.value); setPage(1) }}
            className="input w-auto"
          >
            <option value="">All categories</option>
            {categorias?.map((cat) => (
              <option key={cat.id} value={cat.id}>{cat.nombre}</option>
            ))}
          </select>
          <select
            value={filtroPais}
            onChange={(e) => { setFiltroPais(e.target.value); setPage(1) }}
            className="input w-auto"
          >
            <option value="">All countries</option>
            {paises?.map((pais) => (
              <option key={pais.id} value={pais.id}>{pais.nombre}</option>
            ))}
          </select>
          {filtroActivo === 'false' && filtroPais && (
            <button
              onClick={() => {
                if (confirm(`Delete all inactive feeds from selected country? (${feedsData?.total || 0} feeds)`)) {
                  deleteInactiveFeeds.mutate({ paisId: filtroPais })
                }
              }}
              disabled={deleteInactiveFeeds.isPending}
              className="btn-secondary flex items-center gap-2 border-rss-red/50 text-rss-red"
            >
              <Trash className="h-4 w-4" />
              {deleteInactiveFeeds.isPending ? 'Deleting...' : 'Delete all inactive'}
            </button>
          )}
        </div>
      </div>

      {showAddForm && (
        <form onSubmit={handleSubmit} className="panel">
          <h2 className="text-lg font-semibold text-gray-50 mb-4">New Feed</h2>
          <div className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <input
                type="text"
                placeholder="Name *"
                value={newFeed.nombre}
                onChange={(e) => setNewFeed({ ...newFeed, nombre: e.target.value })}
                className="input"
                required
              />
              <input
                type="url"
                placeholder="RSS Feed URL *"
                value={newFeed.url}
                onChange={(e) => setNewFeed({ ...newFeed, url: e.target.value })}
                className="input"
                required
              />
            </div>
            <input
              type="text"
              placeholder="Description"
              value={newFeed.descripcion}
              onChange={(e) => setNewFeed({ ...newFeed, descripcion: e.target.value })}
              className="input"
            />
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <select
                value={newFeed.categoria_id}
                onChange={(e) => setNewFeed({ ...newFeed, categoria_id: e.target.value })}
                className="input"
              >
                <option value="">Category</option>
                {categorias?.map((cat) => (
                  <option key={cat.id} value={cat.id}>{cat.nombre}</option>
                ))}
              </select>
              <select
                value={newFeed.pais_id}
                onChange={(e) => setNewFeed({ ...newFeed, pais_id: e.target.value })}
                className="input"
              >
                <option value="">Country</option>
                {paises?.map((pais) => (
                  <option key={pais.id} value={pais.id}>{pais.nombre}</option>
                ))}
              </select>
              <input
                type="text"
                placeholder="Language (e.g. es, en, fr)"
                value={newFeed.idioma}
                onChange={(e) => setNewFeed({ ...newFeed, idioma: e.target.value })}
                className="input"
              />
            </div>
            <div className="flex gap-2">
              <button type="submit" className="btn-primary" disabled={createFeed.isPending}>
                {createFeed.isPending ? 'Adding...' : 'Add'}
              </button>
              <button type="button" onClick={() => setShowAddForm(false)} className="btn-secondary">
                Cancel
              </button>
            </div>
          </div>
        </form>
      )}

      <div className="text-sm text-gray-400">
        Total: {feedsData?.total || 0} feeds
      </div>

      {selected.length > 0 && (
        <div className="panel flex flex-wrap items-center gap-3 text-sm">
          <span className="text-gray-300">
            {selected.length} canal{selected.length === 1 ? '' : 'es'} seleccionado
            {selected.length === 1 ? '' : 'es'}
          </span>
          <button
            onClick={() => bulkTraducido.mutate({ ids: selected, traducido: true })}
            className="btn-secondary flex items-center gap-2"
            disabled={bulkTraducido.isPending}
            title="Anadir al listado de canales ya traducidos: sus noticias no se procesan"
          >
            <Languages className="h-4 w-4" />
            Marcar como ya traducidos
          </button>
          <button
            onClick={() => bulkTraducido.mutate({ ids: selected, traducido: false })}
            className="btn-secondary"
            disabled={bulkTraducido.isPending}
            title="Quitar del listado de canales ya traducidos: volveran a traducirse"
          >
            Quitar del listado
          </button>
          <button onClick={() => setSelected([])} className="text-gray-400 hover:text-gray-200 underline">
            Cancelar
          </button>
          <button
            onClick={() => {
              if (
                confirm(
                  `Eliminar ${selected.length} fuentes seleccionadas y todas sus noticias?`
                )
              ) {
                bulkDelete.mutate({ ids: selected })
              }
            }}
            className="btn-secondary flex items-center gap-2 text-rss-red"
            disabled={bulkDelete.isPending}
            title="Elimina las fuentes seleccionadas y sus noticias (con traducciones, tags y embeddings)"
          >
            <Trash2 className="h-4 w-4" />
            Eliminar seleccionadas
          </button>
        </div>
      )}

      {busqueda.trim() !== '' && (
        <div className="panel flex flex-wrap items-center gap-3 text-sm">
          <span className="text-gray-300">
            {feedsData?.total || 0} fuentes coinciden con «{busqueda}»
          </span>
          <button
            onClick={() => {
              if (
                confirm(
                  `Eliminar TODAS las fuentes que coinciden con «${busqueda}» y sus noticias?`
                )
              ) {
                bulkDelete.mutate({ q: busqueda.trim() })
              }
            }}
            className="btn-secondary flex items-center gap-2 text-rss-red"
            disabled={bulkDelete.isPending}
            title="Borra todas las fuentes que casan con la busqueda, no solo las de esta pagina"
          >
            <Trash2 className="h-4 w-4" />
            Eliminar todas las que coinciden
          </button>
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-50"></div>
        </div>
      ) : (
        <div className="panel overflow-x-auto">
          <table className="w-full">
            <thead className="border-b border-gray-700">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">
                  <button
                    onClick={() => {
                      const ids = (feedsData?.feeds || []).map((f) => f.id)
                      setSelected(selected.length === ids.length ? [] : ids)
                    }}
                    title="Seleccionar todo"
                    className="text-gray-400 hover:text-gray-200"
                  >
                    {selected.length > 0 &&
                    selected.length === (feedsData?.feeds || []).length ? (
                      <CheckSquare className="h-4 w-4" />
                    ) : (
                      <Square className="h-4 w-4" />
                    )}
                  </button>
                </th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Name</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">URL</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Lang</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Category</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Country</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Status</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400" title="El canal ya esta en espanol: sus noticias no se traducen">
                  Translated
                </th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">News</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Errors</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {feedsData?.feeds?.map((feed) => (
                <tr key={feed.id} className="hover:bg-gray-800/50">
                  <td className="px-4 py-3">
                    <button
                      onClick={() => toggleSelected(feed.id)}
                      className="text-gray-400 hover:text-gray-200"
                      title="Seleccionar"
                    >
                      {selected.includes(feed.id) ? (
                        <CheckSquare className="h-4 w-4" />
                      ) : (
                        <Square className="h-4 w-4" />
                      )}
                    </button>
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-medium text-gray-50">{feed.nombre}</div>
                    {feed.descripcion && (
                      <div className="text-xs text-gray-500 truncate max-w-xs">{feed.descripcion}</div>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <a
                      href={feed.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-primary-300 hover:text-primary-50 flex items-center gap-1 text-sm"
                    >
                      <ExternalLink className="h-3 w-3" />
                      View
                    </a>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-400 font-mono">{feed.idioma || '-'}</td>
                  <td className="px-4 py-3 text-sm text-gray-400">{feed.categoria || '-'}</td>
                  <td className="px-4 py-3 text-sm text-gray-400">{feed.pais || '-'}</td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-2 py-1 rounded ${feed.activo ? 'bg-primary-900/30 text-primary-50' : 'bg-gray-800 text-gray-400'}`}>
                      {feed.activo ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() =>
                        setFeedTraducido.mutate({ id: feed.id, traducido: !feed.traducido })
                      }
                      className={`text-xs px-2 py-1 rounded ${
                        feed.traducido
                          ? 'bg-green-900/40 text-green-300'
                          : 'bg-gray-800 text-gray-500 hover:text-gray-300'
                      }`}
                      title={
                        feed.traducido
                          ? 'Ya en espanol: sus noticias no se traducen. Pulsa para volver a traducirlas.'
                          : 'Marcar como canal ya en espanol (no se procesa)'
                      }
                      disabled={setFeedTraducido.isPending}
                    >
                      {feed.traducido ? 'Traducido' : 'Traducir'}
                    </button>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-400">{feed.noticias_count || 0}</td>
                  <td className="px-4 py-3">
                    {feed.fallos && feed.fallos > 0 ? (
                      <span className="text-rss-red text-sm font-mono" title={feed.last_error || ''}>
                        {feed.fallos}
                      </span>
                    ) : (
                      <span className="text-gray-600">-</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2">
                      {feed.fallos && feed.fallos > 0 && (
                        <button
                          onClick={() => reactivateFeed.mutate(feed.id)}
                          className="p-1 rounded text-amber-400 hover:text-amber-300"
                          title={`Reactivate (${feed.fallos} errors)`}
                          disabled={reactivateFeed.isPending}
                        >
                          <AlertCircle className="h-4 w-4" />
                        </button>
                      )}
                      <button
                        onClick={() => toggleFeed.mutate(feed.id)}
                        className={`p-1 rounded ${feed.activo ? 'text-gray-400 hover:text-rss-red' : 'text-primary-50 hover:text-primary-300'}`}
                        title={feed.activo ? 'Deactivate' : 'Activate'}
                      >
                        {feed.activo ? <PowerOff className="h-4 w-4" /> : <Power className="h-4 w-4" />}
                      </button>
                      <button
                        onClick={() => {
                          if (confirm('Delete this feed?')) {
                            deleteFeed.mutate(feed.id)
                          }
                        }}
                        className="p-1 text-gray-400 hover:text-rss-red"
                        title="Delete"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!isLoading && feedsData && feedsData.total_pages > 1 && (
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 text-sm bg-gray-900 p-4 rounded-lg border border-gray-700">
          <div className="text-gray-400">
            Page <span className="font-semibold text-gray-50">{page}</span> of <span className="font-semibold text-gray-50">{feedsData.total_pages}</span>
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="btn-secondary disabled:opacity-50"
            >
              Previous
            </button>
            <button
              onClick={() => setPage(p => Math.min(feedsData.total_pages, p + 1))}
              disabled={page >= feedsData.total_pages}
              className="btn-secondary disabled:opacity-50"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
