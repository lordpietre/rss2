import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiService, Feed, Category, Country } from '../services/api'
import { Plus, Trash2, ExternalLink, Upload, Download, RefreshCw, Power, PowerOff, AlertCircle } from 'lucide-react'

export function Feeds() {
  const queryClient = useQueryClient()
  const [showAddForm, setShowAddForm] = useState(false)
  const [newFeed, setNewFeed] = useState({ nombre: '', url: '', descripcion: '', categoria_id: '', pais_id: '', idioma: '' })
  const [filtroActivo, setFiltroActivo] = useState('')
  const [filtroCategoria, setFiltroCategoria] = useState('')
  const [filtroPais, setFiltroPais] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [importResult, setImportResult] = useState<{ imported: number; skipped: number; failed: number; message: string } | null>(null)

  const { data: feedsData, isLoading } = useQuery({
    queryKey: ['feeds', filtroActivo, filtroCategoria, filtroPais],
    queryFn: () => apiService.getFeeds({
      activo: filtroActivo || undefined,
      categoria_id: filtroCategoria || undefined,
      pais_id: filtroPais || undefined,
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

  const importFeeds = useMutation({
    mutationFn: apiService.importFeeds,
    onSuccess: (data) => {
      setImportResult(data)
      queryClient.invalidateQueries({ queryKey: ['feeds'] })
    },
    onError: (error: any) => {
      const message = error?.response?.status === 401 
        ? 'Debes iniciar sesión para importar archivos' 
        : error?.response?.data?.error || 'Error al importar el archivo'
      setImportResult({ imported: 0, skipped: 0, failed: 0, message })
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
    <div>
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Feeds RSS</h1>
        <div className="flex gap-2">
          <button onClick={() => fileInputRef.current?.click()} className="btn-secondary flex items-center gap-2">
            <Upload className="h-4 w-4" />
            Importar CSV
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
            Exportar
          </button>
          <button onClick={() => setShowAddForm(!showAddForm)} className="btn-primary flex items-center gap-2">
            <Plus className="h-4 w-4" />
            Añadir Feed
          </button>
        </div>
      </div>

      {importResult && (
        <div className={`card p-4 mb-4 ${importResult.failed > 0 || importResult.message.includes('Error') || importResult.message.includes('Debes') ? 'bg-red-50 border-red-200' : 'bg-green-50 border-green-200'}`}>
          <p className={importResult.failed > 0 || importResult.message.includes('Error') || importResult.message.includes('Debes') ? 'text-red-800' : 'text-green-800'}>
            {importResult.message}
          </p>
          {importResult.imported > 0 && (
            <p className="text-sm text-green-600 mt-1">
              Importados: {importResult.imported}, Omitidos: {importResult.skipped}, Errores: {importResult.failed}
            </p>
          )}
          <button onClick={() => setImportResult(null)} className="text-sm text-gray-500 underline mt-2">
            Cerrar
          </button>
        </div>
      )}

      <div className="card p-4 mb-6">
        <div className="flex flex-wrap gap-4">
          <select
            value={filtroActivo}
            onChange={(e) => setFiltroActivo(e.target.value)}
            className="input w-auto"
          >
            <option value="">Todos</option>
            <option value="true">Activos</option>
            <option value="false">Inactivos</option>
          </select>
          <select
            value={filtroCategoria}
            onChange={(e) => setFiltroCategoria(e.target.value)}
            className="input w-auto"
          >
            <option value="">Todas las categorías</option>
            {categorias?.map((cat) => (
              <option key={cat.id} value={cat.id}>{cat.nombre}</option>
            ))}
          </select>
          <select
            value={filtroPais}
            onChange={(e) => setFiltroPais(e.target.value)}
            className="input w-auto"
          >
            <option value="">Todos los países</option>
            {paises?.map((pais) => (
              <option key={pais.id} value={pais.id}>{pais.nombre}</option>
            ))}
          </select>
        </div>
      </div>

      {showAddForm && (
        <form onSubmit={handleSubmit} className="card p-6 mb-8">
          <h2 className="text-lg font-semibold mb-4">Nuevo Feed</h2>
          <div className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <input
                type="text"
                placeholder="Nombre *"
                value={newFeed.nombre}
                onChange={(e) => setNewFeed({ ...newFeed, nombre: e.target.value })}
                className="input"
                required
              />
              <input
                type="url"
                placeholder="URL del feed RSS *"
                value={newFeed.url}
                onChange={(e) => setNewFeed({ ...newFeed, url: e.target.value })}
                className="input"
                required
              />
            </div>
            <input
              type="text"
              placeholder="Descripción"
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
                <option value="">Categoría</option>
                {categorias?.map((cat) => (
                  <option key={cat.id} value={cat.id}>{cat.nombre}</option>
                ))}
              </select>
              <select
                value={newFeed.pais_id}
                onChange={(e) => setNewFeed({ ...newFeed, pais_id: e.target.value })}
                className="input"
              >
                <option value="">País</option>
                {paises?.map((pais) => (
                  <option key={pais.id} value={pais.id}>{pais.nombre}</option>
                ))}
              </select>
              <input
                type="text"
                placeholder="Idioma (ej: es, en, fr)"
                value={newFeed.idioma}
                onChange={(e) => setNewFeed({ ...newFeed, idioma: e.target.value })}
                className="input"
              />
            </div>
            <div className="flex gap-2">
              <button type="submit" className="btn-primary" disabled={createFeed.isPending}>
                {createFeed.isPending ? 'Añadiendo...' : 'Añadir'}
              </button>
              <button type="button" onClick={() => setShowAddForm(false)} className="btn-secondary">
                Cancelar
              </button>
            </div>
          </div>
        </form>
      )}

      <div className="text-sm text-gray-500 mb-4">
        Total: {feedsData?.total || 0} feeds
      </div>

      {isLoading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Nombre</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">URL</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Idioma</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Categoría</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">País</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Estado</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Noticias</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Errores</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900">Acciones</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {feedsData?.feeds?.map((feed) => (
                <tr key={feed.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    <div className="font-medium text-gray-900">{feed.nombre}</div>
                    {feed.descripcion && (
                      <div className="text-xs text-gray-500 truncate max-w-xs">{feed.descripcion}</div>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <a
                      href={feed.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-primary-600 hover:underline flex items-center gap-1 text-sm"
                    >
                      <ExternalLink className="h-3 w-3" />
                      Ver
                    </a>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">{feed.idioma || '-'}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">{feed.categoria || '-'}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">{feed.pais || '-'}</td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-2 py-1 rounded ${feed.activo ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'}`}>
                      {feed.activo ? 'Activo' : 'Inactivo'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">{feed.noticias_count || 0}</td>
                  <td className="px-4 py-3">
                    {feed.fallos && feed.fallos > 0 ? (
                      <span className="text-red-600 text-sm" title={feed.last_error || ''}>
                        {feed.fallos}
                      </span>
                    ) : (
                      <span className="text-gray-400">-</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2">
                      {feed.fallos && feed.fallos > 0 && (
                        <button
                          onClick={() => reactivateFeed.mutate(feed.id)}
                          className="p-1 rounded text-orange-600 hover:text-orange-700"
                          title={`Reactivar (${feed.fallos} errores)`}
                          disabled={reactivateFeed.isPending}
                        >
                          <AlertCircle className="h-4 w-4" />
                        </button>
                      )}
                      <button
                        onClick={() => toggleFeed.mutate(feed.id)}
                        className={`p-1 rounded ${feed.activo ? 'text-gray-400 hover:text-red-600' : 'text-green-600 hover:text-green-700'}`}
                        title={feed.activo ? 'Desactivar' : 'Activar'}
                      >
                        {feed.activo ? <PowerOff className="h-4 w-4" /> : <Power className="h-4 w-4" />}
                      </button>
                      <button
                        onClick={() => {
                          if (confirm('¿Eliminar este feed?')) {
                            deleteFeed.mutate(feed.id)
                          }
                        }}
                        className="p-1 text-red-600 hover:text-red-700"
                        title="Eliminar"
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
    </div>
  )
}
