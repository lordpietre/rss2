import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiService, api, Alerta, News } from '../services/api'
import { Bell, CheckCheck, ChevronDown, ChevronUp, FileText } from 'lucide-react'

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

  const [newsModal, setNewsModal] = useState<Alerta | null>(null)
  const [entityNews, setEntityNews] = useState<News[]>([])
  const [newsLoading, setNewsLoading] = useState(false)
  const [newsTotal, setNewsTotal] = useState(0)

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

  const openNews = async (alerta: Alerta) => {
    setNewsModal(alerta)
    setNewsLoading(true)
    setEntityNews([])
    setNewsTotal(0)
    try {
      const params = new URLSearchParams()
      params.append('valor', alerta.valor)
      params.append('tipo', alerta.tipo)
      params.append('per_page', '50')
      const res = await api.get(`/entities/news?${params}`)
      setEntityNews(res.data.news || [])
      setNewsTotal(res.data.total || 0)
    } catch (err) {
      console.error(err)
    } finally {
      setNewsLoading(false)
    }
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
                    <div className="flex items-center gap-2">
                      <button onClick={() => openNews(a)} className="btn-secondary text-xs py-1 px-3 flex items-center gap-1">
                        <FileText className="h-3.5 w-3.5" />
                        Noticias
                      </button>
                      {a.status === 'nueva' && (
                        <button onClick={() => markRead(a.id)} className="btn-secondary text-xs py-1 px-3">
                          Marcar leída
                        </button>
                      )}
                    </div>
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

      {/* Noticias relacionadas del concepto */}
      {newsModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setNewsModal(null)}>
          <div
            className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-2xl max-h-[85vh] overflow-hidden flex flex-col"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-6 py-4 border-b dark:border-gray-700">
              <div>
                <h2 className="text-lg font-bold">Noticias sobre &quot;{newsModal.valor}&quot;</h2>
                <p className="text-sm text-gray-500 mt-0.5">
                  {newsTotal} noticias encontradas · {TIPO_LABEL[newsModal.tipo] || newsModal.tipo} · {newsModal.periodo}
                </p>
              </div>
              <button
                onClick={() => setNewsModal(null)}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 text-xl leading-none"
              >
                ✕
              </button>
            </div>

            <div className="p-4 overflow-y-auto">
              {newsLoading ? (
                <div className="text-center py-10 text-gray-400">Cargando...</div>
              ) : entityNews.length === 0 ? (
                <div className="text-center py-10 text-gray-400">No hay noticias que mencionen a este concepto.</div>
              ) : (
                <ul className="space-y-2">
                  {entityNews.map((n) => (
                    <li key={n.id}>
                      <Link
                        to={`/news/${n.id}`}
                        onClick={() => setNewsModal(null)}
                        className="block p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors border border-gray-100 dark:border-gray-700"
                      >
                        <div className="flex items-start gap-3">
                          {n.imagen_url && (
                            <img
                              src={n.imagen_url}
                              alt=""
                              className="w-20 h-14 object-cover rounded-md shrink-0"
                              onError={(e) => (e.currentTarget.style.display = 'none')}
                            />
                          )}
                          <div className="min-w-0">
                            <div className="font-medium text-sm line-clamp-2">
                              {n.title_translated || n.titulo}
                            </div>
                            <p className="text-xs text-gray-500 mt-1 line-clamp-2">
                              {n.summary_translated || n.resumen}
                            </p>
                            {n.fecha && (
                              <span className="text-[11px] text-gray-400 mt-1 block">
                                {new Date(n.fecha).toLocaleDateString('es-ES')}
                              </span>
                            )}
                          </div>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}