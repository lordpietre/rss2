import { useState, useEffect, useRef } from 'react'
import { Facebook, Linkedin, Twitter, Instagram, Share2, Send } from 'lucide-react'
import { News } from '../../services/api'

interface ShareButtonsProps {
  news: News
  size?: 'sm' | 'md' | 'lg'
  showLabel?: boolean
  className?: string
}

// Helper to get API base URL
function getApiBase() {
  return import.meta.env.VITE_API_URL || '/api'
}

// Helper to get the share preview URL for a news item
function getShareUrl(news: News): string {
  const base = getApiBase()
  return `${base}/share/${news.id}`
}

// Helper to get the best title and text for sharing
function getShareContent(news: News) {
  const title = news.title_translated || news.titulo
  const text = news.summary_translated || news.resumen
  const shareUrl = getShareUrl(news)
  
  return { title, text, shareUrl }
}

// Share to Facebook
function shareToFacebook(news: News) {
  const { shareUrl, title, text } = getShareContent(news)
  const facebookUrl = `https://www.facebook.com/sharer/sharer.php?u=${encodeURIComponent(shareUrl)}&quote=${encodeURIComponent(title + '\n\n' + text)}`
  window.open(facebookUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
}

// Share to LinkedIn
function shareToLinkedIn(news: News) {
  const { shareUrl } = getShareContent(news)
  const linkedInUrl = `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(shareUrl)}`
  window.open(linkedInUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
}

// Share to X (Twitter)
function shareToX(news: News) {
  const { shareUrl, title } = getShareContent(news)
  const xUrl = `https://twitter.com/intent/tweet?text=${encodeURIComponent(title)}&url=${encodeURIComponent(shareUrl)}`
  window.open(xUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
}

// Share to Instagram - Opens the URL (Instagram doesn't have a web share API)
function shareToInstagram(news: News) {
  const { shareUrl, title, text } = getShareContent(news)
  // Instagram doesn't support web sharing, so we copy to clipboard and notify
  const content = `${title}\n\n${text}\n\nLeer más: ${shareUrl}`
  
  if (navigator.clipboard) {
    navigator.clipboard.writeText(content).then(() => {
      alert('Contenido copiado al portapapeles. Puedes pegarlo en Instagram.')
    }).catch(() => {
      window.open(shareUrl, '_blank', 'noopener,noreferrer')
    })
  } else {
    window.open(shareUrl, '_blank', 'noopener,noreferrer')
  }
}

// Share to Telegram - uses original URL so Telegram shows its preview
function shareToTelegram(news: News) {
  const { title, text } = getShareContent(news)
  // Telegram will show a preview card with the URL's OG tags
  const telegramUrl = `https://t.me/share/url?url=${encodeURIComponent(news.url)}&text=${encodeURIComponent(title + '\n\n' + text)}`
  window.open(telegramUrl, '_blank', 'width=600,height=400,noopener,noreferrer')
}

// Generic share using Web Share API (mobile)
async function nativeShare(news: News) {
  const { title, text, shareUrl } = getShareContent(news)
  
  if (navigator.share) {
    try {
      await navigator.share({
        title,
        text: text.substring(0, 200) + (text.length > 200 ? '...' : '') + '\n\n' + shareUrl,
        url: shareUrl,
      })
    } catch (err) {
      // User cancelled or error
    }
  } else {
    // Fallback: copy to clipboard
    const content = `${title}\n\n${text}\n\n${shareUrl}`
    if (navigator.clipboard) {
      navigator.clipboard.writeText(content)
      alert('Contenido copiado al portapapeles')
    }
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

export function ShareButtons({ news, size = 'md', showLabel = false, className = '' }: ShareButtonsProps) {
  return (
    <div className={`flex items-center gap-1 ${className}`}>
      <span className="text-xs text-gray-400 mr-1">
        <Share2 className={iconSizes[size]} />
      </span>
      
      <button
        onClick={() => shareToFacebook(news)}
        className={`rounded-full text-gray-400 hover:text-[#1877f2] hover:bg-[#1877f2]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en Facebook"
      >
        <Facebook className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareToLinkedIn(news)}
        className={`rounded-full text-gray-400 hover:text-[#0a66c2] hover:bg-[#0a66c2]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en LinkedIn"
      >
        <Linkedin className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareToX(news)}
        className={`rounded-full text-gray-400 hover:text-black hover:bg-black/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en X (Twitter)"
      >
        <Twitter className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareToInstagram(news)}
        className={`rounded-full text-gray-400 hover:text-[#e4405f] hover:bg-[#e4405f]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en Instagram"
      >
        <Instagram className={iconSizes[size]} />
      </button>
      
      <button
        onClick={() => shareToTelegram(news)}
        className={`rounded-full text-gray-400 hover:text-[#0088cc] hover:bg-[#0088cc]/10 transition-colors ${sizeClasses[size]}`}
        title="Compartir en Telegram"
      >
        <Send className={iconSizes[size]} />
      </button>
    </div>
  )
}

// Standalone share button that opens a dropdown
export function ShareButton({ news, size = 'md' }: { news: News; size?: 'sm' | 'md' | 'lg' }) {
  const [showMenu, setShowMenu] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])
  
  return (
    <div className="relative" ref={menuRef}>
      <button
        onClick={() => setShowMenu(!showMenu)}
        className={`rounded-full text-gray-400 hover:text-primary-600 hover:bg-primary-50 transition-colors ${sizeClasses[size]}`}
        title="Compartir"
      >
        <Share2 className={iconSizes[size]} />
      </button>
      
      {showMenu && (
        <div className="absolute right-0 mt-1 w-48 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 py-1 z-50">
          <button
            onClick={() => { shareToFacebook(news); setShowMenu(false) }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            <Facebook className="h-4 w-4 text-[#1877f2]" />
            Facebook
          </button>
          <button
            onClick={() => { shareToLinkedIn(news); setShowMenu(false) }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            <Linkedin className="h-4 w-4 text-[#0a66c2]" />
            LinkedIn
          </button>
          <button
            onClick={() => { shareToX(news); setShowMenu(false) }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            <Twitter className="h-4 w-4" />
            X (Twitter)
          </button>
          <button
            onClick={() => { shareToInstagram(news); setShowMenu(false) }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            <Instagram className="h-4 w-4 text-[#e4405f]" />
            Instagram
          </button>
          <button
            onClick={() => { shareToTelegram(news); setShowMenu(false) }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            <Send className="h-4 w-4 text-[#0088cc]" />
            Telegram
          </button>
          <hr className="my-1 border-gray-200 dark:border-gray-700" />
          <button
            onClick={() => { nativeShare(news); setShowMenu(false) }}
            className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            <Share2 className="h-4 w-4" />
            Compartir...
          </button>
        </div>
      )}
    </div>
  )
}
