import { useState } from 'react'
import { X } from 'lucide-react'

interface CreateListModalProps {
  isOpen: boolean
  onClose: () => void
  onCreate: (name: string, keywords?: string) => Promise<void>
  mode?: 'create' | 'add-to'
  existingLists?: Array<{ id: number; name: string }>
  selectedListId?: number
  onSelectList?: (id: number) => void
  initialName?: string
  initialKeywords?: string
}

export function CreateListModal({
  isOpen,
  onClose,
  onCreate,
  mode = 'create',
  existingLists = [],
  selectedListId,
  onSelectList,
  initialName = '',
  initialKeywords = '',
}: CreateListModalProps) {
  const [name, setName] = useState(initialName)
  const [keywords, setKeywords] = useState(initialKeywords)
  const [creating, setCreating] = useState(false)

  if (!isOpen) return null

  const handleCreate = async () => {
    if (!name.trim()) return
    setCreating(true)
    console.log('CreateListModal: calling onCreate with:', name.trim(), 'keywords:', keywords.trim())
    try {
      await onCreate(name.trim(), keywords.trim())
      console.log('CreateListModal: onCreate completed')
      setName('')
      setKeywords('')
      onClose()
    } catch (err) {
      console.error('Error creating list:', err)
    } finally {
      setCreating(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleCreate()
    } else if (e.key === 'Escape') {
      onClose()
    }
  }

  if (mode === 'add-to' && existingLists.length > 0) {
    return (
      <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={onClose}>
        <div
          className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md"
          onClick={e => e.stopPropagation()}
        >
          <div className="flex items-center justify-between px-6 py-4 border-b dark:border-gray-700">
            <div>
              <h2 className="text-lg font-bold">Añadir a lista</h2>
              <p className="text-sm text-gray-500 mt-0.5">Selecciona una lista o crea una nueva</p>
            </div>
            <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
              <X className="h-5 w-5" />
            </button>
          </div>

          <div className="p-6">
            {existingLists.length > 0 && (
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">
                  Listas existentes
                </label>
                <div className="space-y-2 max-h-48 overflow-y-auto">
                  {existingLists.map(list => (
                    <button
                      key={list.id}
                      onClick={() => {
                        onSelectList?.(list.id)
                      }}
                      className={`w-full text-left px-3 py-2 rounded-lg border transition-colors ${
                        selectedListId === list.id
                          ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/30'
                          : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'
                      }`}
                    >
                      {list.name}
                    </button>
                  ))}
                </div>
              </div>
            )}
            <div className="border-t dark:border-gray-700 pt-4">
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">
                O crear nueva lista
              </label>
              <div className="space-y-2">
                <input
                  type="text"
                  value={name}
                  onChange={e => setName(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Nombre de la lista..."
                  className="input w-full"
                  autoFocus
                />
                <input
                  type="text"
                  value={keywords}
                  onChange={e => setKeywords(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Palabras clave (separadas por comas)..."
                  className="input w-full"
                />
                <button
                  onClick={handleCreate}
                  disabled={!name.trim() || creating}
                  className="btn-primary w-full disabled:opacity-50"
                >
                  {creating ? 'Creando...' : 'Crear'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // create mode
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={onClose}>
      <div
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b dark:border-gray-700">
          <div>
            <h2 className="text-lg font-bold">Nueva lista</h2>
            <p className="text-sm text-gray-500 mt-0.5">Crea una lista para agrupar noticias</p>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">
            Nombre de la lista
          </label>
          <input
            type="text"
            value={name}
            onChange={e => setName(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="ej: Elecciones 2026, Tech News..."
            className="input w-full mb-4"
            autoFocus
          />
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">
            Palabras clave <span className="text-gray-400 font-normal">(separadas por comas)</span>
          </label>
          <input
            type="text"
            value={keywords}
            onChange={e => setKeywords(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="ej: Biden, elecciones, Estados Unidos"
            className="input w-full mb-4"
          />
          <p className="text-xs text-gray-500 mb-4">
            El sistema buscará noticias relacionadas con estas palabras clave automáticamente
          </p>
          <div className="flex gap-3">
            <button onClick={onClose} className="flex-1 btn-secondary">
              Cancelar
            </button>
            <button
              onClick={handleCreate}
              disabled={!name.trim() || creating}
              className="flex-1 btn-primary disabled:opacity-50"
            >
              {creating ? 'Creando...' : 'Crear lista'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
