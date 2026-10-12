import { useState, useEffect, useRef, useCallback } from 'react'
import { X, ZoomIn, ZoomOut, Maximize2, RefreshCw, Network, Search, Loader2 } from 'lucide-react'

interface GraphNode {
  id: number
  valor: string
  tipo: string
  wiki_summary?: string
  news_count: number
  x?: number
  y?: number
}

interface GraphLink {
  source: number
  target: number
  value: number
}

interface EntityGraphData {
  nodes: GraphNode[]
  links: GraphLink[]
}

interface EntitySuggestion {
  id: number
  valor: string
  tipo: string
  count: number
  wiki_summary?: string
}

const TIPO_COLORS: Record<string, string> = {
  persona: '#78B9FF',
  organizacion: '#FFC66D',
  lugar: '#72FF8A',
  tema: '#B794F4',
}

const TIPO_LABELS: Record<string, string> = {
  persona: 'Person',
  organizacion: 'Organization',
  lugar: 'Place',
  tema: 'Topic',
}

const TIPO_OPTIONS = [
  { value: '', label: 'All' },
  { value: 'persona', label: 'Person' },
  { value: 'organizacion', label: 'Organization' },
  { value: 'lugar', label: 'Place' },
  { value: 'tema', label: 'Topic' },
]

export function EntityGraph() {
  const containerRef = useRef<HTMLDivElement>(null)
  const [graphData, setGraphData] = useState<EntityGraphData>({ nodes: [], links: [] })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null)
  const [tipo, setTipo] = useState<string>('')
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [isDragging, setIsDragging] = useState(false)
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 })
  const svgRef = useRef<SVGSVGElement>(null)

  // Search state
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<EntitySuggestion[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [focusedEntityId, setFocusedEntityId] = useState<number | null>(null)
  const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Compute simple force layout for nodes
  const computeLayout = useCallback((nodes: GraphNode[], links: GraphLink[]): GraphNode[] => {
    if (nodes.length === 0) return nodes
    
    const positioned = nodes.map((n) => ({
      ...n,
      x: n.x ?? Math.random() * 600 + 50,
      y: n.y ?? Math.random() * 400 + 50
    }))
    
    const iterations = 100
    for (let i = 0; i < iterations; i++) {
      for (let a = 0; a < positioned.length; a++) {
        for (let b = a + 1; b < positioned.length; b++) {
          const dx = (positioned[b].x || 0) - (positioned[a].x || 0)
          const dy = (positioned[b].y || 0) - (positioned[a].y || 0)
          const dist = Math.sqrt(dx * dx + dy * dy) || 1
          const force = 2000 / (dist * dist)
          const fx = (dx / dist) * force
          const fy = (dy / dist) * force
          positioned[a].x = (positioned[a].x || 0) - fx
          positioned[a].y = (positioned[a].y || 0) - fy
          positioned[b].x = (positioned[b].x || 0) + fx
          positioned[b].y = (positioned[b].y || 0) + fy
        }
      }
      
      for (const link of links) {
        const source = positioned.find(n => n.id === link.source)
        const target = positioned.find(n => n.id === link.target)
        if (source && target) {
          const dx = (target.x || 0) - (source.x || 0)
          const dy = (target.y || 0) - (source.y || 0)
          const dist = Math.sqrt(dx * dx + dy * dy) || 1
          const idealDist = 120
          const force = (dist - idealDist) * 0.05
          const fx = (dx / dist) * force
          const fy = (dy / dist) * force
          source.x = (source.x || 0) - fx
          source.y = (source.y || 0) - fy
          target.x = (target.x || 0) + fx
          target.y = (target.y || 0) + fy
        }
      }
    }
    
    const centerX = 400
    const centerY = 300
    const minX = Math.min(...positioned.map(n => n.x || 0))
    const maxX = Math.max(...positioned.map(n => n.x || 0))
    const minY = Math.min(...positioned.map(n => n.y || 0))
    const maxY = Math.max(...positioned.map(n => n.y || 0))
    const offsetX = centerX - (minX + maxX) / 2
    const offsetY = centerY - (minY + maxY) / 2
    
    return positioned.map(n => ({
      ...n,
      x: (n.x || 0) + offsetX,
      y: (n.y || 0) + offsetY
    }))
  }, [])

  // Fetch graph data
  const fetchGraph = useCallback(async (params: { search?: string; entity_id?: number; tipo?: string; limit?: number }) => {
    setLoading(true)
    setError(null)
    try {
      const queryParams = new URLSearchParams()
      if (params.search) queryParams.set('search', params.search)
      if (params.entity_id) queryParams.set('entity_id', params.entity_id.toString())
      if (params.tipo) queryParams.set('tipo', params.tipo)
      queryParams.set('limit', '30')
      
      const response = await fetch(`/api/entities/graph?${queryParams.toString()}`)
      if (!response.ok) throw new Error('Failed to fetch graph data')
      
      const data = await response.json()
      const nodesWithLayout = computeLayout(data.nodes || [], data.links || [])
      setGraphData({ nodes: nodesWithLayout, links: data.links || [] })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error')
    } finally {
      setLoading(false)
    }
  }, [computeLayout])

  // Initial load
  useEffect(() => {
    fetchGraph({ tipo })
  }, [fetchGraph, tipo])

  // Search entities with debounce
  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current)
    }
    
    if (searchQuery.length < 2) {
      setSearchResults([])
      return
    }
    
    setIsSearching(true)
    searchTimeoutRef.current = setTimeout(async () => {
      try {
        const params = new URLSearchParams()
        if (tipo) params.set('tipo', tipo)
        if (searchQuery) params.set('q', searchQuery)
        params.set('per_page', '10')
        const response = await fetch(`/api/entities?${params.toString()}`)
        const data = await response.json()
        setSearchResults(data.entities || [])
      } catch (err) {
        console.error('Search failed:', err)
        setSearchResults([])
      } finally {
        setIsSearching(false)
      }
    }, 300)
    
    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current)
      }
    }
  }, [searchQuery, tipo])

  // Handle entity selection from search results
  const handleEntitySelect = (entity: EntitySuggestion) => {
    setSearchQuery('')
    setSearchResults([])
    setFocusedEntityId(entity.id)
    fetchGraph({ entity_id: entity.id, tipo })
  }

  // Handle type change
  const handleTipoChange = (newTipo: string) => {
    setTipo(newTipo)
    setFocusedEntityId(null)
    setSearchQuery('')
    setSearchResults([])
  }

  // Reset to default view
  const handleReset = () => {
    setTipo('')
    setFocusedEntityId(null)
    setSearchQuery('')
    setSearchResults([])
    fetchGraph({ tipo: '' })
  }

  // Mouse handlers for pan
  const handleMouseDown = (e: React.MouseEvent) => {
    if (e.button !== 0) return
    setIsDragging(true)
    setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y })
  }

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!isDragging) return
    setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y })
  }

  const handleMouseUp = () => {
    setIsDragging(false)
  }

  // Wheel zoom
  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault()
    const delta = e.deltaY > 0 ? 0.9 : 1.1
    setZoom(z => Math.min(Math.max(z * delta, 0.2), 5))
  }

  // Node click handler
  const handleNodeClick = (node: GraphNode) => {
    setSelectedNode(node)
  }

  const layoutedNodes = graphData.nodes
  const nodeMap = new Map(layoutedNodes.map(n => [n.id, n]))

  return (
    <div className="flex flex-col h-full bg-rss-bg text-rss-text">
      {/* Search Bar */}
      <div className="flex items-center gap-3 p-4 border-b border-rss-border">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search entities..."
            className="w-full pl-10 pr-4 py-2 bg-rss-bg-panel border border-rss-border rounded-lg text-rss-text placeholder:text-gray-500 focus:outline-none focus:border-rss-green"
          />
          {isSearching && (
            <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-rss-green animate-spin" />
          )}
          
          {/* Search Results Dropdown */}
          {searchResults.length > 0 && (
            <div className="absolute top-full left-0 right-0 mt-1 bg-rss-bg-panel border border-rss-border rounded-lg shadow-xl z-50 max-h-64 overflow-y-auto">
              {searchResults.map((entity) => (
                <button
                  key={entity.id}
                  onClick={() => handleEntitySelect(entity)}
                  className="w-full px-4 py-2 text-left hover:bg-rss-border/50 flex items-center gap-2"
                >
                  <span
                    className="px-2 py-0.5 text-xs rounded"
                    style={{ backgroundColor: TIPO_COLORS[entity.tipo] + '30', color: TIPO_COLORS[entity.tipo] }}
                  >
                    {TIPO_LABELS[entity.tipo]}
                  </span>
                  <span className="flex-1 truncate">{entity.valor}</span>
                  <span className="text-xs text-gray-500">{entity.count}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Type Filter */}
        <select
          value={tipo}
          onChange={(e) => handleTipoChange(e.target.value)}
          className="px-3 py-2 bg-rss-bg-panel border border-rss-border rounded-lg text-rss-text focus:outline-none focus:border-rss-green"
        >
          {TIPO_OPTIONS.map(opt => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>

        {/* Focused Entity Info */}
        {focusedEntityId && (
          <div className="flex items-center gap-2 px-3 py-1 bg-rss-bg-panel border border-rss-green rounded-lg">
            <span className="text-sm text-rss-green">Focused on entity</span>
            <button
              onClick={handleReset}
              className="p-1 hover:bg-rss-border rounded"
            >
              <X className="w-3 h-3" />
            </button>
          </div>
        )}

        {/* Reset Button */}
        <button
          onClick={handleReset}
          className="p-2 bg-rss-bg-panel border border-rss-border rounded-lg hover:border-rss-green"
          title="Reset view"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      {/* Graph Container */}
      <div
        ref={containerRef}
        className="flex-1 relative overflow-hidden"
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
        onWheel={handleWheel}
      >
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-rss-bg/80 z-10">
            <Loader2 className="w-8 h-8 text-rss-green animate-spin" />
          </div>
        )}
        
        {error && (
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="text-red-400 text-center">
              <p className="mb-2">Error: {error}</p>
              <button
                onClick={() => fetchGraph({ tipo })}
                className="px-4 py-2 bg-rss-green/20 hover:bg-rss-green/30 rounded"
              >
                Retry
              </button>
            </div>
          </div>
        )}

        {!loading && !error && graphData.nodes.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center text-gray-500">
            <div className="text-center">
              <Network className="w-12 h-12 mx-auto mb-2 opacity-50" />
              <p>No entities found. Try a different search.</p>
            </div>
          </div>
        )}

        <svg
          ref={svgRef}
          className="w-full h-full cursor-grab active:cursor-grabbing"
          viewBox="0 0 800 600"
          preserveAspectRatio="xMidYMid meet"
        >
          <g transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}>
            {/* Links */}
            {graphData.links.map((link, i) => {
              const source = nodeMap.get(link.source)
              const target = nodeMap.get(link.target)
              if (!source || !target) return null
              return (
                <line
                  key={`link-${i}`}
                  x1={source.x} y1={source.y}
                  x2={target.x} y2={target.y}
                  stroke="#374151"
                  strokeWidth={Math.min(link.value / 30, 4)}
                  strokeOpacity={0.6}
                />
              )
            })}
            
            {/* Nodes */}
            {layoutedNodes.map((node) => (
              <g
                key={node.id}
                transform={`translate(${node.x}, ${node.y})`}
                onClick={() => handleNodeClick(node)}
                className="cursor-pointer"
              >
                <circle
                  r={focusedEntityId === node.id ? 16 : 12}
                  fill={TIPO_COLORS[node.tipo] || '#888'}
                  stroke={selectedNode?.id === node.id ? '#fff' : 'transparent'}
                  strokeWidth={2}
                />
                <text
                  dy={25}
                  textAnchor="middle"
                  className="text-xs fill-rss-text pointer-events-none"
                  style={{ fontSize: '10px' }}
                >
                  {node.valor.length > 15 ? node.valor.slice(0, 15) + '...' : node.valor}
                </text>
              </g>
            ))}
          </g>
        </svg>
      </div>

      {/* Legend */}
      <div className="flex items-center justify-center gap-6 p-2 border-t border-rss-border text-xs">
        {Object.entries(TIPO_LABELS).map(([tipo, label]) => (
          <div key={tipo} className="flex items-center gap-1">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: TIPO_COLORS[tipo] }}
            />
            <span className="text-gray-400">{label}</span>
          </div>
        ))}
      </div>

      {/* Selected Node Panel */}
      {selectedNode && (
        <div className="absolute bottom-16 right-4 w-72 bg-rss-bg-panel border border-rss-border rounded-lg shadow-xl p-4">
          <div className="flex items-start justify-between mb-2">
            <div>
              <h3 className="font-semibold text-rss-text">{selectedNode.valor}</h3>
              <span
                className="inline-block px-2 py-0.5 text-xs rounded mt-1"
                style={{ backgroundColor: TIPO_COLORS[selectedNode.tipo] + '30', color: TIPO_COLORS[selectedNode.tipo] }}
              >
                {TIPO_LABELS[selectedNode.tipo]}
              </span>
            </div>
            <button
              onClick={() => setSelectedNode(null)}
              className="p-1 hover:bg-rss-border rounded"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
          
          {selectedNode.wiki_summary && (
            <p className="text-sm text-gray-400 mb-3 line-clamp-3">
              {selectedNode.wiki_summary}
            </p>
          )}
          
          <div className="flex items-center gap-4 text-sm">
            <div>
              <span className="text-gray-500">News:</span>{' '}
              <span className="text-rss-green">{selectedNode.news_count}</span>
            </div>
            <button
              onClick={() => {
                setFocusedEntityId(selectedNode.id)
                fetchGraph({ entity_id: selectedNode.id, tipo })
              }}
              className="text-rss-green hover:underline"
            >
              View relations →
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
