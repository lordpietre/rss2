import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiService, api, Alerta, News } from '../services/api'
import { CheckCheck, ChevronDown, ChevronUp, FileText, XCircle, Heart, FolderPlus } from 'lucide-react'
import { WikiTooltip } from '../components/ui/WikiTooltip'
import { CreateListModal } from '../components/CreateListModal'
import { useApiFavorites, useApiLists } from '../hooks/useApiFavorites'

const TIPO_LABEL: Record<string, string> = {
  persona: 'Person',
  lugar: 'Place',
  organizacion: 'Organization',
  tema: 'Topic',
}

export function Alertas() {
  const [alertas, setAlertas] = useState<Alerta[]>([])
  const [loading, setLoading] = useState(true)
  const [nuevas, setNuevas] = useState(0)
  const [total, setTotal] = useState(0)
  const [limit, setLimit] = useState(50)
  const [tipoFiltro, setTipoFiltro] = useState('')
  const [estadoFiltro, setEstadoFiltro] = useState('')

  const [newsModal, setNewsModal] = useState<Alerta | null>(null)
  const [entityNews, setEntityNews] = useState<News[]>([])
  const [newsLoading, setNewsLoading] = useState(false)
  const [newsTotal, setNewsTotal] = useState(0)
  // Por defecto solo se listan las noticias de la hora del pico (es lo que
  // hace que la alerta saltara). Se puede cambiar a todas las del concepto.
  const [soloHora, setSoloHora] = useState(true)

  // Favoritos y listas: las mismas acciones que en la seccion de
  // Inteligencia al seleccionar un concepto.
  const [showListModal, setShowListModal] = useState(false)
  const [selectedNewsForList, setSelectedNewsForList] = useState<News | null>(null)
  const { toggleFavorite, isFavorite } = useApiFavorites()
  const { lists, createList, addToList } = useApiLists()

  const load = async () => {
    setLoading(true)
    try {
      const r = await apiService.getAlertas({ limit, tipo: tipoFiltro || undefined, status: estadoFiltro || undefined })
      setAlertas(r.alertas)
      setNuevas(r.nuevas)
      setTotal(r.total)
    } catch {
      setAlertas([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [limit, tipoFiltro, estadoFiltro])

  const markRead = async (id: number) => {
    await apiService.markAlertaRead(id)
    load()
  }

  const dismiss = async (id: number) => {
    await apiService.dismissAlerta(id)
    load()
  }

  const markAll = async () => {
    await apiService.markAllAlertasRead()
    load()
  }

  const openNews = async (alerta: Alerta, soloPeriodo?: boolean) => {
    const soloLaHora = soloPeriodo ?? soloHora
    setNewsModal(alerta)
    setNewsLoading(true)
    setEntityNews([])
    setNewsTotal(0)
    try {
      const params = new URLSearchParams()
      params.append('valor', alerta.valor)
      params.append('tipo', alerta.tipo)
      params.append('per_page', '50')
      // La hora exacta del pico: es lo que hace que la alerta saltara.
      if (soloLaHora) params.append('periodo', alerta.periodo)
      const res = await api.get(`/entities/news?${params}`)
      setEntityNews(res.data.news || [])
      setNewsTotal(res.data.total || 0)
    } catch (err) {
      console.error(err)
    } finally {
      setNewsLoading(false)
    }
  }

  const handleAddToList = (news: News) => {
    setSelectedNewsForList(news)
    setShowListModal(true)
  }

  const handleSelectList = async (listId: number) => {
    if (selectedNewsForList) {
      await addToList(listId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
  }

  const handleCreateAndAdd = async (name: string) => {
    const newListId = await createList(name)
    if (selectedNewsForList) {
      await addToList(newListId, selectedNewsForList.id)
    }
    setShowListModal(false)
    setSelectedNewsForList(null)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">Alert Center</h1>
          <p className="text-sm text-gray-400 mt-1">Activity spike detection and monitoring</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="px-3 py-1 rounded-full bg-rss-red/20 text-rss-red text-sm font-mono">
            {nuevas} new
          </span>
        </div>
      </div>

      <div className="flex justify-between items-center flex-wrap gap-3">
        <div className="flex items-center gap-2 flex-wrap">
          <select
            value={tipoFiltro}
            onChange={(e) => setTipoFiltro(e.target.value)}
            className="input w-auto text-sm"
            aria-label="Filter by type"
          >
            <option value="">All types</option>
            <option value="persona">People</option>
            <option value="organizacion">Organizations</option>
            <option value="lugar">Places</option>
            <option value="tema">Topics</option>
          </select>
          <select
            value={estadoFiltro}
            onChange={(e) => setEstadoFiltro(e.target.value)}
            className="input w-auto text-sm"
            aria-label="Filter by status"
          >
            <option value="">Active alerts</option>
            <option value="nueva">New</option>
            <option value="leida">Read</option>
            <option value="descartada">Dismissed</option>
          </select>
          <button onClick={markAll} className="btn-secondary flex items-center gap-2" disabled={nuevas === 0}>
            <CheckCheck className="h-4 w-4" />
            Mark all read
          </button>
        </div>
      </div>

      <p className="text-xs text-gray-400 -mt-2">
        Alerts when a concept appears ≥5 times in an hour and multiplies ×5 its previous average
        (topics require more; reviewed hourly).
        {nuevas > 0 && (
          <span className="ml-2 text-rss-red font-semibold">{nuevas} unread.</span>
        )}
        {total > 0 && (
          <span className="ml-2">{total} total.</span>
        )}
      </p>

      {loading ? (
        <div className="panel flex justify-center py-16">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-50" />
        </div>
      ) : alertas.length === 0 ? (
        <div className="panel text-center py-16 text-gray-400">
          No alerts yet. The system reviews activity spikes every hour.
        </div>
      ) : (
        <div className="panel overflow-x-auto">
          <table className="w-full">
            <thead className="border-b border-gray-700">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Concept</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Type</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Period</th>
                <th className="px-4 py-3 text-right text-xs font-mono uppercase text-gray-400">News</th>
                <th className="px-4 py-3 text-right text-xs font-mono uppercase text-gray-400">Baseline</th>
                <th className="px-4 py-3 text-right text-xs font-mono uppercase text-gray-400">Ratio</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Status</th>
                <th className="px-4 py-3 text-left text-xs font-mono uppercase text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {alertas.map((a) => (
                <tr key={a.id} className="hover:bg-gray-800/50">
                  <td className="px-4 py-3">
                    <div className="flex items-start gap-3 min-w-0">
                      {a.image_path && (
                        <img
                          src={a.image_path}
                          alt={a.valor}
                          className="w-12 h-12 object-cover rounded-lg shrink-0 bg-gray-800"
                          onError={(e) => (e.currentTarget.style.display = 'none')}
                        />
                      )}
                      <div className="min-w-0">
                        <WikiTooltip
                          name={a.valor}
                          summary={a.wiki_summary || undefined}
                          imagePath={a.image_path || undefined}
                          wikiUrl={a.wiki_url || undefined}
                        >
                          <span className="font-medium text-gray-50">{a.valor}</span>
                        </WikiTooltip>
                        {a.wiki_summary && (
                          <p className="text-xs text-gray-500 mt-0.5 line-clamp-2 max-w-xs">
                            {a.wiki_summary}
                          </p>
                        )}
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="text-xs px-2 py-1 rounded bg-gray-800 text-gray-300">
                      {TIPO_LABEL[a.tipo] || a.tipo}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-400 font-mono">{a.periodo}</td>
                  <td className="px-4 py-3 text-right text-sm text-gray-50">{a.hits}</td>
                  <td className="px-4 py-3 text-right text-sm text-gray-400">{a.baseline.toFixed(1)}</td>
                  <td className="px-4 py-3 text-right">
                    <span className={`text-sm font-semibold ${a.ratio >= 10 ? 'text-rss-red' : 'text-amber-400'}`}>
                      ×{a.ratio.toFixed(1)}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-2 py-1 rounded ${
                      a.status === 'nueva'
                        ? 'bg-rss-red/20 text-rss-red'
                        : a.status === 'descartada'
                          ? 'bg-amber-900/20 text-amber-400'
                          : 'bg-gray-800 text-gray-400'
                    }`}>
                      {a.status === 'nueva' ? 'New' : a.status === 'descartada' ? 'Dismissed' : 'Read'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button onClick={() => openNews(a)} className="btn-ghost text-xs py-1 px-2 flex items-center gap-1">
                        <FileText className="h-3.5 w-3.5" />
                        News
                      </button>
                      {a.status === 'nueva' && (
                        <button onClick={() => markRead(a.id)} className="btn-ghost text-xs py-1 px-2">
                          Mark read
                        </button>
                      )}
                      {a.status !== 'descartada' && (
                        <button
                          onClick={() => dismiss(a.id)}
                          title="False alert: won't show again"
                          className="btn-ghost text-xs py-1 px-2 flex items-center gap-1 text-amber-400"
                        >
                          <XCircle className="h-3.5 w-3.5" />
                          Dismiss
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

      <div className="flex items-center justify-end gap-2">
        <button
          onClick={() => setLimit(limit > 50 ? 50 : 200)}
          className="btn-secondary flex items-center gap-2 text-sm"
        >
          {limit > 50 ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
          {limit > 50 ? 'Show less' : 'Show more'} ({limit})
        </button>
      </div>

      {/* News modal */}
      {newsModal && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4" onClick={() => setNewsModal(null)}>
          <div
            className="bg-gray-900 rounded-xl shadow-2xl w-full max-w-2xl max-h-[85vh] overflow-hidden flex flex-col border border-gray-700"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-start justify-between px-6 py-4 border-b border-gray-700 gap-4">
              <div>
                <h2 className="text-lg font-bold text-gray-50">News about &quot;{newsModal.valor}&quot;</h2>
                <p className="text-sm text-gray-400 mt-0.5">
                  {newsTotal} news found · {TIPO_LABEL[newsModal.tipo] || newsModal.tipo} · {newsModal.periodo}
                </p>
                <button
                  onClick={() => {
                    // Se pasa el valor nuevo explicitamente: setSoloHora es
                    // asincrono y openNews leeria el valor anterior.
                    const nuevo = !soloHora
                    setSoloHora(nuevo)
                    if (newsModal) openNews(newsModal, nuevo)
                  }}
                  className="mt-1 text-xs text-primary-300 hover:text-primary-50 underline"
                >
                  {soloHora
                    ? 'Ver todas las noticias del concepto (no solo esa hora)'
                    : 'Ver solo las noticias de esa hora'}
                </button>
              </div>
              <button
                onClick={() => setNewsModal(null)}
                className="text-gray-400 hover:text-gray-200 text-xl leading-none"
              >
                ✕
              </button>
            </div>

            <div className="p-4 overflow-y-auto">
              {newsLoading ? (
                <div className="text-center py-10 text-gray-400">Loading...</div>
              ) : entityNews.length === 0 ? (
                <div className="text-center py-10 text-gray-400">No news mentions this concept.</div>
              ) : (
                <ul className="space-y-2">
                  {entityNews.map((n) => (
                    <li
                      key={n.id}
                      className="flex items-start gap-2 p-3 rounded-lg hover:bg-gray-800 transition-colors border border-gray-700"
                    >
                      <Link
                        to={`/news/${n.id}`}
                        onClick={() => setNewsModal(null)}
                        className="flex items-start gap-3 min-w-0 flex-1"
                      >
                        {n.imagen_url && (
                          <img
                            src={n.imagen_url}
                            alt=""
                            className="w-20 h-14 object-cover rounded-md shrink-0"
                            onError={(e) => (e.currentTarget.style.display = 'none')}
                          />
                        )}
                        <div className="min-w-0">
                          <div className="font-medium text-sm text-gray-50 line-clamp-2">
                            {n.title_translated || n.titulo}
                          </div>
                          <p className="text-xs text-gray-400 mt-1 line-clamp-2">
                            {n.summary_translated || n.resumen}
                          </p>
                          {n.fecha && (
                            <span className="text-[11px] text-gray-500 mt-1 block font-mono">
                              {new Date(n.fecha).toLocaleDateString('es-ES')}
                            </span>
                          )}
                        </div>
                      </Link>
                      {/* Mismas acciones que en la seccion de Inteligencia:
                          guardar en favoritos y anadir a una lista. */}
                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          onClick={() => toggleFavorite(n)}
                          className={`p-1 rounded-full transition-colors ${
                            isFavorite(n.id)
                              ? 'text-red-500 bg-red-50 hover:bg-red-100'
                              : 'text-gray-400 hover:text-red-500 hover:bg-red-50'
                          }`}
                          title={isFavorite(n.id) ? 'Quitar de favoritos' : 'Añadir a favoritos'}
                          aria-label={isFavorite(n.id) ? 'Quitar de favoritos' : 'Añadir a favoritos'}
                        >
                          <Heart className={`h-3.5 w-3.5 ${isFavorite(n.id) ? 'fill-current' : ''}`} />
                        </button>
                        <button
                          onClick={() => handleAddToList(n)}
                          className="p-1 rounded-full text-gray-400 hover:text-blue-500 hover:bg-blue-50 transition-colors"
                          title="Añadir a lista"
                          aria-label="Añadir a lista"
                        >
                          <FolderPlus className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Anadir la noticia seleccionada a una lista (o crear una nueva) */}
      <CreateListModal
        isOpen={showListModal}
        onClose={() => {
          setShowListModal(false)
          setSelectedNewsForList(null)
        }}
        onCreate={handleCreateAndAdd}
        mode="add-to"
        existingLists={lists.map((l) => ({ id: l.id, name: l.name }))}
        onSelectList={handleSelectList}
        initialName={
          selectedNewsForList
            ? (selectedNewsForList.title_translated || selectedNewsForList.titulo || '').slice(0, 60)
            : ''
        }
      />
    </div>
  )
}
