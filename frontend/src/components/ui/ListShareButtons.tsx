import { Send, Facebook, Linkedin, Twitter, Instagram, Share2 } from 'lucide-react'
import { News } from '../../services/api'

interface ListShareButtonsProps {
  listName: string
  news: News[]
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

// Helper to get API base URL
function getApiBase() {
  return import.meta.env.VITE_API_URL || '/api'
}

// Compile all news into a single shareable text
function compileListContent(listName: string, news: News[]): string {
  const parts: string[] = []
  
  parts.push(`📋 ${listName}`)
  parts.push(`=${'='.repeat(listName.length)}`)
  parts.push('')
  
  news.forEach((n, index) => {
    const title = n.title_translated || n.titulo
    const summary = n.summary_translated || n.resumen
    
    parts.push(`📰 ${index + 1}. ${title}`)
    parts.push(`   ${n.fuente_nombre || 'Unknown'} • ${n.fecha ? new Date(n.fecha).toLocaleDateString('es-ES') : ''}`)
    parts.push('')
    parts.push(`   ${summary}`)
    parts.push('')
    parts.push(`   🔗 ${n.url}`)
    parts.push('')
    parts.push('─'.repeat(50))
    parts.push('')
  })
  
  parts.push(`📊 Total: ${news.length} noticias`)
  parts.push('Generado por RSS2 News')
  
  return parts.join('\n')
}

// Share to Telegram - copies full content to clipboard then opens
async function shareListToTelegram(listName: string, news: News[]) {
  const content = compileListContent(listName, news)
  
  try {
    await navigator.clipboard.writeText(content)
    // Open Telegram
    window.open('https://t.me/share/url?url=https://t.me', '_blank', 'width=600,height=400,noopener,noreferrer')
    
    // Show feedback
    setTimeout(() => {
      alert('Contenido completo copiado al portapapeles.\n\nAbre Telegram y pega el contenido.')
    }, 500)
  } catch (err) {
    console.error('Failed to copy:', err)
    alert('No se pudo copiar al portapapeles')
  }
}

// Share to Facebook
function shareListToFacebook(listName: string, news: News[]) {
  // Facebook can't handle long text, so we share the first news as preview
  if (news.length > 0) {
    const firstNews = news[0]
    const base = getApiBase()
    const shareUrl = `${base}/share/${firstNews.id}`
    const facebookUrl = `https://www.facebook.com/sharer/sharer.php?u=${encodeURIComponent(shareUrl)}&quote=${encodeURIComponent(`📋 ${listName} (${news.length} noticias)`)}`
    window.open(facebookUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
  }
}

// Share to LinkedIn
function shareListToLinkedIn(listName: string, news: News[]) {
  if (news.length > 0) {
    const firstNews = news[0]
    const base = getApiBase()
    const shareUrl = `${base}/share/${firstNews.id}`
    const linkedInUrl = `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(shareUrl)}`
    window.open(linkedInUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
  }
}

// Share to X (Twitter)
function shareListToX(listName: string, news: News[]) {
  if (news.length > 0) {
    const firstNews = news[0]
    const base = getApiBase()
    const shareUrl = `${base}/share/${firstNews.id}`
    const text = `📋 ${listName} (${news.length} noticias)`
    const xUrl = `https://twitter.com/intent/tweet?text=${encodeURIComponent(text)}&url=${encodeURIComponent(shareUrl)}`
    window.open(xUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
  }
}

// Share to Instagram - copies full content to clipboard
async function shareListToInstagram(listName: string, news: News[]) {
  const content = compileListContent(listName, news)
  
  try {
    await navigator.clipboard.writeText(content)
    alert('Contenido copiado al portapapeles. Abre Instagram y pégalo.')
  } catch (err) {
    console.error('Failed to copy:', err)
    alert('No se pudo copiar al portapapeles')
  }
}

const sizeClasses = {
  sm: 'p-1.5',
  md: 'p-2',
  lg: 'p-2.5',
}

const iconSizes = {
  sm: 'h-3.5 w-3.5',
  md: 'h-4 w-4',
  lg: 'h-5 w-5',
}

export function ListShareButtons({ listName, news, size = 'md', className = '' }: ListShareButtonsProps) {
  if (news.length === 0) {
    return null
  }

  return (
    <div className={`flex items-center gap-1 ${className}`}>
      <span className="text-xs text-gray-400 mr-1">
        <Share2 className={iconSizes[size]} />
      </span>
      
      <button
        onClick={() => shareListToFacebook(listName, news)}
        className={`rounded-full text-gray-400 hover:text-[#1877f2] hover:bg-[#1877f2]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en Facebook"
      >
        <Facebook className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareListToLinkedIn(listName, news)}
        className={`rounded-full text-gray-400 hover:text-[#0a66c2] hover:bg-[#0a66c2]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en LinkedIn"
      >
        <Linkedin className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareListToX(listName, news)}
        className={`rounded-full text-gray-400 hover:text-black hover:bg-black/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en X (Twitter)"
      >
        <Twitter className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareListToInstagram(listName, news)}
        className={`rounded-full text-gray-400 hover:text-[#e4405f] hover:bg-[#e4405f]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en Instagram"
      >
        <Instagram className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareListToTelegram(listName, news)}
        className={`rounded-full text-gray-400 hover:text-[#0088cc] hover:bg-[#0088cc]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en Telegram (con contenido completo)"
      >
        <Send className={iconSizes[size]} />
      </button>
    </div>
  )
}
