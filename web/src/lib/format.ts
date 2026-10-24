import i18n, { locale } from "@/i18n"

const t = i18n.t.bind(i18n)

const nf = (digits: number) => new Intl.NumberFormat(locale(), { maximumFractionDigits: digits })

export const fmtNumber = (v: number | undefined | null, digits = 0) => (v == null || Number.isNaN(v) ? "—" : nf(digits).format(v))

/** Formats liters, switching to m³ for large volumes. */
export function fmtVolume(liters: number | undefined | null, unit: "auto" | "L" | "m3" = "auto") {
  if (liters == null) return "—"
  if (unit === "L" || (unit === "auto" && Math.abs(liters) < 10000)) return `${fmtNumber(liters, 1)} L`
  return `${fmtNumber(liters / 1000, 3)} m³`
}

const toDate = (v: string | Date) => (typeof v === "string" ? new Date(v) : v)

export function fmtDateTime(v: string | Date | undefined | null) {
  if (!v) return "—"
  return toDate(v).toLocaleString(locale(), { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" })
}

export function fmtDate(v: string | Date | undefined | null) {
  if (!v) return "—"
  return toDate(v).toLocaleDateString(locale(), { year: "numeric", month: "2-digit", day: "2-digit" })
}

export function fmtTime(v: string | Date) {
  return toDate(v).toLocaleTimeString(locale(), { hour: "2-digit", minute: "2-digit" })
}

export function fmtRelative(v: string | undefined | null) {
  if (!v) return t("format.never")
  const s = Math.round((new Date(v).getTime() - Date.now()) / 1000)
  const abs = Math.abs(s)
  if (abs < 5) return t("format.justNow")
  const rtf = new Intl.RelativeTimeFormat(locale(), { numeric: "auto" })
  if (abs < 60) return rtf.format(s, "second")
  if (abs < 3600) return rtf.format(Math.round(s / 60), "minute")
  if (abs < 86400) return rtf.format(Math.round(s / 3600), "hour")
  return rtf.format(Math.round(s / 86400), "day")
}

export function fmtBytes(n: number) {
  if (n < 1024) return `${fmtNumber(n)} B`
  if (n < 1024 ** 2) return `${fmtNumber(n / 1024, 1)} KiB`
  return `${fmtNumber(n / 1024 ** 2, 1)} MiB`
}

export function fmtDuration(seconds: number) {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return [d && t("format.days", { n: d }), (d || h) && t("format.hours", { n: h }), t("format.minutes", { n: m })].filter(Boolean).join(" ")
}

/**
 * Renders a server schedule description ("every 1h (aligned)" or
 * "cron 0 * * * *") in the current language; `aligned` reports whether it is
 * an interval aligned to the wall clock.
 */
export function parseSchedule(v: string) {
  const every = /^every (\S+)( \(aligned\))?$/.exec(v)
  if (every) return { text: t("format.every", { interval: fmtGoDuration(every[1]) }), aligned: !!every[2] }
  const cron = /^cron (.+)$/.exec(v)
  if (cron) return { text: t("format.cron", { expr: cron[1] }), aligned: false }
  return { text: v, aligned: false }
}

/** A schedule description, noting alignment to the wall clock. */
export function fmtSchedule(v: string) {
  const s = parseSchedule(v)
  return s.aligned ? t("format.aligned", { schedule: s.text }) : s.text
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
  if (total === 0) return t("format.seconds", { n: 0 })
  if (total < 1) return t("format.milliseconds", { n: fmtNumber(total * 1000) })
  const parts: string[] = []
  const d = Math.floor(total / 86400)
  const h = Math.floor((total % 86400) / 3600)
  const min = Math.floor((total % 3600) / 60)
  const sec = Math.round((total % 60) * 1000) / 1000
  if (d) parts.push(t("format.days", { n: d }))
  if (h) parts.push(t("format.hours", { n: h }))
  if (min) parts.push(t("format.minutes", { n: min }))
  if (sec) parts.push(t("format.seconds", { n: fmtNumber(sec, 3) }))
  return parts.join(" ")
}
