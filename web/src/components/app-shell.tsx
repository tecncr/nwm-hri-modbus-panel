import { useState, type ReactNode } from "react"
import { LogOutIcon, MenuIcon, MoonIcon, SunIcon } from "lucide-react"
import { useTheme } from "next-themes"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { WithTip } from "@/components/hint"
import { LanguageSwitcher } from "@/components/language-switcher"
import { setApiKey } from "@/lib/api"
import { href } from "@/hooks/use-route"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet"

const NAV = ["overview", "devices", "measurements", "exports", "system"] as const

export function AppShell({ section, children }: { section: string; children: ReactNode }) {
  const { t } = useTranslation()
  const { resolvedTheme, setTheme } = useTheme()
  const nav = NAV.map((id) => ({ id, label: t(`nav.${id}`), tip: t(`nav.${id}Tip`) }))
  const [menu, setMenu] = useState(false)
  const active = section === "meters" ? "overview" : section
  return (
    <div className="min-h-svh bg-muted/40">
      <header className="sticky top-0 z-40 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-2 px-4 md:gap-6">
          <Sheet open={menu} onOpenChange={setMenu}>
            <WithTip tip={t("nav.openMenuTip")}>
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon" className="md:hidden" aria-label={t("nav.openMenu")}>
                  <MenuIcon />
                </Button>
              </SheetTrigger>
            </WithTip>
            <SheetContent side="left" className="w-64">
              <SheetHeader>
                <SheetTitle>{t("common.appName")}</SheetTitle>
                <SheetDescription className="sr-only">{t("nav.menuDescription")}</SheetDescription>
              </SheetHeader>
              <nav className="grid gap-1 px-4">
                {nav.map((n) => (
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
            {t("common.appName")}
          </a>
          <nav className="hidden min-w-0 flex-1 items-center gap-1 md:flex">
            {nav.map((n) => (
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
            <LanguageSwitcher />
            <WithTip tip={t("nav.toggleThemeTip")}>
              <Button variant="ghost" size="icon" aria-label={t("nav.toggleTheme")} onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}>
                {resolvedTheme === "dark" ? <SunIcon /> : <MoonIcon />}
              </Button>
            </WithTip>
            <WithTip tip={t("nav.signOutTip")}>
              <Button variant="ghost" size="icon" aria-label={t("nav.signOut")} onClick={() => setApiKey(null)}>
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
