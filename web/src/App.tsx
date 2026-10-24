import { useEffect, useSyncExternalStore } from "react"
import { useQueryClient } from "@tanstack/react-query"

import { AppShell } from "@/components/app-shell"
import { getApiKey, onApiKeyChange } from "@/lib/api"
import { useRoute } from "@/hooks/use-route"
import { LoginPage } from "@/pages/login"
import { OverviewPage } from "@/pages/overview"
import { MeterPage } from "@/pages/meter"
import { DevicesPage } from "@/pages/devices"
import { DevicePage } from "@/pages/device"
import { MeasurementsPage } from "@/pages/measurements"
import { ExportsPage } from "@/pages/exports"
import { SystemPage } from "@/pages/system"
import { NotFound } from "@/pages/not-found"

function useApiKey() {
  return useSyncExternalStore(onApiKeyChange, getApiKey)
}

export default function App() {
  const key = useApiKey()
  const qc = useQueryClient()
  const route = useRoute()

  // Drop cached data from the previous session when signing out.
  useEffect(() => {
    if (!key) qc.clear()
  }, [key, qc])

  if (!key) return <LoginPage />

  const [section, id] = route
  let page
  switch (section ?? "overview") {
    case "overview":
      page = <OverviewPage />
      break
    case "meters":
      page = id ? <MeterPage id={id} /> : <OverviewPage />
      break
    case "devices":
      page = id ? <DevicePage id={id} /> : <DevicesPage />
      break
    case "measurements":
      page = <MeasurementsPage />
      break
    case "exports":
      page = <ExportsPage />
      break
    case "system":
      page = <SystemPage />
      break
    default:
      page = <NotFound />
  }
  return <AppShell section={section ?? "overview"}>{page}</AppShell>
}
