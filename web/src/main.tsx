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

type SummaryMetric = {
  metric: string
  count: number
  min: number
  max: number
  avg: number
}

type SummaryResponse = {
  range: string
  metrics: SummaryMetric[]
}

type RangeKey = '1h' | '24h' | '7d' | '30d'
type MetricKey = 'co2' | 'voc' | 'temperature' | 'humidity' | 'pressure' | 'radon_short' | 'radon_long'
type TempUnit = 'C' | 'F'
type Theme = 'light' | 'dark' | null

const ranges: RangeKey[] = ['1h', '24h', '7d', '30d']

const metrics: Record<MetricKey, {
  label: string
  unit: string
  currentKey: keyof Current
  precision: number
  trendThreshold: number
  info?: string
}> = {
  co2:         { label: 'CO2',         unit: 'ppm',   currentKey: 'co2_ppm',          precision: 0, trendThreshold: 50 },
  voc:         { label: 'VOC',         unit: 'ppb',   currentKey: 'voc_ppb',          precision: 0, trendThreshold: 25 },
  temperature: { label: 'Temperature', unit: 'C',     currentKey: 'temperature_c',    precision: 1, trendThreshold: 0.5 },
  humidity:    { label: 'Humidity',    unit: '%',     currentKey: 'humidity_percent', precision: 1, trendThreshold: 2 },
  pressure:    { label: 'Pressure',    unit: 'hPa',   currentKey: 'pressure_hpa',     precision: 1, trendThreshold: 2 },
  radon_short: {
    label: 'Radon short', unit: 'Bq/m³', currentKey: 'radon_short_bqm3', precision: 0, trendThreshold: 10,
    info: 'WHO: 100 · EU action: 300 Bq/m³',
  },
  radon_long: {
    label: 'Radon long', unit: 'Bq/m³', currentKey: 'radon_long_bqm3', precision: 0, trendThreshold: 10,
    info: 'Long-term avg · WHO: 100 Bq/m³',
  },
}

// --- pure helpers -------------------------------------------------------

function evaluateLevel(value: number, bands: ThresholdBand[]): BandLevel {
  for (const band of bands) {
    if (band.min !== undefined && value < band.min) continue
    if (band.max !== undefined && value >= band.max) continue
    return band.level
  }
  return 'good'
}

function levelScore(level: string): number {
  if (level === 'critical') return 2
  if (level === 'bad') return 1
  return 0
}

function computeAirQuality(
  current: Current | null,
  thresholds: Partial<Record<MetricKey, ThresholdBand[]>>,
): { level: BandLevel | 'unknown'; label: string; detail: string } {
  if (!current) return { level: 'unknown', label: 'Unknown', detail: 'Waiting for data' }
  let worst: BandLevel = 'good'
  let worstLabel = ''
  for (const [key, meta] of Object.entries(metrics) as [MetricKey, typeof metrics[MetricKey]][]) {
    const raw = current[meta.currentKey]
    if (typeof raw !== 'number') continue
    const bands = thresholds[key] ?? []
    if (bands.length === 0) continue
    const level = evaluateLevel(raw, bands)
    if (levelScore(level) > levelScore(worst)) {
      worst = level
      worstLabel = meta.label
    }
  }
  if (worst === 'critical') return { level: 'critical', label: 'Poor',    detail: `${worstLabel} needs attention` }
  if (worst === 'bad')      return { level: 'bad',      label: 'Fair',    detail: `${worstLabel} is elevated` }
  return                           { level: 'good',     label: 'Good',    detail: 'All metrics are in range' }
}

function computeTrend(
  raw: unknown,
  key: MetricKey,
  hourSummary: Partial<Record<MetricKey, SummaryMetric>>,
): '↑' | '↓' | '→' | null {
  if (typeof raw !== 'number') return null
  const s = hourSummary[key]
  if (!s || s.count === 0) return null
  const diff = raw - s.avg
  const threshold = metrics[key].trendThreshold
  if (diff > threshold)  return '↑'
  if (diff < -threshold) return '↓'
  return '→'
}

function displayValue(raw: unknown, key: MetricKey, precision: number, tempUnit: TempUnit): string {
  if (typeof raw !== 'number') return 'No data'
  const value = key === 'temperature' && tempUnit === 'F' ? raw * 9 / 5 + 32 : raw
  return value.toFixed(precision)
}

function displayUnit(key: MetricKey, unit: string, tempUnit: TempUnit): string {
  return key === 'temperature' ? `°${tempUnit}` : unit
}

function transformValue(value: number, key: MetricKey, tempUnit: TempUnit): number {
  return key === 'temperature' && tempUnit === 'F' ? value * 9 / 5 + 32 : value
}

// --- App ---------------------------------------------------------------

function App() {
  const [current, setCurrent]         = useState<Current | null>(null)
  const [status, setStatus]           = useState<Status | null>(null)
  const [range, setRange]             = useState<RangeKey>('24h')
  const [metric, setMetric]           = useState<MetricKey>('co2')
  const [points, setPoints]           = useState<Point[]>([])
  const [thresholds, setThresholds]   = useState<Partial<Record<MetricKey, ThresholdBand[]>>>({})
  const [summary, setSummary]         = useState<Partial<Record<MetricKey, SummaryMetric>>>({})
  const [hourSummary, setHourSummary] = useState<Partial<Record<MetricKey, SummaryMetric>>>({})
  const [error, setError]             = useState<string>('')
  const [tempUnit, setTempUnit]       = useState<TempUnit>(() =>
    (localStorage.getItem('tempUnit') as TempUnit) ?? 'C',
  )
  const [theme, setTheme] = useState<Theme>(() =>
    (localStorage.getItem('theme') as Theme) ?? null,
  )

  const isDark = theme === 'dark' ||
    (theme === null && typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches)

  useEffect(() => {
    if (theme === null) {
      document.documentElement.removeAttribute('data-theme')
      localStorage.removeItem('theme')
    } else {
      document.documentElement.dataset.theme = theme
      localStorage.setItem('theme', theme)
    }
  }, [theme])

  // current + status + 1h summary polled every 30s
  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [currentRes, statusRes, hourRes] = await Promise.all([
          fetch('/api/current'),
          fetch('/api/status'),
          fetch('/api/summary?range=1h'),
        ])
        if (!currentRes.ok || !statusRes.ok) throw new Error('API request failed')
        const nextCurrent = (await currentRes.json()) as Current
        const nextStatus  = (await statusRes.json()) as Status
        if (!cancelled) {
          setCurrent(nextCurrent)
          setStatus(nextStatus)
          setError('')
        }
        if (hourRes.ok) {
          const body = (await hourRes.json()) as SummaryResponse
          if (!cancelled) setHourSummary(indexSummary(body.metrics))
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load status')
      }
    }
    load()
    const timer = window.setInterval(load, 30_000)
    return () => { cancelled = true; window.clearInterval(timer) }
  }, [])

  // thresholds (once)
  useEffect(() => {
    let cancelled = false
    fetch('/api/thresholds')
      .then(r => r.ok ? r.json() : Promise.reject(new Error('Threshold request failed')))
      .then((body: ThresholdsResponse) => { if (!cancelled) setThresholds(body.metrics ?? {}) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [])

  // range summary — updates when selected range changes
  useEffect(() => {
    let cancelled = false
    fetch(`/api/summary?range=${range}`)
      .then(r => r.ok ? r.json() : Promise.reject())
      .then((body: SummaryResponse) => { if (!cancelled) setSummary(indexSummary(body.metrics)) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [range])

  // chart data
  useEffect(() => {
    let cancelled = false
    fetch(`/api/readings?metric=${metric}&range=${range}`)
      .then(r => r.ok ? r.json() : Promise.reject(new Error('Chart request failed')))
      .then((body: ReadingsResponse) => { if (!cancelled) setPoints(body.points ?? []) })
      .catch(err => { if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load chart') })
    return () => { cancelled = true }
  }, [metric, range])

  const stale = status?.sensor_stale ?? status?.stale ?? current?.sensor_stale ?? true
  const metricCards = useMemo(() => Object.entries(metrics) as [MetricKey, typeof metrics[MetricKey]][], [])
  const airQuality = useMemo(() => computeAirQuality(current, thresholds), [current, thresholds])

  const meta = metrics[metric]
  const unit = displayUnit(metric, meta.unit, tempUnit)

  function toggleTempUnit() {
    const next: TempUnit = tempUnit === 'C' ? 'F' : 'C'
    setTempUnit(next)
    localStorage.setItem('tempUnit', next)
  }

  function toggleTheme() {
    setTheme(isDark ? 'light' : 'dark')
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <h1>Airthings Monitor</h1>
          <p>{status ? `Sensor ${status.sensor_address}` : 'Connecting to service'}</p>
        </div>
        <div className="topbar-controls">
          <div className={`quality-badge ${airQuality.level}`}>
            {airQuality.label}
            <span>{airQuality.detail}</span>
          </div>
          <div className={`status-pill ${stale ? 'warning' : 'ok'}`}>{stale ? 'Stale' : 'Live'}</div>
          <button className="ctrl-btn" onClick={toggleTempUnit} title="Toggle temperature unit" type="button">
            {tempUnit === 'C' ? '°F' : '°C'}
          </button>
          <button className="ctrl-btn" onClick={toggleTheme} title="Toggle dark mode" type="button">
            {isDark ? '☀' : '☽'}
          </button>
        </div>
      </header>

      {error && <div className="notice">{error}</div>}
      {stale && (
        <div className="notice">
          Sensor data is stale. Last successful read:{' '}
          {formatDate(status?.last_successful_read ?? status?.last_success_at ?? current?.last_successful_read ?? current?.last_read_at)}
        </div>
      )}
      {status?.last_error && (
        <div className="notice muted">
          Last error{status.last_error_at ? ` at ${formatDate(status.last_error_at)}` : ''}: {status.last_error}
          {status.last_retry_delay_seconds ? ` Retrying in about ${status.last_retry_delay_seconds}s.` : ''}
        </div>
      )}

      <section className="metric-grid">
        {metricCards.map(([key, m]) => {
          const raw = current?.[m.currentKey]
          const trend = computeTrend(raw, key, hourSummary)
          return (
            <button
              key={key}
              className={`metric-card ${metric === key ? 'selected' : ''}`}
              onClick={() => setMetric(key)}
              type="button"
            >
              <span>{m.label}</span>
              <strong>
                {displayValue(raw, key, m.precision, tempUnit)}
                {trend && <span className="metric-trend" aria-hidden="true">{trend}</span>}
              </strong>
              <small>{displayUnit(key, m.unit, tempUnit)}</small>
              {m.info && <small className="metric-info">{m.info}</small>}
            </button>
          )
        })}
      </section>

      <section className="panel">
        <div className="chart-header">
          <div>
            <h2>{meta.label}</h2>
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
        <LineChart
          points={points}
          unit={unit}
          precision={meta.precision}
          bands={thresholds[metric] ?? []}
          metricKey={metric}
          tempUnit={tempUnit}
          summaryAvg={summary[metric]?.count ? summary[metric]!.avg : null}
        />
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

function indexSummary(metrics: SummaryMetric[] | undefined): Partial<Record<MetricKey, SummaryMetric>> {
  const map: Partial<Record<MetricKey, SummaryMetric>> = {}
  for (const m of metrics ?? []) map[m.metric as MetricKey] = m
  return map
}

// --- LineChart ---------------------------------------------------------

function LineChart({
  points, unit, precision, bands, metricKey, tempUnit, summaryAvg,
}: {
  points: Point[]
  unit: string
  precision: number
  bands: ThresholdBand[]
  metricKey: MetricKey
  tempUnit: TempUnit
  summaryAvg: number | null
}) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)

  if (points.length === 0) {
    return <div className="empty-chart">No samples in this range</div>
  }

  const width = 760
  const height = 320
  const pad = { top: 18, right: 24, bottom: 48, left: 58 }
  const plotWidth  = width  - pad.left - pad.right
  const plotHeight = height - pad.top  - pad.bottom

  const values = points.map(p => transformValue(p.value, metricKey, tempUnit))

  const transformedBands = bands.map(b => ({
    level: b.level,
    min: b.min !== undefined ? transformValue(b.min, metricKey, tempUnit) : undefined,
    max: b.max !== undefined ? transformValue(b.max, metricKey, tempUnit) : undefined,
  }))

  const finiteThresholds = transformedBands
    .flatMap(b => [b.min, b.max])
    .filter((v): v is number => typeof v === 'number' && Number.isFinite(v))

  const rawMin = Math.min(...values, ...finiteThresholds)
  const rawMax = Math.max(...values, ...finiteThresholds)
  const yPadding = Math.max((rawMax - rawMin) * 0.08, 1)
  const yMin = rawMin === rawMax ? rawMin - 1 : rawMin - yPadding
  const yMax = rawMin === rawMax ? rawMax + 1 : rawMax + yPadding
  const ySpan = yMax - yMin || 1

  const times = points.map((p, i) => {
    const ts = Date.parse(p.recorded_at)
    return Number.isFinite(ts) ? ts : i
  })
  const timeMin  = Math.min(...times)
  const timeMax  = Math.max(...times)
  const timeSpan = timeMax - timeMin || 1

  const xForTime  = (t: number) => pad.left + ((t - timeMin) / timeSpan) * plotWidth
  const yForValue = (v: number) => pad.top + plotHeight - ((v - yMin) / ySpan) * plotHeight

  const path = values
    .map((v, i) => `${i === 0 ? 'M' : 'L'} ${xForTime(times[i]).toFixed(2)} ${yForValue(v).toFixed(2)}`)
    .join(' ')

  const latest = values[values.length - 1]
  const yTicks  = makeTicks(yMin, yMax, 5)
  const xTicks  = makeTicks(timeMin, timeMax, Math.min(4, points.length))

  const hoverValue = hoverIndex === null ? null : values[hoverIndex]
  const hoverPoint = hoverIndex === null ? null : points[hoverIndex]
  const hoverTime  = hoverIndex === null ? null : times[hoverIndex]
  const hoverX     = hoverTime  === null ? null : xForTime(hoverTime)
  const hoverY     = hoverValue === null ? null : yForValue(hoverValue)

  function onPointerMove(event: PointerEvent<SVGSVGElement>) {
    const rect = event.currentTarget.getBoundingClientRect()
    const pointerX = ((event.clientX - rect.left) / rect.width) * width
    let nearestIndex = 0
    let nearestDistance = Number.POSITIVE_INFINITY
    times.forEach((t, i) => {
      const d = Math.abs(xForTime(t) - pointerX)
      if (d < nearestDistance) { nearestDistance = d; nearestIndex = i }
    })
    setHoverIndex(nearestIndex)
  }

  return (
    <div className="chart-wrap">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Historical readings chart" onPointerMove={onPointerMove} onPointerLeave={() => setHoverIndex(null)}>
        {transformedBands.map((band, i) => {
          const bandMin = band.min ?? yMin
          const bandMax = band.max ?? yMax
          const visMin  = Math.max(bandMin, yMin)
          const visMax  = Math.min(bandMax, yMax)
          if (visMin >= visMax) return null
          const y = yForValue(visMax)
          return <rect key={`${band.level}-${i}`} className={`chart-band ${band.level}`} x={pad.left} y={y} width={plotWidth} height={yForValue(visMin) - y} />
        })}
        {yTicks.map(tick => {
          const y = yForValue(tick)
          return (
            <g key={tick} className="chart-tick">
              <line x1={pad.left} y1={y} x2={width - pad.right} y2={y} />
              <text x={pad.left - 10} y={y} textAnchor="end" dominantBaseline="middle">{tick.toFixed(precision)}</text>
            </g>
          )
        })}
        {xTicks.map(tick => {
          const x = xForTime(tick)
          return (
            <g key={tick} className="chart-tick x-axis">
              <line x1={x} y1={pad.top} x2={x} y2={height - pad.bottom} />
              <text x={x} y={height - 18} textAnchor="middle">{formatAxisDate(tick)}</text>
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
      {hoverPoint && hoverValue !== null && hoverX !== null && hoverY !== null && (
        <div className="chart-tooltip" style={{ left: `${(hoverX / width) * 100}%`, top: `${(hoverY / height) * 100}%` }}>
          <strong>{hoverValue.toFixed(precision)} {unit}</strong>
          <span>{formatHoverDate(hoverPoint.recorded_at)}</span>
        </div>
      )}
      <div className="chart-meta">
        <span>Min {Math.min(...values).toFixed(precision)} {unit}</span>
        {summaryAvg !== null && <span>Avg {transformValue(summaryAvg, metricKey, tempUnit).toFixed(precision)} {unit}</span>}
        <span>Latest {latest.toFixed(precision)} {unit}</span>
        <span>Max {Math.max(...values).toFixed(precision)} {unit}</span>
      </div>
    </div>
  )
}

// --- utilities ---------------------------------------------------------

function makeTicks(min: number, max: number, count: number) {
  if (count <= 1 || min === max) return [min]
  return Array.from({ length: count }, (_, i) => min + ((max - min) / (count - 1)) * i)
}

function formatDate(value?: string | null) {
  if (!value) return 'No data'
  return new Intl.DateTimeFormat(undefined, {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  }).format(new Date(value))
}

function formatAxisDate(value: number) {
  return new Intl.DateTimeFormat(undefined, {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  }).format(new Date(value))
}

function formatHoverDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium', timeStyle: 'short',
  }).format(new Date(value))
}

function formatChecks(status: Status | null) {
  if (!status) return 'Unknown'
  return `${status.database_ok ? 'DB ok' : 'DB issue'}, ${status.bluetooth_ok ? 'BLE ok' : 'BLE retrying'}`
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
