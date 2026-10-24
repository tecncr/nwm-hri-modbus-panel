import { useState, type FormEvent } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { HintLabel, Hint } from "@/components/hint"
import { LanguageSwitcher } from "@/components/language-switcher"
import { api, errorMessage, setApiKey } from "@/lib/api"

export function LoginPage() {
  const { t } = useTranslation()
  const [key, setKey] = useState("")
  const [remember, setRemember] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await api.check(key.trim())
      setApiKey(key.trim(), remember)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-xl font-bold">{t("common.appName")}</CardTitle>
          <CardDescription>{t("login.description")}</CardDescription>
          <CardAction>
            <LanguageSwitcher />
          </CardAction>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="grid gap-4">
            <div className="grid gap-2">
              <HintLabel htmlFor="key" hint={t("login.apiKeyHint")}>
                {t("login.apiKey")}
              </HintLabel>
              <Input id="key" type="password" autoComplete="current-password" autoFocus value={key} onChange={(e) => setKey(e.target.value)} required />
            </div>
            <div className="flex items-center gap-2">
              <Checkbox id="remember" checked={remember} onCheckedChange={(v) => setRemember(v === true)} />
              <Label htmlFor="remember" className="font-normal">
                {t("login.remember")}
              </Label>
              <Hint>{t("login.rememberHint")}</Hint>
            </div>
            {error && (
              <p role="alert" className="rounded-md border bg-muted p-2 text-sm">
                {error}
              </p>
            )}
            <Button type="submit" disabled={busy || !key.trim()}>
              {busy && <Spinner />} {t("login.signIn")}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
