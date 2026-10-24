const nf = (digits: number) => new Intl.NumberFormat(undefined, { maximumFractionDigits: digits })

export const fmtNumber = (v: number | undefined | null, digits = 0) => (v == null || Number.isNaN(v) ? "—" : nf(digits).format(v))

/** Formats liters, switching to m³ for large volumes. */
export function fmtVolume(liters: number | undefined | null, unit: "auto" | "L" | "m3" = "auto") {
  if (liters == null) return "—"
  if (unit === "L" || (unit === "auto" && Math.abs(liters) < 10000)) return `${fmtNumber(liters, 1)} L`
  return `${fmtNumber(liters / 1000, 3)} m³`
}

export function fmtDateTime(v: string | Date | undefined | null) {
  if (!v) return "—"
  const d = typeof v === "string" ? new Date(v) : v
  return d.toLocaleString(undefined, { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" })
}

export function fmtTime(v: string | Date) {
  const d = typeof v === "string" ? new Date(v) : v
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
}

export function fmtRelative(v: string | undefined | null) {
  if (!v) return "never"
  const s = Math.round((Date.now() - new Date(v).getTime()) / 1000)
  if (Math.abs(s) < 5) return "just now"
  const abs = Math.abs(s)
  const [n, u] = abs < 60 ? [abs, "s"] : abs < 3600 ? [Math.round(abs / 60), "min"] : abs < 86400 ? [Math.round(abs / 3600), "h"] : [Math.round(abs / 86400), "d"]
  return s >= 0 ? `${n} ${u} ago` : `in ${n} ${u}`
}

export function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KiB`
  return `${(n / 1024 ** 2).toFixed(1)} MiB`
}

export function fmtDuration(seconds: number) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return [d && `${d}d`, (d || h) && `${h}h`, `${m}m`].filter(Boolean).join(" ")
}

/** Value for <input type="datetime-local"> in local time. */
export function toLocalInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Renders a Go duration string ("8784h0m0s") compactly ("366d"). */
export function fmtGoDuration(v: string | undefined) {
  if (!v) return "—"
  const m = /^(?:(\d+)h)?(?:(\d+)m(?!s))?(?:([\d.]+)s)?(?:([\d.]+)ms)?$/.exec(v)
  if (!m) return v
  const total = Number(m[1] ?? 0) * 3600 + Number(m[2] ?? 0) * 60 + Number(m[3] ?? 0) + Number(m[4] ?? 0) / 1000
  if (total === 0) return "0s"
  if (total < 1) return `${Math.round(total * 1000)}ms`
  const parts: string[] = []
  const d = Math.floor(total / 86400)
  const h = Math.floor((total % 86400) / 3600)
  const min = Math.floor((total % 3600) / 60)
  const sec = Math.round((total % 60) * 1000) / 1000
  if (d) parts.push(`${d}d`)
  if (h) parts.push(`${h}h`)
  if (min) parts.push(`${min}m`)
  if (sec) parts.push(`${sec}s`)
  return parts.join(" ")
}
