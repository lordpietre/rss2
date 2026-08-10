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
  return config
})

export interface News {
  id: string
  titulo: string
  resumen: string
  url: string
  fecha: string
  imagen_url?: string
  categoria_id?: number
  pais_id?: number
  fuente_nombre: string
  title_translated?: string
  summary_translated?: string
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

export interface Stats {
  total_news: number
  total_feeds: number
  total_users: number
  total_translated: number
  news_today: number
  news_this_week: number
  news_this_month: number
}

export const apiService = {
  getNews: async (params: { page?: number; per_page?: number; q?: string; category_id?: string; country_id?: string; translated_only?: string }): Promise<NewsListResponse> => {
    const { data } = await api.get('/news', { params })
    return data
  },

  getNewsById: async (id: string): Promise<News> => {
    const { data } = await api.get(`/news/${id}`)
    return data
  },

  getFeeds: async (params?: { page?: number; per_page?: number; activo?: string; categoria_id?: string; pais_id?: string }): Promise<FeedListResponse> => {
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

  toggleFeed: async (id: number): Promise<void> => {
    await api.post(`/feeds/${id}/toggle`)
  },

  reactivateFeed: async (id: number): Promise<void> => {
    await api.post(`/feeds/${id}/reactivate`)
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

  search: async (params: { q: string; page?: number; lang?: string }): Promise<NewsListResponse> => {
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
}
