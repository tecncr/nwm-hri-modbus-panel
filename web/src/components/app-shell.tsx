import { useState, type ReactNode } from "react"
import { LogOutIcon, MenuIcon, MoonIcon, SunIcon } from "lucide-react"
import { useTheme } from "next-themes"

import { Button } from "@/components/ui/button"
import { WithTip } from "@/components/hint"
import { setApiKey } from "@/lib/api"
import { href } from "@/hooks/use-route"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet"

const NAV = [
  { id: "overview", label: "Overview", tip: "Live readings and consumption of every meter." },
  { id: "devices", label: "Devices", tip: "Manage buses and encoders, scan for devices and work with registers." },
  { id: "measurements", label: "Measurements", tip: "Browse and download the recorded measurements database." },
  { id: "exports", label: "Exports", tip: "Periodic CSV files generated in the export directory." },
  { id: "system", label: "System", tip: "Server version, recorder status, storage and effective configuration." },
]

export function AppShell({ section, children }: { section: string; children: ReactNode }) {
  const { resolvedTheme, setTheme } = useTheme()
  const [menu, setMenu] = useState(false)
  const active = section === "meters" ? "overview" : section
  return (
    <div className="min-h-svh bg-muted/40">
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-2 px-4 md:gap-6">
          <Sheet open={menu} onOpenChange={setMenu}>
            <WithTip tip="Open the navigation menu.">
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon" className="md:hidden" aria-label="Open menu">
                  <MenuIcon />
                </Button>
              </SheetTrigger>
            </WithTip>
            <SheetContent side="left" className="w-64">
              <SheetHeader>
                <SheetTitle>NWM-HRI Panel</SheetTitle>
                <SheetDescription className="sr-only">Navigate between sections of the panel.</SheetDescription>
              </SheetHeader>
              <nav className="grid gap-1 px-4">
                {NAV.map((n) => (
                  <Button key={n.id} asChild variant={active === n.id ? "secondary" : "ghost"} className="justify-start" onClick={() => setMenu(false)}>
                    <a href={href(n.id)} aria-current={active === n.id ? "page" : undefined}>
                      {n.label}
                    </a>
                  </Button>
                ))}
              </nav>
            </SheetContent>
          </Sheet>
          <a href={href("overview")} className="flex-1 truncate text-sm font-semibold tracking-tight md:flex-none">
            NWM-HRI Panel
          </a>
          <nav className="hidden min-w-0 flex-1 items-center gap-1 md:flex">
            {NAV.map((n) => (
              <WithTip key={n.id} tip={n.tip}>
                <Button asChild variant={active === n.id ? "secondary" : "ghost"} size="sm">
                  <a href={href(n.id)} aria-current={active === n.id ? "page" : undefined}>
                    {n.label}
                  </a>
                </Button>
              </WithTip>
            ))}
          </nav>
          <div className="flex items-center gap-1">
            <WithTip tip="Switch between the light and dark theme.">
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
      <main className="mx-auto max-w-7xl px-4 py-4 sm:py-6">{children}</main>
    </div>
  )
}

/** Page title row with optional actions. */
export function PageHeader({ title, description, actions }: { title: ReactNode; description?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-4 flex flex-col gap-3 sm:mb-6 sm:flex-row sm:flex-wrap sm:items-end sm:justify-between sm:gap-4">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">{title}</h1>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}
