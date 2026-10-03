import { useState, useEffect, useCallback } from 'react'

export interface SavedSearch {
  id: string
  type: 'news' | 'entity'
  label: string
  params: Record<string, string | number | boolean>
  createdAt: string
}

const STORAGE_KEY = 'savedSearches'

function loadFromStorage(): SavedSearch[] {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
}

function saveToStorage(searches: SavedSearch[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(searches))
}

export function useSavedSearches() {
  const [searches, setSearches] = useState<SavedSearch[]>([])

  useEffect(() => {
    setSearches(loadFromStorage())
  }, [])

  const addSearch = useCallback((search: Omit<SavedSearch, 'id' | 'createdAt'>) => {
    const newSearch: SavedSearch = {
      ...search,
      id: crypto.randomUUID(),
      createdAt: new Date().toISOString(),
    }
    setSearches(prev => {
      const updated = [...prev, newSearch]
      saveToStorage(updated)
      return updated
    })
    return newSearch
  }, [])

  const removeSearch = useCallback((id: string) => {
    setSearches(prev => {
      const updated = prev.filter(s => s.id !== id)
      saveToStorage(updated)
      return updated
    })
  }, [])

  const updateLabel = useCallback((id: string, label: string) => {
    setSearches(prev => {
      const updated = prev.map(s => s.id === id ? { ...s, label } : s)
      saveToStorage(updated)
      return updated
    })
  }, [])

  const hasActiveFilters = useCallback((type: 'news' | 'entity', params: Record<string, any>): boolean => {
    if (type === 'news') {
      return !!(params.q || params.lang || params.categoria || params.pais)
    }
    // entity
    return !!(params.tipo || params.countryId || params.categoryId || params.q)
  }, [])

  return {
    searches,
    addSearch,
    removeSearch,
    updateLabel,
    hasActiveFilters,
  }
}
