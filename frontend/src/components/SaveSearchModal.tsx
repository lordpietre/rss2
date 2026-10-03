import { useState } from 'react'
import { X } from 'lucide-react'

interface SaveSearchModalProps {
  isOpen: boolean
  onClose: () => void
  onSave: (label: string) => void
  typeLabel: string
}

export function SaveSearchModal({ isOpen, onClose, onSave, typeLabel }: SaveSearchModalProps) {
  const [label, setLabel] = useState('')
  const [saving, setSaving] = useState(false)

  if (!isOpen) return null

  const handleSave = () => {
    if (!label.trim()) return
    setSaving(true)
    onSave(label.trim())
    setLabel('')
    setSaving(false)
    onClose()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSave()
    } else if (e.key === 'Escape') {
      onClose()
    }
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={onClose}>
      <div
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b dark:border-gray-700">
          <div>
            <h2 className="text-lg font-bold">Guardar búsqueda</h2>
            <p className="text-sm text-gray-500 mt-0.5">Guardar {typeLabel} en favoritos</p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-xl leading-none"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="p-6">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-2">
            Nombre para esta búsqueda
          </label>
          <input
            type="text"
            value={label}
            onChange={e => setLabel(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="ej: Noticias de tecnología"
            className="input w-full mb-4"
            autoFocus
          />
          <div className="flex gap-3">
            <button
              onClick={onClose}
              className="flex-1 btn-secondary"
            >
              Cancelar
            </button>
            <button
              onClick={handleSave}
              disabled={!label.trim() || saving}
              className="flex-1 btn-primary disabled:opacity-50"
            >
              {saving ? 'Guardando...' : 'Guardar'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
