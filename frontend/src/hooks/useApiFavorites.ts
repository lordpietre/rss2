import { useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'
import { News } from '../services/api'

interface SavedList {
  id: number
  name: string
  keywords: string
  created_at: string
  updated_at: string
  item_count: number
}

interface ListTag {
  valor: string
  tipo: string
  count: number
}

interface SavedSearch {
  id: number
  type: 'news' | 'entity'
  label: string
  params: Record<string, any>
  created_at: string
}

// --- Favorites ---
export function useApiFavorites() {
  const [favorites, setFavorites] = useState<News[]>([])
  const [loading, setLoading] = useState(true)

  const fetchFavorites = useCallback(async () => {
    try {
      const res = await api.get('/favorites')
      setFavorites(res.data.favorites || [])
    } catch (err) {
      console.error('Failed to fetch favorites:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchFavorites()
  }, [fetchFavorites])

  const addFavorite = useCallback(async (noticiaId: string) => {
    try {
      await api.post(`/favorites/${noticiaId}`)
      await fetchFavorites()
    } catch (err) {
      console.error('Failed to add favorite:', err)
    }
  }, [fetchFavorites])

  const removeFavorite = useCallback(async (noticiaId: string) => {
    try {
      await api.delete(`/favorites/${noticiaId}`)
      await fetchFavorites()
    } catch (err) {
      console.error('Failed to remove favorite:', err)
    }
  }, [fetchFavorites])

  const toggleFavorite = useCallback(async (news: News) => {
    const isFav = favorites.some(f => f.id === news.id)
    if (isFav) {
      await removeFavorite(news.id)
    } else {
      await addFavorite(news.id)
    }
  }, [favorites, addFavorite, removeFavorite])

  const isFavorite = useCallback((noticiaId: string): boolean => {
    return favorites.some(f => f.id === noticiaId)
  }, [favorites])

  return { favorites, loading, addFavorite, removeFavorite, toggleFavorite, isFavorite, refresh: fetchFavorites }
}

// --- Lists ---
export function useApiLists() {
  const [lists, setLists] = useState<SavedList[]>([])
  const [loading, setLoading] = useState(true)

  const fetchLists = useCallback(async () => {
    try {
      const res = await api.get('/lists')
      setLists(res.data.lists || [])
    } catch (err) {
      console.error('Failed to fetch lists:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchLists()
  }, [fetchLists])

  const createList = useCallback(async (name: string, keywords?: string): Promise<number> => {
    console.log('createList: calling API with name:', name, 'keywords:', keywords)
    const res = await api.post('/lists', { name, keywords: keywords || '' })
    console.log('createList: API response:', res.data)
    await fetchLists()
    return res.data.id as number
  }, [fetchLists])

  const updateList = useCallback(async (id: number, name?: string, keywords?: string) => {
    const data: { name?: string; keywords?: string } = {}
    if (name !== undefined) data.name = name
    if (keywords !== undefined) data.keywords = keywords
    await api.put(`/lists/${id}`, data)
    await fetchLists()
  }, [fetchLists])

  const deleteList = useCallback(async (id: number) => {
    await api.delete(`/lists/${id}`)
    await fetchLists()
  }, [fetchLists])

  const addToList = useCallback(async (listId: number, noticiaId: string) => {
    try {
      await api.post(`/lists/${listId}/items/${noticiaId}`)
      await fetchLists()
    } catch (err) {
      console.error('Failed to add to list:', err)
    }
  }, [fetchLists])

  const removeFromList = useCallback(async (listId: number, noticiaId: string) => {
    try {
      await api.delete(`/lists/${listId}/items/${noticiaId}`)
      await fetchLists()
    } catch (err) {
      console.error('Failed to remove from list:', err)
    }
  }, [fetchLists])

  const getListItems = useCallback(async (listId: number): Promise<News[]> => {
    try {
      const res = await api.get(`/lists/${listId}/items`)
      return res.data.items || []
    } catch (err) {
      console.error('Failed to fetch list items:', err)
      return []
    }
  }, [])

  const getListTags = useCallback(async (listId: number): Promise<ListTag[]> => {
    try {
      const res = await api.get(`/lists/${listId}/tags`)
      return res.data.tags || []
    } catch (err) {
      console.error('Failed to fetch list tags:', err)
      return []
    }
  }, [])

  const getListRelatedNews = useCallback(async (listId: number): Promise<News[]> => {
    try {
      const res = await api.get(`/lists/${listId}/related`)
      return res.data.news || []
    } catch (err) {
      console.error('Failed to fetch related news:', err)
      return []
    }
  }, [])

  return { lists, loading, createList, updateList, deleteList, addToList, removeFromList, getListItems, getListTags, getListRelatedNews, refresh: fetchLists }
}

// --- Suggested News (for sidebar) ---
export function useSuggestedNews() {
  const [suggestedNews, setSuggestedNews] = useState<News[]>([])
  const [loading, setLoading] = useState(true)

  const fetchSuggestedNews = useCallback(async () => {
    try {
      const res = await api.get('/lists/suggested')
      setSuggestedNews(res.data.news || [])
    } catch (err) {
      console.error('Failed to fetch suggested news:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (token) {
      fetchSuggestedNews()
    } else {
      setLoading(false)
    }
  }, [fetchSuggestedNews])

  return { suggestedNews, loading, refresh: fetchSuggestedNews }
}

// --- Saved Searches ---
export function useApiSavedSearches() {
  const [searches, setSearches] = useState<SavedSearch[]>([])
  const [loading, setLoading] = useState(true)

  const fetchSearches = useCallback(async () => {
    try {
      const res = await api.get('/saved-searches')
      setSearches(res.data.searches || [])
    } catch (err) {
      console.error('Failed to fetch saved searches:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchSearches()
  }, [fetchSearches])

  const createSearch = useCallback(async (type: 'news' | 'entity', label: string, params: Record<string, any>) => {
    try {
      await api.post('/saved-searches', { type, label, params })
      await fetchSearches()
    } catch (err) {
      console.error('Failed to create saved search:', err)
    }
  }, [fetchSearches])

  const updateSearch = useCallback(async (id: number, label: string) => {
    try {
      await api.put(`/saved-searches/${id}`, { label })
      await fetchSearches()
    } catch (err) {
      console.error('Failed to update saved search:', err)
    }
  }, [fetchSearches])

  const deleteSearch = useCallback(async (id: number) => {
    try {
      await api.delete(`/saved-searches/${id}`)
      await fetchSearches()
    } catch (err) {
      console.error('Failed to delete saved search:', err)
    }
  }, [fetchSearches])

  return { searches, loading, createSearch, updateSearch, deleteSearch, refresh: fetchSearches }
}
