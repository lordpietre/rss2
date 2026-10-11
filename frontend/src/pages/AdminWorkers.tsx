import { useState, useEffect } from 'react'
import { api } from '../services/api'
import { Cpu, Play, Square, Settings, RefreshCw, Loader2, Server, Wifi, Plus, Trash2, Key, TrendingUp } from 'lucide-react'

interface WorkerStatus {
  type: string
  workers: number
  status: string
  running: number
}

interface TranslationStats {
  translations_last_1min: number
  translations_last_5min: number
  rate_per_second: number
  rate_per_minute: number
  active_workers: number
  remote_workers_online: number
}

interface RemoteWorker {
  id: number
  name: string
  api_key?: string
  capabilities: string
  status: string
  last_seen?: string
  created_at: string
}

export function AdminWorkers() {
  const [status, setStatus] = useState<WorkerStatus | null>(null)
  const [tStats, setTStats] = useState<TranslationStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [configType, setConfigType] = useState<'cpu' | 'gpu'>('cpu')
  const [configWorkers, setConfigWorkers] = useState(2)

  const [remoteWorkers, setRemoteWorkers] = useState<RemoteWorker[]>([])
  const [showAddForm, setShowAddForm] = useState(false)
  const [newWorkerName, setNewWorkerName] = useState('')
  const [newWorkerCapabilities, setNewWorkerCapabilities] = useState('cpu')
  const [newWorkerKey, setNewWorkerKey] = useState<string | null>(null)

  const fetchStatus = async () => {
    setLoading(true)
    try {
      const res = await api.get('/admin/workers/status')
      setStatus(res.data)
      setConfigType(res.data.type)
      setConfigWorkers(res.data.workers)

      const statsRes = await api.get('/admin/workers/stats')
      setTStats(statsRes.data)
    } catch (err: any) {
      console.error('Error fetching status:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchRemoteWorkers = async () => {
    try {
      const res = await api.get('/admin/workers/remote')
      setRemoteWorkers(res.data)
    } catch (err: any) {
      console.error('Error fetching remote workers:', err)
    }
  }

  useEffect(() => {
    fetchStatus()
    fetchRemoteWorkers()
  }, [])

  const handleStart = async () => {
    setActionLoading('start')
    try {
      await api.post('/admin/workers/start', {
        type: configType,
        workers: configWorkers
      })
      await fetchStatus()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error starting workers')
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
      alert(err.response?.data?.error || 'Error stopping workers')
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
      alert(err.response?.data?.error || 'Error saving configuration')
    } finally {
      setActionLoading(null)
    }
  }

  const handleAddRemoteWorker = async () => {
    if (!newWorkerName.trim()) return
    setActionLoading('add')
    try {
      const res = await api.post('/admin/workers/remote', {
        name: newWorkerName,
        capabilities: newWorkerCapabilities
      })
      setNewWorkerKey(res.data.api_key)
      setNewWorkerName('')
      fetchRemoteWorkers()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error creating worker')
    } finally {
      setActionLoading(null)
    }
  }

  const handleDeleteRemoteWorker = async (id: number) => {
    if (!confirm('Delete this remote worker?')) return
    try {
      await api.delete(`/admin/workers/remote/${id}`)
      fetchRemoteWorkers()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error deleting worker')
    }
  }

  const handleToggleRemoteWorker = async (id: number) => {
    try {
      await api.post(`/admin/workers/remote/${id}/toggle`)
      fetchRemoteWorkers()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error changing status')
    }
  }

  const handleRegenerateKey = async (id: number) => {
    if (!confirm('Regenerate API key? The previous one will stop working.')) return
    try {
      const res = await api.post(`/admin/workers/remote/${id}/regenerate-key`)
      setNewWorkerKey(res.data.api_key)
    } catch (err: any) {
      alert(err.response?.data?.error || 'Error regenerating key')
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-primary-50" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50 flex items-center gap-3">
            <Server className="h-6 w-6 text-primary-50" />
            System Status
          </h1>
          <p className="text-sm text-gray-400 mt-1">Translation workers and remote worker management</p>
        </div>
        <button
          onClick={fetchStatus}
          className="btn-secondary flex items-center gap-2"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </button>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Configuration */}
        <div className="panel">
          <h2 className="text-lg font-semibold text-gray-50 mb-4 flex items-center gap-2">
            <Settings className="h-5 w-5" />
            Configuration
          </h2>

          <div className="space-y-4">
            <div>
              <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Processing Type</label>
              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => setConfigType('cpu')}
                  className={`p-3 rounded-lg border-2 transition-all flex flex-col items-center ${
                    configType === 'cpu'
                      ? 'border-primary-50 bg-primary-900/30'
                      : 'border-gray-700 hover:border-primary-500/50'
                  }`}
                >
                  <Cpu className="h-6 w-6 text-primary-50 mb-1" />
                  <span className="font-medium text-gray-50">CPU</span>
                </button>
                <button
                  type="button"
                  onClick={() => setConfigType('gpu')}
                  className={`p-3 rounded-lg border-2 transition-all flex flex-col items-center ${
                    configType === 'gpu'
                      ? 'border-blue-400 bg-blue-900/30'
                      : 'border-gray-700 hover:border-blue-400/50'
                  }`}
                >
                  <svg className="h-6 w-6 text-blue-400 mb-1" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                  </svg>
                  <span className="font-medium text-gray-50">GPU</span>
                </button>
              </div>
            </div>

            <div>
              <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Number of Workers</label>
              <div className="flex flex-wrap gap-2">
                {[1, 2, 3, 4, 5, 6, 7, 8].map((num) => (
                  <button
                    key={num}
                    type="button"
                    onClick={() => setConfigWorkers(num)}
                    className={`w-10 h-10 rounded-lg font-medium transition-all ${
                      configWorkers === num
                        ? 'bg-primary-50 text-gray-900'
                        : 'bg-gray-800 text-gray-300 hover:bg-gray-700'
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
              Save Configuration
            </button>
          </div>
        </div>

        {/* Current Status */}
        <div className="panel">
          <h2 className="text-lg font-semibold text-gray-50 mb-4 flex items-center gap-2">
            <Server className="h-5 w-5" />
            Current Status
          </h2>

          <div className="space-y-4">
            <div className="flex items-center justify-between p-4 bg-gray-800 rounded-lg">
              <span className="font-medium text-gray-300">Status</span>
              <span className={`px-3 py-1 rounded-full text-sm font-medium ${
                status?.status === 'running'
                  ? 'bg-primary-900/30 text-primary-50 border border-primary-500/30'
                  : 'bg-gray-700 text-gray-300'
              }`}>
                {status?.status === 'running' ? 'Active' : 'Stopped'}
              </span>
            </div>

            <div className="flex items-center justify-between p-4 bg-gray-800 rounded-lg">
              <span className="font-medium text-gray-300">Type</span>
              <span className={`px-3 py-1 rounded-full text-sm font-medium ${
                status?.type === 'gpu'
                  ? 'bg-blue-900/30 text-blue-400 border border-blue-500/30'
                  : 'bg-primary-900/30 text-primary-50 border border-primary-500/30'
              }`}>
                {status?.type === 'gpu' ? 'GPU' : 'CPU'}
              </span>
            </div>

            <div className="flex items-center justify-between p-4 bg-gray-800 rounded-lg">
              <span className="font-medium text-gray-300">Workers Configured</span>
              <span className="text-lg font-bold text-gray-50">{status?.workers}</span>
            </div>

            <div className="flex items-center justify-between p-4 bg-gray-800 rounded-lg">
              <span className="font-medium text-gray-300">Active Workers</span>
              <span className="text-lg font-bold text-primary-50">{tStats?.active_workers ?? status?.running ?? 0}</span>
            </div>

            <div className="rounded-lg border border-primary-500/20 p-4">
              <h3 className="text-sm font-semibold text-gray-50 mb-3 flex items-center gap-2">
                <TrendingUp className="h-4 w-4 text-primary-50" />
                Translation Performance
              </h3>
              <div className="grid grid-cols-3 gap-3 text-center">
                <div className="p-3 bg-gray-800 rounded-lg">
                  <div className="text-xl font-bold text-primary-50">{tStats?.rate_per_second ?? 0}</div>
                  <div className="text-xs text-gray-500">news/sec</div>
                </div>
                <div className="p-3 bg-gray-800 rounded-lg">
                  <div className="text-xl font-bold text-primary-50">{tStats?.rate_per_minute ?? 0}</div>
                  <div className="text-xs text-gray-500">news/min</div>
                </div>
                <div className="p-3 bg-gray-800 rounded-lg">
                  <div className="text-xl font-bold text-primary-50">{tStats?.translations_last_1min ?? 0}</div>
                  <div className="text-xs text-gray-500">last 1 min</div>
                </div>
              </div>
              <div className="text-xs text-gray-500 mt-2">
                {tStats?.translations_last_5min ?? 0} translated in last 5 min · {tStats?.remote_workers_online ?? 0} remote online
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              {status?.status === 'running' ? (
                <button
                  onClick={handleStop}
                  disabled={actionLoading === 'stop'}
                  className="btn-secondary flex-1 flex items-center justify-center gap-2 border-rss-red/50 text-rss-red hover:bg-rss-red/10"
                >
                  {actionLoading === 'stop' ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <Square className="h-4 w-4" />
                  )}
                  Stop
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
                  Start
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Remote Workers */}
      <div className="panel">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2">
            <Wifi className="h-5 w-5" />
            Remote Workers
          </h2>
          <button
            onClick={() => setShowAddForm(true)}
            className="btn-primary flex items-center gap-2"
          >
            <Plus className="h-4 w-4" />
            Add Worker
          </button>
        </div>

        {showAddForm && (
          <div className="mb-6 p-4 bg-gray-800 rounded-lg">
            <h3 className="font-medium text-gray-50 mb-3">New Remote Worker</h3>
            <div className="flex gap-3 mb-3">
              <input
                type="text"
                placeholder="Worker name"
                value={newWorkerName}
                onChange={(e) => setNewWorkerName(e.target.value)}
                className="input flex-1"
              />
              <select
                value={newWorkerCapabilities}
                onChange={(e) => setNewWorkerCapabilities(e.target.value)}
                className="input w-32"
              >
                <option value="cpu">CPU</option>
                <option value="gpu">GPU</option>
              </select>
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleAddRemoteWorker}
                disabled={actionLoading === 'add'}
                className="btn-primary"
              >
                {actionLoading === 'add' ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Create'}
              </button>
              <button
                onClick={() => { setShowAddForm(false); setNewWorkerKey(null) }}
                className="btn-secondary"
              >
                Cancel
              </button>
            </div>
          </div>
        )}

        {newWorkerKey && (
          <div className="mb-4 p-4 bg-primary-900/20 border border-primary-500/30 rounded-lg">
            <div className="flex items-center gap-2 mb-2">
              <Key className="h-4 w-4 text-primary-50" />
              <span className="font-medium text-primary-50">API Key Created</span>
            </div>
            <code className="block p-2 bg-gray-900 rounded text-sm break-all font-mono text-gray-300">
              {newWorkerKey}
            </code>
            <p className="text-sm text-gray-400 mt-2">
              Copy this key. It will not be shown again.
            </p>
            <button
              onClick={() => setNewWorkerKey(null)}
              className="mt-2 text-sm text-primary-300 underline"
            >
              Hide
            </button>
          </div>
        )}

        {remoteWorkers.length === 0 ? (
          <p className="text-gray-500 text-center py-8">No remote workers configured</p>
        ) : (
          <div className="space-y-3">
            {remoteWorkers.map((worker) => (
              <div key={worker.id} className="flex items-center justify-between p-4 bg-gray-800 rounded-lg">
                <div className="flex items-center gap-3">
                  <div className={`w-3 h-3 rounded-full ${worker.status === 'online' ? 'bg-primary-50 animate-pulse' : 'bg-gray-600'}`} />
                  <div>
                    <span className="font-medium text-gray-50">{worker.name}</span>
                    <span className="ml-2 text-sm text-gray-500 font-mono">{worker.capabilities}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleToggleRemoteWorker(worker.id)}
                    className="px-3 py-1 text-xs rounded border border-gray-700 hover:bg-gray-700"
                  >
                    {worker.status === 'disabled' ? 'Enable' : 'Disable'}
                  </button>
                  <button
                    onClick={() => handleRegenerateKey(worker.id)}
                    className="p-2 text-gray-400 hover:text-gray-200"
                    title="Regenerate API Key"
                  >
                    <Key className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => handleDeleteRemoteWorker(worker.id)}
                    className="p-2 text-gray-400 hover:text-rss-red"
                    title="Delete"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
