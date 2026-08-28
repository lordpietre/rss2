import React, { ReactNode, useRef, useEffect } from 'react';
import { ExternalLink } from 'lucide-react';

interface WikiTooltipProps {
  children: ReactNode;
  summary?: string;
  imagePath?: string;
  wikiUrl?: string;
  name: string;
}

export function WikiTooltip({ children, summary, imagePath, wikiUrl, name }: WikiTooltipProps) {
  if (!summary && !imagePath) {
    return <span className="text-gray-900">{children}</span>;
  }

  const tooltipRef = useRef<HTMLDivElement>(null)

  // Close tooltip on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        // Force blur to close tooltip
        const activeElement = document.activeElement as HTMLElement
        activeElement?.blur()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [])

  return (
    <div className="relative group inline-block" role="tooltip">
      <span 
        className="cursor-help underline decoration-dotted decoration-gray-400 underline-offset-4 text-primary-700 font-medium focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 rounded"
        tabIndex={0}
        aria-describedby={`wiki-tooltip-${name.replace(/\s+/g, '-')}`}
      >
        {children}
      </span>
      
      <div 
        id={`wiki-tooltip-${name.replace(/\s+/g, '-')}`}
        ref={tooltipRef}
        className="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-72 opacity-0 invisible group-hover:opacity-100 group-hover:visible group-focus-within:opacity-100 group-focus-within:visible transition-all duration-300 ease-out pointer-events-none group-hover:pointer-events-auto group-focus-within:pointer-events-auto origin-bottom transform scale-95 group-hover:scale-100 group-focus-within:scale-100"
        role="tooltip"
      >
        <div className="bg-white rounded-xl shadow-[0_10px_40px_-10px_rgba(0,0,0,0.15)] border border-gray-100 overflow-hidden text-left flex flex-col">
          {imagePath && (
            <div className="w-full h-44 bg-gray-100 overflow-hidden relative border-b border-gray-100">
              <img 
                src={imagePath} 
                alt={`${name} - Wikipedia image`} 
                className="w-full h-full object-cover"
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = 'none';
                }}
              />
            </div>
          )}
          <div className="p-4 w-full">
            <h4 className="font-bold text-gray-900 text-base mb-1">{name}</h4>
            {summary && (
              <p className="text-sm text-gray-600 line-clamp-4 leading-relaxed">
                {summary}
              </p>
            )}
            {wikiUrl && (
              <a 
                href={wikiUrl} 
                target="_blank" 
                rel="noopener noreferrer" 
                className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-primary-600 hover:text-primary-800 transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 rounded"
                onClick={(e) => e.stopPropagation()}
              >
                Ver en Wikipedia <ExternalLink className="h-3 w-3" aria-hidden="true" />
              </a>
            )}
          </div>
        </div>
        {/* Triangle arrow */}
        <div className="absolute top-full left-1/2 -translate-x-1/2 -mt-[1px] border-8 border-transparent border-t-white z-10" aria-hidden="true"></div>
        <div className="absolute top-full left-1/2 -translate-x-1/2 -mt-[0px] border-8 border-transparent border-t-gray-100 -z-10 blur-[1px]" aria-hidden="true"></div>
      </div>
    </div>
  );
}
