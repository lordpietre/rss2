import { useState, useEffect } from 'react'

const STORAGE_KEY = 'rss2_search_history'
const MAX_HISTORY = 10

export function useSearchHistory() {
  const [history, setHistory] = useState<string[]>([])

  // Load from localStorage on mount
  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      try {
        setHistory(JSON.parse(stored))
      } catch {
        // Invalid JSON, ignore
      }
    }
  }, [])

  // Add search to history
  const addToHistory = (query: string) => {
    if (!query.trim()) return
    const newHistory = [
      query.trim(),
      ...history.filter(h => h !== query.trim())
    ].slice(0, MAX_HISTORY)
    setHistory(newHistory)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(newHistory))
  }

  // Clear all history
  const clearHistory = () => {
    setHistory([])
    localStorage.removeItem(STORAGE_KEY)
  }

  // Remove single item from history
  const removeFromHistory = (query: string) => {
    const newHistory = history.filter(h => h !== query)
    setHistory(newHistory)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(newHistory))
  }

  return { history, addToHistory, clearHistory, removeFromHistory }
}
