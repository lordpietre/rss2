import React, { ReactNode } from 'react';
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

  return (
    <div className="relative group inline-block">
      <span className="cursor-help underline decoration-dotted decoration-gray-400 underline-offset-4 text-primary-700 font-medium">
        {children}
      </span>
      
      <div className="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-72 opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-300 ease-out pointer-events-none group-hover:pointer-events-auto origin-bottom transform scale-95 group-hover:scale-100">
        <div className="bg-white rounded-xl shadow-[0_10px_40px_-10px_rgba(0,0,0,0.15)] border border-gray-100 overflow-hidden text-left flex flex-col">
          {imagePath && (
            <div className="w-full h-44 bg-gray-100 overflow-hidden relative border-b border-gray-100">
              <img 
                src={imagePath} 
                alt={name} 
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
                rel="noreferrer" 
                className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-primary-600 hover:text-primary-800 transition-colors"
                onClick={(e) => e.stopPropagation()}
              >
                Ver en Wikipedia <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </div>
        </div>
        {/* Triangle arrow */}
        <div className="absolute top-full left-1/2 -translate-x-1/2 -mt-[1px] border-8 border-transparent border-t-white z-10"></div>
        <div className="absolute top-full left-1/2 -translate-x-1/2 -mt-[0px] border-8 border-transparent border-t-gray-100 -z-10 blur-[1px]"></div>
      </div>
    </div>
  );
}
