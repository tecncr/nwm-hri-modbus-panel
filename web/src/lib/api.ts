// Thin fetch wrapper for the nwm-panel API. URLs are resolved relative to the
// document, so the dashboard works under any reverse-proxy path prefix.

const STORAGE_KEY = "nwm-panel.api-key"

export class ApiError extends Error {
  status: number
  code: string
  details?: unknown

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message)
    this.status = status
    this.code = code
    this.details = details
  }
}

let apiKey: string | null = readStoredKey()
const listeners = new Set<() => void>()

function readStoredKey(): string | null {
  try {
    return sessionStorage.getItem(STORAGE_KEY) ?? localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

export function getApiKey() {
  return apiKey
}

/** Stores the key for this tab, or across sessions when remember is set. */
export function setApiKey(key: string | null, remember = false) {
  apiKey = key
  try {
    sessionStorage.removeItem(STORAGE_KEY)
    localStorage.removeItem(STORAGE_KEY)
    if (key) (remember ? localStorage : sessionStorage).setItem(STORAGE_KEY, key)
  } catch {
    // Storage unavailable (private mode): keep the key in memory only.
  }
  listeners.forEach((l) => l())
}

export function onApiKeyChange(fn: () => void) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

export function apiUrl(path: string, params?: Record<string, string | number | boolean | undefined | null>) {
  const url = new URL("api/v1/" + path.replace(/^\//, ""), document.baseURI)
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v !== undefined && v !== null && v !== "") url.searchParams.set(k, String(v))
  }
  return url.toString()
}

type Params = Record<string, string | number | boolean | undefined | null>

async function request<T>(method: string, path: string, opts: { params?: Params; body?: unknown; key?: string } = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" }
  const key = opts.key ?? apiKey
  if (key) headers.Authorization = `Bearer ${key}`
  if (opts.body !== undefined) headers["Content-Type"] = "application/json"
  const res = await fetch(apiUrl(path, opts.params), {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    credentials: "same-origin",
  })
  if (!res.ok) {
    let code = "error"
    let message = res.statusText
    let details: unknown
    try {
      const body = await res.json()
      code = body?.error?.code ?? code
      message = body?.error?.message ?? message
      details = body?.error?.details
    } catch {
      // Non-JSON error body.
    }
    if (res.status === 401 && !opts.key) setApiKey(null)
    throw new ApiError(res.status, code, message, details)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T>(path: string, params?: Params) => request<T>("GET", path, { params }),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, { body }),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, { body }),
  del: <T>(path: string) => request<T>("DELETE", path),
  /** Validates a key without storing it. */
  check: (key: string) => request<{ ok: boolean }>("GET", "auth", { key }),
}

/** Downloads an authenticated file (the key cannot go in a plain link). */
export async function download(path: string, params?: Params, fallbackName = "download.csv") {
  const res = await fetch(apiUrl(path, params), { headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {} })
  if (!res.ok) {
    let message = res.statusText
    try {
      message = (await res.json())?.error?.message ?? message
    } catch {
      // ignore
    }
    throw new ApiError(res.status, "download_failed", message)
  }
  const disposition = res.headers.get("Content-Disposition") ?? ""
  const name = /filename="?([^";]+)"?/.exec(disposition)?.[1] ?? fallbackName
  const blob = await res.blob()
  const href = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = href
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(href), 1000)
}

export function errorMessage(err: unknown) {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return String(err)
}
