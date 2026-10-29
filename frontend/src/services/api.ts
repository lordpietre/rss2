import axios from 'axios'

const API_BASE = import.meta.env.VITE_API_URL || '/api'

export const api = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  // For FormData uploads the browser must set the multipart boundary itself.
  // The instance default 'application/json' (or a manual 'multipart/form-data'
  // without boundary) would prevent that and the backend would see no file.
  if (config.data instanceof FormData) {
    delete config.headers['Content-Type']
  }
  return config
})

// If the session token expires (or is invalid), clear it and ask to log in again
// instead of failing with confusing "Invalid token" errors on admin actions.
api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status
    const url = error?.config?.url || ''
    const isAuthCall = url.includes('/auth/login') || url.includes('/auth/register')
    if (status === 401 && !isAuthCall) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      if (!window.location.pathname.startsWith('/login')) {
        window.location.href = '/login?expired=1'
      }
    }
    return Promise.reject(error)
  },
)

export interface News {
  id: string
  titulo: string
  resumen: string
  url: string
  fecha: string
  news_created_at?: string  // When scraped
  added_at?: string         // When added to list
  imagen_url?: string
  categoria_id?: number
  pais_id?: number
  fuente_nombre: string
  title_translated?: string
  summary_translated?: string
  content_translated?: string
  contenido?: string
  lang_translated?: string
  entities?: any[]
}

export interface NewsListResponse {
  news: News[]
  total: number
  page: number
  per_page: number
  total_pages: number
}

export interface Feed {
  id: number
  nombre: string
  descripcion?: string
  url: string
  categoria_id?: number
  pais_id?: number
  idioma?: string
  activo: boolean
  fallos?: number
  last_error?: string
  categoria?: string
  pais?: string
  noticias_count?: number
  /** El canal ya publica en espanol: sus noticias no se traducen. */
  traducido?: boolean
}

export interface FeedListResponse {
  feeds: Feed[]
  total: number
  page: number
  per_page: number
  total_pages: number
}

export interface Category {
  id: number
  nombre: string
}

export interface Country {
  id: number
  nombre: string
  continente: string
}

export interface Idioma {
  codigo: string
  nombre: string
  nombre_en: string
  flag_emoji: string
  activo: boolean
}

export interface MentionPoint {
  fecha: string
  count: number
}

export interface MentionSeries {
  valor: string
  tipo: string
  count: number
  data: MentionPoint[]
}

export interface MentionsResponse {
  days: number
  series: MentionSeries[]
}

export interface EntitySuggestion {
  valor: string
  tipo: string
  count: number
  last_date?: string
}

export interface Alerta {
  id: number
  valor: string
  tipo: string
  periodo: string
  hits: number
  baseline: number
  ratio: number
  status: string
  created_at: string
  wiki_summary?: string | null
  wiki_url?: string | null
  image_path?: string | null
}

export interface Stats {
  total_news: number
  total_feeds: number
  total_users: number
  total_translated: number
  news_today: number
  news_this_week: number
  news_this_month: number
  translations_pending: number
  translations_done: number
  translations_error: number
  top_languages: Array<{ lang: string; count: number; dias_activos: number }>
  translation_stats_12h: Array<{ hour: string; total: number; items: number }>
  translation_stats_24h: Array<{ hour: string; total: number; items: number }>
  translation_stats_2d: Array<{ hour: string; total: number; items: number }>
  translation_stats_3d: Array<{ hour: string; total: number; items: number }>
  translation_stats_4d: Array<{ hour: string; total: number; items: number }>
  translation_stats_5d: Array<{ hour: string; total: number; items: number }>
  top_categories: Array<{ categoria_id: number; categoria_nombre: string; count: number }>
  top_countries: Array<{ pais_id: number; pais_nombre: string; flag_emoji: string; count: number }>
}

// ---------------------------------------------------------------------------
// Narracion de noticias (TTS)
// ---------------------------------------------------------------------------

export interface TtsVoice {
  id: number
  slug: string
  display_name: string
  engine: string
  language: string
  speaker_id?: string | null
  duration_seconds?: number | null
  sample_rate?: number | null
  quality_status: string
  is_builtin: boolean
}

export interface TtsScript {
  id: number
  noticia_id?: string | null
  language: string
  title?: string | null
  content: string
  content_length?: number | null
  source_title?: string | null
  source_url?: string | null
  source_name?: string | null
  status: 'draft' | 'approved' | 'rejected'
  approved_at?: string | null
  created_at: string
}

export interface TtsAudio {
  id: number
  job_id: number
  script_id: number
  engine: string
  engine_version?: string | null
  duration_seconds?: number | null
  size_bytes?: number | null
  status: 'pending_review' | 'approved' | 'rejected'
  noticia_id?: string | null
  source_url?: string | null
  source_name?: string | null
  created_at: string
}

export interface TtsJob {
  id: number
  script_id: number
  voice_id?: number | null
  engine: string
  speed?: number
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  attempts: number
  max_attempts: number
  error_code?: string | null
  error_message?: string | null
  queued_at: string
  started_at?: string | null
  finished_at?: string | null
  elapsed_seconds?: number
  audio?: TtsAudio | null
}

export interface TtsBulletinItem {
  id: number
  bulletin_id: number
  script_id: number
  position: number
  audio_id?: number | null
  duration_seconds?: number | null
  start_offset?: number | null
  source_url?: string | null
  source_name?: string | null
  source_title?: string | null
  noticia_id?: string | null
  script_title?: string | null
}

export interface TtsBulletin {
  id: number
  title: string
  language: string
  status: 'pending' | 'ready' | 'failed' | 'approved' | 'rejected'
  voice_id?: number | null
  voice_slug?: string
  engine?: string
  speed?: number
  audio_path?: string | null
  duration_seconds?: number | null
  size_bytes?: number | null
  item_count: number
  silence_ms: number
  approved_at?: string | null
  created_at: string
  items?: TtsBulletinItem[]
}

export interface TtsTrendCandidate {
  id: string
  title: string
  summary: string
  content_length: number
  source: string
  url: string
  date?: string | null
  image_url?: string | null
  cluster_id?: number | null
  cluster_popularity?: number | null
  cluster_importancia?: string | null
}

export interface TtsTrendCluster {
  id: number
  title: string
  importance: 'breaking' | 'major' | 'normal' | 'minor'
  popularity: number
  news_count: number
  start_date?: string | null
  end_date?: string | null
  representative?: TtsTrendCandidate | null
}

export interface TtsTrendTopic {
  id: number
  name: string
  news: number
  score: number
}

export interface TtsTrends {
  clusters: TtsTrendCluster[]
  topics: TtsTrendTopic[]
  params: Record<string, unknown>
}

export interface TtsScriptPreview {
  script_id: number
  noticia_id: string
  title: string
  content: string
  source_name: string
  source_url: string
  status: string
  existing: boolean
}

export const apiService = {
  getNews: async (params: { page?: number; per_page?: number; q?: string; category_id?: string; country_id?: string; translated_only?: string; semantic?: string; lang_to?: string }): Promise<NewsListResponse> => {
    const { data } = await api.get('/news', { params })
    return data
  },

  getNewsById: async (id: string): Promise<News> => {
    const { data } = await api.get(`/news/${id}`)
    return data
  },

  getFeeds: async (params?: {
    page?: number
    per_page?: number
    activo?: string
    categoria_id?: string
    pais_id?: string
    /** Busqueda simple por nombre, descripcion o URL. */
    q?: string
    /** Filtra por la lista de canales ya traducidos. */
    traducido?: string
  }): Promise<FeedListResponse> => {
    const { data } = await api.get('/feeds', { params })
    return data
  },

  createFeed: async (feed: { nombre: string; url: string; descripcion?: string; categoria_id?: number; pais_id?: number; idioma?: string }): Promise<{ id: number }> => {
    const { data } = await api.post('/feeds', feed)
    return data
  },

  updateFeed: async (id: number, feed: { nombre: string; url: string; descripcion?: string; categoria_id?: number; pais_id?: number; idioma?: string; activo?: boolean }): Promise<void> => {
    const { data } = await api.put(`/feeds/${id}`, feed)
    return data
  },

  deleteFeed: async (id: number): Promise<void> => {
    await api.delete(`/feeds/${id}`)
  },

  deleteInactiveFeeds: async (paisId?: string): Promise<{ deleted_count: number }> => {
    const { data } = await api.delete('/feeds/inactive', {
      params: paisId ? { pais_id: paisId } : undefined
    })
    return data
  },

  toggleFeed: async (id: number): Promise<void> => {
    await api.post(`/feeds/${id}/toggle`)
  },

  reactivateFeed: async (id: number): Promise<void> => {
    await api.post(`/feeds/${id}/reactivate`)
  },

  /** Marca (o desmarca) un canal como ya traducido: no se procesa. */
  setFeedTraducido: async (id: number, traducido: boolean): Promise<void> => {
    await api.post(`/feeds/${id}/traducido`, { traducido })
  },

  /** Marca (o desmarca) varios canales de una vez. */
  bulkSetFeedsTraducido: async (
    ids: number[],
    traducido: boolean
  ): Promise<{ message: string; updated: number }> => {
    const { data } = await api.post('/feeds/traducido/bulk', { ids, traducido })
    return data
  },

  /**
   * Borra varios canales de golpe. `q` borra todos los que casan con la
   * busqueda (p.ej. las Wikipedia de cambios recientes).
   *
   * El backend exige `confirmar: true`: sin el responde 412 con el numero de
   * fuentes y noticias afectadas y NO borra nada.
   */
  bulkDeleteFeeds: async (params: {
    ids?: number[]
    q?: string
  }): Promise<{ message: string; feeds_deleted: number; noticias_deleted: number }> => {
    const { data } = await api.post('/feeds/bulk-delete', { ...params, confirmar: true })
    return data
  },

  exportFeeds: async (params?: { activo?: string; categoria_id?: string; pais_id?: string }): Promise<Blob> => {
    const { data } = await api.get('/feeds/export', { params, responseType: 'blob' })
    return data
  },

  importFeeds: async (file: File): Promise<{ imported: number; skipped: number; failed: number; message: string }> => {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await api.post('/feeds/import', formData)
    return data
  },

  search: async (params: {
    q: string
    page?: number
    per_page?: number
    lang?: string
    categoria_id?: string
    pais_id?: string
    semantic?: boolean
    desde?: string
    hasta?: string
    orden?: 'reciente' | 'relevancia'
    min_chars?: number
    dedup?: 'cluster'
    topic_id?: number
    translated_only?: string
  }): Promise<NewsListResponse> => {
    const { data } = await api.get('/search', { params })
    return data
  },

  getStats: async (): Promise<Stats> => {
    const { data } = await api.get('/stats')
    return data
  },

  getCategories: async (): Promise<Category[]> => {
    const { data } = await api.get('/categories')
    return data
  },

  getCountries: async (): Promise<Country[]> => {
    const { data } = await api.get('/countries')
    return data
  },

  getIdiomas: async (): Promise<Idioma[]> => {
    const { data } = await api.get('/idiomas')
    return data.idiomas || []
  },

  getEntityMentions: async (params: { values: string[]; hours?: number }): Promise<MentionsResponse> => {
    const { data } = await api.get('/entities/mentions', {
      params: { values: params.values.join(','), hours: params.hours || 720 },
    })
    return data
  },

  getEntityNews: async (params: { valor: string; tipo?: string; page?: number; per_page?: number; days?: number }): Promise<NewsListResponse> => {
    const { data } = await api.get('/entities/news', { params })
    return data
  },

  searchEntities: async (params: { tipo: string; q?: string; page?: number; per_page?: number; sort_by?: string }): Promise<{ entities: EntitySuggestion[]; total: number }> => {
    const { data } = await api.get('/entities', { params })
    return data
  },

  getAlertas: async (params?: { status?: string; tipo?: string; limit?: number }): Promise<{ alertas: Alerta[]; total: number; nuevas: number }> => {
    const { data } = await api.get('/alerts', { params })
    return data
  },

  markAlertaRead: async (id: number): Promise<void> => {
    await api.post(`/alerts/${id}/read`)
  },

  dismissAlerta: async (id: number): Promise<void> => {
    await api.post(`/alerts/${id}/dismiss`)
  },

  markAllAlertasRead: async (): Promise<void> => {
    await api.post('/alerts/read-all')
  },

  login: async (email: string, password: string): Promise<{ token: string; user: any }> => {
    const { data } = await api.post('/auth/login', { email, password })
    return data
  },

  register: async (email: string, username: string, password: string): Promise<{ token: string; user: any; is_first_user?: boolean }> => {
    const { data } = await api.post('/auth/register', { email, username, password })
    return data
  },

  resetDatabase: async (): Promise<{ message: string; tables_cleared: string[] }> => {
    const { data } = await api.post('/admin/reset-db')
    return data
  },

  backupNewsZipped: async () => {
    try {
      const response = await api.get('/admin/backup/news', {
        responseType: 'blob',
      });
      
      const url = window.URL.createObjectURL(new Blob([response.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', `backup_noticias_${new Date().toISOString().split('T')[0]}.zip`);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Failed to download backup:', error);
      alert('Error al descargar la copia de seguridad. Verifica tu conexión o permisos.');
    }
  },

  restoreDatabase: async (file: File): Promise<{ message: string; filename: string; output?: string }> => {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await api.post('/admin/restore', formData)
    return data
  },

  // Subscription methods
  getSubscriptionStatus: async (): Promise<{
    plan: string
    status: string
    ends_at?: string
    limits: {
      feeds: number
      translations_per_day: number
      search_enabled: boolean
      alerts_limit: number
    }
    needs_upgrade: boolean
  }> => {
    const { data } = await api.get('/app/subscription/status')
    return data
  },

  validateSubscription: async (receiptToken: string, productId: string, period: string): Promise<{
    success: boolean
    plan: string
    status: string
    ends_at?: string
  }> => {
    const { data } = await api.post('/app/subscription/validate', {
      receipt_token: receiptToken,
      product_id: productId,
      period,
    })
    return data
  },

  cancelSubscription: async (): Promise<{ success: boolean; status: string }> => {
    const { data } = await api.post('/app/subscription/cancel')
    return data
  },

  getUsage: async (): Promise<{
    plan: string
    limits: {
      feeds: number
      translations_per_day: number
      search_enabled: boolean
      alerts_limit: number
    }
    usage: {
      feeds_used: number
      translations_today: number
      alerts_used: number
    }
  }> => {
    const { data } = await api.get('/app/usage')
    return data
  },

  checkLimit: async (action: string): Promise<{ allowed: boolean; action: string; reason?: string }> => {
    const { data } = await api.get('/app/limits/check', { params: { action } })
    return data
  },

  // --- Narracion (TTS) ---------------------------------------------------

  getTtsStatus: async (): Promise<{ status: string; engines: Record<string, unknown>; queue: Record<string, unknown> }> => {
    const { data } = await api.get('/tts/status')
    return data
  },

  listTtsVoices: async (): Promise<{ voices: TtsVoice[] }> => {
    const { data } = await api.get('/tts/voices')
    return data
  },

  // Hablantes del modelo base (256 en el modelo español).
  getTtsSpeakers: async (engine?: string): Promise<{ engine: string; speakers: { speaker_id: string; language: string }[] }> => {
    const { data } = await api.get('/tts/speakers', { params: { engine } })
    return data
  },

  // Registra un hablante del modelo como voz utilizable.
  registerTtsSpeaker: async (payload: {
    speaker_id: number
    engine?: string
    display_name?: string
  }): Promise<{ voice: TtsVoice }> => {
    const { data } = await api.post('/tts/speakers', payload)
    return data
  },

  // Sube una muestra de voz propia (solo admin). La voz se clona con
  // Chatterbox por defecto; engine='openvoice' usa la conversion clasica.
  uploadTtsVoice: async (payload: { file: File; slug?: string; display_name?: string; language?: string; engine?: string }): Promise<{ voice: TtsVoice }> => {
    const form = new FormData()
    form.append('sample', payload.file)
    if (payload.slug) form.append('slug', payload.slug)
    if (payload.display_name) form.append('display_name', payload.display_name)
    if (payload.language) form.append('language', payload.language)
    if (payload.engine) form.append('engine', payload.engine)
    const { data } = await api.post('/tts/voices', form, { headers: { 'Content-Type': 'multipart/form-data' } })
    return data
  },

  listTtsScripts: async (params?: { status?: string; noticia_id?: string; page?: number; per_page?: number }): Promise<{ scripts: TtsScript[]; total: number }> => {
    const { data } = await api.get('/tts/scripts', { params })
    return data
  },

  getTtsScript: async (id: number): Promise<TtsScript> => {
    const { data } = await api.get(`/tts/scripts/${id}`)
    return data
  },

  createTtsScript: async (payload: { noticia_id?: string; language?: string; title?: string; content?: string }): Promise<TtsScript> => {
    const { data } = await api.post('/tts/scripts', payload)
    return data
  },

  setTtsScriptStatus: async (id: number, status: 'approved' | 'rejected'): Promise<TtsScript> => {
    const { data } = await api.post(`/tts/scripts/${id}/status`, null, { params: { status } })
    return data
  },

  // Prepara los guiones de las noticias elegidas para poder revisarlos.
  prepareTtsScripts: async (payload: {
    noticia_ids: string[]
    language?: string
  }): Promise<{ scripts: TtsScriptPreview[] }> => {
    const { data } = await api.post('/tts/scripts/prepare', payload)
    return data
  },

  // Guarda el texto revisado por el editor.
  updateTtsScript: async (id: number, content: string): Promise<TtsScript> => {
    const { data } = await api.put(`/tts/scripts/${id}`, { content })
    return data
  },

  listTtsJobs: async (params?: { status?: string; script_id?: number }): Promise<{ jobs: TtsJob[] }> => {
    const { data } = await api.get('/tts/jobs', { params })
    return data
  },

  createTtsJob: async (payload: { script_id: number; voice?: string; engine?: string }): Promise<{ job: TtsJob; created: boolean }> => {
    const { data } = await api.post('/tts/jobs', payload)
    return data
  },

  getTtsJob: async (id: number): Promise<TtsJob> => {
    const { data } = await api.get(`/tts/jobs/${id}`)
    return data
  },

  cancelTtsJob: async (id: number): Promise<TtsJob> => {
    const { data } = await api.post(`/tts/jobs/${id}/cancel`)
    return data
  },

  listTtsAudios: async (params?: { status?: string; script_id?: number }): Promise<{ audios: TtsAudio[] }> => {
    const { data } = await api.get('/tts/audios', { params })
    return data
  },

  getTtsAudio: async (id: number): Promise<TtsAudio> => {
    const { data } = await api.get(`/tts/audios/${id}`)
    return data
  },

  setTtsAudioStatus: async (id: number, status: 'approved' | 'rejected'): Promise<TtsAudio> => {
    const { data } = await api.post(`/tts/audios/${id}/status`, null, { params: { status } })
    return data
  },

  // URL del audio generado. Se sirve con autenticacion: el <audio> necesita
  // el token, por eso el cliente lo descarga como blob.
  getTtsAudioBlob: async (id: number): Promise<Blob> => {
    const { data } = await api.get(`/tts/audios/${id}/file`, { responseType: 'blob' })
    return data
  },

  // --- Noticiario (varias narraciones unidas) ---------------------------

  listTtsBulletins: async (params?: { status?: string }): Promise<{ bulletins: TtsBulletin[] }> => {
    const { data } = await api.get('/tts/bulletins', { params })
    return data
  },

  getTtsBulletin: async (id: number): Promise<TtsBulletin> => {
    const { data } = await api.get(`/tts/bulletins/${id}`)
    return data
  },

  // Crea un noticiario a partir de las noticias elegidas. Los guiones se
  // crean en borrador; la narracion se encola al aprobarlo.
  createTtsBulletin: async (payload: {
    title: string
    language?: string
    noticia_ids?: string[]
    script_ids?: number[]
    silence_ms?: number
    voice?: string
    engine?: string
    speed?: number
  }): Promise<TtsBulletin> => {
    const { data } = await api.post('/tts/bulletins', payload)
    return data
  },

  approveTtsBulletin: async (id: number): Promise<{ bulletin: TtsBulletin; queued: number }> => {
    const { data } = await api.post(`/tts/bulletins/${id}/approve`)
    return data
  },

  // Cancela un noticiario en curso (para salir de un atasco).
  cancelTtsBulletin: async (id: number): Promise<TtsBulletin> => {
    const { data } = await api.post(`/tts/bulletins/${id}/cancel`)
    return data
  },

  setTtsBulletinStatus: async (id: number, status: 'approved' | 'rejected'): Promise<TtsBulletin> => {
    const { data } = await api.post(`/tts/bulletins/${id}/status`, null, { params: { status } })
    return data
  },

  getTtsBulletinBlob: async (id: number): Promise<Blob> => {
    const { data } = await api.get(`/tts/bulletins/${id}/file`, { responseType: 'blob' })
    return data
  },

  // --- Tendencias para el informativo -------------------------------------

  getTtsTrends: async (params?: {
    dias?: number
    importancia?: string
    limite?: number
    min_chars?: number
  }): Promise<TtsTrends> => {
    const { data } = await api.get('/tts/trends', { params })
    return data
  },
}
