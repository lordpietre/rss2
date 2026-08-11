import { useEffect, useState } from 'react'
import { apiService, Alerta } from '../services/api'
import { Bell, CheckCheck, ChevronDown, ChevronUp } from 'lucide-react'

const TIPO_LABEL: Record<string, string> = {
  persona: 'Persona',
  lugar: 'Lugar',
  organizacion: 'Organización',
  tema: 'Tema',
}

export function Alertas() {
  const [alertas, setAlertas] = useState<Alerta[]>([])
  const [loading, setLoading] = useState(true)
  const [nuevas, setNuevas] = useState(0)
  const [limit, setLimit] = useState(50)

  const load = async () => {
    setLoading(true)
    try {
      const r = await apiService.getAlertas({ limit })
      setAlertas(r.alertas)
      setNuevas(r.nuevas)
    } catch {
      setAlertas([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [limit])

  const markRead = async (id: number) => {
    await apiService.markAlertaRead(id)
    load()
  }

  const markAll = async () => {
    await apiService.markAllAlertasRead()
    load()
  }

  return (
    <div>
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <Bell className="h-7 w-7 text-primary-600" />
          Alertas de actividad
        </h1>
        <button onClick={markAll} className="btn-secondary flex items-center gap-2" disabled={nuevas === 0}>
          <CheckCheck className="h-4 w-4" />
          Marcar todas como leídas
        </button>
      </div>

      <p className="text-gray-500 dark:text-gray-400 -mt-4 mb-6">
        Se genera un aviso cuando un concepto supera con claridad su actividad media de días anteriores.
        {nuevas > 0 && (
          <span className="ml-1 text-primary-600 font-semibold">Tienes {nuevas} sin leer.</span>
        )}
      </p>

      {loading ? (
        <div className="card p-16 flex justify-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-600" />
        </div>
      ) : alertas.length === 0 ? (
        <div className="card p-16 text-center text-gray-500 dark:text-gray-400">
          No hay alertas todavía. El sistema revisa cada dos horas los picos de actividad.
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-800 border-b">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Concepto</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Tipo</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Fecha</th>
                <th className="px-4 py-3 text-right text-sm font-semibold text-gray-900 dark:text-white">Noticias</th>
                <th className="px-4 py-3 text-right text-sm font-semibold text-gray-900 dark:text-white">Media previa</th>
                <th className="px-4 py-3 text-right text-sm font-semibold text-gray-900 dark:text-white">Ratio</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Estado</th>
                <th className="px-4 py-3 text-left text-sm font-semibold text-gray-900 dark:text-white">Acciones</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {alertas.map((a) => (
                <tr key={a.id} className="hover:bg-gray-50 dark:hover:bg-gray-800">
                  <td className="px-4 py-3 font-medium text-gray-900 dark:text-white">{a.valor}</td>
                  <td className="px-4 py-3">
                    <span className="text-xs px-2 py-1 rounded bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
                      {TIPO_LABEL[a.tipo] || a.tipo}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">{a.periodo}</td>
                  <td className="px-4 py-3 text-right text-sm text-gray-900 dark:text-white">{a.hits}</td>
                  <td className="px-4 py-3 text-right text-sm text-gray-500 dark:text-gray-400">{a.baseline.toFixed(1)}</td>
                  <td className="px-4 py-3 text-right">
                    <span className={`text-sm font-semibold ${a.ratio >= 10 ? 'text-red-600' : 'text-orange-600'}`}>
                      ×{a.ratio.toFixed(1)}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-2 py-1 rounded ${a.status === 'nueva' ? 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300' : 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'}`}>
                      {a.status === 'nueva' ? 'Nueva' : 'Leída'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    {a.status === 'nueva' ? (
                      <button onClick={() => markRead(a.id)} className="btn-secondary text-xs py-1 px-3">
                        Marcar leída
                      </button>
                    ) : (
                      <span className="text-sm text-gray-300 dark:text-gray-600">—</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="mt-6 flex items-center justify-end gap-2">
        <button
          onClick={() => setLimit(limit > 50 ? 50 : 200)}
          className="btn-secondary flex items-center gap-2 text-sm"
        >
          {limit > 50 ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          Mostrar {limit > 50 ? 'menos' : 'más'} ({limit})
        </button>
      </div>
    </div>
  )
}