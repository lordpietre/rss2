import { useState, useEffect, useRef, useCallback } from 'react'
import { api } from '../../services/api'
import { X, ZoomIn, ZoomOut, Maximize2, RefreshCw, Network } from 'lucide-react'

interface GraphNode {
  id: number
  valor: string
  tipo: string
  wiki_summary?: string
  news_count: number
  // Layout position (computed)
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

export function EntityGraph() {
  const containerRef = useRef<HTMLDivElement>(null)
  const [graphData, setGraphData] = useState<EntityGraphData>({ nodes: [], links: [] })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null)
  const [tipo, setTipo] = useState<string>('persona')
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [isDragging, setIsDragging] = useState(false)
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 })
  const svgRef = useRef<SVGSVGElement>(null)

  // Compute simple force layout for nodes
  const computeLayout = (nodes: GraphNode[], links: GraphLink[]): GraphNode[] => {
    if (nodes.length === 0) return nodes
    
    // Copy nodes with initial random positions
    const positioned = nodes.map((n) => ({
      ...n,
      x: n.x ?? Math.random() * 600 + 50,
      y: n.y ?? Math.random() * 400 + 50
    }))
    
    const iterations = 100
    for (let i = 0; i < iterations; i++) {
      // Repulsion between all nodes
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

      // Attraction along links
      for (const link of links) {
        const source = positioned.find(n => n.id === link.source)
        const target = positioned.find(n => n.id === link.target)
        if (source && target) {
          const dx = (target.x || 0) - (source.x || 0)
          const dy = (target.y || 0) - (source.y || 0)
          const dist = Math.sqrt(dx * dx + dy * dy) || 1
          const force = (dist - 100) * 0.01
          const fx = (dx / dist) * force
          const fy = (dy / dist) * force
          source.x = (source.x || 0) + fx
          source.y = (source.y || 0) + fy
          target.x = (target.x || 0) - fx
          target.y = (target.y || 0) - fy
        }
      }

      // Center gravity
      for (const node of positioned) {
        node.x = ((node.x || 0) + (350 - (node.x || 0)) * 0.01)
        node.y = ((node.y || 0) + (250 - (node.y || 0)) * 0.01)
      }

      // Boundary constraints
      for (const node of positioned) {
        node.x = Math.max(50, Math.min(650, node.x || 350))
        node.y = Math.max(50, Math.min(450, node.y || 250))
      }
    }
    
    return positioned
  }

  // Fetch graph data
  const fetchGraph = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.get(`/entities/graph`, {
        params: { tipo, limit: 30 }
      })
      const links = Array.isArray(res.data.links) ? res.data.links : []
      // Initialize positions and compute layout
      const nodesWithPositions = computeLayout(res.data.nodes, links)
      setGraphData({ nodes: nodesWithPositions, links })
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to load entity graph')
    } finally {
      setLoading(false)
    }
  }, [tipo])

  useEffect(() => {
    fetchGraph()
  }, [fetchGraph])
 // Only re-run when tipo changes

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault()
    const delta = e.deltaY > 0 ? 0.9 : 1.1
    setZoom(z => Math.max(0.3, Math.min(3, z * delta)))
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    if (e.button === 0) {
      setIsDragging(true)
      setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y })
    }
  }

  const handleMouseMove = (e: React.MouseEvent) => {
    if (isDragging) {
      setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y })
    }
  }

  const handleMouseUp = () => {
    setIsDragging(false)
  }

  const handleZoomIn = () => setZoom(z => Math.min(3, z * 1.3))
  const handleZoomOut = () => setZoom(z => Math.max(0.3, z / 1.3))
  const handleFit = () => { setZoom(1); setPan({ x: 0, y: 0 }) }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <RefreshCw className="h-8 w-8 animate-spin text-rss-green-phosphor mx-auto" />
          <div className="text-sm text-gray-400 mt-2">Loading entity graph...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center text-rss-red">
          <p className="font-mono">{error}</p>
          <button onClick={fetchGraph} className="mt-2 text-sm text-primary-300 hover:text-primary-50">
            Retry
          </button>
        </div>
      </div>
    )
  }

  if (graphData.nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center text-gray-500">
          <Network className="h-12 w-12 mx-auto mb-3 opacity-50" />
          <p>No entity data available</p>
          <p className="text-xs mt-1">Try a different filter or wait for more data</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <select
            value={tipo}
            onChange={(e) => setTipo(e.target.value)}
            className="input w-40"
          >
            <option value="">All types</option>
            <option value="persona">Person</option>
            <option value="organizacion">Organization</option>
            <option value="lugar">Place</option>
            <option value="tema">Topic</option>
          </select>
          <button onClick={fetchGraph} className="btn-secondary flex items-center gap-1">
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>
        
        <div className="flex items-center gap-1">
          <button onClick={handleZoomIn} className="p-2 rounded hover:bg-gray-800 text-gray-400" title="Zoom in">
            <ZoomIn className="w-4 h-4" />
          </button>
          <button onClick={handleZoomOut} className="p-2 rounded hover:bg-gray-800 text-gray-400" title="Zoom out">
            <ZoomOut className="w-4 h-4" />
          </button>
          <button onClick={handleFit} className="p-2 rounded hover:bg-gray-800 text-gray-400" title="Fit to view">
            <Maximize2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Legend */}
      <div className="flex flex-wrap gap-4 text-xs">
        {Object.entries(TIPO_LABELS).map(([key, label]) => (
          <div key={key} className="flex items-center gap-2">
            <div className="w-3 h-3 rounded-full" style={{ backgroundColor: TIPO_COLORS[key] }} />
            <span className="text-gray-400">{label}</span>
          </div>
        ))}
        <div className="text-gray-600 ml-4">
          Node size = recent count | Links = co-occurrences
        </div>
      </div>

      {/* Graph Container */}
      <div 
        ref={containerRef}
        className="relative h-[500px] bg-rss-bg-secondary rounded-lg overflow-hidden border border-rss-border cursor-grab active:cursor-grabbing"
        onWheel={handleWheel}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
      >
        <svg
          ref={svgRef}
          width="100%"
          height="100%"
          viewBox="0 0 700 500"
          style={{ transform: `scale(${zoom}) translate(${pan.x}px, ${pan.y}px)` }}
        >
          {/* Links */}
          <g>
            {graphData.links.map((link, i) => {
              const source = graphData.nodes.find(n => n.id === link.source)
              const target = graphData.nodes.find(n => n.id === link.target)
              if (!source?.x || !target?.x) return null
              const width = Math.min(link.value / 10, 3)
              return (
                <line
                  key={`link-${i}`}
                  x1={source.x}
                  y1={source.y}
                  x2={target.x}
                  y2={target.y}
                  stroke="#26362B"
                  strokeWidth={width}
                  opacity={0.6}
                />
              )
            })}
          </g>

          {/* Nodes */}
          <g>
            {graphData.nodes.map((node) => {
              const size = Math.sqrt(node.news_count + 1) * 3 + 8
              const isSelected = selectedNode?.id === node.id
              return (
                <g
                  key={node.id}
                  transform={`translate(${node.x || 0}, ${node.y || 0})`}
                  onClick={() => setSelectedNode(isSelected ? null : node)}
                  style={{ cursor: 'pointer' }}
                >
                  <circle
                    r={size}
                    fill={TIPO_COLORS[node.tipo] || '#72FF8A'}
                    stroke={isSelected ? '#72FF8A' : '#080B0A'}
                    strokeWidth={isSelected ? 2 : 1}
                    opacity={selectedNode && !isSelected ? 0.4 : 1}
                  />
                  <text
                    y={size + 12}
                    textAnchor="middle"
                    fill="#DCE8DE"
                    fontSize={10}
                    fontFamily="monospace"
                  >
                    {node.valor.length > 15 ? node.valor.substring(0, 12) + '...' : node.valor}
                  </text>
                </g>
              )
            })}
          </g>
        </svg>
        
        {/* Selected Node Panel */}
        {selectedNode && (
          <div className="absolute top-4 right-4 w-72 panel bg-rss-bg-panel/95 backdrop-blur-sm">
            <div className="flex items-start justify-between">
              <div>
                <span 
                  className="text-xs px-2 py-0.5 rounded" 
                  style={{ backgroundColor: (TIPO_COLORS[selectedNode.tipo] || '#72FF8A') + '30', color: TIPO_COLORS[selectedNode.tipo] || '#72FF8A' }}
                >
                  {TIPO_LABELS[selectedNode.tipo] || selectedNode.tipo}
                </span>
                <h3 className="text-lg font-bold text-gray-50 mt-2">{selectedNode.valor}</h3>
              </div>
              <button onClick={() => setSelectedNode(null)} className="text-gray-400 hover:text-gray-200">
                <X className="w-4 h-4" />
              </button>
            </div>
            
            {selectedNode.wiki_summary && (
              <p className="text-gray-400 text-xs line-clamp-3 mt-2">{selectedNode.wiki_summary}</p>
            )}
            
            <div className="mt-4 flex gap-2">
              <a
                href={`/search?entity=${encodeURIComponent(selectedNode.valor)}&tipo=${selectedNode.tipo}`}
                className="btn-primary text-sm flex-1 text-center"
              >
                View News
              </a>
              <a
                href={`/analisis?entity=${encodeURIComponent(selectedNode.valor)}`}
                className="btn-secondary text-sm flex-1 text-center"
              >
                Evolution
              </a>
            </div>
          </div>
        )}
      </div>

      {/* Stats */}
      <div className="text-xs text-gray-500 font-mono flex justify-between">
        <span>{graphData.nodes.length} entities</span>
        <span>{graphData.links.length} connections</span>
        <span>Zoom: {Math.round(zoom * 100)}%</span>
      </div>
    </div>
  )
}
