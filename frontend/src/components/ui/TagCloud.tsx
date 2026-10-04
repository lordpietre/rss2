import { useNavigate } from 'react-router-dom'

interface Tag {
  valor: string
  tipo: string
  count: number
}

interface TagCloudProps {
  tags: Tag[]
  onTagClick?: (tag: Tag) => void
}

const TIPO_LABELS: Record<string, string> = {
  persona: '👤',
  organizacion: '🏢',
  lugar: '📍',
  tema: '📰',
}

const TIPO_COLORS: Record<string, string> = {
  persona: 'bg-blue-100 text-blue-700 hover:bg-blue-200',
  organizacion: 'bg-purple-100 text-purple-700 hover:bg-purple-200',
  lugar: 'bg-green-100 text-green-700 hover:bg-green-200',
  tema: 'bg-orange-100 text-orange-700 hover:bg-orange-200',
}

export function TagCloud({ tags, onTagClick }: TagCloudProps) {
  const navigate = useNavigate()

  if (tags.length === 0) {
    return (
      <div className="text-sm text-gray-500 italic">
        No hay etiquetas disponibles
      </div>
    )
  }

  // Calculate font sizes based on count
  const maxCount = Math.max(...tags.map(t => t.count))
  const minCount = Math.min(...tags.map(t => t.count))
  const range = maxCount - minCount || 1

  const getFontSize = (count: number): string => {
    const normalized = (count - minCount) / range
    if (normalized > 0.7) return 'text-lg'
    if (normalized > 0.4) return 'text-base'
    return 'text-sm'
  }

  const handleTagClick = (tag: Tag) => {
    if (onTagClick) {
      onTagClick(tag)
    } else {
      // Default behavior: navigate to populares with the tag as search
      navigate(`/populares?tipo=${tag.tipo}&q=${encodeURIComponent(tag.valor)}`)
    }
  }

  return (
    <div className="flex flex-wrap gap-2">
      {tags.map((tag, idx) => (
        <button
          key={`${tag.valor}-${tag.tipo}-${idx}`}
          onClick={() => handleTagClick(tag)}
          className={`
            px-2.5 py-1 rounded-full font-medium transition-colors
            ${TIPO_COLORS[tag.tipo] || 'bg-gray-100 text-gray-700 hover:bg-gray-200'}
            ${getFontSize(tag.count)}
          `}
          title={`${tag.count} menciones`}
        >
          {TIPO_LABELS[tag.tipo] || '📰'} {tag.valor}
        </button>
      ))}
    </div>
  )
}
