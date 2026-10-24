import { WithTip } from "@/components/hint"
import { useState, type ReactNode } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { PlusIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { DeviceDialog } from "@/components/device-form"
import { FormField } from "@/components/form-field"
import { ErrorBox } from "@/components/query-state"
import { useAction } from "@/hooks/use-action"
import { api, ApiError } from "@/lib/api"
import type { BusView, ScanJob } from "@/lib/types"

/** Probes a range of slave addresses and lists the encoders that answer. */
export function ScanDialog({ bus, buses, children, tip }: { tip?: string; bus: string; buses: BusView[]; children: ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [params, setParams] = useState({ from: 1, to: 247, timeout_ms: 300 })
  const qc = useQueryClient()
  const job = useQuery({
    queryKey: ["scan", bus],
    queryFn: () =>
      api.get<ScanJob>(`buses/${encodeURIComponent(bus)}/scan`).catch((e) => {
        if (e instanceof ApiError && e.status === 404) return null
        throw e
      }),
    enabled: open,
    refetchInterval: (q) => (q.state.data && !q.state.data.done ? 750 : false),
  })
  const start = useAction(() => api.post<ScanJob>(`buses/${encodeURIComponent(bus)}/scan`, params), {
    onSuccess: (j) => qc.setQueryData(["scan", bus], j),
  })
  const cancel = useAction(() => api.del(`buses/${encodeURIComponent(bus)}/scan`), { invalidate: [["scan", bus]] })
  const running = job.data && !job.data.done
  const estimate = Math.ceil(((params.to - params.from + 1) * params.timeout_ms) / 1000)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <WithTip tip={tip}>
        <DialogTrigger asChild>{children}</DialogTrigger>
      </WithTip>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("scan.title", { bus })}</DialogTitle>
          <DialogDescription>{t("scan.description")}</DialogDescription>
        </DialogHeader>
        <div className="grid grid-cols-3 gap-4">
          <FormField id="scan-from" label={t("scan.from")} hint={t("scan.fromHint")}>
            <Input id="scan-from" type="number" min={1} max={247} value={params.from} onChange={(e) => setParams({ ...params, from: Number(e.target.value) })} />
          </FormField>
          <FormField id="scan-to" label={t("scan.to")} hint={t("scan.toHint")}>
            <Input id="scan-to" type="number" min={1} max={247} value={params.to} onChange={(e) => setParams({ ...params, to: Number(e.target.value) })} />
          </FormField>
          <FormField id="scan-timeout" label={t("scan.timeout")} hint={t("scan.timeoutHint")}>
            <Input id="scan-timeout" type="number" min={20} max={5000} value={params.timeout_ms} onChange={(e) => setParams({ ...params, timeout_ms: Number(e.target.value) })} />
          </FormField>
        </div>
        {start.error && <ErrorBox error={start.error} />}
        {job.data && (
          <div className="grid gap-3">
            <div className="flex items-center justify-between text-sm">
              <span>
                {job.data.done ? (job.data.cancelled ? t("scan.cancelled") : t("scan.finished")) : t("scan.probing", { address: job.data.current })} · {t("scan.found", { count: job.data.found.length })}
              </span>
              <span className="tabular-nums">{Math.round(job.data.progress * 100)}%</span>
            </div>
            <div className="h-2 w-full rounded-sm border" role="progressbar" aria-valuenow={Math.round(job.data.progress * 100)} aria-valuemin={0} aria-valuemax={100}>
              <div className="h-full bg-foreground" style={{ width: `${job.data.progress * 100}%` }} />
            </div>
            {job.data.found.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("scan.address")}</TableHead>
                    <TableHead>{t("scan.type")}</TableHead>
                    <TableHead>{t("scan.firmware")}</TableHead>
                    <TableHead>{t("scan.configuredAs")}</TableHead>
                    <TableHead />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {job.data.found.map((f) => (
                    <TableRow key={f.slave}>
                      <TableCell className="font-semibold tabular-nums">{f.slave}</TableCell>
                      <TableCell>{f.device_type ?? "—"}</TableCell>
                      <TableCell>{f.firmware?.text ?? "—"}</TableCell>
                      <TableCell>{f.configured_as ?? "—"}</TableCell>
                      <TableCell className="text-right">
                        {!f.configured_as && (
                          <DeviceDialog buses={buses} preset={{ bus, slave: f.slave, id: `${bus}-${f.slave}` }}>
                            <Button size="sm" variant="outline">
                              <PlusIcon /> {t("common.add")}
                            </Button>
                          </DeviceDialog>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        )}
        <DialogFooter>
          {running ? (
            <Button variant="outline" onClick={() => cancel.mutate(undefined)}>
              {t("scan.cancel")}
            </Button>
          ) : (
            <Button onClick={() => start.mutate(undefined)} disabled={start.isPending || params.from > params.to}>
              {start.isPending && <Spinner />} {t("scan.start", { seconds: estimate })}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Assigns a slave address to every encoder on the bus (commissioning). */
export function BroadcastAddressDialog({ bus, children, tip }: { tip?: string; bus: string; children: ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [slave, setSlave] = useState(1)
  const [confirm, setConfirm] = useState(false)
  const send = useAction(() => api.post(`buses/${encodeURIComponent(bus)}/broadcast/slave-address`, { slave, confirm }), {
    success: t("broadcastAddress.success", { slave }),
    onSuccess: () => setOpen(false),
  })
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        setConfirm(false)
      }}
    >
      <WithTip tip={tip}>
        <DialogTrigger asChild>{children}</DialogTrigger>
      </WithTip>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("broadcastAddress.title")}</DialogTitle>
          <DialogDescription>{t("broadcastAddress.description", { bus })}</DialogDescription>
        </DialogHeader>
        <FormField id="bc-slave" label={t("broadcastAddress.newAddress")} hint={t("broadcastAddress.newAddressHint")}>
          <Input id="bc-slave" type="number" min={1} max={247} value={slave} onChange={(e) => setSlave(Number(e.target.value))} />
        </FormField>
        <div className="flex items-center gap-2">
          <Checkbox id="bc-confirm" checked={confirm} onCheckedChange={(v) => setConfirm(v === true)} />
          <Label htmlFor="bc-confirm" className="font-normal">
            {t("broadcastAddress.confirm")}
          </Label>
        </div>
        <DialogFooter>
          <Button onClick={() => send.mutate(undefined)} disabled={!confirm || send.isPending}>
            {send.isPending && <Spinner />} {t("broadcastAddress.send")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Sets the clock of every encoder on the bus to the server time. */
export function BroadcastClockDialog({ bus, children, tip }: { tip?: string; bus: string; children: ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [tz, setTz] = useState("")
  const send = useAction(() => api.post<{ time: string }>(`buses/${encodeURIComponent(bus)}/broadcast/clock`, tz ? { timezone: tz } : {}), {
    success: (r) => t("broadcastClock.success", { time: r.time }),
    onSuccess: () => setOpen(false),
  })
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <WithTip tip={tip}>
        <DialogTrigger asChild>{children}</DialogTrigger>
      </WithTip>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("broadcastClock.title")}</DialogTitle>
          <DialogDescription>{t("broadcastClock.description", { bus })}</DialogDescription>
        </DialogHeader>
        <FormField id="bc-tz" label={t("broadcastClock.timezone")} hint={t("broadcastClock.timezoneHint")}>
          <Input id="bc-tz" placeholder={t("broadcastClock.timezonePlaceholder")} value={tz} onChange={(e) => setTz(e.target.value)} />
        </FormField>
        <DialogFooter>
          <Button onClick={() => send.mutate(undefined)} disabled={send.isPending}>
            {send.isPending && <Spinner />} {t("broadcastClock.send")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
