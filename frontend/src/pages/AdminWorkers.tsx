import { useState, useEffect } from 'react'
import { api } from '../services/api'
import { Cpu, Play, Square, Settings, RefreshCw, Loader2, Server } from 'lucide-react'

interface WorkerStatus {
  type: string
  workers: number
  status: string
  running: number
}

export function AdminWorkers() {
  const [status, setStatus] = useState<WorkerStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [configType, setConfigType] = useState<'cpu' | 'gpu'>('cpu')
  const [configWorkers, setConfigWorkers] = useState(2)

  useEffect(() => {
    fetchStatus()
  }, [])

  const fetchStatus = async () => {
    setLoading(true)
    try {
      const res = await api.get('/admin/workers/status')
      setStatus(res.data)
      setConfigType(res.data.type)
      setConfigWorkers(res.data.workers)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  const handleStart = async () => {
    setActionLoading('start')
    try {
      await api.post('/admin/workers/start', {
        type: configType,
        workers: configWorkers
      })
      await fetchStatus()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error al iniciar workers')
    } finally {
      setActionLoading(null)
    }
  }

  const handleStop = async () => {
    setActionLoading('stop')
    try {
      await api.post('/admin/workers/stop')
      await fetchStatus()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error al detener workers')
    } finally {
      setActionLoading(null)
    }
  }

  const handleConfig = async () => {
    setActionLoading('config')
    try {
      await api.post('/admin/workers/config', {
        type: configType,
        workers: configWorkers
      })
      await fetchStatus()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error al guardar configuración')
    } finally {
      setActionLoading(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-primary-600" />
      </div>
    )
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <Server className="h-6 w-6" />
          Workers de Traducción
        </h1>
        <button
          onClick={fetchStatus}
          className="btn-secondary flex items-center gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Actualizar
        </button>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Settings className="h-5 w-5" />
            Configuración
          </h2>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium mb-2">Tipo de procesamiento</label>
              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => setConfigType('cpu')}
                  className={`p-3 rounded-lg border-2 transition-all flex flex-col items-center ${
                    configType === 'cpu'
                      ? 'border-green-500 bg-green-50 dark:bg-green-900/20'
                      : 'border-gray-200 dark:border-gray-700 hover:border-green-300'
                  }`}
                >
                  <Cpu className="h-6 w-6 text-green-600 mb-1" />
                  <span className="font-medium">CPU</span>
                </button>
                <button
                  type="button"
                  onClick={() => setConfigType('gpu')}
                  className={`p-3 rounded-lg border-2 transition-all flex flex-col items-center ${
                    configType === 'gpu'
                      ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                      : 'border-gray-200 dark:border-gray-700 hover:border-blue-300'
                  }`}
                >
                  <svg className="h-6 w-6 text-blue-600 mb-1" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                  </svg>
                  <span className="font-medium">GPU</span>
                </button>
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium mb-2">Número de workers</label>
              <div className="flex flex-wrap gap-2">
                {[1, 2, 3, 4, 5, 6, 7, 8].map((num) => (
                  <button
                    key={num}
                    type="button"
                    onClick={() => setConfigWorkers(num)}
                    className={`w-10 h-10 rounded-lg font-medium transition-all ${
                      configWorkers === num
                        ? 'bg-primary-600 text-white'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
                    }`}
                  >
                    {num}
                  </button>
                ))}
              </div>
            </div>

            <button
              onClick={handleConfig}
              disabled={actionLoading === 'config'}
              className="btn-primary w-full flex items-center justify-center gap-2"
            >
              {actionLoading === 'config' ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Settings className="h-4 w-4" />
              )}
              Guardar Configuración
            </button>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
            <Server className="h-5 w-5" />
            Estado Actual
          </h2>

          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <span className="font-medium">Estado</span>
              <span className={`px-3 py-1 rounded-full text-sm font-medium ${
                status?.status === 'running'
                  ? 'bg-green-100 text-green-800'
                  : 'bg-gray-100 text-gray-800'
              }`}>
                {status?.status === 'running' ? 'Activo' : 'Detenido'}
              </span>
            </div>

            <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <span className="font-medium">Tipo</span>
              <span className={`px-3 py-1 rounded-full text-sm font-medium ${
                status?.type === 'gpu'
                  ? 'bg-blue-100 text-blue-800'
                  : 'bg-green-100 text-green-800'
              }`}>
                {status?.type === 'gpu' ? 'GPU' : 'CPU'}
              </span>
            </div>

            <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <span className="font-medium">Workers configurados</span>
              <span className="text-lg font-bold">{status?.workers}</span>
            </div>

            <div className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <span className="font-medium">Workers activos</span>
              <span className="text-lg font-bold">{status?.running || 0}</span>
            </div>

            <div className="flex gap-3 pt-2">
              {status?.status === 'running' ? (
                <button
                  onClick={handleStop}
                  disabled={actionLoading === 'stop'}
                  className="btn-danger flex-1 flex items-center justify-center gap-2"
                >
                  {actionLoading === 'stop' ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Square className="h-4 w-4" />
                  )}
                  Detener
                </button>
              ) : (
                <button
                  onClick={handleStart}
                  disabled={actionLoading === 'start'}
                  className="btn-primary flex-1 flex items-center justify-center gap-2"
                >
                  {actionLoading === 'start' ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Play className="h-4 w-4" />
                  )}
                  Iniciar
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      <div className="mt-6 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
        <h3 className="font-medium text-blue-800 dark:text-blue-200 mb-2">Información</h3>
        <ul className="text-sm text-blue-700 dark:text-blue-300 space-y-1">
          <li>• CPU: Más lento pero funciona en cualquier equipo</li>
          <li>• GPU: Mucho más rápido, necesita tarjeta NVIDIA</li>
          <li>• Los cambios en la configuración se aplicarán al iniciar los workers</li>
          <li>• Recomendado: 2-4 workers para CPU, 1-2 para GPU</li>
        </ul>
      </div>
    </div>
  )
}
