import { useState, useEffect } from 'react'
import { api } from '../services/api'

interface Alias {
  id: number
  alias: string
  canonical_name: string
  tipo: string
  created_at: string
}

export function AdminAliases() {
  const [aliases, setAliases] = useState<Alias[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingAlias, setEditingAlias] = useState<Alias | null>(null)
  const [tipoFilter, setTipoFilter] = useState('')

  const [form, setForm] = useState({
    alias: '',
    canonical_name: '',
    tipo: 'persona'
  })

  useEffect(() => {
    fetchAliases()
  }, [tipoFilter])

  const fetchAliases = async () => {
    setLoading(true)
    try {
      const params = tipoFilter ? `?tipo=${tipoFilter}` : ''
      const res = await api.get(`/admin/aliases${params}`)
      setAliases(res.data.aliases)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingAlias) {
        await api.put(`/admin/aliases/${editingAlias.id}`, form)
      } else {
        await api.post('/admin/aliases', form)
      }
      setShowModal(false)
      setEditingAlias(null)
      setForm({ alias: '', canonical_name: '', tipo: 'persona' })
      fetchAliases()
    } catch (err) {
      console.error(err)
      alert('Error al guardar alias')
    }
  }

  const handleEdit = (alias: Alias) => {
    setEditingAlias(alias)
    setForm({
      alias: alias.alias,
      canonical_name: alias.canonical_name,
      tipo: alias.tipo
    })
    setShowModal(true)
  }

  const handleDelete = async (id: number) => {
    if (!confirm('¿Eliminar este alias?')) return
    try {
      await api.delete(`/admin/aliases/${id}`)
      fetchAliases()
    } catch (err) {
      console.error(err)
    }
  }

  const handleExport = async () => {
    try {
      const res = await api.get('/admin/aliases/export', { responseType: 'blob' })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', 'aliases.csv')
      document.body.appendChild(link)
      link.click()
      link.remove()
    } catch (err) {
      console.error(err)
    }
  }

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const formData = new FormData()
    formData.append('file', file)

    try {
      const res = await api.post('/admin/aliases/import', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      })
      alert(`Importados: ${res.data.inserted}, Omitidos: ${res.data.skipped}`)
      fetchAliases()
    } catch (err) {
      console.error(err)
      alert('Error al importar')
    }
  }

  const openNewModal = () => {
    setEditingAlias(null)
    setForm({ alias: '', canonical_name: '', tipo: 'persona' })
    setShowModal(true)
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Gestión de Alias</h1>
        <div className="flex gap-2">
          <button onClick={handleExport} className="btn-secondary">
            Exportar CSV
          </button>
          <label className="btn-secondary cursor-pointer">
            Importar CSV
            <input type="file" accept=".csv" onChange={handleImport} className="hidden" />
          </label>
          <button onClick={openNewModal} className="btn-primary">
            + Nuevo Alias
          </button>
        </div>
      </div>

      <div className="mb-4">
        <select
          value={tipoFilter}
          onChange={(e) => setTipoFilter(e.target.value)}
          className="p-2 border rounded dark:bg-gray-700"
        >
          <option value="">Todos los tipos</option>
          <option value="persona">Personas</option>
          <option value="organizacion">Organizaciones</option>
          <option value="lugar">Lugares</option>
          <option value="tema">Temas</option>
        </select>
      </div>

      {loading ? (
        <div className="text-center py-8">Cargando...</div>
      ) : aliases.length === 0 ? (
        <div className="text-center py-8 text-gray-500">No hay aliases</div>
      ) : (
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left">Alias</th>
                <th className="px-4 py-3 text-left">Nombre Canónico</th>
                <th className="px-4 py-3 text-left">Tipo</th>
                <th className="px-4 py-3 text-right">Acciones</th>
              </tr>
            </thead>
            <tbody>
              {aliases.map((alias) => (
                <tr key={alias.id} className="border-t dark:border-gray-700">
                  <td className="px-4 py-3 font-mono">{alias.alias}</td>
                  <td className="px-4 py-3">{alias.canonical_name}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-1 rounded text-xs ${
                      alias.tipo === 'persona' ? 'bg-blue-100 text-blue-800' :
                      alias.tipo === 'organizacion' ? 'bg-purple-100 text-purple-800' :
                      alias.tipo === 'lugar' ? 'bg-green-100 text-green-800' :
                      'bg-gray-100 text-gray-800'
                    }`}>
                      {alias.tipo}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button onClick={() => handleEdit(alias)} className="text-blue-600 hover:underline mr-3">
                      Editar
                    </button>
                    <button onClick={() => handleDelete(alias.id)} className="text-red-600 hover:underline">
                      Eliminar
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">
              {editingAlias ? 'Editar Alias' : 'Nuevo Alias'}
            </h2>
            <form onSubmit={handleSubmit}>
              <div className="mb-4">
                <label className="block text-sm font-medium mb-1">Alias (ej: starmer)</label>
                <input
                  type="text"
                  value={form.alias}
                  onChange={(e) => setForm({ ...form, alias: e.target.value })}
                  className="w-full p-2 border rounded dark:bg-gray-700"
                  required
                />
              </div>
              <div className="mb-4">
                <label className="block text-sm font-medium mb-1">Nombre Canónico (ej: Keir Starmer)</label>
                <input
                  type="text"
                  value={form.canonical_name}
                  onChange={(e) => setForm({ ...form, canonical_name: e.target.value })}
                  className="w-full p-2 border rounded dark:bg-gray-700"
                  required
                />
              </div>
              <div className="mb-4">
                <label className="block text-sm font-medium mb-1">Tipo</label>
                <select
                  value={form.tipo}
                  onChange={(e) => setForm({ ...form, tipo: e.target.value })}
                  className="w-full p-2 border rounded dark:bg-gray-700"
                >
                  <option value="persona">Persona</option>
                  <option value="organizacion">Organización</option>
                  <option value="lugar">Lugar</option>
                  <option value="tema">Tema</option>
                </select>
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="px-4 py-2 border rounded hover:bg-gray-100 dark:hover:bg-gray-700"
                >
                  Cancelar
                </button>
                <button type="submit" className="btn-primary">
                  Guardar
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
