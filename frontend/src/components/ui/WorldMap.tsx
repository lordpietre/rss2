import { useMemo } from 'react'

interface CountryData {
  pais_id: number
  pais_nombre: string
  flag_emoji: string
  count: number
}

interface WorldMapProps {
  countries: CountryData[]
  selectedCountry?: number
  onCountryClick?: (countryId: number) => void
  maxCount?: number
}

// Simplified world map using SVG paths
// Country codes match ISO 3166-1 numeric codes used in the database
const COUNTRY_PATHS: Record<string, { name: string; path: string }> = {
  'ES': { name: 'Spain', path: 'M337.5,158.5 L342.5,152.5 L347.5,150.5 L347.5,160.5 L342.5,165.5 L337.5,163.5 Z' },
  'FR': { name: 'France', path: 'M322.5,145.5 L332.5,145.5 L337.5,152.5 L337.5,158.5 L327.5,158.5 L322.5,153.5 Z' },
  'DE': { name: 'Germany', path: 'M347.5,125.5 L357.5,125.5 L360.5,140.5 L352.5,145.5 L345.5,140.5 Z' },
  'IT': { name: 'Italy', path: 'M347.5,152.5 L352.5,155.5 L355.5,175.5 L350.5,180.5 L345.5,170.5 L347.5,158.5 Z' },
  'GB': { name: 'United Kingdom', path: 'M312.5,118.5 L320.5,115.5 L322.5,125.5 L318.5,132.5 L312.5,130.5 Z' },
  'PT': { name: 'Portugal', path: 'M322.5,158.5 L330.5,158.5 L332.5,165.5 L327.5,168.5 L322.5,165.5 Z' },
  'BR': { name: 'Brazil', path: 'M205.5,230.5 L225.5,225.5 L235.5,250.5 L225.5,275.5 L210.5,280.5 L200.5,260.5 Z' },
  'MX': { name: 'Mexico', path: 'M115.5,175.5 L135.5,165.5 L145.5,180.5 L140.5,200.5 L125.5,205.5 L115.5,190.5 Z' },
  'US': { name: 'United States', path: 'M100.5,130.5 L150.5,125.5 L160.5,150.5 L155.5,175.5 L130.5,180.5 L110.5,165.5 Z' },
  'CA': { name: 'Canada', path: 'M100.5,90.5 L165.5,85.5 L170.5,115.5 L160.5,130.5 L115.5,130.5 L100.5,115.5 Z' },
  'RU': { name: 'Russia', path: 'M380.5,80.5 L550.5,75.5 L560.5,120.5 L500.5,130.5 L420.5,130.5 L380.5,120.5 Z' },
  'CN': { name: 'China', path: 'M480.5,145.5 L540.5,140.5 L550.5,175.5 L520.5,190.5 L480.5,180.5 L470.5,160.5 Z' },
  'IN': { name: 'India', path: 'M440.5,165.5 L475.5,160.5 L480.5,195.5 L460.5,210.5 L435.5,200.5 L435.5,175.5 Z' },
  'JP': { name: 'Japan', path: 'M555.5,150.5 L565.5,145.5 L570.5,160.5 L565.5,175.5 L555.5,170.5 Z' },
  'AU': { name: 'Australia', path: 'M520.5,250.5 L570.5,245.5 L580.5,275.5 L560.5,290.5 L525.5,285.5 Z' },
  'AR': { name: 'Argentina', path: 'M200.5,275.5 L215.5,270.5 L220.5,310.5 L210.5,330.5 L195.5,315.5 Z' },
  'CO': { name: 'Colombia', path: 'M165.5,210.5 L185.5,205.5 L190.5,225.5 L180.5,235.5 L165.5,230.5 Z' },
  'PE': { name: 'Peru', path: 'M165.5,230.5 L180.5,225.5 L185.5,260.5 L175.5,275.5 L160.5,265.5 Z' },
  'VE': { name: 'Venezuela', path: 'M175.5,200.5 L195.5,195.5 L200.5,210.5 L190.5,220.5 L175.5,215.5 Z' },
  'CL': { name: 'Chile', path: 'M185.5,275.5 L195.5,270.5 L198.5,340.5 L190.5,345.5 L185.5,310.5 Z' },
  'ZA': { name: 'South Africa', path: 'M365.5,285.5 L385.5,280.5 L390.5,305.5 L375.5,315.5 L360.5,305.5 Z' },
  'EG': { name: 'Egypt', path: 'M375.5,170.5 L395.5,165.5 L400.5,185.5 L390.5,195.5 L375.5,190.5 Z' },
  'NG': { name: 'Nigeria', path: 'M340.5,205.5 L355.5,200.5 L360.5,220.5 L350.5,230.5 L340.5,225.5 Z' },
  'KE': { name: 'Kenya', path: 'M395.5,220.5 L410.5,215.5 L415.5,235.5 L405.5,245.5 L395.5,240.5 Z' },
  'SA': { name: 'Saudi Arabia', path: 'M405.5,175.5 L440.5,170.5 L445.5,195.5 L420.5,205.5 L400.5,195.5 Z' },
  'TR': { name: 'Turkey', path: 'M375.5,150.5 L405.5,145.5 L410.5,160.5 L385.5,165.5 L375.5,160.5 Z' },
  'UA': { name: 'Ukraine', path: 'M385.5,130.5 L410.5,125.5 L415.5,145.5 L395.5,150.5 L385.5,145.5 Z' },
  'PL': { name: 'Poland', path: 'M360.5,125.5 L375.5,120.5 L380.5,135.5 L370.5,142.5 L360.5,138.5 Z' },
  'NL': { name: 'Netherlands', path: 'M335.5,118.5 L345.5,115.5 L347.5,125.5 L340.5,130.5 L335.5,127.5 Z' },
  'BE': { name: 'Belgium', path: 'M330.5,127.5 L338.5,125.5 L340.5,133.5 L335.5,137.5 L330.5,134.5 Z' },
  'SE': { name: 'Sweden', path: 'M352.5,95.5 L360.5,90.5 L365.5,120.5 L358.5,128.5 L352.5,122.5 Z' },
  'NO': { name: 'Norway', path: 'M345.5,90.5 L355.5,85.5 L360.5,115.5 L352.5,122.5 L345.5,115.5 Z' },
  'FI': { name: 'Finland', path: 'M365.5,88.5 L375.5,85.5 L380.5,110.5 L372.5,118.5 L365.5,112.5 Z' },
  'DK': { name: 'Denmark', path: 'M345.5,112.5 L352.5,108.5 L355.5,118.5 L350.5,123.5 L345.5,120.5 Z' },
  'CH': { name: 'Switzerland', path: 'M337.5,138.5 L345.5,136.5 L347.5,145.5 L342.5,148.5 L337.5,145.5 Z' },
  'AT': { name: 'Austria', path: 'M350.5,135.5 L362.5,132.5 L365.5,145.5 L357.5,150.5 L350.5,146.5 Z' },
  'GR': { name: 'Greece', path: 'M370.5,155.5 L382.5,152.5 L385.5,165.5 L377.5,170.5 L370.5,165.5 Z' },
  'RO': { name: 'Romania', path: 'M375.5,140.5 L392.5,137.5 L395.5,152.5 L385.5,157.5 L375.5,153.5 Z' },
  'HU': { name: 'Hungary', path: 'M365.5,138.5 L378.5,135.5 L382.5,147.5 L372.5,152.5 L365.5,148.5 Z' },
  'CZ': { name: 'Czech Republic', path: 'M357.5,130.5 L368.5,127.5 L372.5,138.5 L363.5,143.5 L357.5,139.5 Z' },
  'IE': { name: 'Ireland', path: 'M315.5,122.5 L325.5,118.5 L328.5,132.5 L320.5,138.5 L315.5,134.5 Z' },
}

// Map country names to ISO codes for matching
const COUNTRY_NAME_TO_CODE: Record<string, string> = {
  'españa': 'ES', 'spain': 'ES',
  'francia': 'FR', 'france': 'FR',
  'alemania': 'DE', 'germany': 'DE',
  'italia': 'IT', 'italy': 'IT',
  'reino unido': 'GB', 'united kingdom': 'GB', 'uk': 'GB',
  'portugal': 'PT',
  'brasil': 'BR', 'brazil': 'BR',
  'méxico': 'MX', 'mexico': 'MX',
  'estados unidos': 'US', 'united states': 'US', 'usa': 'US',
  'canadá': 'CA', 'canada': 'CA',
  'rusia': 'RU', 'russia': 'RU',
  'china': 'CN',
  'india': 'IN',
  'japón': 'JP', 'japan': 'JP',
  'australia': 'AU',
  'argentina': 'AR',
  'colombia': 'CO',
  'perú': 'PE', 'peru': 'PE',
  'venezuela': 'VE',
  'chile': 'CL',
  'sudáfrica': 'ZA', 'south africa': 'ZA',
  'egipto': 'EG', 'egypt': 'EG',
  'nigeria': 'NG',
  'kenia': 'KE', 'kenya': 'KE',
  'arabia saudita': 'SA', 'saudi arabia': 'SA',
  'turquía': 'TR', 'turkey': 'TR',
  'ucrania': 'UA', 'ukraine': 'UA',
  'polonia': 'PL', 'poland': 'PL',
  'países bajos': 'NL', 'netherlands': 'NL', 'holanda': 'NL',
  'bélgica': 'BE', 'belgium': 'BE',
  'suecia': 'SE', 'sweden': 'SE',
  'noruega': 'NO', 'norway': 'NO',
  'finlandia': 'FI', 'finland': 'FI',
  'dinamarca': 'DK', 'denmark': 'DK',
  'suiza': 'CH', 'switzerland': 'CH',
  'austria': 'AT',
  'grecia': 'GR', 'greece': 'GR',
  'rumania': 'RO', 'romania': 'RO',
  'hungary': 'HU', 'hungría': 'HU', 'hungria': 'HU',
  'czech': 'CZ', 'checa': 'CZ',
  'irlanda': 'IE', 'ireland': 'IE',
}

export function WorldMap({ countries, selectedCountry, onCountryClick, maxCount }: WorldMapProps) {
  // Create a lookup by country name
  const countryByCode = useMemo(() => {
    const map = new Map<string, CountryData>()
    if (!countries || !Array.isArray(countries)) return map
    countries.forEach(c => {
      if (!c?.pais_nombre) return
      const code = COUNTRY_NAME_TO_CODE[c.pais_nombre.toLowerCase()]
      if (code) {
        map.set(code, c)
      }
    })
    return map
  }, [countries])

  const effectiveMax = useMemo(() => {
    if (!countries || !Array.isArray(countries) || countries.length === 0) return 1
    return maxCount || Math.max(...countries.map(c => c?.count || 0), 1)
  }, [countries, maxCount])

  const getIntensity = (count: number): string => {
    if (!count || !effectiveMax) return '#1a2e1f'
    const ratio = count / effectiveMax
    if (ratio > 0.75) return '#72FF8A'
    if (ratio > 0.5) return '#36C96B'
    if (ratio > 0.25) return '#244C32'
    return '#1a2e1f'
  }

  return (
    <div className="relative w-full h-full bg-gray-900/50 rounded-lg overflow-hidden">
      <svg viewBox="0 0 600 400" className="w-full h-full">
        {/* Background */}
        <rect x="0" y="0" width="600" height="400" fill="#080B0A" />
        
        {/* Grid lines for aesthetic */}
        {[0, 100, 200, 300, 400, 500, 600].map(x => (
          <line key={`v${x}`} x1={x} y1={0} x2={x} y2={400} stroke="#151C17" strokeWidth="0.5" />
        ))}
        {[0, 100, 200, 300, 400].map(y => (
          <line key={`h${y}`} x1={0} y1={y} x2={600} y2={y} stroke="#151C17" strokeWidth="0.5" />
        ))}

        {/* Countries */}
        {Object.entries(COUNTRY_PATHS).map(([code, { name, path }]) => {
          const countryData = countryByCode.get(code)
          const hasData = !!countryData
          const count = countryData?.count || 0
          const isSelected = selectedCountry === countryData?.pais_id

          return (
            <g key={code}>
              <path
                d={path}
                fill={hasData ? getIntensity(count) : '#151C17'}
                stroke={isSelected ? '#72FF8A' : '#26362B'}
                strokeWidth={isSelected ? 2 : 0.5}
                className="cursor-pointer transition-all duration-200"
                onClick={() => hasData && onCountryClick?.(countryData.pais_id)}
                style={{
                  opacity: hasData ? 1 : 0.3,
                }}
                onMouseEnter={(e) => {
                  if (hasData) {
                    e.currentTarget.style.stroke = '#72FF8A'
                    e.currentTarget.style.strokeWidth = '1.5'
                  }
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.stroke = isSelected ? '#72FF8A' : '#26362B'
                  e.currentTarget.style.strokeWidth = isSelected ? '2' : '0.5'
                }}
              />
            </g>
          )
        })}
      </svg>

      {/* Legend */}
      <div className="absolute bottom-3 left-3 bg-rss-bg-panel/90 backdrop-blur-sm rounded-lg p-3 border border-rss-border">
        <div className="text-xs font-mono text-gray-400 mb-2">Activity Level</div>
        <div className="flex items-center gap-2">
          <div className="w-4 h-3 rounded" style={{ backgroundColor: '#1a2e1f' }} />
          <span className="text-xs text-gray-500">None</span>
        </div>
        <div className="flex items-center gap-2 mt-1">
          <div className="w-4 h-3 rounded" style={{ backgroundColor: '#244C32' }} />
          <span className="text-xs text-gray-500">Low</span>
        </div>
        <div className="flex items-center gap-2 mt-1">
          <div className="w-4 h-3 rounded" style={{ backgroundColor: '#36C96B' }} />
          <span className="text-xs text-gray-500">Medium</span>
        </div>
        <div className="flex items-center gap-2 mt-1">
          <div className="w-4 h-3 rounded" style={{ backgroundColor: '#72FF8A' }} />
          <span className="text-xs text-gray-500">High</span>
        </div>
      </div>

      {/* Country count tooltip area */}
      <div className="absolute top-3 right-3 bg-rss-bg-panel/90 backdrop-blur-sm rounded-lg p-2 border border-rss-border">
        <div className="text-xs font-mono text-gray-400">
          {countries.length} countries
        </div>
      </div>
    </div>
  )
}

export { COUNTRY_NAME_TO_CODE }
