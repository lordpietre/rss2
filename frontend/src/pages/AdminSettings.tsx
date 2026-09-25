import { useEffect, useState } from 'react'
import { apiService } from '../services/api'
import type { Country } from '../services/api'
import { AlertTriangle, Database, RefreshCw, Download, FileArchive, Rss, Upload } from 'lucide-react'

export function AdminSettings() {
  const [resetting, setResetting] = useState(false)
  const [restoring, setRestoring] = useState(false)
  const [restoreFile, setRestoreFile] = useState<File | null>(null)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [paisFilter, setPaisFilter] = useState('')
  const [paises, setPaises] = useState<Country[]>([])

  useEffect(() => {
    apiService.getCountries().then(setPaises).catch(() => setPaises([]))
  }, [])

  const handleExportFeeds = async () => {
    try {
      const blob = await apiService.exportFeeds({ pais_id: paisFilter || undefined })
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'feeds_export.csv'
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } catch {
      setMessage({ type: 'error', text: 'Error al exportar feeds' })
    }
  }

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

  const handleRestore = async () => {
    if (!restoreFile) {
      setMessage({ type: 'error', text: 'Selecciona un archivo de copia de seguridad (.sql o .zip)' })
      return
    }

    if (!confirm('¿Restaurar la base de datos?\n\nEsta acción sobrescribirá los datos existentes con el contenido del archivo de copia de seguridad. No se puede deshacer.')) return

    setRestoring(true)
    setMessage(null)

    try {
      const result = await apiService.restoreDatabase(restoreFile)
      setMessage({ type: 'success', text: result.message })
      setRestoreFile(null)
    } catch (err: any) {
      setMessage({
        type: 'error',
        text: err?.response?.data?.details || err?.response?.data?.error || 'Error al restaurar la base de datos',
      })
    } finally {
      setRestoring(false)
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-6 flex items-center gap-2">
        <Database className="h-6 w-6" />
        Configuración del Sistema
      </h1>

      {message && (
        <div className={`max-w-2xl mb-6 p-3 rounded-md text-sm ${
          message.type === 'success' 
            ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-200'
            : 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-200'
        }`}>
          {message.text}
        </div>
      )}

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
                  <Rss className="h-4 w-4 text-blue-600" />
                  Feeds (CSV por país)
                </h3>
                <p className="text-sm text-gray-500 mb-4">
                  Descarga un archivo CSV con feeds filtrados por país. Ideal para respaldar fuentes específicas.
                </p>
                <div className="flex items-center gap-3 flex-wrap">
                  <select
                    value={paisFilter}
                    onChange={(e) => setPaisFilter(e.target.value)}
                    className="input w-auto"
                  >
                    <option value="">Todos los países</option>
                    {paises?.map((pais) => (
                      <option key={pais.id} value={pais.id}>{pais.nombre}</option>
                    ))}
                  </select>
                  <button
                    onClick={handleExportFeeds}
                    className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
                  >
                    <Download className="h-4 w-4" />
                    Descargar Backup Feeds
                  </button>
                </div>
              </div>

            <div className="p-4 border border-gray-100 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900/50">
              <h3 className="font-medium mb-1 flex items-center gap-2">
                <Upload className="h-4 w-4 text-blue-600" />
                Restaurar Base de Datos
              </h3>
              <p className="text-sm text-gray-500 mb-4">
                Sube un archivo de copia de seguridad (.sql o .zip) para restaurar la base de datos. 
                La restauración sobrescribe los datos existentes. Se recomienda restaurar después de un reset.
              </p>
              <div className="flex items-center gap-3 flex-wrap">
                <input
                  type="file"
                  accept=".sql,.zip"
                  onChange={(e) => setRestoreFile(e.target.files?.[0] ?? null)}
                  className="block w-full max-w-sm text-sm text-gray-500 dark:text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:bg-blue-600 file:text-white file:cursor-pointer hover:file:bg-blue-700"
                />
                <button
                  onClick={handleRestore}
                  disabled={restoring || !restoreFile}
                  className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {restoring ? (
                    <>
                      <RefreshCw className="h-4 w-4 animate-spin" />
                      Restaurando...
                    </>
                  ) : (
                    <>
                      <Upload className="h-4 w-4" />
                      Restaurar Base de Datos
                    </>
                  )}
                </button>
              </div>
              {restoreFile && (
                <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  Archivo seleccionado: {restoreFile.name}
                </p>
              )}
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
