import { useEffect, useState } from 'react'
import { apiService, EntitySuggestion } from '../services/api'
import { LineChart } from '../components/LineChart'
import { X, Search, Activity } from 'lucide-react'

const TIPOS = [
  { value: '', label: 'All' },
  { value: 'persona', label: 'People' },
  { value: 'lugar', label: 'Places' },
  { value: 'organizacion', label: 'Organizations' },
  { value: 'tema', label: 'Topics' },
]

const VENTANAS = [
  { value: 1, label: '1h' },
  { value: 2, label: '2h' },
  { value: 3, label: '3h' },
  { value: 4, label: '4h' },
  { value: 5, label: '5h' },
  { value: 6, label: '6h' },
  { value: 12, label: '12h' },
  { value: 24, label: '24h' },
  { value: 48, label: '48h' },
  { value: 168, label: '7d' },
  { value: 360, label: '15d' },
  { value: 720, label: '30d' },
  { value: 1440, label: '60d' },
]

interface Selected {
  valor: string
  tipo: string
}

export function Analisis() {
  const [tipo, setTipo] = useState('')
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<EntitySuggestion[]>([])
  const [selected, setSelected] = useState<Selected[]>([])
  const [ventana, setVentana] = useState(720)
  const [series, setSeries] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [showResults, setShowResults] = useState(false)

  useEffect(() => {
    if (!query.trim()) {
      setResults([])
      return
    }
    const timer = setTimeout(() => {
      apiService
        .searchEntities({ tipo, q: query.trim(), per_page: 8 })
        .then((r) => setResults(r.entities || []))
        .catch(() => setResults([]))
    }, 250)
    return () => clearTimeout(timer)
  }, [query, tipo])

  useEffect(() => {
    if (selected.length === 0) {
      setSeries([])
      return
    }
    setLoading(true)
    apiService
      .getEntityMentions({ values: selected.map((s) => s.valor), hours: ventana })
      .then((r) => setSeries(r.series))
      .catch(() => setSeries([]))
      .finally(() => setLoading(false))
  }, [selected, ventana])

  const add = (valor: string, t: string) => {
    if (selected.some((s) => s.valor.toLowerCase() === valor.toLowerCase())) return
    setSelected([...selected, { valor, tipo: t }])
    setQuery('')
    setResults([])
    setShowResults(false)
  }

  const remove = (valor: string) => {
    setSelected(selected.filter((s) => s.valor !== valor))
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50">News Evolution</h1>
          <p className="text-sm text-gray-400 mt-1">Track entity activity over time</p>
        </div>
      </div>

      <div className="panel">
        <div className="flex flex-col md:flex-row md:items-end gap-4">
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Type</label>
            <select value={tipo} onChange={(e) => setTipo(e.target.value)} className="input w-auto">
              {TIPOS.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </select>
          </div>
          <div className="relative flex-1">
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Search & Add</label>
            <div className="relative">
              <Search className="h-4 w-4 absolute left-3 top-3 text-gray-400" />
              <input
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value)
                  setShowResults(true)
                }}
                onFocus={() => setShowResults(true)}
                placeholder="Search a concept..."
                className="input pl-9"
              />
            </div>
            {showResults && query.trim() && (
              <ul className="absolute z-20 mt-1 w-full bg-gray-900 border border-gray-700 rounded-lg shadow-lg max-h-72 overflow-auto">
                {results.length === 0 && (
                  <li className="px-4 py-2 text-sm text-gray-400">No results</li>
                )}
                {results.map((r) => (
                  <li key={r.valor + r.tipo}>
                    <button
                      onClick={() => add(r.valor, r.tipo)}
                      className="w-full text-left px-4 py-2 text-sm hover:bg-gray-800 flex items-center justify-between gap-2"
                    >
                      <span className="text-gray-200">{r.valor}</span>
                      <span className="text-xs text-gray-500 font-mono">{r.cnt} news</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div>
            <label className="block text-xs font-mono uppercase text-gray-400 mb-2">Window</label>
            <select
              value={ventana}
              onChange={(e) => setVentana(Number(e.target.value))}
              className="input w-auto"
            >
              {VENTANAS.map((v) => (
                <option key={v.value} value={v.value}>{v.label}</option>
              ))}
            </select>
          </div>
        </div>

        {selected.length > 0 && (
          <div className="flex flex-wrap gap-2 mt-4 pt-4 border-t border-gray-700">
            {selected.map((s) => (
              <span
                key={s.valor}
                className="inline-flex items-center gap-1.5 bg-primary-900/30 text-primary-50 text-sm px-3 py-1 rounded-full border border-primary-500/30"
              >
                {s.valor}
                <button onClick={() => remove(s.valor)} className="hover:text-rss-red" title="Remove">
                  <X className="h-3.5 w-3.5" />
                </button>
              </span>
            ))}
          </div>
        )}
      </div>

      {selected.length === 0 ? (
        <div className="panel p-16 text-center text-gray-400">
          <Activity className="w-12 h-12 mx-auto mb-4 text-gray-600" />
          Add at least one concept to view its time evolution.
        </div>
      ) : loading ? (
        <div className="panel p-16 flex justify-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-50" />
        </div>
      ) : (
        <div className="panel">
          <LineChart series={series} />
        </div>
      )}
    </div>
  )
}
