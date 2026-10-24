import { useState, type FormEvent, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { GaugeIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { PageHeader } from "@/components/app-shell"
import { Confirm } from "@/components/confirm"
import { FormField } from "@/components/form-field"
import { Hint, WithTip } from "@/components/hint"
import { ErrorBox, Loading, QueryState } from "@/components/query-state"
import { OnlineBadge } from "@/components/status"
import { useAction } from "@/hooks/use-action"
import { href } from "@/hooks/use-route"
import { api } from "@/lib/api"
import { fmtDateTime, fmtNumber, toLocalInput } from "@/lib/format"
import type { DeviceView, RegisterDef, RegisterMap, RegisterValue, Snapshot, SnapshotResponse, SystemInfo } from "@/lib/types"
import i18n from "@/i18n"
import type en from "@/i18n/en"

const hex = (n: number, w = 4) => "0x" + n.toString(16).toUpperCase().padStart(w, "0")

/** Title and description of a register in the current language (the server's English text for unknown keys). */
function registerText(t: TFunction, r: RegisterDef) {
  if (!i18n.exists(`registers.${r.key}`)) return r
  const key = r.key as keyof typeof en.registers
  return { title: t(`registers.${key}.title`), description: t(`registers.${key}.description`) }
}

function decoded(t: TFunction, key: string, s: Snapshot): string {
  switch (key) {
    case "slave_address":
      return String(s.slave_address)
    case "total_flow":
      return t("device.units", { value: fmtNumber(s.counters.total) })
    case "forward_flow":
      return t("device.units", { value: fmtNumber(s.counters.forward) })
    case "reverse_flow":
      return t("device.units", { value: fmtNumber(s.counters.reverse) })
    case "clock_year_month":
      return `${s.clock.year}-${String(s.clock.month).padStart(2, "0")}`
    case "clock_day_hour":
      return t("device.clockDayHour", { day: s.clock.day, hour: s.clock.hour })
    case "clock_minute_second":
      return t("device.clockMinuteSecond", { minute: s.clock.minute, second: s.clock.second })
    case "device_type":
      return String(s.device_type)
    case "firmware_year":
      return String(s.firmware.year)
    case "firmware_month_day":
      return `${s.firmware.month}/${s.firmware.day}`
    case "reverse_flow_threshold":
      return String(s.reverse_flow_threshold)
    case "battery_status":
      return String(s.battery_status)
    case "failure":
      return s.failure === 0 ? t("device.failureNone") : String(s.failure)
    case "base_pulse_equivalent":
      return t("device.litersPerPulse", { value: s.base_pulse_liters })
    case "output_pulse_equivalent":
      return t("device.litersPerUnit", { value: s.output_pulse_liters })
    case "forward_base_reading":
      return t("device.units", { value: fmtNumber(s.forward_base_reading) })
    case "reverse_base_reading":
      return t("device.units", { value: fmtNumber(s.reverse_base_reading) })
    case "debug_numbers":
      return s.debug_numbers?.join(", ") ?? s.debug_error ?? "—"
  }
  return ""
}

export function DevicePage({ id }: { id: string }) {
  const { t } = useTranslation()
  const base = `devices/${encodeURIComponent(id)}`
  const device = useQuery({ queryKey: ["device", id], queryFn: () => api.get<DeviceView>(base) })
  const map = useQuery({ queryKey: ["register-map"], queryFn: () => api.get<RegisterMap>("registers"), staleTime: Infinity })
  const snap = useQuery({ queryKey: ["snapshot", id], queryFn: () => api.get<SnapshotResponse>(`${base}/snapshot`), staleTime: 0 })
  const system = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system") })
  const invalidate = [["snapshot", id], ["device", id], ["meter", id], ["meters"], ["devices"]]
  const v = device.data
  const s = snap.data?.snapshot
  const rawWrites = system.data?.modbus.allow_raw_writes ?? false

  return (
    <>
      <PageHeader
        title={v?.config.name || id}
        description={
          <span className="inline-flex flex-wrap items-center gap-2">
            {v && <OnlineBadge online={v.state.online} enabled={v.enabled} lastError={v.state.last_error} lastOk={v.state.last_ok_at} />}
            <span>
              {t("device.subtitle", { id })}
              {v ? t("device.subtitleBus", { bus: v.config.bus, slave: v.config.slave }) : ""}
            </span>
          </span>
        }
        actions={
          <>
            <WithTip tip={t("device.refreshTip")}>
              <Button variant="outline" onClick={() => snap.refetch()} disabled={snap.isFetching}>
                {snap.isFetching ? <Spinner /> : <RefreshCwIcon />} {t("device.refresh")}
              </Button>
            </WithTip>
            <WithTip tip={t("device.meterTip")}>
              <Button asChild variant="outline">
                <a href={href("meters", id)}>
                  <GaugeIcon /> {t("device.meter")}
                </a>
              </Button>
            </WithTip>
          </>
        }
      />

      {device.error && <ErrorBox error={device.error} />}
      {snap.error && (
        <div className="mb-6">
          <ErrorBox error={snap.error} title={t("device.readFailed")} />
        </div>
      )}

      <Tabs defaultValue="config">
        <TabsList className="mb-4">
          <TabsTrigger value="config">{t("device.tabConfig")}</TabsTrigger>
          <TabsTrigger value="map">{t("device.tabMap")}</TabsTrigger>
          <TabsTrigger value="raw">{t("device.tabRaw")}</TabsTrigger>
        </TabsList>

        <TabsContent value="config">
          {snap.isPending ? (
            <Loading className="h-96" />
          ) : (
            s &&
            v && (
              <div className="grid gap-6 lg:grid-cols-2">
                <SlaveCard key={s.read_at} base={base} current={v.config.slave} invalidate={invalidate} />
                <ClockCard key={s.read_at} base={base} snap={snap.data!} tz={v.timezone} invalidate={invalidate} />
                <PulseCard key={s.read_at} base={base} s={s} expected={v.config.meter} invalidate={invalidate} />
                <BaseReadingsCard key={s.read_at} base={base} s={s} invalidate={invalidate} />
                <ValueCard
                  key={`${s.read_at}-type`}
                  base={base}
                  path="device-type"
                  title={t("device.deviceType")}
                  hint={t("device.deviceTypeHint")}
                  current={s.device_type}
                  invalidate={invalidate}
                />
                <ValueCard
                  key={`${s.read_at}-threshold`}
                  base={base}
                  path="reverse-flow-threshold"
                  title={t("device.threshold")}
                  hint={t("device.thresholdHint")}
                  current={s.reverse_flow_threshold}
                  invalidate={invalidate}
                />
                <Card className="lg:col-span-2">
                  <CardHeader>
                    <CardTitle className="flex items-center gap-1.5">
                      {t("device.diagnostics")} <Hint>{t("device.diagnosticsHint")}</Hint>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
                    <Field label={t("device.battery")} hint={t("device.batteryHint")} value={s.battery_status} />
                    <Field label={t("device.failure")} hint={t("device.failureHint")} value={s.failure === 0 ? t("device.failureValueNone") : s.failure} />
                    <Field label={t("device.firmware")} hint={t("device.firmwareHint")} value={s.firmware.text} />
                    <Field label={t("device.debug")} hint={t("device.debugHint")} value={s.debug_numbers?.join(" ") ?? t("device.unavailable")} />
                  </CardContent>
                </Card>
              </div>
            )
          )}
        </TabsContent>

        <TabsContent value="map">
          <Card>
            <CardHeader>
              <CardTitle>{t("device.mapTitle")}</CardTitle>
              <CardDescription>{s ? t("device.mapRead", { time: fmtDateTime(s.read_at) }) : t("device.mapAll")} {t("device.mapWords")}</CardDescription>
            </CardHeader>
            <CardContent>
              <QueryState query={map}>
                {(m) => (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("device.address")}</TableHead>
                        <TableHead>{t("device.register")}</TableHead>
                        <TableHead>{t("device.access")}</TableHead>
                        <TableHead>{t("device.raw")}</TableHead>
                        <TableHead>{t("device.value")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {m.registers.map((r) => {
                        const raws = s?.raw.filter((x: RegisterValue) => x.key === r.key) ?? []
                        const text = registerText(t, r)
                        return (
                          <TableRow key={r.key}>
                            <TableCell className="font-mono text-xs whitespace-nowrap">
                              {hex(r.address)}
                              {r.words > 1 ? `–${hex(r.address + r.words - 1)}` : ""}
                            </TableCell>
                            <TableCell>
                              <span className="inline-flex items-center gap-1.5 font-medium">
                                {text.title} <Hint>{text.description}</Hint>
                              </span>
                            </TableCell>
                            <TableCell>
                              <Badge variant={r.access === "rw" ? "default" : "outline"}>{r.access === "rw" ? t("device.readWrite") : t("device.readOnly")}</Badge>
                            </TableCell>
                            <TableCell className="font-mono text-xs">{raws.map((x) => x.hex).join(" ") || "—"}</TableCell>
                            <TableCell className="tabular-nums">{s ? decoded(t, r.key, s) : "—"}</TableCell>
                          </TableRow>
                        )
                      })}
                    </TableBody>
                  </Table>
                )}
              </QueryState>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="raw">
          <RawAccess base={base} allowWrites={rawWrites} invalidate={invalidate} />
        </TabsContent>
      </Tabs>
    </>
  )
}

function Field({ label, hint, value }: { label: string; hint: string; value: ReactNode }) {
  return (
    <div>
      <div className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
        {label} <Hint>{hint}</Hint>
      </div>
      <div className="mt-1 font-mono">{value}</div>
    </div>
  )
}

function CardShell({ title, hint, description, children }: { title: string; hint: string; description?: ReactNode; children: ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-1.5">
          {title} <Hint>{hint}</Hint>
        </CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}

function SlaveCard({ base, current, invalidate }: { base: string; current: number; invalidate: unknown[][] }) {
  const { t } = useTranslation()
  const [slave, setSlave] = useState(current)
  const save = useAction(() => api.put(`${base}/slave-address`, { slave }), { success: t("device.slaveChanged", { slave }), invalidate })
  return (
    <CardShell title={t("device.slaveTitle")} hint={t("device.slaveHint")} description={t("device.currently", { value: current })}>
      <div className="flex items-end gap-2">
        <FormField id="slave" label={t("device.newAddress")} hint={t("device.newAddressHint")} className="flex-1">
          <Input id="slave" type="number" min={1} max={247} value={slave} onChange={(e) => setSlave(Number(e.target.value))} />
        </FormField>
        <Confirm title={t("device.slaveConfirmTitle", { slave })} description={t("device.slaveConfirmDescription")} onConfirm={() => save.mutate(undefined)} tip={t("device.slaveTip")}>
          <Button disabled={slave === current || slave < 1 || slave > 247 || save.isPending}>{save.isPending && <Spinner />} {t("device.change")}</Button>
        </Confirm>
      </div>
    </CardShell>
  )
}

function ClockCard({ base, snap, tz, invalidate }: { base: string; snap: SnapshotResponse; tz: string; invalidate: unknown[][] }) {
  const { t } = useTranslation()
  const [custom, setCustom] = useState(() => toLocalInput(new Date()))
  const sync = useAction((time?: string) => api.put<{ time: string }>(`${base}/clock`, time ? { time } : {}), { success: (r) => t("device.clockSet", { time: r.time }), invalidate })
  const drift = snap.clock_drift_seconds
  return (
    <CardShell
      title={t("device.clockTitle")}
      hint={t("device.clockHint")}
      description={
        snap.snapshot.clock.valid ? (
          <>
            {fmtDateTime(snap.clock_time)} ({tz}) · {t("device.drift", { value: drift !== undefined ? t("device.driftSeconds", { value: `${drift > 0 ? "+" : ""}${fmtNumber(drift, 0)}` }) : "—" })}
          </>
        ) : (
          t("device.clockInvalid")
        )
      }
    >
      <div className="grid gap-3">
        <WithTip tip={t("device.syncServerTip")}>
          <Button variant="outline" onClick={() => sync.mutate(undefined)} disabled={sync.isPending}>
            {sync.isPending && <Spinner />} {t("device.syncServer")}
          </Button>
        </WithTip>
        <div className="flex items-end gap-2">
          <FormField id="clock-custom" label={t("device.setTime")} hint={t("device.setTimeHint")} className="flex-1">
            <Input id="clock-custom" type="datetime-local" step={1} value={custom} onChange={(e) => setCustom(e.target.value)} />
          </FormField>
          <WithTip tip={t("device.setTimeTip")}>
            <Button onClick={() => sync.mutate(new Date(custom).toISOString().replace(/\.\d+Z$/, "Z"))} disabled={sync.isPending || !custom}>
              {t("common.set")}
            </Button>
          </WithTip>
        </div>
      </div>
    </CardShell>
  )
}

function PulseCard({ base, s, expected, invalidate }: { base: string; s: Snapshot; expected: { base_liters_per_pulse: number; output_liters_per_unit: number }; invalidate: unknown[][] }) {
  const { t } = useTranslation()
  const [vals, setVals] = useState({ base: s.base_pulse_liters, output: s.output_pulse_liters })
  const save = useAction(() => api.put(`${base}/pulse-equivalents`, { base_liters_per_pulse: vals.base, output_liters_per_unit: vals.output }), { success: t("device.pulseWritten"), invalidate })
  const apply = useAction(() => api.post(`${base}/pulse-equivalents/apply`), { success: t("device.pulseApplied"), invalidate })
  const invalid = vals.base < 1 || vals.base > 10000 || vals.output < 1 || vals.output > 1000 || vals.output < vals.base
  const hasExpected = expected.base_liters_per_pulse > 0
  const mismatch = hasExpected && (expected.base_liters_per_pulse !== s.base_pulse_liters || expected.output_liters_per_unit !== s.output_pulse_liters)
  return (
    <CardShell
      title={t("device.pulseTitle")}
      hint={t("device.pulseHint")}
      description={t("device.pulseDescription", { base: s.base_pulse_liters, output: s.output_pulse_liters }) + (hasExpected ? t("device.pulseExpected", { base: expected.base_liters_per_pulse, output: expected.output_liters_per_unit }) : "")}
    >
      <form
        className="grid gap-3"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          save.mutate(undefined)
        }}
      >
        <div className="grid grid-cols-2 gap-3">
          <FormField id="pulse-base" label={t("device.pulseBase")} hint={t("device.pulseBaseHint")}>
            <Input id="pulse-base" type="number" min={1} max={10000} value={vals.base} onChange={(e) => setVals({ ...vals, base: Number(e.target.value) })} />
          </FormField>
          <FormField id="pulse-out" label={t("device.pulseOutput")} hint={t("device.pulseOutputHint")}>
            <Input id="pulse-out" type="number" min={1} max={1000} value={vals.output} onChange={(e) => setVals({ ...vals, output: Number(e.target.value) })} />
          </FormField>
        </div>
        {invalid && <p className="text-sm font-medium">{t("device.pulseInvalid")}</p>}
        {mismatch && (
          <Alert>
            <AlertTitle>{t("device.pulseMismatch")}</AlertTitle>
            <AlertDescription>{t("device.pulseMismatchDescription")}</AlertDescription>
          </Alert>
        )}
        <div className="flex flex-wrap gap-2">
          <WithTip tip={t("device.pulseWriteTip")}>
            <Button type="submit" disabled={invalid || save.isPending}>
              {save.isPending && <Spinner />} {t("common.write")}
            </Button>
          </WithTip>
          {hasExpected && (
            <WithTip tip={t("device.pulseApplyTip")}>
              <Button type="button" variant="outline" onClick={() => apply.mutate(undefined)} disabled={apply.isPending}>
                {t("device.pulseApply")}
              </Button>
            </WithTip>
          )}
        </div>
      </form>
    </CardShell>
  )
}

function BaseReadingsCard({ base, s, invalidate }: { base: string; s: Snapshot; invalidate: unknown[][] }) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<"liters" | "raw">("liters")
  const [fwd, setFwd] = useState("")
  const [rev, setRev] = useState("")
  const body = () => {
    const b: Record<string, number> = {}
    if (fwd !== "") b[mode === "raw" ? "forward" : "forward_liters"] = Number(fwd)
    if (rev !== "") b[mode === "raw" ? "reverse" : "reverse_liters"] = Number(rev)
    return b
  }
  const save = useAction(() => api.put(`${base}/base-readings`, body()), {
    success: t("device.basePreset"),
    invalidate,
    onSuccess: () => {
      setFwd("")
      setRev("")
    },
  })
  return (
    <CardShell
      title={t("device.baseTitle")}
      hint={t("device.baseHint")}
      description={t("device.baseDescription", { forward: fmtNumber(s.forward_base_reading), reverse: fmtNumber(s.reverse_base_reading) })}
    >
      <div className="grid gap-3">
        <div className="flex items-center gap-2">
          <ToggleGroup type="single" variant="outline" size="sm" value={mode} onValueChange={(v) => v && setMode(v as "liters" | "raw")}>
            <WithTip tip={t("device.litersTip")}>
              <ToggleGroupItem value="liters">{t("device.liters")}</ToggleGroupItem>
            </WithTip>
            <WithTip tip={t("device.rawUnitsTip")}>
              <ToggleGroupItem value="raw">{t("device.rawUnits")}</ToggleGroupItem>
            </WithTip>
          </ToggleGroup>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <FormField id="base-fwd" label={mode === "raw" ? t("device.forwardUnits") : t("device.forwardLiters")} hint={t("device.forwardHint")}>
            <Input id="base-fwd" type="number" min={0} step="any" value={fwd} onChange={(e) => setFwd(e.target.value)} />
          </FormField>
          <FormField id="base-rev" label={mode === "raw" ? t("device.reverseUnits") : t("device.reverseLiters")} hint={t("device.reverseHint")}>
            <Input id="base-rev" type="number" min={0} step="any" value={rev} onChange={(e) => setRev(e.target.value)} />
          </FormField>
        </div>
        <Confirm title={t("device.baseConfirmTitle")} description={t("device.baseConfirmDescription")} onConfirm={() => save.mutate(undefined)} tip={t("device.baseTip")}>
          <Button disabled={(fwd === "" && rev === "") || save.isPending} className="justify-self-start">
            {save.isPending && <Spinner />} {t("common.write")}
          </Button>
        </Confirm>
      </div>
    </CardShell>
  )
}

function ValueCard({ base, path, title, hint, current, invalidate }: { base: string; path: string; title: string; hint: string; current: number; invalidate: unknown[][] }) {
  const { t } = useTranslation()
  const [value, setValue] = useState(current)
  const save = useAction(() => api.put(`${base}/${path}`, { value }), { success: t("device.valueWritten", { title }), invalidate })
  return (
    <CardShell title={title} hint={hint} description={t("device.currently", { value: current })}>
      <div className="flex items-end gap-2">
        <FormField id={path} label={t("device.newValue")} hint={t("device.newValueHint")} className="flex-1">
          <Input id={path} type="number" min={0} max={65535} value={value} onChange={(e) => setValue(Number(e.target.value))} />
        </FormField>
        <WithTip tip={t("device.valueTip")}>
          <Button onClick={() => save.mutate(undefined)} disabled={value === current || value < 0 || value > 65535 || save.isPending}>
            {save.isPending && <Spinner />} {t("common.write")}
          </Button>
        </WithTip>
      </div>
    </CardShell>
  )
}

function parseNum(s: string) {
  const t = s.trim()
  return t.startsWith("0x") || t.startsWith("0X") ? parseInt(t, 16) : Number(t)
}

function RawAccess({ base, allowWrites, invalidate }: { base: string; allowWrites: boolean; invalidate: unknown[][] }) {
  const { t } = useTranslation()
  const [read, setRead] = useState({ address: "0x0000", count: "22" })
  const [result, setResult] = useState<RegisterValue[] | null>(null)
  const [write, setWrite] = useState({ address: "0x000D", values: "" })
  const doRead = useAction(() => api.get<{ registers: RegisterValue[] }>(`${base}/registers`, { address: parseNum(read.address), count: parseNum(read.count) }), { onSuccess: (r) => setResult(r.registers) })
  const values = write.values.split(/[\s,]+/).filter(Boolean).map(parseNum)
  const doWrite = useAction(
    () =>
      values.length === 1
        ? api.put(`${base}/registers/${parseNum(write.address)}`, { value: values[0] })
        : api.post(`${base}/registers`, { address: parseNum(write.address), values }),
    { success: values.length === 1 ? t("device.writtenSingle") : t("device.writtenMultiple", { count: values.length }), invalidate },
  )
  const badWrite = values.length === 0 || values.some((v) => !Number.isInteger(v) || v < 0 || v > 65535) || !Number.isInteger(parseNum(write.address))

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <CardShell title={t("device.readTitle")} hint={t("device.readHint")} description={t("device.readDescription")}>
        <div className="grid gap-3">
          <div className="flex items-end gap-2">
            <FormField id="raw-addr" label={t("device.startAddress")} hint={t("device.startAddressHint")} className="flex-1">
              <Input id="raw-addr" value={read.address} onChange={(e) => setRead({ ...read, address: e.target.value })} />
            </FormField>
            <FormField id="raw-count" label={t("device.count")} hint={t("device.countHint")} className="w-24">
              <Input id="raw-count" value={read.count} onChange={(e) => setRead({ ...read, count: e.target.value })} />
            </FormField>
            <WithTip tip={t("device.readTip")}>
              <Button onClick={() => doRead.mutate(undefined)} disabled={doRead.isPending}>
                {doRead.isPending && <Spinner />} {t("common.read")}
              </Button>
            </WithTip>
          </div>
          {result && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("device.address")}</TableHead>
                  <TableHead>{t("device.hex")}</TableHead>
                  <TableHead className="text-right">{t("device.decimal")}</TableHead>
                  <TableHead>{t("device.register")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.map((r) => (
                  <TableRow key={r.address}>
                    <TableCell className="font-mono text-xs">{hex(r.address)}</TableCell>
                    <TableCell className="font-mono text-xs">{r.hex}</TableCell>
                    <TableCell className="text-right tabular-nums">{r.value}</TableCell>
                    <TableCell className="text-xs">{r.key || "—"}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </CardShell>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-1.5">
            {t("device.writeTitle")} <Hint>{t("device.writeHint")}</Hint>
          </CardTitle>
          <CardDescription>{allowWrites ? t("device.writeAllowed") : t("device.writeDisabled")}</CardDescription>
          <CardAction>
            <Badge variant="outline">{t("device.advanced")}</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="grid grid-cols-3 gap-2">
            <FormField id="w-addr" label={t("device.address")} hint={t("device.writeAddressHint")}>
              <Input id="w-addr" value={write.address} disabled={!allowWrites} onChange={(e) => setWrite({ ...write, address: e.target.value })} />
            </FormField>
            <FormField id="w-vals" label={t("device.values")} hint={t("device.valuesHint")} className="col-span-2">
              <Input id="w-vals" placeholder={t("device.valuesPlaceholder")} value={write.values} disabled={!allowWrites} onChange={(e) => setWrite({ ...write, values: e.target.value })} />
            </FormField>
          </div>
          <Confirm title={t("device.writeConfirmTitle")} description={t("device.writeConfirmDescription", { count: values.length, address: write.address })} onConfirm={() => doWrite.mutate(undefined)} tip={t("device.writeTip")}>
            <Button disabled={!allowWrites || badWrite || doWrite.isPending} className="justify-self-start">
              {doWrite.isPending && <Spinner />} {t("common.write")}
            </Button>
          </Confirm>
        </CardContent>
      </Card>
    </div>
  )
}
