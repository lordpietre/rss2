/**
 * API Client para RSS2 - Soporta modo Local y Cloud
 * 
 * Modo Cloud: Se conecta a tu servidor central (ej: https://api.tusrss.com)
 * Modo Local: Se conecta al servidor desplegado en el teléfono
 */

import axios, { AxiosInstance, AxiosError } from 'axios'

// =============================================================================
// TIPOS
// =============================================================================

export type ServerMode = 'cloud' | 'local'

export interface ServerConfig {
  mode: ServerMode
  cloudUrl: string      // URL del servidor cloud (ej: https://api.tusrss.com)
  localUrl: string       // URL del servidor local (ej: http://localhost:8080)
  localPort: number      // Puerto del servidor local
}

export interface User {
  id: number
  email: string
  username: string
  is_admin: boolean
  role: string
  plan: string
  subscription_status: string
  subscription_end?: string
  limits: UserLimits
}

export interface UserLimits {
  feeds: number
  translations_per_day: number
  search_enabled: boolean
  alerts_limit: number
}

export interface AuthResponse {
  token: string
  user: User
  is_first_user?: boolean
}

export interface SubscriptionStatus {
  plan: string
  status: string
  ends_at?: string
  limits: UserLimits
  needs_upgrade: boolean
}

export interface UsageStats {
  plan: string
  limits: UserLimits
  usage: {
    feeds_used: number
    translations_today: number
    alerts_used: number
  }
}

export interface LimitCheckResponse {
  allowed: boolean
  action: string
  reason?: string
}

export interface News {
  id: string
  titulo: string
  resumen: string
  url: string
  fecha: string
  imagen_url?: string
  fuente_nombre: string
  title_translated?: string
  summary_translated?: string
  lang_translated?: string
  entities?: Entity[]
}

export interface Entity {
  valor: string
  tipo: string
  count: number
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
  url: string
  activo: boolean
}

export interface FeedListResponse {
  feeds: Feed[]
  total: number
}

// =============================================================================
// CONSTANTES
// =============================================================================

const DEFAULT_CLOUD_URL = 'https://api.rss2app.com'  // Cambiar por tu dominio
const DEFAULT_LOCAL_PORT = 8080

// =============================================================================
// CONFIGURACIÓN
// =============================================================================

class ServerConfigManager {
  private config: ServerConfig
  private listeners: Set<(config: ServerConfig) => void> = new Set()

  constructor() {
    this.config = this.loadConfig()
  }

  private loadConfig(): ServerConfig {
    const stored = localStorage.getItem('server_config')
    if (stored) {
      try {
        return JSON.parse(stored)
      } catch {
        // Config inválida, usar defaults
      }
    }
    return {
      mode: 'cloud',
      cloudUrl: DEFAULT_CLOUD_URL,
      localUrl: 'http://localhost',
      localPort: DEFAULT_LOCAL_PORT,
    }
  }

  private saveConfig(): void {
    localStorage.setItem('server_config', JSON.stringify(this.config))
    this.notifyListeners()
  }

  private notifyListeners(): void {
    this.listeners.forEach(listener => listener(this.config))
  }

  getConfig(): ServerConfig {
    return { ...this.config }
  }

  getMode(): ServerMode {
    return this.config.mode
  }

  getApiUrl(): string {
    if (this.config.mode === 'local') {
      return `${this.config.localUrl}:${this.config.localPort}`
    }
    return this.config.cloudUrl
  }

  setMode(mode: ServerMode): void {
    this.config.mode = mode
    this.saveConfig()
  }

  setCloudUrl(url: string): void {
    this.config.cloudUrl = url
    this.saveConfig()
  }

  setLocalUrl(url: string, port: number): void {
    this.config.localUrl = url
    this.config.localPort = port
    this.saveConfig()
  }

  subscribe(listener: (config: ServerConfig) => void): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  // Detectar si el servidor local está disponible
  async checkLocalServer(): Promise<boolean> {
    try {
      const response = await axios.get(`${this.config.localUrl}:${this.config.localPort}/health`, {
        timeout: 3000,
      })
      return response.data?.status === 'ok'
    } catch {
      return false
    }
  }
}

export const serverConfig = new ServerConfigManager()

// =============================================================================
// API CLIENT
// =============================================================================

class ApiClient {
  private client: AxiosInstance
  private configManager: ServerConfigManager

  constructor() {
    this.configManager = serverConfig
    
    // Crear cliente Axios inicial
    this.client = this.createClient()
    
    // Suscribir a cambios de configuración
    this.configManager.subscribe((config) => {
      this.client = this.createClient(config)
    })
  }

  private createClient(config?: ServerConfig): AxiosInstance {
    const cfg = config || this.configManager.getConfig()
    const baseURL = cfg.mode === 'local' 
      ? `${cfg.localUrl}:${cfg.localPort}` 
      : cfg.cloudUrl

    const instance = axios.create({
      baseURL,
      headers: {
        'Content-Type': 'application/json',
      },
      timeout: 10000, // 10s timeout
    })

    // Interceptor de requests - añadir token
    instance.interceptors.request.use((config) => {
      const token = localStorage.getItem('token')
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      }
      if (config.data instanceof FormData) {
        delete config.headers['Content-Type']
      }
      return config
    })

    // Interceptor de responses - manejar errores
    instance.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => {
        // Error de conexión
        if (error.code === 'ECONNABORTED' || error.code === 'ERR_NETWORK') {
          throw new Error('NO_CONNECTION')
        }
        
        // 401 - Token expirado
        if (error.response?.status === 401) {
          const url = error.config?.url || ''
          const isAuth = url.includes('/auth/login') || url.includes('/auth/register')
          if (!isAuth) {
            localStorage.removeItem('token')
            localStorage.removeItem('user')
            throw new Error('TOKEN_EXPIRED')
          }
        }
        
        throw error
      }
    )

    return instance
  }

  // =============================================================================
  // AUTENTICACIÓN
  // =============================================================================

  async checkFirstUser(): Promise<{ is_first_user: boolean; total_users: number }> {
    const { data } = await this.client.get('/api/auth/check-first-user')
    return data
  }

  async login(email: string, password: string): Promise<AuthResponse> {
    const { data } = await this.client.post('/api/auth/login', { email, password })
    return data
  }

  async register(email: string, username: string, password: string): Promise<AuthResponse> {
    const { data } = await this.client.post('/api/auth/register', { email, username, password })
    return data
  }

  async getCurrentUser(): Promise<User> {
    const { data } = await this.client.get('/api/auth/me')
    return data
  }

  // =============================================================================
  // SUSCRIPCIONES (Cloud only)
  // =============================================================================

  async getSubscriptionStatus(): Promise<SubscriptionStatus> {
    const { data } = await this.client.get('/api/app/subscription/status')
    return data
  }

  async validateSubscription(receiptToken: string, productId: string, period: string): Promise<{ success: boolean; plan: string }> {
    const { data } = await this.client.post('/api/app/subscription/validate', {
      receipt_token: receiptToken,
      product_id: productId,
      period,
    })
    return data
  }

  async cancelSubscription(): Promise<{ success: boolean }> {
    const { data } = await this.client.post('/api/app/subscription/cancel')
    return data
  }

  async getUsage(): Promise<UsageStats> {
    const { data } = await this.client.get('/api/app/usage')
    return data
  }

  async checkLimit(action: 'create_feed' | 'translate' | 'create_alert' | 'search'): Promise<LimitCheckResponse> {
    const { data } = await this.client.get('/api/app/limits/check', { params: { action } })
    return data
  }

  // =============================================================================
  // NEWS
  // =============================================================================

  async getNews(params: {
    page?: number
    per_page?: number
    q?: string
    category_id?: string
    country_id?: string
    translated_only?: string
    lang_to?: string
  }): Promise<NewsListResponse> {
    const { data } = await this.client.get('/api/news', { params })
    return data
  }

  async getNewsById(id: string): Promise<News> {
    const { data } = await this.client.get(`/api/news/${id}`)
    return data
  }

  // =============================================================================
  // FEEDS
  // =============================================================================

  async getFeeds(params?: {
    page?: number
    per_page?: number
    activo?: string
    categoria_id?: string
    pais_id?: string
  }): Promise<FeedListResponse> {
    const { data } = await this.client.get('/api/feeds', { params })
    return data
  }

  async createFeed(feed: {
    nombre: string
    url: string
    descripcion?: string
    categoria_id?: number
    pais_id?: number
  }): Promise<{ id: number }> {
    const { data } = await this.client.post('/api/feeds', feed)
    return data
  }

  async deleteFeed(id: number): Promise<void> {
    await this.client.delete(`/api/feeds/${id}`)
  }

  async toggleFeed(id: number): Promise<void> {
    await this.client.post(`/api/feeds/${id}/toggle`)
  }

  // =============================================================================
  // SEARCH
  // =============================================================================

  async search(params: {
    q: string
    page?: number
    lang?: string
    categoria_id?: string
    pais_id?: string
  }): Promise<NewsListResponse> {
    const { data } = await this.client.get('/api/search', { params })
    return data
  }

  // =============================================================================
  // CATEGORÍAS Y PAÍSES
  // =============================================================================

  async getCategories(): Promise<Array<{ id: number; nombre: string }>> {
    const { data } = await this.client.get('/api/categories')
    return data
  }

  async getCountries(): Promise<Array<{ id: number; nombre: string; continente: string }>> {
    const { data } = await this.client.get('/api/countries')
    return data
  }

  // =============================================================================
  // STATS
  // =============================================================================

  async getStats(): Promise<any> {
    const { data } = await this.client.get('/api/stats')
    return data
  }
}

// Singleton
export const apiClient = new ApiClient()

// Helper para usar directamente
export const getApiUrl = () => serverConfig.getApiUrl()
export const getServerMode = () => serverConfig.getMode()
export const setServerMode = (mode: ServerMode) => serverConfig.setMode(mode)
export const checkLocalServer = () => serverConfig.checkLocalServer()
