import { useState, useEffect, useCallback } from 'react'
import { News } from '../services/api'

export interface SavedList {
  id: string
  name: string
  newsIds: string[]      // IDs referencing favorites
  createdAt: string
  updatedAt: string
}

const STORAGE_KEY = 'savedLists'

function loadFromStorage(): SavedList[] {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
}

function saveToStorage(lists: SavedList[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(lists))
}

function loadFavorites(): News[] {
  try {
    const stored = localStorage.getItem('favorites')
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
}

export function useSavedLists() {
  const [lists, setLists] = useState<SavedList[]>([])

  useEffect(() => {
    setLists(loadFromStorage())
  }, [])

  const createList = useCallback((name: string): SavedList => {
    const newList: SavedList = {
      id: crypto.randomUUID(),
      name: name.trim(),
      newsIds: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }
    setLists(prev => {
      const updated = [...prev, newList]
      saveToStorage(updated)
      return updated
    })
    return newList
  }, [])

  const deleteList = useCallback((id: string) => {
    setLists(prev => {
      const updated = prev.filter(l => l.id !== id)
      saveToStorage(updated)
      return updated
    })
  }, [])

  const renameList = useCallback((id: string, name: string) => {
    setLists(prev => {
      const updated = prev.map(l =>
        l.id === id ? { ...l, name: name.trim(), updatedAt: new Date().toISOString() } : l
      )
      saveToStorage(updated)
      return updated
    })
  }, [])

  const addNewsToList = useCallback((listId: string, newsId: string) => {
    setLists(prev => {
      const updated = prev.map(l => {
        if (l.id === listId && !l.newsIds.includes(newsId)) {
          return { ...l, newsIds: [...l.newsIds, newsId], updatedAt: new Date().toISOString() }
        }
        return l
      })
      saveToStorage(updated)
      return updated
    })
  }, [])

  const removeNewsFromList = useCallback((listId: string, newsId: string) => {
    setLists(prev => {
      const updated = prev.map(l => {
        if (l.id === listId) {
          return { ...l, newsIds: l.newsIds.filter(id => id !== newsId), updatedAt: new Date().toISOString() }
        }
        return l
      })
      saveToStorage(updated)
      return updated
    })
  }, [])

  const getNewsInList = useCallback((listId: string): News[] => {
    const favorites = loadFavorites()
    const list = lists.find(l => l.id === listId)
    if (!list) return []
    return favorites.filter(n => list.newsIds.includes(n.id))
  }, [lists])

  const isNewsInAnyList = useCallback((newsId: string): boolean => {
    return lists.some(l => l.newsIds.includes(newsId))
  }, [lists])

  const getListsContainingNews = useCallback((newsId: string): SavedList[] => {
    return lists.filter(l => l.newsIds.includes(newsId))
  }, [lists])

  return {
    lists,
    createList,
    deleteList,
    renameList,
    addNewsToList,
    removeNewsFromList,
    getNewsInList,
    isNewsInAnyList,
    getListsContainingNews,
  }
}
