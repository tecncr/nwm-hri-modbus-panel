import { useSyncExternalStore } from "react"

// Hash-based routing keeps deep links working behind any path prefix without
// server-side rewrites: #/meters/building-a -> ["meters", "building-a"].

function subscribe(fn: () => void) {
  window.addEventListener("hashchange", fn)
  return () => window.removeEventListener("hashchange", fn)
}

const snapshot = () => window.location.hash

export function useRoute(): string[] {
  const hash = useSyncExternalStore(subscribe, snapshot)
  return hash.replace(/^#\/?/, "").split("?")[0].split("/").filter(Boolean).map(decodeURIComponent)
}

export function navigate(...parts: string[]) {
  window.location.hash = "/" + parts.map(encodeURIComponent).join("/")
}

export function href(...parts: string[]) {
  return "#/" + parts.map(encodeURIComponent).join("/")
}
