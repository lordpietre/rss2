import { Search, Rss, Globe, AlertTriangle, Inbox } from 'lucide-react'

interface EmptyStateProps {
  icon?: 'search' | 'rss' | 'globe' | 'alert' | 'inbox'
  title: string
  description: string
  action?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({
  icon = 'search',
  title,
  description,
  action,
  className = ''
}: EmptyStateProps) {
  const icons = {
    search: Search,
    rss: Rss,
    globe: Globe,
    alert: AlertTriangle,
    inbox: Inbox,
  }

  const Icon = icons[icon]

  return (
    <div className={`flex flex-col items-center justify-center py-16 px-4 text-center ${className}`}>
      <div className="w-16 h-16 bg-gray-100 rounded-full flex items-center justify-center mb-4">
        <Icon className="w-8 h-8 text-gray-400" />
      </div>
      <h3 className="text-lg font-medium text-gray-900 mb-2">{title}</h3>
      <p className="text-gray-500 max-w-md mb-6">{description}</p>
      {action && (
        <button
          onClick={action.onClick}
          className="btn-primary"
        >
          {action.label}
        </button>
      )}
    </div>
  )
}

export function NoResultsFound({
  query,
  onClearFilters
}: {
  query?: string
  onClearFilters?: () => void
}) {
  return (
    <EmptyState
      icon="search"
      title="No se encontraron resultados"
      description={query
        ? `No hay noticias que coincidan con "${query}". Intenta con otros términos de búsqueda.`
        : 'No hay noticias que coincidan con los filtros seleccionados.'}
      action={onClearFilters ? {
        label: 'Limpiar filtros',
        onClick: onClearFilters
      } : undefined}
    />
  )
}

export function NoNewsYet({
  onAddFeed
}: {
  onAddFeed?: () => void
}) {
  return (
    <EmptyState
      icon="rss"
      title="No hay noticias todavía"
      description="Aún no se han agregado fuentes RSS. Agrega tu primera fuente para empezar a ver noticias."
      action={onAddFeed ? {
        label: 'Agregar fuente RSS',
        onClick: onAddFeed
      } : undefined}
    />
  )
}

export function NoFeedsConfigured({
  onAddFeed
}: {
  onAddFeed?: () => void
}) {
  return (
    <EmptyState
      icon="rss"
      title="No hay fuentes configuradas"
      description="No se han agregado fuentes RSS. Agrega tu primera fuente para empezar a recibir noticias."
      action={onAddFeed ? {
        label: 'Agregar fuente RSS',
        onClick: onAddFeed
      } : undefined}
    />
  )
}

export function NoSearchResults({
  query,
  onSearch
}: {
  query?: string
  onSearch?: () => void
}) {
  return (
    <EmptyState
      icon="search"
      title="Sin resultados"
      description={query
        ? `No se encontraron resultados para "${query}". Prueba con términos más generales.`
        : 'Ingresa un término de búsqueda para encontrar noticias.'}
      action={onSearch ? {
        label: 'Nueva búsqueda',
        onClick: onSearch
      } : undefined}
    />
  )
}

export function OfflineState({
  onRetry
}: {
  onRetry?: () => void
}) {
  return (
    <EmptyState
      icon="alert"
      title="Sin conexión"
      description="No se puede conectar al servidor. Verifica tu conexión a internet e intenta de nuevo."
      action={onRetry ? {
        label: 'Reintentar',
        onClick: onRetry
      } : undefined}
    />
  )
}

export function ErrorState({
  message = 'Ha ocurrido un error inesperado',
  onRetry
}: {
  message?: string
  onRetry?: () => void
}) {
  return (
    <EmptyState
      icon="alert"
      title="Error"
      description={message}
      action={onRetry ? {
        label: 'Reintentar',
        onClick: onRetry
      } : undefined}
    />
  )
}