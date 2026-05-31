import { useState, useEffect, useRef } from 'react'
import { RefreshCw, Calendar, AlertCircle, Loader, Search, BarChart3, ScrollText, Sun, Moon } from 'lucide-react'
import Dashboard from './Dashboard'

function App() {
  const [view, setView]                     = useState('dashboard')
  const [theme, setTheme]                   = useState(() => localStorage.getItem('theme') || 'dark')
  const [logs, setLogs]                     = useState([])
  const [selectedDate, setSelectedDate]     = useState('')
  const [availableDates, setAvailableDates] = useState([])
  const [searchTerm, setSearchTerm]         = useState('')
  const [loading, setLoading]               = useState(false)
  const [autoRefresh, setAutoRefresh]       = useState(true)
  const [error, setError]                   = useState(null)
  const checkpointRef      = useRef('')
  const searchTermRef      = useRef('')
  const logEndRef          = useRef(null)
  const scrollContainerRef = useRef(null)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  const toggleTheme = () => setTheme(t => t === 'dark' ? 'light' : 'dark')

  const fetchAvailableDates = async () => {
    try {
      const response = await fetch('/api/logs/dates')
      if (response.ok) {
        const dates = await response.json()
        setAvailableDates(dates.sort().reverse())
        if (dates.length > 0 && !selectedDate) {
          setSelectedDate(dates[0])
        }
      }
    } catch (err) {
      console.error('Failed to fetch dates:', err)
    }
  }

  const fetchCheckpoint = async (date) => {
    try {
      const res = await fetch(`/api/logs/checkpoint/${date}`)
      if (res.ok) {
        const data = await res.json()
        return data.checkpoint || ''
      }
    } catch {}
    return ''
  }

  const fetchLogs = async (date) => {
    if (!date) return
    const params = new URLSearchParams()
    if (searchTermRef.current.trim()) params.set('q', searchTermRef.current.trim())
    const query = params.toString()
    const url = `/api/logs/${date}${query ? `?${query}` : ''}`

    setLoading(true)
    setError(null)
    try {
      const response = await fetch(url)
      if (!response.ok) throw new Error(`Failed to fetch logs: ${response.statusText}`)
      const data = await response.json()
      setLogs(data.logs || [])
      const cp = await fetchCheckpoint(date)
      checkpointRef.current = cp
    } catch (err) {
      setError(err.message)
      setLogs([])
    } finally {
      setLoading(false)
    }
  }

  const fetchDiff = async (date, since) => {
    try {
      const params = new URLSearchParams({ since })
      if (searchTermRef.current.trim()) params.set('q', searchTermRef.current.trim())
      const res = await fetch(`/api/logs/${date}?${params.toString()}`)
      if (!res.ok) return
      const data = await res.json()
      const newLogs = data.logs || []
      if (newLogs.length > 0) {
        setLogs(prev => [...newLogs, ...prev])
      }
    } catch {}
  }

  useEffect(() => { searchTermRef.current = searchTerm }, [searchTerm])

  useEffect(() => {
    if (autoRefresh && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs, autoRefresh])

  useEffect(() => { fetchAvailableDates() }, [])

  useEffect(() => {
    if (selectedDate) {
      checkpointRef.current = ''
      fetchLogs(selectedDate)
    }
  }, [selectedDate])

  useEffect(() => {
    if (!selectedDate) return
    const t = setTimeout(() => fetchLogs(selectedDate), 400)
    return () => clearTimeout(t)
  }, [searchTerm, selectedDate])

  useEffect(() => {
    if (!autoRefresh || !selectedDate) return
    const interval = setInterval(async () => {
      const latestCheckpoint = await fetchCheckpoint(selectedDate)
      if (!latestCheckpoint || latestCheckpoint === checkpointRef.current) return
      const prevCheckpoint = checkpointRef.current
      checkpointRef.current = latestCheckpoint
      if (prevCheckpoint) {
        await fetchDiff(selectedDate, prevCheckpoint)
      } else {
        await fetchLogs(selectedDate)
      }
    }, 30000)
    return () => clearInterval(interval)
  }, [autoRefresh, selectedDate])

  const formatDateDisplay = (dateStr) => {
    if (!dateStr) return ''
    return `${dateStr.substring(0, 4)}-${dateStr.substring(4, 6)}-${dateStr.substring(6, 8)}`
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-theme-bg text-theme-text">
      {/* Header */}
      <header className={`px-4 md:px-8 py-3 md:py-4 shrink-0 border-b border-theme-border transition-colors ${
        theme === 'dark'
          ? 'bg-gradient-to-r from-[#18182a] via-[#1c1c30] to-[#18223a] shadow-[0_1px_30px_rgba(59,130,246,0.1)]'
          : 'bg-theme-surface shadow-sm'
      }`}>
        {/*
          Mobile layout (flex-wrap):
            Row 1: Brand (order-1) ··· Theme toggle (order-2, ml-auto)
            Row 2: Tabs (order-3, w-full)
            Row 3: Log controls (order-4, w-full) — only in logs view
          Desktop layout (md:flex-nowrap):
            Single row: Brand | Tabs (ml-6) | Log controls (ml-auto) | Theme toggle
        */}
        <div className="flex flex-wrap md:flex-nowrap items-center gap-2">

          {/* Brand */}
          <div className="order-1 flex items-center gap-2 shrink-0">
            <img
              src={theme === 'dark' ? '/logo-white-front-no-background.svg' : '/logo-black-front-no-background.svg'}
              alt="SkyProton logo"
              className="h-12 md:h-[52px] w-auto"
            />
            <h1 className="text-lg md:text-xl font-bold text-theme-text m-0 tracking-tight">
              Crypto<span className="text-blue-500">Alert</span>
            </h1>
          </div>

          {/* Theme toggle — right side of row 1 on mobile, far right on desktop */}
          <button
            onClick={toggleTheme}
            title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            className={`order-2 md:order-4 ${view === 'logs' ? 'ml-auto md:ml-2' : 'ml-auto'} w-8 h-8 shrink-0 rounded-lg border border-theme-border bg-theme-input flex items-center justify-center text-theme-text-muted hover:text-theme-text hover:border-blue-500/40 transition-all`}
          >
            {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </button>

          {/* Tab navigation — full-width row 2 on mobile, inline on desktop */}
          <div className="order-3 md:order-2 w-full md:w-auto md:ml-6 flex gap-1 bg-theme-input rounded-lg p-1 border border-theme-border">
            <button
              onClick={() => setView('dashboard')}
              className={`flex-1 md:flex-none flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-all duration-150 ${
                view === 'dashboard'
                  ? 'bg-blue-500 text-white shadow-[0_0_14px_rgba(59,130,246,0.45)]'
                  : 'text-theme-text-muted hover:text-theme-text hover:bg-theme-card'
              }`}
            >
              <BarChart3 className="w-3.5 h-3.5" />
              Dashboard
            </button>
            <button
              onClick={() => setView('logs')}
              className={`flex-1 md:flex-none flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-all duration-150 ${
                view === 'logs'
                  ? 'bg-blue-500 text-white shadow-[0_0_14px_rgba(59,130,246,0.45)]'
                  : 'text-theme-text-muted hover:text-theme-text hover:bg-theme-card'
              }`}
            >
              <ScrollText className="w-3.5 h-3.5" />
              Logs
            </button>
          </div>

          {/* Log controls — full-width row 3 on mobile, inline on desktop */}
          {view === 'logs' && (
            <div className="order-4 md:order-3 md:ml-auto w-full md:w-auto flex gap-2 items-center flex-row flex-wrap">
              <div className="flex items-center gap-2 bg-theme-input px-3 py-2 rounded-lg border border-theme-border flex-1 md:flex-none hover:border-blue-500/40 transition-colors">
                <Calendar className="w-4 h-4 text-blue-500/70 shrink-0" />
                <select
                  value={selectedDate}
                  onChange={(e) => setSelectedDate(e.target.value)}
                  className="bg-transparent border-none text-theme-text text-sm cursor-pointer outline-none flex-1 min-w-0"
                >
                  <option value="">Select date...</option>
                  {availableDates.map(date => (
                    <option key={date} value={date}>{formatDateDisplay(date)}</option>
                  ))}
                </select>
              </div>
              <button
                onClick={() => {
                  fetchAvailableDates()
                  if (selectedDate) fetchLogs(selectedDate)
                }}
                className="flex items-center gap-2 bg-blue-500 text-white border-none px-4 py-2 rounded-lg cursor-pointer text-sm font-medium transition-all hover:bg-blue-400 hover:shadow-[0_0_14px_rgba(59,130,246,0.45)] disabled:opacity-50 disabled:cursor-not-allowed shrink-0"
                disabled={loading}
              >
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin-slow' : ''}`} />
                Refresh
              </button>
            </div>
          )}

        </div>
      </header>

      {/* Dashboard view */}
      {view === 'dashboard' && <Dashboard theme={theme} />}

      {/* Logs view */}
      {view === 'logs' && (
        <>
          <div className="flex justify-between items-center px-4 md:px-8 py-3 bg-theme-surface border-b border-theme-border gap-4 flex-wrap shrink-0">
            <div className="flex items-center bg-theme-input border border-theme-border-subtle rounded-lg px-3 py-2 flex-1 min-w-[200px] max-w-[480px] focus-within:border-blue-500/50 transition-colors">
              <Search className="w-4 h-4 text-blue-400/60 mr-2 shrink-0" />
              <input
                type="text"
                placeholder="Search logs..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="flex-1 bg-transparent border-none text-theme-text text-sm outline-none placeholder:text-theme-text-secondary"
              />
            </div>
            <label className="flex items-center gap-2 text-theme-text-muted text-xs cursor-pointer select-none hover:text-theme-text transition-colors">
              <div className={`relative w-8 h-4 rounded-full transition-colors duration-200 ${autoRefresh ? 'bg-blue-500' : 'bg-theme-toggle'}`}>
                <div className={`absolute top-0.5 w-3 h-3 rounded-full bg-white shadow transition-transform duration-200 ${autoRefresh ? 'translate-x-4' : 'translate-x-0.5'}`} />
                <input
                  type="checkbox"
                  checked={autoRefresh}
                  onChange={(e) => setAutoRefresh(e.target.checked)}
                  className="sr-only"
                />
              </div>
              Auto-refresh (30s)
            </label>
          </div>

          <div className="flex-1 overflow-y-auto px-4 md:px-8 py-4 bg-theme-bg scrollbar-thin" ref={scrollContainerRef}>
            {error && (
              <div className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500/40 rounded-lg text-red-400 mb-4">
                <AlertCircle className="w-5 h-5 shrink-0" />
                {error}
              </div>
            )}

            {loading && logs.length === 0 && (
              <div className="flex items-center justify-center gap-3 py-16 text-theme-text-muted text-sm">
                <Loader className="w-5 h-5 animate-spin-slow text-blue-400" />
                Loading logs...
              </div>
            )}

            {!loading && logs.length === 0 && !error && (
              <div className="text-center py-16 text-theme-text-secondary text-sm">
                {searchTerm
                  ? `No logs match "${searchTerm}" for ${formatDateDisplay(selectedDate)}`
                  : `No logs found for ${formatDateDisplay(selectedDate)}`}
              </div>
            )}

            {logs.map((entry, index) => (
              <div
                key={entry.ts ? `${entry.ts}-${index}` : index}
                className="p-3 mb-1.5 rounded-lg bg-theme-card border border-theme-border border-l-[2px] border-l-blue-500/50 hover:bg-theme-card-hover hover:border-l-blue-400 transition-all duration-100 group"
              >
                <div className="text-theme-log text-xs break-words whitespace-pre-wrap font-mono leading-relaxed group-hover:text-theme-log-hover transition-colors">
                  {typeof entry === 'string' ? entry : entry.message}
                </div>
              </div>
            ))}
            <div ref={logEndRef} />
          </div>

          <footer className="bg-theme-surface border-t border-theme-border px-4 md:px-8 py-3 shrink-0">
            <div className="flex justify-between items-center text-theme-text-muted text-xs flex-col md:flex-row gap-2 md:gap-0">
              <span className="font-mono">
                {searchTerm ? <><span className="text-blue-400/70">search:</span> &ldquo;{searchTerm}&rdquo; &mdash; </> : ''}{logs.length} entries
              </span>
              {selectedDate && (
                <span className="font-mono text-theme-text-secondary">{formatDateDisplay(selectedDate)}</span>
              )}
            </div>
          </footer>
        </>
      )}
    </div>
  )
}

export default App
