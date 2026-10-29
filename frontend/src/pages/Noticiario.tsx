import { useState, useEffect, useMemo, useRef, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Search as SearchIcon,
  Mic,
  Plus,
  Upload,
  X,
  ChevronUp,
  ChevronDown,
  Play,
  Download,
  Check,
  AlertTriangle,
  Loader2,
  Radio,
  ListPlus,
  Flame,
  Sparkles,
  Gauge,
  FileEdit,
} from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { es } from 'date-fns/locale'
import { apiService, News, TtsBulletin, TtsTrendCandidate, TtsScriptPreview } from '../services/api'
import { useApiLists } from '../hooks/useApiFavorites'

// Maximo de noticias por noticiario (limite del backend).
const MAX_ITEMS = 50

// Caracteres por segundo de la locucion en espanol (medido: 2774 chars ->
// 167 s a velocidad 1.0). Sirve para estimar la duracion mientras se elige.
const CHARS_PER_SECOND = 16.6

// Ventanas temporales ofrecidas.
const TIME_WINDOWS = [
  { value: '', label: 'Sin filtro' },
  { value: '6h', label: 'Últimas 6 h' },
  { value: '24h', label: 'Últimas 24 h' },
  { value: '48h', label: 'Últimos 2 días' },
  { value: '7d', label: 'Última semana' },
  { value: '30d', label: 'Último mes' },
]

/**
 * Usuario actual.
 *
 * La app monta QueryClientProvider y BrowserRouter, pero no AppProvider: por
 * eso useApp() no puede usarse aqui (lanzaria y el ErrorBoundary mostraria
 * "Algo salio mal"). Se lee de localStorage, como hace el layout.
 */
function useCurrentUser(): { isAdmin: boolean } {
  const [isAdmin, setIsAdmin] = useState(false)

  useEffect(() => {
    try {
      const raw = localStorage.getItem('user')
      setIsAdmin(raw ? JSON.parse(raw)?.is_admin === true : false)
    } catch {
      setIsAdmin(false)
    }
  }, [])

  return { isAdmin }
}

// Tipo local del guion en revision: el texto que el editor esta escribiendo
// puede diferir del guardado.
type ReviewScript = TtsScriptPreview & { dirty?: boolean; raw?: string }

function formatDuration(seconds?: number | null): string {
  if (!seconds || seconds <= 0) return '--'
  const mins = Math.floor(seconds / 60)
  const secs = Math.round(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

function formatBytes(bytes?: number | null): string {
  if (!bytes) return '--'
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatElapsed(seconds?: number): string {
  if (!seconds || seconds <= 0) return '0 s'
  const mins = Math.floor(seconds / 60)
  const secs = Math.round(seconds % 60)
  if (mins === 0) return `${secs} s`
  return `${mins} min ${secs.toString().padStart(2, '0')} s`
}

const STATUS_LABEL: Record<string, { text: string; className: string }> = {
  pending: { text: 'Generando', className: 'badge-amber' },
  ready: { text: 'Listo para revisar', className: 'badge-blue' },
  approved: { text: 'Aprobado', className: 'badge-green' },
  rejected: { text: 'Rechazado', className: 'badge-red' },
  failed: { text: 'Error', className: 'badge-red' },
}

export function Noticiario() {
  const { isAdmin } = useCurrentUser()
  const { lists, getListItems } = useApiLists()

  // --- Busqueda ---------------------------------------------------------
  const [modo, setModo] = useState<'busqueda' | 'tendencias'>('busqueda')
  const [query, setQuery] = useState('')
  const [searchTerm, setSearchTerm] = useState('')
  const [lang, setLang] = useState('es')
  const [categoria, setCategoria] = useState('')
  const [pais, setPais] = useState('')
  const [page, setPage] = useState(1)
  // Filtros pensados para hilar un informativo.
  const [ventana, setVentana] = useState('7d')
  const [orden, setOrden] = useState<'reciente' | 'relevancia'>('reciente')
  const [minChars, setMinChars] = useState(150)
  const [dedup, setDedup] = useState(true)
  const [semantic, setSemantic] = useState(false)
  // El informativo se locuta a partir del texto traducido: por defecto solo
  // se listan noticias con la traduccion ya terminada.
  const [soloTraducidas, setSoloTraducidas] = useState(true)

  // --- Tendencias --------------------------------------------------------
  const [diasTendencia, setDiasTendencia] = useState(2)
  const [importancia, setImportancia] = useState('')

  // --- Seleccion --------------------------------------------------------
  const [selected, setSelected] = useState<News[]>([])

  // --- Revision del guion ------------------------------------------------
  const [scripts, setScripts] = useState<ReviewScript[]>([])
  const [reviewOpen, setReviewOpen] = useState(false)
  const [savingScript, setSavingScript] = useState<number | null>(null)

  // --- Noticiario --------------------------------------------------------
  const [title, setTitle] = useState('')
  const [silenceMs, setSilenceMs] = useState(800)
  const [voice, setVoice] = useState('es-base')
  const [speed, setSpeed] = useState(1.0)
  const [sampleFile, setSampleFile] = useState<File | null>(null)
  const [bulletinId, setBulletinId] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [audioUrl, setAudioUrl] = useState<string | null>(null)
  const audioRef = useRef<string | null>(null)

  const { data: categorias } = useQuery({
    queryKey: ['categories'],
    queryFn: apiService.getCategories,
  })

  const { data: paises } = useQuery({
    queryKey: ['countries'],
    queryFn: apiService.getCountries,
  })

  const { data: results, isLoading: searching } = useQuery({
    queryKey: ['noticiario-search', searchTerm, lang, categoria, pais, page, ventana, orden, minChars, dedup, semantic, soloTraducidas],
    queryFn: () =>
      apiService.search({
        q: searchTerm,
        lang: lang || undefined,
        categoria_id: categoria || undefined,
        pais_id: pais || undefined,
        page,
        per_page: 20,
        semantic: semantic || undefined,
        desde: ventana || undefined,
        orden,
        min_chars: minChars || undefined,
        dedup: dedup ? 'cluster' : undefined,
        translated_only: soloTraducidas ? 'true' : undefined,
      }),
    enabled: !!searchTerm && modo === 'busqueda',
  })

  // Tendencias: historias consolidadas (una por tema) y temas en tendencia.
  const { data: trends, isLoading: loadingTrends } = useQuery({
    queryKey: ['tts-trends', diasTendencia, importancia, minChars],
    queryFn: () =>
      apiService.getTtsTrends({
        dias: diasTendencia,
        importancia: importancia || undefined,
        limite: 40,
        min_chars: minChars || undefined,
      }),
    enabled: modo === 'tendencias',
  })

  // Voces disponibles para el noticiario.
  const { data: voices, refetch: refetchVoices } = useQuery({
    queryKey: ['tts-voices'],
    queryFn: () => apiService.listTtsVoices(),
  })

  // Hablantes del modelo base (256 en el modelo español).
  const { data: speakers } = useQuery({
    queryKey: ['tts-speakers'],
    queryFn: () => apiService.getTtsSpeakers(),
    retry: false,
  })
  const speakerCount = speakers?.speakers?.length ?? 0
  const [speakerNumber, setSpeakerNumber] = useState(0)

  // --- Noticiario en curso -----------------------------------------------
  const { data: bulletin } = useQuery({
    queryKey: ['tts-bulletin', bulletinId],
    queryFn: () => apiService.getTtsBulletin(bulletinId!),
    enabled: !!bulletinId,
    refetchInterval: (query) => {
      const status = (query.state.data as TtsBulletin | undefined)?.status
      return status === 'pending' ? 5000 : false
    },
  })

  // --- Noticiarios anteriores -------------------------------------------
  const { data: bulletinList, refetch: refetchBulletins } = useQuery({
    queryKey: ['tts-bulletins'],
    queryFn: () => apiService.listTtsBulletins(),
  })

  // Tareas de locucion en curso (para ver el progreso y el tiempo).
  const jobsQuery = useQuery({
    queryKey: ['tts-jobs'],
    queryFn: () => apiService.listTtsJobs(),
    enabled: !!bulletinId,
    refetchInterval: (query) => {
      const jobs = (query.state.data as { jobs?: { status: string }[] } | undefined)?.jobs || []
      return jobs.some((j) => j.status === 'running' || j.status === 'queued') ? 5000 : false
    },
  })

  // Cuando el noticiario esta listo se carga el audio para poder escucharlo.
  useEffect(() => {
    if (bulletin?.status === 'ready' && bulletin.audio_path) {
      let cancelled = false
      apiService
        .getTtsBulletinBlob(bulletin.id)
        .then((blob) => {
          if (cancelled) return
          if (audioRef.current) URL.revokeObjectURL(audioRef.current)
          const url = URL.createObjectURL(blob)
          audioRef.current = url
          setAudioUrl(url)
        })
        .catch(() => {})
      return () => {
        cancelled = true
      }
    }
  }, [bulletin?.status, bulletin?.id, bulletin?.audio_path])

  useEffect(() => {
    return () => {
      if (audioRef.current) URL.revokeObjectURL(audioRef.current)
    }
  }, [])

  // --- Seleccion --------------------------------------------------------
  const isSelected = useCallback(
    (id: string) => selected.some((n) => n.id === id),
    [selected]
  )

  const toggleSelect = useCallback((news: News) => {
    setSelected((current) => {
      if (current.some((n) => n.id === news.id)) {
        return current.filter((n) => n.id !== news.id)
      }
      if (current.length >= MAX_ITEMS) {
        setError(`Un noticiario admite hasta ${MAX_ITEMS} noticias.`)
        return current
      }
      setError(null)
      return [...current, news]
    })
  }, [])

  // Añade la noticia representativa de una tendencia.
  const addCandidate = useCallback((candidate: TtsTrendCandidate) => {
    setSelected((current) => {
      if (current.some((n) => n.id === candidate.id)) {
        return current
      }
      if (current.length >= MAX_ITEMS) {
        setError(`Un noticiario admite hasta ${MAX_ITEMS} noticias.`)
        return current
      }
      setError(null)
      return [
        ...current,
        {
          id: candidate.id,
          titulo: candidate.title,
          resumen: candidate.summary,
          url: candidate.url,
          fecha: candidate.date || '',
          imagen_url: candidate.image_url || undefined,
          fuente_nombre: candidate.source,
          title_translated: candidate.title,
          summary_translated: candidate.summary,
        },
      ]
    })
  }, [])

  // Estimacion de la duracion del informativo segun el texto elegido y la
  // velocidad. Es una aproximacion: el tiempo real depende del motor.
  const estimatedSeconds = useMemo(() => {
    const total = selected.reduce((acc, news) => {
      const chars = (
        news.content_translated ||
        news.contenido ||
        news.summary_translated ||
        news.resumen ||
        ''
      ).length
      return acc + chars / (CHARS_PER_SECOND * speed)
    }, 0)
    const pauses = Math.max(0, selected.length - 1) * (silenceMs / 1000)
    return total + pauses
  }, [selected, speed, silenceMs])

  const moveItem = useCallback((index: number, direction: -1 | 1) => {
    setSelected((current) => {
      const target = index + direction
      if (target < 0 || target >= current.length) return current
      const copy = [...current]
      const [item] = copy.splice(index, 1)
      copy.splice(target, 0, item)
      return copy
    })
  }, [])

  const removeItem = useCallback((index: number) => {
    setSelected((current) => current.filter((_, i) => i !== index))
  }, [])

  const loadFromList = useCallback(
    async (listId: number) => {
      const items = await getListItems(listId)
      setSelected(items.slice(0, MAX_ITEMS))
      setError(null)
    },
    [getListItems]
  )

  // --- Acciones ---------------------------------------------------------

  // Prepara los guiones de las noticias elegidas (revisables) y abre la
  // revision. Si ya estaban preparados, no se duplican.
  const openReview = useCallback(async () => {
    if (selected.length === 0) {
      setError('Selecciona al menos una noticia.')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const result = await apiService.prepareTtsScripts({
        noticia_ids: selected.map((n) => n.id),
        language: 'es',
      })
      setScripts(result.scripts as ReviewScript[])
      setReviewOpen(true)
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudieron preparar los guiones.')
    } finally {
      setBusy(false)
    }
  }, [selected])

  // Guarda el texto editado de un guion.
  const saveScript = useCallback(async (scriptId: number, content: string) => {
    setSavingScript(scriptId)
    setError(null)
    try {
      const updated = await apiService.updateTtsScript(scriptId, content)
      setScripts((current) =>
        current.map((s) =>
          s.script_id === scriptId
            ? { ...s, content: updated.content, raw: content, dirty: false }
            : s
        )
      )
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo guardar el texto.')
    } finally {
      setSavingScript(null)
    }
  }, [])

  // Registra un hablante del modelo como voz utilizable.
  const registerSpeaker = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      const result = await apiService.registerTtsSpeaker({
        speaker_id: speakerNumber,
        engine: 'melotts',
        display_name: `Hablante ${speakerNumber}`,
      })
      await refetchVoices()
      if (result?.voice?.slug) {
        setVoice(result.voice.slug)
      }
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo registrar el hablante.')
    } finally {
      setBusy(false)
    }
  }, [speakerNumber, refetchVoices])

  // Sube una muestra de voz propia y la registra como voz clonada (solo
  // admin). Por defecto se clona con Chatterbox: aprende timbre y acento.
  const uploadVoice = useCallback(async () => {
    if (!sampleFile) return
    setBusy(true)
    setError(null)
    try {
      const name = sampleFile.name.replace(/\.[^.]+$/, '')
      const result = await apiService.uploadTtsVoice({
        file: sampleFile,
        slug: name,
        display_name: name,
        language: 'es',
      })
      setSampleFile(null)
      await refetchVoices()
      if (result?.voice?.slug) {
        setVoice(result.voice.slug)
      }
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo subir la muestra de voz.')
    } finally {
      setBusy(false)
    }
  }, [sampleFile, refetchVoices])

  // Procesa el noticiario completo: guarda lo revisado, crea el noticiario y
  // encola la locucion de todas las noticias.
  const processBulletin = async () => {
    if (selected.length === 0) {
      setError('Selecciona al menos una noticia.')
      return
    }
    setBusy(true)
    setError(null)
    try {
      // 1. Guiones (preparados) y guardado de las correcciones.
      let prepared = scripts
      if (prepared.length === 0) {
        const result = await apiService.prepareTtsScripts({
          noticia_ids: selected.map((n) => n.id),
          language: 'es',
        })
        prepared = result.scripts as ReviewScript[]
        setScripts(prepared)
      }
      for (const script of prepared) {
        if (script.dirty) {
          await apiService.updateTtsScript(script.script_id, script.content)
        }
      }

      // 2. Noticiario con los guiones ya revisados.
      const created = await apiService.createTtsBulletin({
        title: title.trim() || `Noticiario ${new Date().toLocaleDateString('es-ES')}`,
        language: 'es',
        script_ids: prepared.map((s) => s.script_id),
        silence_ms: silenceMs,
        voice,
        speed,
      })
      setBulletinId(created.id)

      // 3. Aprobacion del contenido: encola las narraciones.
      await apiService.approveTtsBulletin(created.id)

      refetchBulletins()
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo procesar el noticiario.')
    } finally {
      setBusy(false)
    }
  }

  const approveBulletin = async () => {
    if (!bulletinId) return
    setBusy(true)
    setError(null)
    try {
      await apiService.approveTtsBulletin(bulletinId)
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo aprobar el noticiario.')
    } finally {
      setBusy(false)
    }
  }

  const setBulletinStatus = async (status: 'approved' | 'rejected') => {
    if (!bulletinId) return
    setBusy(true)
    setError(null)
    try {
      await apiService.setTtsBulletinStatus(bulletinId, status)
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo actualizar el noticiario.')
    } finally {
      setBusy(false)
    }
  }

  const openBulletin = (id: number) => {
    setBulletinId(id)
    setError(null)
  }

  // Cancela un noticiario en curso: util si se queda atascado.
  const cancelBulletin = useCallback(async () => {
    if (!bulletinId) return
    setBusy(true)
    setError(null)
    try {
      await apiService.cancelTtsBulletin(bulletinId)
      refetchBulletins()
    } catch (err: any) {
      setError(err?.response?.data?.message || err?.message || 'No se pudo cancelar el noticiario.')
    } finally {
      setBusy(false)
    }
  }, [bulletinId, refetchBulletins])

  const downloadAudio = async () => {
    if (!bulletinId) return
    try {
      const blob = await apiService.getTtsBulletinBlob(bulletinId)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `noticiario-${bulletinId}.wav`)
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch {
      setError('No se pudo descargar el audio.')
    }
  }

  const statusInfo = bulletin ? STATUS_LABEL[bulletin.status] : null
  const canApproveContent = bulletin?.status === 'pending' && bulletin.item_count > 0

  const pendingItems = useMemo(
    () => (bulletin?.items || []).filter((item) => !item.audio_id).length,
    [bulletin]
  )

  // La tarea que esta corriendo ahora mismo (para mostrar el tiempo).
  const runningJob = useMemo(() => {
    const jobs = jobsQuery.data?.jobs || []
    return jobs.find((job) => job.status === 'running' || job.status === 'queued')
  }, [jobsQuery.data])

  return (
    <div className="space-y-6">
      {/* Cabecera */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <h1 className="text-2xl font-bold text-gray-50 flex items-center gap-2">
            <Mic className="h-6 w-6 text-rss-green-phosphor" />
            Noticiario por voz
          </h1>
          <p className="text-sm text-gray-400 mt-1">
            Busca noticias, elige las del informativo y genera la locución.
            Cada audio requiere aprobación editorial antes de publicarse.
          </p>
        </div>
        {bulletin?.status === 'pending' && (
          <span className="badge-amber flex items-center gap-2">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Generando narraciones
          </span>
        )}
      </div>

      {error && (
        <div className="panel border-amber-900/60 flex items-start gap-2">
          <AlertTriangle className="h-4 w-4 text-amber-400 shrink-0 mt-0.5" />
          <p className="text-sm text-amber-200">{error}</p>
        </div>
      )}

      {/* ============ REVISION DEL GUION ============ */}
      {reviewOpen && (
        <div className="panel">
          <div className="panel-header">
            <span className="panel-title">Revisión del guion</span>
            <button onClick={() => setReviewOpen(false)} className="btn-ghost text-xs py-1">
              <X className="h-3.5 w-3.5" /> Cerrar
            </button>
          </div>

          <p className="text-sm text-gray-400 mb-4">
            Texto que se va a locutar. Puedes escribir cualquier corrección: las
            cifras y las fechas se vuelven a leer al guardar.
          </p>

          <div className="space-y-5">
            {scripts.map((script, index) => (
              <div key={script.script_id} className="bg-rss-bg-secondary rounded-lg p-4">
                <div className="flex items-start justify-between gap-3 mb-2">
                  <div className="min-w-0">
                    <p className="font-mono text-xs text-rss-green-phosphor">
                      {index + 1}. {script.title || 'Sin título'}
                    </p>
                    <p className="text-xs text-gray-400 truncate">
                      {script.source_name}
                      {script.source_url ? ' · ' + script.source_url : ''}
                    </p>
                  </div>
                  <span className="badge shrink-0">{script.status === 'draft' ? 'borrador' : script.status}</span>
                </div>

                <textarea
                  value={script.content}
                  onChange={(e) =>
                    setScripts((current) =>
                      current.map((s) =>
                        s.script_id === script.script_id
                          ? { ...s, content: e.target.value, dirty: true }
                          : s
                      )
                    )
                  }
                  rows={8}
                  className="input w-full font-mono text-sm leading-relaxed"
                  aria-label={`Texto de la noticia ${index + 1}`}
                />

                <div className="flex items-center justify-between mt-2">
                  <span className="text-xs text-gray-500 font-mono">
                    {script.content.length} caracteres ·{' '}
                    ~{formatDuration(script.content.length / (CHARS_PER_SECOND * speed))}
                  </span>
                  <button
                    onClick={() => saveScript(script.script_id, script.content)}
                    disabled={!script.dirty || savingScript === script.script_id || !script.content.trim()}
                    className="btn-secondary text-xs py-1 flex items-center gap-1 disabled:opacity-40"
                  >
                    {savingScript === script.script_id ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Check className="h-3.5 w-3.5" />
                    )}
                    Guardar
                  </button>
                </div>
              </div>
            ))}
          </div>

          <div className="flex items-center justify-between mt-4 pt-3 border-t border-rss-border">
            <p className="text-xs text-gray-400">
              {scripts.filter((s) => s.dirty).length > 0
                ? 'Tienes cambios sin guardar: se guardan al procesar.'
                : 'Todo guardado.'}
            </p>
            <button
              onClick={processBulletin}
              disabled={busy}
              className="btn-primary flex items-center gap-2"
            >
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
              Procesar y generar el audio
            </button>
          </div>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_380px]">
        {/* ================= COLUMNA IZQUIERDA: BUSQUEDA ================= */}
        <div className="space-y-4">
          {/* Modo de seleccion */}
          <div className="flex gap-2">
            <button
              onClick={() => setModo('busqueda')}
              className={modo === 'busqueda' ? 'btn-primary flex items-center gap-2' : 'btn-secondary flex items-center gap-2'}
            >
              <SearchIcon className="h-4 w-4" />
              Por palabra clave
            </button>
            <button
              onClick={() => setModo('tendencias')}
              className={modo === 'tendencias' ? 'btn-primary flex items-center gap-2' : 'btn-secondary flex items-center gap-2'}
            >
              <Flame className="h-4 w-4" />
              Por tendencia
            </button>
          </div>

          {modo === 'busqueda' ? (
          <form
            className="panel"
            noValidate
            onSubmit={(e) => {
              e.preventDefault()
              setPage(1)
              setSearchTerm(query.trim())
            }}
          >
            <div className="flex flex-wrap gap-4 items-end">
              <div className="flex-1 min-w-[220px]">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Buscar noticias
                </label>
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder="Palabras clave, temas, entidades..."
                  className="input w-full"
                />
              </div>

              <div className="w-32">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Idioma</label>
                <select value={lang} onChange={(e) => setLang(e.target.value)} className="input w-full">
                  <option value="es">Español</option>
                  <option value="en">English</option>
                  <option value="fr">Français</option>
                  <option value="de">Deutsch</option>
                  <option value="pt">Português</option>
                </select>
              </div>

              <div className="w-40">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Categoría</label>
                <select value={categoria} onChange={(e) => setCategoria(e.target.value)} className="input w-full">
                  <option value="">Todas</option>
                  {categorias?.map((cat) => (
                    <option key={cat.id} value={cat.id}>{cat.nombre}</option>
                  ))}
                </select>
              </div>

              <div className="w-40">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">País</label>
                <select value={pais} onChange={(e) => setPais(e.target.value)} className="input w-full">
                  <option value="">Todos</option>
                  {paises?.map((p) => (
                    <option key={p.id} value={p.id}>{p.nombre}</option>
                  ))}
                </select>
              </div>

              <button type="submit" className="btn-primary flex items-center gap-2">
                <SearchIcon className="h-4 w-4" />
                Buscar
              </button>
            </div>

            {/* Filtros para hilar un informativo */}
            <div className="flex flex-wrap gap-4 items-end mt-4 pt-4 border-t border-rss-border">
              <div className="w-36">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Ventana</label>
                <select
                  value={ventana}
                  onChange={(e) => {
                    setVentana(e.target.value)
                    setPage(1)
                    if (query.trim()) setSearchTerm(query.trim())
                  }}
                  className="input w-full"
                >
                  {TIME_WINDOWS.map((w) => (
                    <option key={w.value} value={w.value}>{w.label}</option>
                  ))}
                </select>
              </div>

              <div className="w-36">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Orden</label>
                <select
                  value={orden}
                  onChange={(e) => {
                    setOrden(e.target.value as 'reciente' | 'relevancia')
                    setPage(1)
                    if (query.trim()) setSearchTerm(query.trim())
                  }}
                  className="input w-full"
                >
                  <option value="reciente">Más recientes</option>
                  <option value="relevancia">Más relevantes</option>
                </select>
              </div>

              <div className="w-40">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Texto mínimo (chars)
                </label>
                <input
                  type="number"
                  min={0}
                  step={50}
                  value={minChars}
                  onChange={(e) => {
                    setMinChars(Number(e.target.value))
                    setPage(1)
                    if (query.trim()) setSearchTerm(query.trim())
                  }}
                  className="input w-full"
                  title="Filtra las noticias demasiado cortas para locutar"
                />
              </div>

              <label className="flex items-center gap-2 text-sm text-gray-300 pb-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={dedup}
                  onChange={(e) => {
                    setDedup(e.target.checked)
                    setPage(1)
                    if (query.trim()) setSearchTerm(query.trim())
                  }}
                />
                Una por tema
              </label>

              <label
                className="flex items-center gap-2 text-sm text-gray-300 pb-2 cursor-pointer"
                title="Solo noticias con la traduccion al español ya terminada, para poder locutarlas"
              >
                <input
                  type="checkbox"
                  checked={soloTraducidas}
                  onChange={(e) => {
                    setSoloTraducidas(e.target.checked)
                    setPage(1)
                    if (query.trim()) setSearchTerm(query.trim())
                  }}
                />
                Solo traducidas
              </label>

              <label className="flex items-center gap-2 text-sm text-gray-300 pb-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={semantic}
                  onChange={(e) => {
                    setSemantic(e.target.checked)
                    setPage(1)
                    if (query.trim()) setSearchTerm(query.trim())
                  }}
                />
                <Sparkles className="h-3.5 w-3.5" />
                Semántica
              </label>
            </div>
          </form>
          ) : (
            /* ============ MODO TENDENCIAS ============ */
            <div className="panel">
              <div className="flex flex-wrap gap-4 items-end mb-4">
                <div className="w-40">
                  <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Periodo</label>
                  <select
                    value={diasTendencia}
                    onChange={(e) => setDiasTendencia(Number(e.target.value))}
                    className="input w-full"
                  >
                    <option value={1}>Últimas 24 h</option>
                    <option value={2}>Últimos 2 días</option>
                    <option value={7}>Última semana</option>
                    <option value={30}>Último mes</option>
                  </select>
                </div>
                <div className="w-44">
                  <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Importancia</label>
                  <select
                    value={importancia}
                    onChange={(e) => setImportancia(e.target.value)}
                    className="input w-full"
                  >
                    <option value="">Todas</option>
                    <option value="breaking">Última hora</option>
                    <option value="major">Importantes</option>
                    <option value="normal">Normales</option>
                  </select>
                </div>
              </div>

              {loadingTrends && (
                <p className="text-sm text-gray-400 font-mono">Cargando tendencias...</p>
              )}

              {trends?.topics && trends.topics.length > 0 && (
                <div className="mb-4">
                  <p className="text-xs font-mono uppercase text-gray-400 mb-2">Temas en tendencia</p>
                  <div className="flex flex-wrap gap-2">
                    {trends.topics.map((topic) => (
                      <span
                        key={topic.id}
                        className="badge-blue"
                        title={`${topic.news} noticias · confianza ${topic.score.toFixed(2)}`}
                      >
                        {topic.name} · {topic.news}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {trends?.clusters && trends.clusters.length > 0 && (
                <div className="space-y-3">
                  {trends.clusters.map((cluster) => {
                    const rep = cluster.representative
                    const chosen = rep ? isSelected(rep.id) : false
                    return (
                      <div key={cluster.id} className="bg-rss-bg-secondary rounded-lg p-3">
                        <div className="flex items-start gap-2 mb-1">
                          <span
                            className={
                              cluster.importance === 'breaking'
                                ? 'badge-red shrink-0'
                                : cluster.importance === 'major'
                                ? 'badge-amber shrink-0'
                                : 'badge shrink-0'
                            }
                          >
                            {cluster.importance}
                          </span>
                          <span className="text-xs text-gray-400">
                            {cluster.popularity} medios ·{' '}
                            {cluster.start_date
                              ? formatDistanceToNow(new Date(cluster.start_date), { addSuffix: true, locale: es })
                              : ''}
                          </span>
                        </div>
                        <p className="text-sm font-semibold text-gray-100 mb-1">
                          {cluster.title || rep?.title}
                        </p>
                        {rep && (
                          <p className="text-xs text-gray-400 line-clamp-2 mb-2">
                            {rep.title} — {rep.source}
                          </p>
                        )}
                        {rep && (
                          <button
                            onClick={() => addCandidate(rep)}
                            className={chosen ? 'btn-primary text-xs py-1' : 'btn-secondary text-xs py-1'}
                          >
                            {chosen ? (
                              <>
                                <Check className="h-3.5 w-3.5" /> Añadida
                              </>
                            ) : (
                              <>
                                <Plus className="h-3.5 w-3.5" /> Añadir la noticia principal
                              </>
                            )}
                          </button>
                        )}
                      </div>
                    )
                  })}
                </div>
              )}

              {!loadingTrends && trends?.clusters && trends.clusters.length === 0 && (
                <p className="text-sm text-gray-400">
                  No hay historias agrupadas en este periodo. El agrupamiento se genera con el
                  worker de eventos.
                </p>
              )}
            </div>
          )}

          {searching && (
            <div className="panel text-sm text-gray-400 font-mono">Buscando...</div>
          )}

          {results && results.total > 0 && (
            <p className="text-sm text-gray-400">
              {results.total.toLocaleString()} resultados · página {page} de {results.total_pages}
            </p>
          )}

          {results && results.news?.length === 0 && (
            <div className="panel text-sm text-gray-400">
              Ningún resultado para «{searchTerm}».
              {soloTraducidas && (
                <span className="block mt-2 text-gray-500">
                  Se está filtrando solo por noticias ya traducidas. Si la traducción va
                  retrasada, prueba a desmarcar «Solo traducidas».
                </span>
              )}
            </div>
          )}

          <div className="space-y-3">
            {results?.news?.map((news: News) => {
              const chosen = isSelected(news.id)
              return (
                <div
                  key={news.id}
                  className={`card p-3 flex gap-3 ${chosen ? 'border-rss-green-dim' : ''}`}
                >
                  {news.imagen_url && news.imagen_url.trim() && (
                    <img
                      src={news.imagen_url}
                      alt=""
                      className="w-20 h-20 object-cover rounded-lg shrink-0"
                    />
                  )}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 text-xs text-gray-400 mb-1 flex-wrap">
                      {news.fuente_nombre && <span className="truncate max-w-[180px]">{news.fuente_nombre}</span>}
                      {news.fecha && (
                        <span>
                          {formatDistanceToNow(new Date(news.fecha), { addSuffix: true, locale: es })}
                        </span>
                      )}
                    </div>
                    <h3 className="text-base font-semibold text-gray-50 line-clamp-2 leading-snug">
                      {news.title_translated || news.titulo}
                    </h3>
                    <p className="text-sm text-gray-300 line-clamp-2 mt-1">
                      {news.summary_translated || news.resumen}
                    </p>
                  </div>
                  <button
                    onClick={() => toggleSelect(news)}
                    className={chosen ? 'btn-primary shrink-0 self-center' : 'btn-secondary shrink-0 self-center'}
                    title={chosen ? 'Quitar del noticiario' : 'Añadir al noticiario'}
                  >
                    {chosen ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
                  </button>
                </div>
              )
            })}
          </div>

          {results && results.total_pages > 1 && (
            <div className="flex justify-center gap-2">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page <= 1}
                className="btn-secondary disabled:opacity-50"
              >
                Anterior
              </button>
              <span className="flex items-center px-4 text-gray-400 font-mono text-sm">
                {page} / {results.total_pages}
              </span>
              <button
                onClick={() => setPage(page + 1)}
                disabled={page >= results.total_pages}
                className="btn-secondary disabled:opacity-50"
              >
                Siguiente
              </button>
            </div>
          )}
        </div>

        {/* ================= COLUMNA DERECHA: SELECCIÓN ================= */}
        <div className="space-y-4">
          {/* Selección */}
          <div className="panel">
            <div className="panel-header">
              <span className="panel-title">Selección</span>
              <span className="badge-blue">
                {selected.length} / {MAX_ITEMS}
              </span>
            </div>

            {lists.length > 0 && (
              <div className="mb-3">
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Cargar desde una lista
                </label>
                <select
                  defaultValue=""
                  onChange={(e) => {
                    if (e.target.value) loadFromList(Number(e.target.value))
                  }}
                  className="input w-full"
                >
                  <option value="">Selecciona una lista...</option>
                  {lists.map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name} ({l.item_count})
                    </option>
                  ))}
                </select>
              </div>
            )}

            {selected.length === 0 ? (
              <p className="text-sm text-gray-400 py-4 text-center">
                Busca noticias y pulsa <Plus className="inline h-3.5 w-3.5" /> para añadir
                las que formen parte del informativo.
              </p>
            ) : (
              <ol className="space-y-2 max-h-[420px] overflow-y-auto pr-1">
                {selected.map((news, index) => (
                  <li
                    key={news.id}
                    className="flex items-start gap-2 p-2 rounded-lg bg-rss-bg-secondary"
                  >
                    <span className="font-mono text-xs text-rss-green-phosphor pt-0.5 w-5 shrink-0">
                      {index + 1}
                    </span>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-gray-100 line-clamp-2 leading-snug">
                        {news.title_translated || news.titulo}
                      </p>
                      {news.fuente_nombre && (
                        <p className="text-xs text-gray-400 truncate mt-0.5">{news.fuente_nombre}</p>
                      )}
                    </div>
                    <div className="flex flex-col gap-0.5 shrink-0">
                      <button
                        onClick={() => moveItem(index, -1)}
                        disabled={index === 0}
                        className="p-1 rounded text-gray-400 hover:text-rss-green-phosphor disabled:opacity-25"
                        title="Subir"
                      >
                        <ChevronUp className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => moveItem(index, 1)}
                        disabled={index === selected.length - 1}
                        className="p-1 rounded text-gray-400 hover:text-rss-green-phosphor disabled:opacity-25"
                        title="Bajar"
                      >
                        <ChevronDown className="h-3.5 w-3.5" />
                      </button>
                    </div>
                    <button
                      onClick={() => removeItem(index)}
                      className="p-1 rounded text-gray-400 hover:text-red-400 shrink-0"
                      title="Quitar"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </li>
                ))}
              </ol>
            )}
          </div>

          {/* Crear noticiario */}
          <div className="panel">
            <div className="panel-header">
              <span className="panel-title">Generar noticiario</span>
            </div>

            <div className="space-y-3">
              <div>
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Título</label>
                <input
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder={`Noticiario ${new Date().toLocaleDateString('es-ES')}`}
                  className="input w-full"
                />
              </div>

              <div>
                <label htmlFor="bulletin-voice" className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Voz
                </label>
                <select
                  id="bulletin-voice"
                  value={voice}
                  onChange={(e) => setVoice(e.target.value)}
                  className="input w-full"
                >
                  {(voices?.voices || []).map((v) => (
                    <option key={v.id} value={v.slug}>
                      {v.display_name} ({v.engine === 'openvoice' || (v.engine === 'chatterbox' && !v.is_builtin) ? 'clone' : 'base'})
                    </option>
                  ))}
                  {!voices?.voices?.length && <option value="es-base">Voz base en español</option>}
                </select>

                {/* Los modelos de MeloTTS traen muchos hablantes: se registran
                    bajo demanda para no llenar el catalogo. */}
                <div className="flex gap-2 mt-2">
                  <input
                    type="number"
                    min={0}
                    max={speakerCount ? speakerCount - 1 : 255}
                    value={speakerNumber}
                    onChange={(e) => setSpeakerNumber(Number(e.target.value))}
                    className="input w-full text-xs"
                    placeholder={`Hablante (0-${speakerCount ? speakerCount - 1 : 255})`}
                    aria-label="Hablante del modelo"
                  />
                  <button
                    onClick={registerSpeaker}
                    disabled={busy}
                    className="btn-secondary text-xs py-1 shrink-0 disabled:opacity-40"
                    title="Registrar este hablante del modelo como voz"
                  >
                    <Plus className="h-3.5 w-3.5" />
                    Añadir voz
                  </button>
                </div>
                {speakerCount > 0 && (
                  <p className="text-xs text-gray-500 mt-1">
                    El modelo tiene {speakerCount} hablantes: cada uno es una voz
                    distinta.
                  </p>
                )}

                {/* Voz propia: solo administradores. El audio se guarda en el
                    volumen del servicio y Chatterbox aprende de el el timbre y
                    el acento. */}
                {isAdmin && (
                  <div className="mt-3 pt-3 border-t border-gray-800">
                    <label htmlFor="voice-sample" className="block text-xs font-mono uppercase text-gray-400 mb-1">
                      Voz propia (muestra de voz)
                    </label>
                    <input
                      id="voice-sample"
                      type="file"
                      accept=".wav,.mp3,.ogg,.oga,.opus,.flac,.aif,.aiff,.au,audio/*"
                      onChange={(e) => setSampleFile(e.target.files?.[0] ?? null)}
                      className="block w-full text-xs text-gray-400 file:mr-2 file:rounded file:border-0 file:bg-gray-800 file:px-2 file:py-1 file:text-gray-300"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Entre 3 y 60 s en WAV, MP3 u OGG, con voz limpia y sin
                      música ni ruido: cuanto más nítida, mejor el clon.
                    </p>
                    {sampleFile && (
                      <button
                        onClick={uploadVoice}
                        disabled={busy}
                        className="btn-secondary text-xs py-1 mt-2 disabled:opacity-40"
                        title="Registrar esta muestra como voz y clonarla con Chatterbox"
                      >
                        <Upload className="h-3.5 w-3.5" />
                        Clonar «{sampleFile.name}»
                      </button>
                    )}
                  </div>
                )}
              </div>

              <div>
                <label htmlFor="bulletin-speed" className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Velocidad de locución ({speed.toFixed(2)}x)
                </label>
                <input
                  id="bulletin-speed"
                  type="range"
                  min={0.7}
                  max={1.5}
                  step={0.05}
                  value={speed}
                  onChange={(e) => setSpeed(Number(e.target.value))}
                  className="w-full"
                />
                <div className="flex justify-between text-xs text-gray-500 font-mono">
                  <span>Lenta</span>
                  <span className="flex items-center gap-1">
                    <Gauge className="h-3 w-3" /> Normal
                  </span>
                  <span>Rápida</span>
                </div>
              </div>

              <div>
                <label className="block text-xs font-mono uppercase text-gray-400 mb-2">
                  Pausa entre noticias (ms)
                </label>
                <input
                  type="number"
                  min={0}
                  max={10000}
                  step={100}
                  value={silenceMs}
                  onChange={(e) => setSilenceMs(Number(e.target.value))}
                  className="input w-full"
                />
              </div>

              {selected.length > 0 && (
                <div className="bg-rss-bg-secondary rounded-lg p-3 flex items-center justify-between">
                  <div>
                    <p className="metric text-lg">{formatDuration(estimatedSeconds)}</p>
                    <p className="metric-label">Duración estimada</p>
                  </div>
                  <p className="text-xs text-gray-400 text-right">
                    según el texto y la velocidad
                    <br />
                    (el valor real lo da el motor)
                  </p>
                </div>
              )}

              {/* Revisión y procesado: el flujo es revisar -> procesar */}
              <div className="grid grid-cols-2 gap-2">
                <button
                  onClick={openReview}
                  disabled={busy || selected.length === 0}
                  className="btn-secondary flex items-center justify-center gap-2 disabled:opacity-50"
                  title="Ver y corregir el texto antes de locutarlo"
                >
                  {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileEdit className="h-4 w-4" />}
                  Revisión
                </button>
                <button
                  onClick={processBulletin}
                  disabled={busy || selected.length === 0}
                  className="btn-primary flex items-center justify-center gap-2 disabled:opacity-50"
                  title="Generar el audio completo del noticiario"
                >
                  {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
                  Procesar
                </button>
              </div>
            </div>
          </div>

          {/* Estado del noticiario */}
          {bulletin && (
            <div className="panel">
              <div className="panel-header">
                <span className="panel-title">Noticiario #{bulletin.id}</span>
                {statusInfo && <span className={statusInfo.className}>{statusInfo.text}</span>}
              </div>

              <h3 className="text-base font-semibold text-gray-50 mb-2">{bulletin.title}</h3>

              <div className="grid grid-cols-3 gap-2 text-center mb-3">
                <div className="bg-rss-bg-secondary rounded-lg p-2">
                  <p className="metric text-lg">{bulletin.item_count}</p>
                  <p className="metric-label">Noticias</p>
                </div>
                <div className="bg-rss-bg-secondary rounded-lg p-2">
                  <p className="metric text-lg">{formatDuration(bulletin.duration_seconds)}</p>
                  <p className="metric-label">Duración</p>
                </div>
                <div className="bg-rss-bg-secondary rounded-lg p-2">
                  <p className="metric text-lg">{formatBytes(bulletin.size_bytes)}</p>
                  <p className="metric-label">Tamaño</p>
                </div>
              </div>

              {bulletin.status === 'pending' && (
                <div className="text-sm text-gray-300 mb-3">
                  <p className="flex items-center gap-2">
                    <Loader2 className="h-4 w-4 animate-spin text-rss-amber" />
                    Generando narraciones{pendingItems > 0 ? ` (${pendingItems} pendientes)` : '...'}
                  </p>
                  {runningJob && (runningJob.elapsed_seconds ?? 0) > 0 && (
                    <p className="text-xs text-gray-400 mt-1">
                      En curso desde hace {formatElapsed(runningJob.elapsed_seconds)}
                      {runningJob.attempts > 1 && ` · intento ${runningJob.attempts}`}
                    </p>
                  )}
                  {runningJob && (runningJob.elapsed_seconds ?? 0) > 1800 && (
                    <p className="text-xs text-amber-300 mt-1 flex items-start gap-1">
                      <AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5" />
                      Tarda más de lo habitual. Si el servicio se ha caído, la
                      tarea se reencolará sola; si no, puedes cancelar el
                      noticiario y volver a procesarlo.
                    </p>
                  )}
                  <p className="text-xs text-gray-400 mt-1">
                    En CPU cada noticia tarda alrededor de un minuto y medio por
                    minuto de audio.
                  </p>
                  <button
                    onClick={cancelBulletin}
                    disabled={busy}
                    className="btn-ghost text-xs py-1 mt-2 flex items-center gap-1"
                  >
                    <X className="h-3.5 w-3.5" />
                    Cancelar el noticiario
                  </button>
                </div>
              )}

              {bulletin.status === 'ready' && (
                <>
                  {audioUrl && (
                    <audio controls src={audioUrl} className="w-full mb-3">
                      Tu navegador no soporta audio.
                    </audio>
                  )}
                  <div className="flex gap-2">
                    <button onClick={downloadAudio} className="btn-secondary flex items-center gap-2">
                      <Download className="h-4 w-4" />
                      Descargar WAV
                    </button>
                    {isAdmin && (
                      <button
                        onClick={() => setBulletinStatus('approved')}
                        disabled={busy}
                        className="btn-primary flex items-center gap-2"
                      >
                        <Check className="h-4 w-4" />
                        Aprobar
                      </button>
                    )}
                  </div>
                  {!isAdmin && (
                    <p className="text-xs text-gray-400 mt-2">
                      Un administrador debe aprobar el audio antes de publicarlo.
                    </p>
                  )}
                </>
              )}

              {bulletin.status === 'approved' && (
                <div className="flex items-center gap-2 text-sm text-rss-green-phosphor">
                  <Check className="h-4 w-4" />
                  Audio aprobado por un administrador
                </div>
              )}

              {canApproveContent && (
                <button
                  onClick={approveBulletin}
                  disabled={busy}
                  className="btn-primary w-full flex items-center justify-center gap-2"
                >
                  {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
                  Aprobar y generar las narraciones
                </button>
              )}

              {/* Manifiesto: qué noticia está en qué segundo */}
              {bulletin.items && bulletin.items.length > 0 && (
                <div className="mt-4">
                  <p className="text-xs font-mono uppercase text-gray-400 mb-2">Guion del informativo</p>
                  <ol className="space-y-1.5">
                    {bulletin.items.map((item) => (
                      <li key={item.id} className="flex gap-2 text-xs">
                        <span className="font-mono text-rss-green-phosphor shrink-0 w-5">
                          {item.position}
                        </span>
                        <span className="font-mono text-gray-400 shrink-0 w-12">
                          {item.start_offset != null ? formatDuration(item.start_offset) : '--:--'}
                        </span>
                        <span className="text-gray-300 line-clamp-2">
                          {item.source_title || item.script_title}
                        </span>
                      </li>
                    ))}
                  </ol>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Noticiarios anteriores */}
      <div className="panel">
        <div className="panel-header">
          <span className="panel-title">Noticiarios</span>
          <Radio className="h-4 w-4 text-gray-500" />
        </div>

        {bulletinList?.bulletins && bulletinList.bulletins.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs font-mono uppercase text-gray-400 border-b border-rss-border">
                  <th className="py-2 pr-3">#</th>
                  <th className="py-2 pr-3">Título</th>
                  <th className="py-2 pr-3">Estado</th>
                  <th className="py-2 pr-3">Noticias</th>
                  <th className="py-2 pr-3">Duración</th>
                  <th className="py-2 pr-3">Creado</th>
                  <th className="py-2"></th>
                </tr>
              </thead>
              <tbody>
                {bulletinList.bulletins.map((b) => {
                  const info = STATUS_LABEL[b.status] || { text: b.status, className: 'badge' }
                  return (
                    <tr key={b.id} className="border-b border-rss-border/40">
                      <td className="py-2 pr-3 font-mono text-gray-400">{b.id}</td>
                      <td className="py-2 pr-3 text-gray-100 max-w-[260px] truncate">{b.title}</td>
                      <td className="py-2 pr-3">
                        <span className={info.className}>{info.text}</span>
                      </td>
                      <td className="py-2 pr-3 font-mono text-gray-300">{b.item_count}</td>
                      <td className="py-2 pr-3 font-mono text-gray-300">{formatDuration(b.duration_seconds)}</td>
                      <td className="py-2 pr-3 text-gray-400">
                        {formatDistanceToNow(new Date(b.created_at), { addSuffix: true, locale: es })}
                      </td>
                      <td className="py-2 text-right">
                        <button onClick={() => openBulletin(b.id)} className="btn-ghost text-xs py-1">
                          Abrir
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-gray-400 py-3">
            Todavía no hay noticiarios. Selecciona noticias y crea el primero.
          </p>
        )}
      </div>
    </div>
  )
}
