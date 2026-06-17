import React, { useEffect, useMemo, useState } from 'react'
import type { PointerEvent } from 'react'
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

type BandLevel = 'good' | 'bad' | 'critical'

type ThresholdBand = {
  level: BandLevel
  min?: number
  max?: number
}

type ThresholdsResponse = {
  metrics: Partial<Record<MetricKey, ThresholdBand[]>>
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
  const [thresholds, setThresholds] = useState<Partial<Record<MetricKey, ThresholdBand[]>>>({})
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
    async function loadThresholds() {
      try {
        const res = await fetch('/api/thresholds')
        if (!res.ok) throw new Error('Threshold request failed')
        const body = (await res.json()) as ThresholdsResponse
        if (!cancelled) setThresholds(body.metrics ?? {})
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load thresholds')
      }
    }
    loadThresholds()
    return () => {
      cancelled = true
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
        <LineChart points={points} unit={metrics[metric].unit} precision={metrics[metric].precision} bands={thresholds[metric] ?? []} />
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

function LineChart({ points, unit, precision, bands }: { points: Point[]; unit: string; precision: number; bands: ThresholdBand[] }) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)

  if (points.length === 0) {
    return <div className="empty-chart">No samples in this range</div>
  }

  const width = 760
  const height = 320
  const pad = { top: 18, right: 24, bottom: 48, left: 58 }
  const plotWidth = width - pad.left - pad.right
  const plotHeight = height - pad.top - pad.bottom
  const values = points.map((p) => p.value)
  const finiteThresholds = bands.flatMap((band) => [band.min, band.max]).filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
  const rawMin = Math.min(...values, ...finiteThresholds)
  const rawMax = Math.max(...values, ...finiteThresholds)
  const yPadding = Math.max((rawMax - rawMin) * 0.08, 1)
  const yMin = rawMin === rawMax ? rawMin - 1 : rawMin - yPadding
  const yMax = rawMin === rawMax ? rawMax + 1 : rawMax + yPadding
  const ySpan = yMax - yMin || 1
  const times = points.map((point, index) => {
    const timestamp = Date.parse(point.recorded_at)
    return Number.isFinite(timestamp) ? timestamp : index
  })
  const timeMin = Math.min(...times)
  const timeMax = Math.max(...times)
  const timeSpan = timeMax - timeMin || 1
  const xForTime = (time: number) => pad.left + ((time - timeMin) / timeSpan) * plotWidth
  const yForValue = (value: number) => pad.top + plotHeight - ((value - yMin) / ySpan) * plotHeight
  const path = points
    .map((point, index) => {
      const x = xForTime(times[index])
      const y = yForValue(point.value)
      return `${index === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`
    })
    .join(' ')
  const latest = points[points.length - 1]
  const yTicks = makeTicks(yMin, yMax, 5)
  const xTicks = makeTicks(timeMin, timeMax, Math.min(4, points.length))
  const hoverPoint = hoverIndex === null ? null : points[hoverIndex]
  const hoverTime = hoverIndex === null ? null : times[hoverIndex]
  const hoverX = hoverTime === null ? null : xForTime(hoverTime)
  const hoverY = hoverPoint === null ? null : yForValue(hoverPoint.value)

  function onPointerMove(event: PointerEvent<SVGSVGElement>) {
    const rect = event.currentTarget.getBoundingClientRect()
    const pointerX = ((event.clientX - rect.left) / rect.width) * width
    let nearestIndex = 0
    let nearestDistance = Number.POSITIVE_INFINITY
    times.forEach((time, index) => {
      const distance = Math.abs(xForTime(time) - pointerX)
      if (distance < nearestDistance) {
        nearestDistance = distance
        nearestIndex = index
      }
    })
    setHoverIndex(nearestIndex)
  }

  return (
    <div className="chart-wrap">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Historical readings chart" onPointerMove={onPointerMove} onPointerLeave={() => setHoverIndex(null)}>
        {bands.map((band, index) => {
          const bandMin = band.min ?? yMin
          const bandMax = band.max ?? yMax
          const visibleMin = Math.max(bandMin, yMin)
          const visibleMax = Math.min(bandMax, yMax)
          if (visibleMin >= visibleMax) return null
          const y = yForValue(visibleMax)
          const bandHeight = yForValue(visibleMin) - y
          return <rect key={`${band.level}-${index}`} className={`chart-band ${band.level}`} x={pad.left} y={y} width={plotWidth} height={bandHeight} />
        })}
        {yTicks.map((tick) => {
          const y = yForValue(tick)
          return (
            <g key={tick} className="chart-tick">
              <line x1={pad.left} y1={y} x2={width - pad.right} y2={y} />
              <text x={pad.left - 10} y={y} textAnchor="end" dominantBaseline="middle">
                {tick.toFixed(precision)}
              </text>
            </g>
          )
        })}
        {xTicks.map((tick) => {
          const x = xForTime(tick)
          return (
            <g key={tick} className="chart-tick x-axis">
              <line x1={x} y1={pad.top} x2={x} y2={height - pad.bottom} />
              <text x={x} y={height - 18} textAnchor="middle">
                {formatAxisDate(tick)}
              </text>
            </g>
          )
        })}
        <line className="chart-axis" x1={pad.left} y1={pad.top} x2={pad.left} y2={height - pad.bottom} />
        <line className="chart-axis" x1={pad.left} y1={height - pad.bottom} x2={width - pad.right} y2={height - pad.bottom} />
        <path className="chart-line" d={path} />
        {hoverPoint && hoverX !== null && hoverY !== null && (
          <g className="chart-hover">
            <line x1={hoverX} y1={pad.top} x2={hoverX} y2={height - pad.bottom} />
            <circle cx={hoverX} cy={hoverY} r="5" />
          </g>
        )}
      </svg>
      {hoverPoint && hoverX !== null && hoverY !== null && (
        <div className="chart-tooltip" style={{ left: `${(hoverX / width) * 100}%`, top: `${(hoverY / height) * 100}%` }}>
          <strong>
            {hoverPoint.value.toFixed(precision)} {unit}
          </strong>
          <span>{formatHoverDate(hoverPoint.recorded_at)}</span>
        </div>
      )}
      <div className="chart-meta">
        <span>
          Min {Math.min(...values).toFixed(precision)} {unit}
        </span>
        <span>
          Latest {latest.value.toFixed(precision)} {unit}
        </span>
        <span>
          Max {Math.max(...values).toFixed(precision)} {unit}
        </span>
      </div>
    </div>
  )
}

function makeTicks(min: number, max: number, count: number) {
  if (count <= 1 || min === max) return [min]
  return Array.from({ length: count }, (_, index) => min + ((max - min) / (count - 1)) * index)
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

function formatAxisDate(value: number) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  }).format(new Date(value))
}

function formatHoverDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short'
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
