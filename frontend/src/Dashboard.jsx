import { useState, useEffect } from 'react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from 'recharts'
import { BarChart3, Loader, AlertCircle, ChevronDown, ChevronRight } from 'lucide-react'

const TIME_RANGES = [
  { label: '1H',  value: '1h'  },
  { label: '3H',  value: '3h'  },
  { label: '12H', value: '12h' },
  { label: '1D',  value: '1d'  },
  { label: '3D',  value: '3d'  },
  { label: '1W',  value: '1w'  },
  { label: '1M',  value: '1m'  },
]

// Format x-axis timestamps based on selected time range
function formatTimestamp(isoStr, range) {
  const d = new Date(isoStr)
  const pad = (n) => String(n).padStart(2, '0')
  if (range === '1h' || range === '3h') {
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  if (range === '12h' || range === '1d' || range === '3d') {
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

// Smart value formatter for tooltip and y-axis labels
function formatValue(value, field) {
  if (value === null || value === undefined) return '—'
  const f = field?.toUpperCase()
  if (f === 'PRICE' || f === 'TVL' || f === 'LIQUIDITY') {
    if (value >= 1e9)  return `$${(value / 1e9).toFixed(2)}B`
    if (value >= 1e6)  return `$${(value / 1e6).toFixed(2)}M`
    if (value >= 1e3)  return `$${(value / 1e3).toFixed(2)}K`
    return `$${value.toFixed(4)}`
  }
  if (f === 'APY' || f === 'UTILIZATION') {
    return `${value.toFixed(2)}%`
  }
  if (f === 'MIDPOINT' || f === 'BUY' || f === 'SELL') {
    return `${(value * 100).toFixed(2)}%`
  }
  return value.toFixed(4)
}

// Type badge colors
const TYPE_COLORS = {
  token:   { bg: 'bg-blue-500/20',    text: 'text-blue-400',   label: 'Token'  },
  defi:    { bg: 'bg-emerald-500/20', text: 'text-emerald-400', label: 'DeFi'  },
  predict: { bg: 'bg-violet-500/20',  text: 'text-violet-400',  label: 'Predict' },
}

// Chart line colors per field type
function getLineColor(field) {
  const f = field?.toUpperCase()
  if (f === 'PRICE')       return '#60a5fa'
  if (f === 'TVL')         return '#34d399'
  if (f === 'APY')         return '#fbbf24'
  if (f === 'UTILIZATION') return '#f87171'
  if (f === 'LIQUIDITY')   return '#a78bfa'
  if (f === 'MIDPOINT')    return '#60a5fa'
  if (f === 'BUY')         return '#34d399'
  if (f === 'SELL')        return '#f87171'
  return '#60a5fa'
}

function MetricCard({ metric, range }) {
  const [data, setData]       = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError]     = useState(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    const params = new URLSearchParams({
      type:       metric.type,
      identifier: metric.identifier,
      field:      metric.field,
      range,
    })

    fetch(`/api/metrics/history?${params}`)
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then(json => {
        if (cancelled) return
        setData((json.data || []).map(p => ({
          raw:   p.value,
          time:  formatTimestamp(p.recorded_at, range),
          ts:    p.recorded_at,
        })))
      })
      .catch(e => { if (!cancelled) setError(String(e)) })
      .finally(() => { if (!cancelled) setLoading(false) })

    return () => { cancelled = true }
  }, [metric.type, metric.identifier, metric.field, range])

  const latest     = data.length > 0 ? data[data.length - 1].raw : null
  const typeStyle  = TYPE_COLORS[metric.type] || TYPE_COLORS.token
  const lineColor  = getLineColor(metric.field)

  // Reduce x-axis tick density for large datasets
  const tickInterval = data.length > 60 ? Math.floor(data.length / 8) : 'preserveStartEnd'

  return (
    <div className="bg-dark-surface border border-dark-border rounded-xl p-5 flex flex-col gap-3 hover:border-[#3a3a5e] transition-colors">
      {/* Card header */}
      <div className="flex justify-between items-start gap-2">
        <div className="flex-1 min-w-0">
          <div className="text-dark-text font-semibold text-sm leading-tight truncate" title={metric.label}>
            {metric.label}
          </div>
          <div className="flex items-center gap-2 mt-1.5">
            <span className={`text-xs px-1.5 py-0.5 rounded font-medium ${typeStyle.bg} ${typeStyle.text}`}>
              {typeStyle.label}
            </span>
            <span className="text-dark-text-secondary text-xs">{metric.field}</span>
          </div>
        </div>
        {latest !== null && (
          <div className="text-right shrink-0">
            <div className="text-dark-text font-mono text-sm">{formatValue(latest, metric.field)}</div>
            <div className="text-dark-text-muted text-xs">latest</div>
          </div>
        )}
      </div>

      {/* Chart body */}
      {loading && (
        <div className="flex items-center justify-center h-[130px] text-dark-text-muted">
          <Loader className="w-4 h-4 animate-spin-slow" />
        </div>
      )}

      {error && (
        <div className="flex items-center justify-center h-[130px] text-red-400 text-xs gap-1.5">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span className="truncate">{error}</span>
        </div>
      )}

      {!loading && !error && data.length === 0 && (
        <div className="flex items-center justify-center h-[130px] text-dark-text-secondary text-xs">
          No data for this period
        </div>
      )}

      {!loading && !error && data.length > 0 && (
        <ResponsiveContainer width="100%" height={130}>
          <LineChart data={data} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#2a2a3e" vertical={false} />
            <XAxis
              dataKey="time"
              tick={{ fill: '#6b6b7e', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              interval={tickInterval}
            />
            <YAxis
              tick={{ fill: '#6b6b7e', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              tickFormatter={(v) => formatValue(v, metric.field)}
              width={62}
            />
            <Tooltip
              contentStyle={{
                background: '#141414',
                border: '1px solid #2a2a3e',
                borderRadius: 8,
                fontSize: 12,
              }}
              labelStyle={{ color: '#8b8b9e', marginBottom: 4 }}
              itemStyle={{ color: lineColor }}
              formatter={(v) => [formatValue(v, metric.field), metric.field]}
            />
            <Line
              type="monotone"
              dataKey="raw"
              stroke={lineColor}
              dot={false}
              strokeWidth={2}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}

function CollapsibleSection({ title, children, defaultOpen = true }) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <section className="mb-8">
      <button
        onClick={() => setOpen(o => !o)}
        className="flex items-center gap-2 mb-4 text-dark-text-muted text-xs font-semibold uppercase tracking-widest hover:text-dark-text transition-colors w-full text-left"
      >
        {open
          ? <ChevronDown className="w-3.5 h-3.5 shrink-0" />
          : <ChevronRight className="w-3.5 h-3.5 shrink-0" />}
        {title}
      </button>
      {open && children}
    </section>
  )
}

export default function Dashboard() {
  const [metrics, setMetrics] = useState([])
  const [range, setRange]     = useState('1d')
  const [loading, setLoading] = useState(true)
  const [error, setError]     = useState(null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    fetch('/api/metrics')
      .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
      .then(data => setMetrics(Array.isArray(data) ? data : []))
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false))
  }, [])

  const tokenMetrics   = metrics.filter(m => m.type === 'token')
  const defiMetrics    = metrics.filter(m => m.type === 'defi')
  const predictMetrics = metrics.filter(m => m.type === 'predict')

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      {/* Toolbar */}
      <div className="px-8 py-3 bg-dark-surface border-b border-dark-border flex items-center justify-between gap-4 flex-wrap shrink-0">
        <div className="flex items-center gap-2 text-dark-text-muted text-sm">
          <BarChart3 className="w-4 h-4" />
          <span>
            {loading ? 'Loading…' : `${metrics.length} metric${metrics.length !== 1 ? 's' : ''}`}
          </span>
        </div>
        <div className="flex gap-1">
          {TIME_RANGES.map(({ label, value }) => (
            <button
              key={value}
              onClick={() => setRange(value)}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                range === value
                  ? 'bg-blue-500 text-white'
                  : 'bg-dark-surface-hover text-dark-text-muted hover:text-dark-text'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-8 bg-dark-bg scrollbar-thin">
        {loading && (
          <div className="flex items-center justify-center gap-3 py-16 text-dark-text-muted text-base">
            <Loader className="w-5 h-5 animate-spin-slow" />
            Loading metrics…
          </div>
        )}

        {error && (
          <div className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500 rounded-lg text-red-400 mb-6">
            <AlertCircle className="w-5 h-5 shrink-0" />
            {error}
          </div>
        )}

        {!loading && !error && metrics.length === 0 && (
          <div className="text-center py-16 text-dark-text-secondary">
            No metrics yet — data appears once the monitoring service starts collecting.
          </div>
        )}

        {tokenMetrics.length > 0 && (
          <CollapsibleSection title="Token Prices">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {tokenMetrics.map(m => (
                <MetricCard
                  key={`${m.type}-${m.identifier}-${m.field}`}
                  metric={m}
                  range={range}
                />
              ))}
            </div>
          </CollapsibleSection>
        )}

        {defiMetrics.length > 0 && (
          <CollapsibleSection title="DeFi Protocols">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {defiMetrics.map(m => (
                <MetricCard
                  key={`${m.type}-${m.identifier}-${m.field}`}
                  metric={m}
                  range={range}
                />
              ))}
            </div>
          </CollapsibleSection>
        )}

        {predictMetrics.length > 0 && (
          <CollapsibleSection title="Prediction Markets">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {predictMetrics.map(m => (
                <MetricCard
                  key={`${m.type}-${m.identifier}-${m.field}`}
                  metric={m}
                  range={range}
                />
              ))}
            </div>
          </CollapsibleSection>
        )}
      </div>
    </div>
  )
}
