import { useState, type FormEvent } from "react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { HintLabel, Hint } from "@/components/hint"
import { api, errorMessage, setApiKey } from "@/lib/api"

export function LoginPage() {
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
          <CardTitle className="text-xl font-bold">NWM-HRI Panel</CardTitle>
          <CardDescription>Sign in with the master API key configured on the server.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="grid gap-4">
            <div className="grid gap-2">
              <HintLabel htmlFor="key" hint="The value of the NWM_PANEL_API_KEY environment variable the server was started with. It is sent as a Bearer token with every request.">
                API key
              </HintLabel>
              <Input id="key" type="password" autoComplete="current-password" autoFocus value={key} onChange={(e) => setKey(e.target.value)} required />
            </div>
            <div className="flex items-center gap-2">
              <Checkbox id="remember" checked={remember} onCheckedChange={(v) => setRemember(v === true)} />
              <Label htmlFor="remember" className="font-normal">
                Remember on this device
              </Label>
              <Hint>Unchecked, the key is kept only until this tab is closed. Checked, it is stored in this browser's local storage until you sign out.</Hint>
            </div>
            {error && (
              <p role="alert" className="border border-foreground p-2 text-sm font-medium">
                {error}
              </p>
            )}
            <Button type="submit" disabled={busy || !key.trim()}>
              {busy && <Spinner />} Sign in
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
