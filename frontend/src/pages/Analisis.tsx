import { useEffect, useState } from 'react'
import { apiService, EntitySuggestion } from '../services/api'
import { LineChart } from '../components/LineChart'
import { X, Search, TrendingUp } from 'lucide-react'

const TIPOS = [
  { value: 'persona', label: 'Personas' },
  { value: 'lugar', label: 'Lugares' },
  { value: 'organizacion', label: 'Organizaciones' },
  { value: 'tema', label: 'Temas' },
]

const DIAS = [7, 15, 30, 60, 90]

interface Selected {
  valor: string
  tipo: string
}

export function Analisis() {
  const [tipo, setTipo] = useState('persona')
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<EntitySuggestion[]>([])
  const [selected, setSelected] = useState<Selected[]>([])
  const [days, setDays] = useState(30)
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
      .getEntityMentions({ values: selected.map((s) => s.valor), days })
      .then((r) => setSeries(r.series))
      .catch(() => setSeries([]))
      .finally(() => setLoading(false))
  }, [selected, days])

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
    <div>
      <h1 className="text-3xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
        <TrendingUp className="h-7 w-7 text-primary-600" />
        Evolución de menciones
      </h1>
      <p className="text-gray-500 dark:text-gray-400 mt-1 mb-6">
        Selecciona personas, lugares, organizaciones o temas y compara su actividad día a día.
      </p>

      <div className="card p-4 mb-6">
        <div className="flex flex-col md:flex-row md:items-end gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Tipo</label>
            <select value={tipo} onChange={(e) => setTipo(e.target.value)} className="input w-auto">
              {TIPOS.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </select>
          </div>
          <div className="relative flex-1">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Buscar y añadir</label>
            <div className="relative">
              <Search className="h-4 w-4 absolute left-3 top-3 text-gray-400" />
              <input
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value)
                  setShowResults(true)
                }}
                onFocus={() => setShowResults(true)}
                placeholder="Busca un concepto…"
                className="input pl-9"
              />
            </div>
            {showResults && query.trim() && (
              <ul className="absolute z-20 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg max-h-72 overflow-auto">
                {results.length === 0 && (
                  <li className="px-4 py-2 text-sm text-gray-500">Sin resultados</li>
                )}
                {results.map((r) => (
                  <li key={r.valor + r.tipo}>
                    <button
                      onClick={() => add(r.valor, r.tipo)}
                      className="w-full text-left px-4 py-2 text-sm hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center justify-between gap-2"
                    >
                      <span className="text-gray-800 dark:text-gray-200">{r.valor}</span>
                      <span className="text-xs text-gray-400">{r.cnt} noticias</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Ventana</label>
            <div className="flex gap-1">
              {DIAS.map((d) => (
                <button
                  key={d}
                  onClick={() => setDays(d)}
                  className={`px-3 py-2 rounded-md text-sm border transition-colors ${
                    days === d
                      ? 'bg-primary-600 text-white border-primary-600'
                      : 'border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                  }`}
                >
                  {d}d
                </button>
              ))}
            </div>
          </div>
        </div>

        {selected.length > 0 && (
          <div className="flex flex-wrap gap-2 mt-4 pt-4 border-t border-gray-100 dark:border-gray-700">
            {selected.map((s) => (
              <span
                key={s.valor}
                className="inline-flex items-center gap-1.5 bg-primary-50 dark:bg-gray-700 text-primary-800 dark:text-gray-200 text-sm px-3 py-1 rounded-full"
              >
                {s.valor}
                <button onClick={() => remove(s.valor)} className="hover:text-red-600" title="Quitar">
                  <X className="h-3.5 w-3.5" />
                </button>
              </span>
            ))}
          </div>
        )}
      </div>

      {selected.length === 0 ? (
        <div className="card p-16 text-center text-gray-500 dark:text-gray-400">
          Añade al menos un concepto para ver su evolución por día.
        </div>
      ) : loading ? (
        <div className="card p-16 flex justify-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-600" />
        </div>
      ) : (
        <div className="card p-4">
          <LineChart series={series} />
        </div>
      )}
    </div>
  )
}