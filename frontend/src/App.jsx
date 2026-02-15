import { useState, useEffect, useRef } from 'react'
import { RefreshCw, Calendar, AlertCircle, Loader, Search } from 'lucide-react'

function App() {
  const [logs, setLogs] = useState([])           // { message, ts }[]
  const [selectedDate, setSelectedDate] = useState('')
  const [availableDates, setAvailableDates] = useState([])
  const [searchTerm, setSearchTerm] = useState('')  // sent to backend as ?q=
  const [loading, setLoading] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [error, setError] = useState(null)
  const [nextCursor, setNextCursor] = useState('')  // for incremental fetch: only get logs after this
  const logEndRef = useRef(null)
  const scrollContainerRef = useRef(null)

  // Fetch available log dates
  const fetchAvailableDates = async () => {
    try {
      const response = await fetch('/api/logs/dates')
      if (response.ok) {
        const dates = await response.json()
        setAvailableDates(dates.sort().reverse()) // Most recent first
        if (dates.length > 0 && !selectedDate) {
          setSelectedDate(dates[0])
        }
      }
    } catch (err) {
      console.error('Failed to fetch dates:', err)
    }
  }

  // Fetch logs: full load (no after) or incremental (after=nextCursor). Search is always sent as ?q=.
  const fetchLogs = async (date, append = false) => {
    if (!date) return

    const cursor = append ? nextCursor : ''
    const params = new URLSearchParams()
    if (cursor) params.set('after', cursor)
    if (searchTerm.trim()) params.set('q', searchTerm.trim())
    const query = params.toString()
    const url = `/api/logs/${date}${query ? `?${query}` : ''}`

    setLoading(true)
    setError(null)
    try {
      const response = await fetch(url)
      if (!response.ok) {
        throw new Error(`Failed to fetch logs: ${response.statusText}`)
      }
      const data = await response.json()
      const newLogs = data.logs || []
      if (append && logs.length > 0) {
        setLogs(prev => [...prev, ...newLogs])
      } else {
        setLogs(newLogs)
      }
      setNextCursor(data.nextCursor || '')
    } catch (err) {
      setError(err.message)
      if (!append) setLogs([])
    } finally {
      setLoading(false)
    }
  }

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (autoRefresh && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs, autoRefresh])

  // Initial load
  useEffect(() => {
    fetchAvailableDates()
  }, [])

  // Fetch logs when date changes (full load)
  useEffect(() => {
    if (selectedDate) {
      fetchLogs(selectedDate, false)
    }
  }, [selectedDate])

  // When user types in search: debounce then full load with ?q= (backend does the search)
  useEffect(() => {
    if (!selectedDate) return
    const t = setTimeout(() => {
      fetchLogs(selectedDate, false) // fetchLogs already sends searchTerm as ?q=
    }, 400)
    return () => clearTimeout(t)
  }, [searchTerm, selectedDate]) // run when search term or date changes so ?q= is applied

  // Auto-refresh every 30s: incremental (only new logs after nextCursor) when cursor exists, else full fetch
  useEffect(() => {
    if (!autoRefresh || !selectedDate) return
    const interval = setInterval(() => {
      if (nextCursor) {
        fetchLogs(selectedDate, true)
      } else {
        fetchLogs(selectedDate, false)
      }
    }, 30000)
    return () => clearInterval(interval)
  }, [autoRefresh, selectedDate, nextCursor])

  // Format date for display
  const formatDateDisplay = (dateStr) => {
    if (!dateStr) return ''
    const year = dateStr.substring(0, 4)
    const month = dateStr.substring(4, 6)
    const day = dateStr.substring(6, 8)
    return `${year}-${month}-${day}`
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <header className="bg-gradient-to-br from-[#1a1a2e] to-[#16213e] border-b border-dark-border px-8 py-4 shadow-lg">
        <div className="flex justify-between items-center max-w-full flex-col md:flex-row gap-4 md:gap-0">
          <h1 className="text-2xl font-semibold text-white m-0">📊 Crypto Alert Log Viewer</h1>
          <div className="flex gap-4 items-center w-full md:w-auto flex-col md:flex-row">
            <div className="flex items-center gap-2 bg-[#2a2a3e] px-4 py-2 rounded-md border border-[#3a3a4e] w-full md:w-auto">
              <Calendar className="w-4 h-4 text-dark-text-muted" />
              <select 
                value={selectedDate} 
                onChange={(e) => setSelectedDate(e.target.value)}
                className="bg-transparent border-none text-dark-text text-sm cursor-pointer outline-none flex-1"
              >
                <option value="">Select date...</option>
                {availableDates.map(date => (
                  <option key={date} value={date} className="bg-[#2a2a3e] text-dark-text">
                    {formatDateDisplay(date)}
                  </option>
                ))}
              </select>
            </div>
            <button 
              onClick={() => {
                fetchAvailableDates()
                if (selectedDate) fetchLogs(selectedDate, false)
              }}
              className="flex items-center gap-2 bg-blue-500 text-white border-none px-4 py-2 rounded-md cursor-pointer text-sm transition-colors hover:bg-blue-600 disabled:opacity-60 disabled:cursor-not-allowed w-full md:w-auto"
              disabled={loading}
            >
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin-slow' : ''}`} />
              Refresh
            </button>
          </div>
        </div>
      </header>

      <div className="flex justify-between items-center px-8 py-4 bg-dark-surface border-b border-dark-border gap-4 flex-wrap">
        <div className="flex items-center bg-dark-surface-hover border border-dark-border rounded-md px-4 py-2 flex-1 min-w-[200px] max-w-[500px]">
          <Search className="w-4 h-4 text-dark-text-muted mr-2" />
          <input
            type="text"
            placeholder="Search logs..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="flex-1 bg-transparent border-none text-dark-text text-sm outline-none placeholder:text-dark-text-secondary"
          />
        </div>
        <label className="flex items-center gap-2 text-dark-text text-sm cursor-pointer">
          <input
            type="checkbox"
            checked={autoRefresh}
            onChange={(e) => setAutoRefresh(e.target.checked)}
            className="cursor-pointer"
          />
          Auto-refresh (30s)
        </label>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-4 bg-dark-bg scrollbar-thin" ref={scrollContainerRef}>
        {error && (
          <div className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500 rounded-md text-red-500 mb-4">
            <AlertCircle className="w-5 h-5" />
            {error}
          </div>
        )}
        
        {loading && logs.length === 0 && (
          <div className="flex items-center justify-center gap-4 py-12 text-dark-text-muted text-lg">
            <Loader className="w-6 h-6 animate-spin-slow" />
            Loading logs...
          </div>
        )}

        {!loading && logs.length === 0 && !error && (
          <div className="text-center py-12 text-dark-text-secondary text-base">
            {searchTerm ? `No logs match "${searchTerm}" for ${formatDateDisplay(selectedDate)}` : `No logs found for ${formatDateDisplay(selectedDate)}`}
          </div>
        )}

        {logs.map((entry, index) => (
          <div 
            key={entry.ts ? `${entry.ts}-${index}` : index} 
            className="p-3 mb-2 rounded-lg bg-dark-surface border-l-[3px] border-l-blue-500 hover:bg-dark-surface-hover transition-colors"
          >
            <div className="text-dark-text text-sm break-words whitespace-pre-wrap font-mono">
              {typeof entry === 'string' ? entry : entry.message}
            </div>
          </div>
        ))}
        <div ref={logEndRef} />
      </div>

      <footer className="bg-dark-surface border-t border-dark-border px-8 py-3">
        <div className="flex justify-between items-center text-dark-text-muted text-sm flex-col md:flex-row gap-2 md:gap-0">
          <span>
            {searchTerm ? `Search: "${searchTerm}" — ` : ''}Total: {logs.length} logs
          </span>
          {selectedDate && (
            <span>Viewing: {formatDateDisplay(selectedDate)}</span>
          )}
        </div>
      </footer>
    </div>
  )
}

export default App
