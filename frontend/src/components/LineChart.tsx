import { useMemo, useState } from 'react'

const COLORS = ['#2563eb', '#dc2626', '#16a34a', '#d97706', '#7c3aed', '#0891b2', '#db2777', '#65a30d']

export interface ChartSeries {
  valor: string
  tipo?: string
  count?: number
  data: { fecha: string; count: number }[]
}

interface Props {
  series: ChartSeries[]
  height?: number
}

export function LineChart({ series, height = 320 }: Props) {
  const [hover, setHover] = useState<number | null>(null)
  const width = 1000
  const padL = 46
  const padR = 14
  const padT = 16
  const padB = 34
  const plotW = width - padL - padR
  const plotH = height - padT - padB

  const days = series[0]?.data?.length ?? 0

  const max = useMemo(() => {
    let m = 0
    for (const s of series) for (const p of s.data) if (p.count > m) m = p.count
    return Math.max(1, m)
  }, [series])

  const xAt = (i: number) => padL + (days <= 1 ? 0 : (i / (days - 1)) * plotW)
  const yAt = (v: number) => padT + plotH - (v / max) * plotH

  const paths = series.map((s, si) => {
    const d = s.data
      .map((p, i) => `${i === 0 ? 'M' : 'L'}${xAt(i).toFixed(1)},${yAt(p.count).toFixed(1)}`)
      .join(' ')
    return { d, color: COLORS[si % COLORS.length], s }
  })

  const yTicks = useMemo(() => {
    const ticks = max <= 4 ? [0, max] : []
    if (max <= 4) return ticks
    const step = Math.pow(10, Math.floor(Math.log10(max / 4)))
    const norm = Math.ceil(max / step / 4)
    const nice = step * norm * 4
    const out: number[] = []
    for (let v = 0; v <= nice; v += step * norm) out.push(v)
    return out
  }, [max])

  const xLabelStep = days > 12 ? Math.ceil(days / 8) : 1
  const xLabels: { i: number; label: string }[] = []
  for (let i = 0; i < days; i += xLabelStep) {
    if (days > 0 && i % xLabelStep === 0) {
      xLabels.push({ i, label: series[0]?.data[i]?.fecha?.slice(5) || '' })
    }
  }
  if (days > 0 && xLabels[xLabels.length - 1]?.i !== days - 1) {
    xLabels.push({ i: days - 1, label: series[0]?.data[days - 1]?.fecha?.slice(5) || '' })
  }

  const handleMove = (e: React.MouseEvent<SVGSVGElement>) => {
    if (days === 0) return
    const rect = e.currentTarget.getBoundingClientRect()
    const x = ((e.clientX - rect.left) / rect.width) * width
    const idx = Math.max(0, Math.min(days - 1, Math.round(((x - padL) / plotW) * (days - 1))))
    setHover(idx)
  }

  const hoverX = hover !== null ? xAt(hover) : 0

  return (
    <div className="w-full overflow-x-auto">
      <div className="flex flex-wrap gap-4 mb-3">
        {paths.map(({ color, s }) => (
          <span key={s.valor + color} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <span className="h-2.5 w-2.5 rounded-full inline-block" style={{ backgroundColor: color }} />
            <span className="font-medium">{s.valor}</span>
            <span className="text-gray-400">({s.count ?? 0})</span>
          </span>
        ))}
      </div>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full min-w-[560px]"
        onMouseMove={handleMove}
        onMouseLeave={() => setHover(null)}
      >
        {yTicks.map((t) => (
          <g key={t}>
            <line x1={padL} x2={width - padR} y1={yAt(t)} y2={yAt(t)} stroke="#e5e7eb" strokeWidth={1} />
            <text x={padL - 8} y={yAt(t) + 4} textAnchor="end" fontSize={11} fill="#9ca3af">
              {t}
            </text>
          </g>
        ))}
        {xLabels.map(({ i, label }) => (
          <text key={i} x={xAt(i)} y={height - 12} textAnchor="middle" fontSize={11} fill="#9ca3af">
            {label}
          </text>
        ))}
        {paths.map(({ d, color }) => (
          <path key={color} d={d} fill="none" stroke={color} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
        ))}
        {paths.map(({ color, s }, si) =>
          s.data.map((p, i) => (
            <circle
              key={`${si}-${i}`}
              cx={xAt(i)}
              cy={yAt(p.count)}
              r={hover === i ? 4.5 : 2.5}
              fill={color}
            />
          )),
        )}
        {hover !== null && (
          <g>
            <line x1={hoverX} x2={hoverX} y1={padT} y2={height - padB} stroke="#cbd5e1" strokeWidth={1} strokeDasharray="3 3" />
            {paths.map(({ color, s }) => (
              <circle key={color + hover} cx={hoverX} cy={yAt(s.data[hover]?.count ?? 0)} r={5} fill={color} stroke="#fff" strokeWidth={2} />
            ))}
            <g transform={`translate(${Math.min(hoverX + 12, width - 220)}, ${padT})`}>
              <rect width={208} height={20 + paths.length * 18} rx={6} fill="#111827" opacity={0.92} />
              <text x={10} y={16} fontSize={11} fill="#e5e7eb" fontWeight={600}>
                {series[0]?.data[hover]?.fecha || ''}
              </text>
              {paths.map(({ color, s }, si) => (
                <g key={color}>
                  <rect x={10} y={26 + si * 18} width={10} height={10} rx={2} fill={color} />
                  <text x={26} y={35 + si * 18} fontSize={11} fill="#d1d5db">
                    {s.valor}: {s.data[hover]?.count ?? 0}
                  </text>
                </g>
              ))}
            </g>
          </g>
        )}
      </svg>
    </div>
  )
}