import { useState, useEffect } from 'react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
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

function formatValue(value, field) {
  if (value === null || value === undefined) return '—'
  const f = field?.toUpperCase()
  if (f === 'PRICE' || f === 'TVL' || f === 'LIQUIDITY') {
    if (value >= 1e9)  return `$${(value / 1e9).toFixed(2)}B`
    if (value >= 1e6)  return `$${(value / 1e6).toFixed(2)}M`
    if (value >= 1e3)  return `$${(value / 1e3).toFixed(2)}K`
    return `$${value.toFixed(4)}`
  }
  if (f === 'APY' || f === 'UTILIZATION') return `${value.toFixed(2)}%`
  if (f === 'MIDPOINT' || f === 'BUY' || f === 'SELL') return `${(value * 100).toFixed(2)}%`
  return value.toFixed(4)
}

const TYPE_COLORS = {
  token:   { bg: 'bg-blue-500/15',    text: 'text-blue-400',    border: 'border-blue-500/20',    label: 'Token'   },
  defi:    { bg: 'bg-emerald-500/15', text: 'text-emerald-400', border: 'border-emerald-500/20', label: 'DeFi'    },
  predict: { bg: 'bg-violet-500/15',  text: 'text-violet-400',  border: 'border-violet-500/20',  label: 'Predict' },
}

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

function chartColors(theme) {
  return theme === 'light'
    ? { grid: '#e8e8f5', tick: '#8a8aae', tooltipBg: '#ffffff', tooltipBorder: '#e0e0f0', tooltipLabel: '#6a6a8e' }
    : { grid: '#1e1e36', tick: '#5a5a7e', tooltipBg: '#0e0e1e', tooltipBorder: '#1e1e3a', tooltipLabel: '#6b6b9e' }
}

function MetricCard({ metric, range, theme }) {
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
          raw:  p.value,
          time: formatTimestamp(p.recorded_at, range),
          ts:   p.recorded_at,
        })))
      })
      .catch(e => { if (!cancelled) setError(String(e)) })
      .finally(() => { if (!cancelled) setLoading(false) })

    return () => { cancelled = true }
  }, [metric.type, metric.identifier, metric.field, range])

  const latest    = data.length > 0 ? data[data.length - 1].raw : null
  const typeStyle = TYPE_COLORS[metric.type] || TYPE_COLORS.token
  const lineColor = getLineColor(metric.field)
  const cc        = chartColors(theme)
  const tickInterval = data.length > 60 ? Math.floor(data.length / 8) : 'preserveStartEnd'

  return (
    <div className="bg-theme-card border border-theme-border rounded-xl p-5 flex flex-col gap-3 hover:border-theme-border-focus hover:shadow-[0_0_20px_rgba(59,130,246,0.06)] transition-all duration-200 group">
      <div className="flex justify-between items-start gap-2">
        <div className="flex-1 min-w-0">
          <div className="text-theme-text font-semibold text-sm leading-tight truncate" title={metric.label}>
            {metric.label}
          </div>
          <div className="flex items-center gap-2 mt-1.5">
            <span className={`text-xs px-1.5 py-0.5 rounded border font-medium ${typeStyle.bg} ${typeStyle.text} ${typeStyle.border}`}>
              {typeStyle.label}
            </span>
            <span className="text-theme-text-secondary text-xs">{metric.field}</span>
          </div>
        </div>
        {latest !== null && (
          <div className="text-right shrink-0">
            <div className="text-theme-text font-mono text-sm font-semibold" style={{ color: lineColor }}>
              {formatValue(latest, metric.field)}
            </div>
            <div className="text-theme-text-secondary text-xs">latest</div>
          </div>
        )}
      </div>

      {loading && (
        <div className="flex items-center justify-center h-[130px] text-theme-text-muted">
          <Loader className="w-4 h-4 animate-spin-slow text-blue-400/50" />
        </div>
      )}

      {error && (
        <div className="flex items-center justify-center h-[130px] text-red-400 text-xs gap-1.5">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span className="truncate">{error}</span>
        </div>
      )}

      {!loading && !error && data.length === 0 && (
        <div className="flex items-center justify-center h-[130px] text-theme-text-secondary text-xs">
          No data for this period
        </div>
      )}

      {!loading && !error && data.length > 0 && (
        <ResponsiveContainer width="100%" height={130}>
          <LineChart data={data} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={cc.grid} vertical={false} />
            <XAxis
              dataKey="time"
              tick={{ fill: cc.tick, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              interval={tickInterval}
            />
            <YAxis
              tick={{ fill: cc.tick, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              tickFormatter={(v) => formatValue(v, metric.field)}
              width={62}
            />
            <Tooltip
              contentStyle={{
                background: cc.tooltipBg,
                border: `1px solid ${cc.tooltipBorder}`,
                borderRadius: 8,
                fontSize: 12,
              }}
              labelStyle={{ color: cc.tooltipLabel, marginBottom: 4 }}
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

function PredictMarketCard({ group, range, theme }) {
  const [data, setData]       = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError]     = useState(null)

  const fields = group.fields

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    const promises = fields.map(field => {
      const params = new URLSearchParams({
        type:       group.type,
        identifier: group.identifier,
        field,
        range,
      })
      return fetch(`/api/metrics/history?${params}`)
        .then(r => r.ok ? r.json() : Promise.reject(r.statusText))
        .then(json => ({ field, points: json.data || [] }))
    })

    Promise.all(promises)
      .then(results => {
        if (cancelled) return
        const byTime = {}
        results.forEach(({ field, points }) => {
          points.forEach(p => {
            const key = formatTimestamp(p.recorded_at, range)
            if (!byTime[key]) byTime[key] = { time: key, ts: p.recorded_at }
            byTime[key][field] = p.value
          })
        })
        const merged = Object.values(byTime).sort((a, b) => a.ts.localeCompare(b.ts))
        setData(merged)
      })
      .catch(e => { if (!cancelled) setError(String(e)) })
      .finally(() => { if (!cancelled) setLoading(false) })

    return () => { cancelled = true }
  }, [group.type, group.identifier, fields.join(','), range])

  const tickInterval = data.length > 60 ? Math.floor(data.length / 8) : 'preserveStartEnd'
  const latestRow    = data.length > 0 ? data[data.length - 1] : null
  const cc           = chartColors(theme)

  return (
    <div className="bg-theme-card border border-theme-border rounded-xl p-5 flex flex-col gap-3 hover:border-theme-border-focus hover:shadow-[0_0_20px_rgba(59,130,246,0.06)] transition-all duration-200">
      <div className="flex justify-between items-start gap-2">
        <div className="flex-1 min-w-0">
          <div className="text-theme-text font-semibold text-sm leading-tight truncate" title={group.label}>
            {group.label}
          </div>
          <div className="flex items-center gap-2 mt-1.5">
            <span className="text-xs px-1.5 py-0.5 rounded border font-medium bg-violet-500/15 text-violet-400 border-violet-500/20">
              Predict
            </span>
            <span className="text-theme-text-secondary text-xs">{fields.join(' · ')}</span>
          </div>
        </div>
        {latestRow && (
          <div className="text-right shrink-0 flex flex-col gap-0.5">
            {fields.map(field => latestRow[field] != null && (
              <div key={field} className="flex items-center gap-1.5 justify-end">
                <span className="font-mono text-xs font-semibold" style={{ color: getLineColor(field) }}>
                  {formatValue(latestRow[field], field)}
                </span>
                <span className="text-theme-text-muted text-xs">{field}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {loading && (
        <div className="flex items-center justify-center h-[130px] text-theme-text-muted">
          <Loader className="w-4 h-4 animate-spin-slow text-blue-400/50" />
        </div>
      )}

      {error && (
        <div className="flex items-center justify-center h-[130px] text-red-400 text-xs gap-1.5">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span className="truncate">{error}</span>
        </div>
      )}

      {!loading && !error && data.length === 0 && (
        <div className="flex items-center justify-center h-[130px] text-theme-text-secondary text-xs">
          No data for this period
        </div>
      )}

      {!loading && !error && data.length > 0 && (
        <ResponsiveContainer width="100%" height={130}>
          <LineChart data={data} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={cc.grid} vertical={false} />
            <XAxis
              dataKey="time"
              tick={{ fill: cc.tick, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              interval={tickInterval}
            />
            <YAxis
              tick={{ fill: cc.tick, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              tickFormatter={(v) => `${(v * 100).toFixed(0)}%`}
              width={42}
            />
            <Tooltip
              contentStyle={{
                background: cc.tooltipBg,
                border: `1px solid ${cc.tooltipBorder}`,
                borderRadius: 8,
                fontSize: 12,
              }}
              labelStyle={{ color: cc.tooltipLabel, marginBottom: 4 }}
              formatter={(v, name) => [formatValue(v, name), name]}
            />
            {fields.map(field => (
              <Line
                key={field}
                type="monotone"
                dataKey={field}
                stroke={getLineColor(field)}
                dot={false}
                strokeWidth={2}
                isAnimationActive={false}
              />
            ))}
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
        className="flex items-center gap-2 mb-4 text-theme-text-muted text-xs font-semibold uppercase tracking-widest hover:text-theme-text transition-colors w-full text-left"
      >
        {open
          ? <ChevronDown className="w-3.5 h-3.5 shrink-0" />
          : <ChevronRight className="w-3.5 h-3.5 shrink-0" />}
        {title}
        <span className="ml-1 text-theme-text-secondary normal-case tracking-normal font-normal">
          ({Array.isArray(children?.props?.children) ? children.props.children.length : ''})
        </span>
      </button>
      {open && children}
    </section>
  )
}

export default function Dashboard({ theme = 'dark' }) {
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

  const enabledMetrics = metrics.filter(m => m.enabled !== false)
  const tokenMetrics   = enabledMetrics.filter(m => m.type === 'token')
  const defiMetrics    = enabledMetrics.filter(m => m.type === 'defi')

  const predictGroups = Object.values(
    enabledMetrics
      .filter(m => m.type === 'predict')
      .reduce((acc, m) => {
        if (!acc[m.identifier]) {
          acc[m.identifier] = { type: m.type, identifier: m.identifier, label: m.label, fields: [] }
        }
        acc[m.identifier].fields.push(m.field)
        return acc
      }, {})
  )

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      {/* Toolbar */}
      <div className="px-8 py-3 bg-theme-surface border-b border-theme-border flex items-center justify-between gap-4 flex-wrap shrink-0">
        <div className="flex items-center gap-2 text-theme-text-muted text-sm">
          <BarChart3 className="w-4 h-4 text-blue-400/70" />
          <span>
            {loading ? 'Loading…' : `${enabledMetrics.length} metric${enabledMetrics.length !== 1 ? 's' : ''}`}
          </span>
        </div>
        <div className="flex gap-1 bg-theme-input rounded-lg p-1 border border-theme-border">
          {TIME_RANGES.map(({ label, value }) => (
            <button
              key={value}
              onClick={() => setRange(value)}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all duration-150 ${
                range === value
                  ? 'bg-blue-500 text-white shadow-[0_0_10px_rgba(59,130,246,0.35)]'
                  : 'text-theme-text-muted hover:text-theme-text hover:bg-theme-card'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-8 bg-theme-bg scrollbar-thin">
        {loading && (
          <div className="flex items-center justify-center gap-3 py-16 text-theme-text-muted text-sm">
            <Loader className="w-5 h-5 animate-spin-slow text-blue-400" />
            Loading metrics…
          </div>
        )}

        {error && (
          <div className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500/40 rounded-lg text-red-400 mb-6">
            <AlertCircle className="w-5 h-5 shrink-0" />
            {error}
          </div>
        )}

        {!loading && !error && enabledMetrics.length === 0 && (
          <div className="text-center py-16 text-theme-text-secondary text-sm">
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
                  theme={theme}
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
                  theme={theme}
                />
              ))}
            </div>
          </CollapsibleSection>
        )}

        {predictGroups.length > 0 && (
          <CollapsibleSection title="Prediction Markets">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
              {predictGroups.map(group => (
                <PredictMarketCard
                  key={`predict-${group.identifier}`}
                  group={group}
                  range={range}
                  theme={theme}
                />
              ))}
            </div>
          </CollapsibleSection>
        )}
      </div>
    </div>
  )
}
