import type { ReactNode } from "react"
import { LogOutIcon, MoonIcon, SunIcon } from "lucide-react"
import { useTheme } from "next-themes"

import { Button } from "@/components/ui/button"
import { WithTip } from "@/components/hint"
import { setApiKey } from "@/lib/api"
import { href } from "@/hooks/use-route"
import { cn } from "@/lib/utils"

const NAV = [
  { id: "overview", label: "Overview", tip: "Live readings and consumption of every meter." },
  { id: "devices", label: "Devices", tip: "Manage buses and encoders, scan for devices and work with registers." },
  { id: "measurements", label: "Measurements", tip: "Browse and download the recorded measurements database." },
  { id: "exports", label: "Exports", tip: "Periodic CSV files generated in the export directory." },
  { id: "system", label: "System", tip: "Server version, recorder status, storage and effective configuration." },
]

export function AppShell({ section, children }: { section: string; children: ReactNode }) {
  const { resolvedTheme, setTheme } = useTheme()
  const active = section === "meters" ? "overview" : section
  return (
    <div className="min-h-svh">
      <header className="sticky top-0 z-40 border-b bg-background">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-6 px-4">
          <a href={href("overview")} className="text-sm font-bold tracking-tight whitespace-nowrap">
            NWM-HRI Panel
          </a>
          <nav className="-mb-px flex h-full min-w-0 flex-1 items-stretch gap-1 overflow-x-auto">
            {NAV.map((n) => (
              <WithTip key={n.id} tip={n.tip}>
                <a
                  href={href(n.id)}
                  className={cn(
                    "flex items-center border-b-2 px-2 text-sm whitespace-nowrap",
                    active === n.id ? "border-foreground font-semibold" : "border-transparent hover:underline",
                  )}
                  aria-current={active === n.id ? "page" : undefined}
                >
                  {n.label}
                </a>
              </WithTip>
            ))}
          </nav>
          <div className="flex items-center gap-1">
            <WithTip tip="Switch between the light (white) and dark (black) theme.">
              <Button variant="ghost" size="icon" aria-label="Toggle theme" onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}>
                {resolvedTheme === "dark" ? <SunIcon /> : <MoonIcon />}
              </Button>
            </WithTip>
            <WithTip tip="Forget the API key in this browser and return to the sign-in screen.">
              <Button variant="ghost" size="icon" aria-label="Sign out" onClick={() => setApiKey(null)}>
                <LogOutIcon />
              </Button>
            </WithTip>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-4 py-6">{children}</main>
    </div>
  )
}

/** Page title row with optional actions. */
export function PageHeader({ title, description, actions }: { title: ReactNode; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        {description && <p className="mt-1 text-sm">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}
