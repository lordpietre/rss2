import { useQuery } from '@tanstack/react-query'
import { apiService, Stats } from '../services/api'
import { Link } from 'react-router-dom'
import { 
  Globe, TrendingUp, Activity, Newspaper, 
  MapPin, Radio, Languages, ArrowUp, ArrowDown, Clock, Hash
} from 'lucide-react'
import { useState, useMemo } from 'react'

// Simple SVG-based bar chart component
function MiniBarChart({ data, color }: { data: Array<{ hour: string; total: number }>; color: string }) {
  if (!data || data.length === 0) return <div className="h-full flex items-center justify-center text-gray-500 text-xs">No data</div>
  
  const max = Math.max(...data.map(d => d.total), 1)
  
  return (
    <div className="flex items-end gap-0.5 h-full">
      {data.map((d, i) => {
        const height = (d.total / max) * 100
        return (
          <div 
            key={i} 
            className="flex-1 rounded-t transition-all hover:opacity-80"
            style={{ 
              height: `${Math.max(height, 4)}%`,
              backgroundColor: color,
              opacity: 0.7 + (i / data.length) * 0.3
            }}
            title={`${d.hour}: ${d.total}`}
          />
        )
      })}
    </div>
  )
}

// Language bar component
function LanguageBar({ lang, count, max, flag }: { lang: string; count: number; max: number; flag: string }) {
  const width = (count / max) * 100
  return (
    <div className="flex items-center gap-2">
      <span className="text-lg w-6">{flag}</span>
      <span className="w-12 text-xs font-mono text-gray-400">{lang.toUpperCase()}</span>
      <div className="flex-1 h-2 bg-gray-800 rounded-full overflow-hidden">
        <div 
          className="h-full rounded-full transition-all duration-500"
          style={{ width: `${width}%`, backgroundColor: '#72FF8A' }}
        />
      </div>
      <span className="w-16 text-right text-xs font-mono text-gray-300">{count.toLocaleString()}</span>
    </div>
  )
}

// Category bar component  
function CategoryBar({ name, count, max, color }: { name: string; count: number; max: number; color: string }) {
  const width = (count / max) * 100
  return (
    <div className="flex items-center gap-2">
      <span className="w-4 h-4 rounded" style={{ backgroundColor: color }} />
      <span className="flex-1 text-sm text-gray-300 truncate">{name}</span>
      <div className="w-24 h-2 bg-gray-800 rounded-full overflow-hidden">
        <div 
          className="h-full rounded-full transition-all duration-500"
          style={{ width: `${width}%`, backgroundColor: color }}
        />
      </div>
      <span className="w-16 text-right text-xs font-mono text-gray-400">{count.toLocaleString()}</span>
    </div>
  )
}

const CATEGORY_COLORS = [
  '#72FF8A', '#78B9FF', '#FFC66D', '#FF5F64', '#B794F4',
  '#89B4FA', '#F5A623', '#F472B6', '#34D399', '#FBBF24'
]

export function GlobalMonitor() {
  const { data: stats } = useQuery<Stats>({
    queryKey: ['stats'],
    queryFn: () => apiService.getStats(),
    refetchInterval: 30000,
  })

  const [timeRange, setTimeRange] = useState<'12h' | '24h' | '2d' | '3d'>('24h')

  // Get translation activity data based on time range
  const activityData = useMemo(() => {
    switch (timeRange) {
      case '12h': return stats?.translation_stats_12h || []
      case '24h': return stats?.translation_stats_24h || []
      case '2d': return stats?.translation_stats_2d || []
      case '3d': return stats?.translation_stats_3d || []
      default: return stats?.translation_stats_24h || []
    }
  }, [stats, timeRange])

  // Max values for bars
  const maxLangCount = useMemo(() => 
    Math.max(...(stats?.top_languages?.map(l => l.count) || [1]), 1),
  [stats])

  const maxCatCount = useMemo(() => 
    Math.max(...(stats?.top_categories?.map(c => c.count) || [1]), 1),
  [stats])

  const maxCountryCount = useMemo(() => 
    Math.max(...(stats?.top_countries?.map(c => c.count) || [1]), 1),
  [stats])

  // Calculate totals
  const totalActivity = useMemo(() => 
    activityData.reduce((sum, d) => sum + d.total, 0),
  [activityData])

  const totalItems = useMemo(() => 
    activityData.reduce((sum, d) => sum + d.items, 0),
  [activityData])

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-50 flex items-center gap-3">
            <Globe className="h-6 w-6 text-primary-50" />
            Global Intelligence
          </h1>
          <p className="text-sm text-gray-400 mt-1">
            Real-time news aggregation and translation monitoring
          </p>
        </div>
        <div className="flex items-center gap-2 text-xs font-mono">
          <span className="w-2 h-2 rounded-full bg-primary-50 animate-pulse" />
          <span className="text-gray-400">Live</span>
        </div>
      </div>

      {/* Top Metrics Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="panel relative overflow-hidden">
          <div className="absolute inset-0 opacity-10">
            <MiniBarChart data={activityData.slice(-12)} color="#72FF8A" />
          </div>
          <div className="relative">
            <div className="flex items-center gap-2 mb-2">
              <Newspaper className="h-4 w-4 text-primary-50" />
              <span className="text-xs font-mono uppercase text-gray-400">Total News</span>
            </div>
            <div className="text-3xl font-bold text-gray-50 font-mono">
              {stats?.total_news?.toLocaleString() || '—'}
            </div>
            <div className="flex items-center gap-3 mt-2 text-xs">
              <span className="text-gray-400">Today: <span className="text-primary-50 font-mono">{stats?.news_today?.toLocaleString() || 0}</span></span>
              <span className="text-gray-500">|</span>
              <span className="text-gray-400">Week: <span className="text-gray-300 font-mono">{stats?.news_this_week?.toLocaleString() || 0}</span></span>
            </div>
          </div>
        </div>

        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <Radio className="h-4 w-4 text-blue-400" />
            <span className="text-xs font-mono uppercase text-gray-400">Active Feeds</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {stats?.total_feeds?.toLocaleString() || '—'}
          </div>
          <div className="flex items-center gap-2 mt-2 text-xs">
            <span className="text-gray-400">Sources monitored</span>
          </div>
        </div>

        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <Hash className="h-4 w-4 text-purple-400" />
            <span className="text-xs font-mono uppercase text-gray-400">Translated</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {stats?.total_translated?.toLocaleString() || '—'}
          </div>
          <div className="flex items-center gap-3 mt-2 text-xs">
            <span className="text-gray-400">Pending: <span className="text-amber-400 font-mono">{stats?.translations_pending || 0}</span></span>
            <span className="text-gray-400">Error: <span className="text-rss-red font-mono">{stats?.translations_error || 0}</span></span>
          </div>
        </div>

        <div className="panel">
          <div className="flex items-center gap-2 mb-2">
            <Activity className="h-4 w-4 text-amber-400" />
            <span className="text-xs font-mono uppercase text-gray-400">This Month</span>
          </div>
          <div className="text-3xl font-bold text-gray-50 font-mono">
            {stats?.news_this_month?.toLocaleString() || '—'}
          </div>
          <div className="flex items-center gap-2 mt-2 text-xs">
            <span className="text-gray-400">News aggregated</span>
          </div>
        </div>
      </div>

      {/* Translation Activity Chart */}
      <div className="panel">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2">
              <TrendingUp className="h-5 w-5 text-primary-50" />
              Translation Activity
            </h2>
            <p className="text-xs text-gray-400 mt-1">
              {totalActivity.toLocaleString()} translations · {totalItems.toLocaleString()} items processed
            </p>
          </div>
          <div className="flex gap-1">
            {(['12h', '24h', '2d', '3d'] as const).map((range) => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={`px-3 py-1.5 text-xs font-mono rounded transition-all ${
                  timeRange === range
                    ? 'bg-primary-50 text-gray-900'
                    : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
                }`}
              >
                {range}
              </button>
            ))}
          </div>
        </div>
        
        <div className="h-32">
          <MiniBarChart data={activityData} color="#72FF8A" />
        </div>
        
        <div className="flex justify-between mt-2 text-xs font-mono text-gray-500">
          {activityData.slice(0, 1).map(d => <span key={d.hour}>{d.hour}</span>)}
          <span>Now</span>
        </div>
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Languages */}
        <div className="panel">
          <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2 mb-4">
            <Languages className="h-5 w-5 text-primary-50" />
            Languages
          </h2>
          <div className="space-y-3">
            {stats?.top_languages?.slice(0, 8).map((lang) => (
              <LanguageBar 
                key={lang.lang} 
                lang={lang.lang} 
                count={lang.count} 
                max={maxLangCount}
                flag={getLanguageFlag(lang.lang)}
              />
            ))}
          </div>
          {stats?.top_languages && stats.top_languages.length > 8 && (
            <p className="text-xs text-gray-500 mt-3 text-center">
              +{stats.top_languages.length - 8} more languages
            </p>
          )}
        </div>

        {/* Categories */}
        <div className="panel">
          <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2 mb-4">
            <Activity className="h-5 w-5 text-primary-50" />
            Categories
          </h2>
          <div className="space-y-3">
            {stats?.top_categories?.slice(0, 8).map((cat, idx) => (
              <CategoryBar 
                key={cat.categoria_id} 
                name={cat.categoria_nombre} 
                count={cat.count} 
                max={maxCatCount}
                color={CATEGORY_COLORS[idx % CATEGORY_COLORS.length]}
              />
            ))}
          </div>
          {stats?.top_categories && stats.top_categories.length > 8 && (
            <p className="text-xs text-gray-500 mt-3 text-center">
              +{stats.top_categories.length - 8} more categories
            </p>
          )}
        </div>

        {/* Countries */}
        <div className="panel">
          <h2 className="text-lg font-semibold text-gray-50 flex items-center gap-2 mb-4">
            <MapPin className="h-5 w-5 text-primary-50" />
            Top Countries
          </h2>
          <div className="space-y-3">
            {stats?.top_countries?.slice(0, 10).map((country, idx) => {
              const width = (country.count / maxCountryCount) * 100
              return (
                <Link
                  key={country.pais_id}
                  to={`/search?country_id=${country.pais_id}`}
                  className="flex items-center gap-3 group"
                >
                  <span className="w-5 text-center text-lg">{country.flag_emoji}</span>
                  <span className="w-6 text-xs font-mono text-gray-500">{idx + 1}</span>
                  <span className="flex-1 text-sm text-gray-300 group-hover:text-primary-50 truncate">
                    {country.pais_nombre}
                  </span>
                  <div className="w-20 h-2 bg-gray-800 rounded-full overflow-hidden">
                    <div 
                      className="h-full rounded-full transition-all group-hover:bg-primary-50"
                      style={{ width: `${width}%`, backgroundColor: '#72FF8A' }}
                    />
                  </div>
                  <span className="w-16 text-right text-xs font-mono text-gray-400">
                    {country.count.toLocaleString()}
                  </span>
                </Link>
              )
            })}
          </div>
          <Link 
            to="/feeds"
            className="block mt-4 text-center text-xs text-primary-300 hover:text-primary-50"
          >
            View all countries →
          </Link>
        </div>
      </div>

      {/* Bottom Row - Quick Stats */}
      <div className="grid grid-cols-3 lg:grid-cols-6 gap-4">
        <Link to="/search" className="panel hover:border-primary-500/50 transition-colors group">
          <div className="text-xs font-mono uppercase text-gray-500 mb-1">Search</div>
          <div className="text-lg font-bold text-gray-50 group-hover:text-primary-50">News</div>
        </Link>
        <Link to="/search?translated_only=true" className="panel hover:border-primary-500/50 transition-colors group">
          <div className="text-xs font-mono uppercase text-gray-500 mb-1">Translated</div>
          <div className="text-lg font-bold text-gray-50 group-hover:text-primary-50">Only</div>
        </Link>
        <Link to="/search?semantic=true" className="panel hover:border-primary-500/50 transition-colors group">
          <div className="text-xs font-mono uppercase text-gray-500 mb-1">Semantic</div>
          <div className="text-lg font-bold text-gray-50 group-hover:text-primary-50">Search</div>
        </Link>
        <Link to="/populares" className="panel hover:border-primary-500/50 transition-colors group">
          <div className="text-xs font-mono uppercase text-gray-500 mb-1">Entity</div>
          <div className="text-lg font-bold text-gray-50 group-hover:text-primary-50">Network</div>
        </Link>
        <Link to="/alertas" className="panel hover:border-primary-500/50 transition-colors group">
          <div className="text-xs font-mono uppercase text-gray-500 mb-1">Alert</div>
          <div className="text-lg font-bold text-gray-50 group-hover:text-primary-50">Center</div>
        </Link>
        <Link to="/analisis" className="panel hover:border-primary-500/50 transition-colors group">
          <div className="text-xs font-mono uppercase text-gray-500 mb-1">News</div>
          <div className="text-lg font-bold text-gray-50 group-hover:text-primary-50">Evolution</div>
        </Link>
      </div>
    </div>
  )
}

// Language flag emoji helper
function getLanguageFlag(lang: string): string {
  const flags: Record<string, string> = {
    'es': 'ES', 'en': 'GB', 'fr': 'FR', 'de': 'DE', 'it': 'IT',
    'pt': 'PT', 'ru': 'RU', 'zh': 'CN', 'ja': 'JP', 'ar': 'SA',
    'ko': 'KR', 'hi': 'IN', 'tr': 'TR', 'pl': 'PL', 'nl': 'NL',
    'sv': 'SE', 'da': 'DK', 'no': 'NO', 'fi': 'FI', 'cs': 'CZ',
    'hu': 'HU', 'ro': 'RO', 'uk': 'UA', 'el': 'GR', 'he': 'IL',
    'th': 'TH', 'vi': 'VN', 'id': 'ID', 'ms': 'MY', 'fa': 'IR',
  }
  return flags[lang.toLowerCase()] || lang.toUpperCase()
}
