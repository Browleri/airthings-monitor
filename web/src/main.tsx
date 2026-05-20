import React, { useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import './styles.css'

type Current = {
  co2_ppm: number | null
  voc_ppb: number | null
  temperature_c: number | null
  humidity_percent: number | null
  pressure_hpa: number | null
  radon_short_bqm3: number | null
  radon_long_bqm3: number | null
  last_read_at: string | null
  last_successful_read?: string | null
  sensor_stale?: boolean
}

type Status = {
  ok: boolean
  sensor_stale: boolean
  sensor_address: string
  database_path: string
  stale: boolean
  stale_after_seconds: number
  last_successful_read: string | null
  last_success_at: string | null
  last_attempt_at: string | null
  last_error?: string
  last_error_at?: string | null
  last_retry_delay_seconds?: number | null
  database_ok: boolean
  bluetooth_ok: boolean
  consecutive_failures: number
}

type Point = {
  recorded_at: string
  value: number
}

type ReadingsResponse = {
  metric: string
  range: RangeKey
  points: Point[]
}

type RangeKey = '1h' | '24h' | '7d' | '30d'
type MetricKey = 'co2' | 'voc' | 'temperature' | 'humidity' | 'pressure' | 'radon_short' | 'radon_long'

const ranges: RangeKey[] = ['1h', '24h', '7d', '30d']

const metrics: Record<MetricKey, { label: string; unit: string; currentKey: keyof Current; precision: number }> = {
  co2: { label: 'CO2', unit: 'ppm', currentKey: 'co2_ppm', precision: 0 },
  voc: { label: 'VOC', unit: 'ppb', currentKey: 'voc_ppb', precision: 0 },
  temperature: { label: 'Temperature', unit: 'C', currentKey: 'temperature_c', precision: 1 },
  humidity: { label: 'Humidity', unit: '%', currentKey: 'humidity_percent', precision: 1 },
  pressure: { label: 'Pressure', unit: 'hPa', currentKey: 'pressure_hpa', precision: 1 },
  radon_short: { label: 'Radon short', unit: 'Bq/m3', currentKey: 'radon_short_bqm3', precision: 0 },
  radon_long: { label: 'Radon long', unit: 'Bq/m3', currentKey: 'radon_long_bqm3', precision: 0 }
}

function App() {
  const [current, setCurrent] = useState<Current | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [range, setRange] = useState<RangeKey>('24h')
  const [metric, setMetric] = useState<MetricKey>('co2')
  const [points, setPoints] = useState<Point[]>([])
  const [error, setError] = useState<string>('')

  useEffect(() => {
    let cancelled = false

    async function loadCurrent() {
      try {
        const [currentRes, statusRes] = await Promise.all([fetch('/api/current'), fetch('/api/status')])
        if (!currentRes.ok || !statusRes.ok) {
          throw new Error('API request failed')
        }
        const nextCurrent = (await currentRes.json()) as Current
        const nextStatus = (await statusRes.json()) as Status
        if (!cancelled) {
          setCurrent(nextCurrent)
          setStatus(nextStatus)
          setError('')
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load status')
      }
    }

    loadCurrent()
    const timer = window.setInterval(loadCurrent, 30_000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    async function loadChart() {
      try {
        const res = await fetch(`/api/readings?metric=${metric}&range=${range}`)
        if (!res.ok) throw new Error('Chart request failed')
        const body = (await res.json()) as ReadingsResponse
        if (!cancelled) setPoints(body.points ?? [])
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load chart')
      }
    }
    loadChart()
    return () => {
      cancelled = true
    }
  }, [metric, range])

  const stale = status?.sensor_stale ?? status?.stale ?? current?.sensor_stale ?? true
  const metricCards = useMemo(() => Object.entries(metrics) as [MetricKey, (typeof metrics)[MetricKey]][], [])

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>Airthings Monitor</h1>
          <p>{status ? `Sensor ${status.sensor_address}` : 'Connecting to service'}</p>
        </div>
        <div className={`status-pill ${stale ? 'warning' : 'ok'}`}>{stale ? 'Stale' : 'Live'}</div>
      </header>

      {error && <div className="notice">{error}</div>}
      {stale && (
        <div className="notice">
          Sensor data is stale. Last successful read: {formatDate(status?.last_successful_read ?? status?.last_success_at ?? current?.last_successful_read ?? current?.last_read_at)}
        </div>
      )}
      {status?.last_error && (
        <div className="notice muted">
          Last error{status.last_error_at ? ` at ${formatDate(status.last_error_at)}` : ''}: {status.last_error}
          {status.last_retry_delay_seconds ? ` Retrying in about ${status.last_retry_delay_seconds}s.` : ''}
        </div>
      )}

      <section className="metric-grid">
        {metricCards.map(([key, meta]) => (
          <button
            key={key}
            className={`metric-card ${metric === key ? 'selected' : ''}`}
            onClick={() => setMetric(key)}
            type="button"
          >
            <span>{meta.label}</span>
            <strong>{formatValue(current?.[meta.currentKey], meta.precision)}</strong>
            <small>{meta.unit}</small>
          </button>
        ))}
      </section>

      <section className="panel">
        <div className="chart-header">
          <div>
            <h2>{metrics[metric].label}</h2>
            <p>{points.length} samples</p>
          </div>
          <div className="segmented">
            {ranges.map((item) => (
              <button key={item} className={range === item ? 'active' : ''} onClick={() => setRange(item)} type="button">
                {item}
              </button>
            ))}
          </div>
        </div>
        <LineChart points={points} unit={metrics[metric].unit} precision={metrics[metric].precision} />
      </section>

      <section className="details">
        <div>
          <span>Last successful read</span>
          <strong>{formatDate(status?.last_successful_read ?? status?.last_success_at ?? current?.last_successful_read ?? current?.last_read_at)}</strong>
        </div>
        <div>
          <span>Last attempt</span>
          <strong>{formatDate(status?.last_attempt_at)}</strong>
        </div>
        <div>
          <span>Service checks</span>
          <strong>{formatChecks(status)}</strong>
        </div>
        <div>
          <span>Database</span>
          <strong>{status?.database_path ?? 'Unknown'}</strong>
        </div>
      </section>
    </main>
  )
}

function LineChart({ points, unit, precision }: { points: Point[]; unit: string; precision: number }) {
  if (points.length === 0) {
    return <div className="empty-chart">No samples in this range</div>
  }

  const width = 720
  const height = 260
  const pad = 28
  const values = points.map((p) => p.value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  const path = points
    .map((point, index) => {
      const x = pad + (index / Math.max(points.length - 1, 1)) * (width - pad * 2)
      const y = height - pad - ((point.value - min) / span) * (height - pad * 2)
      return `${index === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`
    })
    .join(' ')
  const latest = points[points.length - 1]

  return (
    <div className="chart-wrap">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Historical readings chart">
        <line x1={pad} y1={pad} x2={pad} y2={height - pad} />
        <line x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} />
        <path d={path} />
      </svg>
      <div className="chart-meta">
        <span>
          Min {min.toFixed(precision)} {unit}
        </span>
        <span>
          Latest {latest.value.toFixed(precision)} {unit}
        </span>
        <span>
          Max {max.toFixed(precision)} {unit}
        </span>
      </div>
    </div>
  )
}

function formatValue(value: unknown, precision: number) {
  if (typeof value !== 'number') return 'No data'
  return value.toFixed(precision)
}

function formatDate(value?: string | null) {
  if (!value) return 'No data'
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

function formatChecks(status: Status | null) {
  if (!status) return 'Unknown'
  const database = status.database_ok ? 'DB ok' : 'DB issue'
  const bluetooth = status.bluetooth_ok ? 'BLE ok' : 'BLE retrying'
  return `${database}, ${bluetooth}`
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
