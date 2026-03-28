import { useState } from 'react'
import { apiService } from '../services/api'
import { AlertTriangle, Database, RefreshCw, Download, FileArchive } from 'lucide-react'

export function AdminSettings() {
  const [resetting, setResetting] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const handleReset = async () => {
    const confirmMsg = '¿Estás seguro de que quieres BORRAR TODA LA BASE DE DATOS?\n\nEsta acción eliminará:\n- Todas las noticias\n- Todos los feeds\n- Todas las traducciones\n- Todos los favoritos\n- Todos los alias\n\nEsta acción NO se puede deshacer.'
    
    if (!confirm(confirmMsg)) return
    
    if (!confirm('¿REALMENTE ESTÁS SEGURO? Escribe "SI" para confirmar.')) return
    
    const input = prompt('Escribe "SI" para confirmar el borrado total:')
    if (input !== 'SI') {
      setMessage({ type: 'error', text: 'Cancelado: no escribiste "SI"' })
      return
    }

    setResetting(true)
    setMessage(null)
    
    try {
      const result = await apiService.resetDatabase()
      setMessage({ type: 'success', text: result.message })
    } catch (err: any) {
      setMessage({ 
        type: 'error', 
        text: err?.response?.data?.error || 'Error al resetear la base de datos' 
      })
    } finally {
      setResetting(false)
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-6 flex items-center gap-2">
        <Database className="h-6 w-6" />
        Configuración del Sistema
      </h1>

      <div className="max-w-2xl space-y-6">
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-red-600" />
            Zona de Peligro
          </h2>
          
          <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
            <h3 className="font-medium text-red-800 dark:text-red-200 mb-2">
              Resetear Base de Datos
            </h3>
            <p className="text-sm text-red-600 dark:text-red-300 mb-4">
              Eliminará todas las noticias, feeds, traducciones, favoritos y alias. 
              Los usuarios NO serán eliminados. Esta acción no se puede deshacer.
            </p>
            
            <button
              onClick={handleReset}
              disabled={resetting}
              className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {resetting ? (
                <>
                  <RefreshCw className="h-4 w-4 animate-spin" />
                  Reseteando...
                </>
              ) : (
                <>
                  <AlertTriangle className="h-4 w-4" />
                  Resetear Base de Datos
                </>
              )}
            </button>

            {message && (
              <div className={`mt-4 p-3 rounded-md text-sm ${
                message.type === 'success' 
                  ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-200'
                  : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-200'
              }`}>
                {message.text}
              </div>
            )}
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Download className="h-5 w-5 text-blue-600" />
            Copia de Seguridad
          </h2>

          <div className="space-y-4">
            <div className="p-4 border border-gray-100 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900/50">
              <h3 className="font-medium mb-1 flex items-center gap-2">
                <FileArchive className="h-4 w-4" />
                Noticias y Traducciones (ZIP)
              </h3>
              <p className="text-sm text-gray-500 mb-4">
                Descarga un archivo comprimido con todas las noticias, traducciones y sus etiquetas (tags). 
                Ideal para respaldar el contenido generado sin incluir configuraciones de sistema.
              </p>
              <button
                onClick={() => apiService.backupNewsZipped()}
                className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
              >
                <Download className="h-4 w-4" />
                Descargar Backup .zip
              </button>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold mb-4">Información del Sistema</h2>
          <div className="space-y-2 text-sm text-gray-600 dark:text-gray-400">
            <p>• Los alias permiten unificar entidades (ej: "Starmer" → "Keir Starmer")</p>
            <p>• Solo los administradores pueden gestionar usuarios y alias</p>
            <p>• El primer usuario registrado se convierte en administrador automáticamente</p>
          </div>
        </div>
      </div>
    </div>
  )
}
