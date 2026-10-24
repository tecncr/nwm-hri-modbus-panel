import { useState, type FormEvent, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { GaugeIcon, RefreshCwIcon } from "lucide-react"

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
import type { DeviceView, RegisterMap, RegisterValue, Snapshot, SnapshotResponse, SystemInfo } from "@/lib/types"

const hex = (n: number, w = 4) => "0x" + n.toString(16).toUpperCase().padStart(w, "0")

function decoded(key: string, s: Snapshot): string {
  switch (key) {
    case "slave_address":
      return String(s.slave_address)
    case "total_flow":
      return `${fmtNumber(s.counters.total)} units`
    case "forward_flow":
      return `${fmtNumber(s.counters.forward)} units`
    case "reverse_flow":
      return `${fmtNumber(s.counters.reverse)} units`
    case "clock_year_month":
      return `${s.clock.year}-${String(s.clock.month).padStart(2, "0")}`
    case "clock_day_hour":
      return `day ${s.clock.day}, ${s.clock.hour} h`
    case "clock_minute_second":
      return `${s.clock.minute} min ${s.clock.second} s`
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
      return s.failure === 0 ? "0 (none)" : String(s.failure)
    case "base_pulse_equivalent":
      return `${s.base_pulse_liters} L/pulse`
    case "output_pulse_equivalent":
      return `${s.output_pulse_liters} L/unit`
    case "forward_base_reading":
      return `${fmtNumber(s.forward_base_reading)} units`
    case "reverse_base_reading":
      return `${fmtNumber(s.reverse_base_reading)} units`
    case "debug_numbers":
      return s.debug_numbers?.join(", ") ?? s.debug_error ?? "—"
  }
  return ""
}

export function DevicePage({ id }: { id: string }) {
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
              Registers of {id}
              {v ? ` · bus ${v.config.bus} · slave ${v.config.slave}` : ""}
            </span>
          </span>
        }
        actions={
          <>
            <WithTip tip="Read every register from the encoder again.">
              <Button variant="outline" onClick={() => snap.refetch()} disabled={snap.isFetching}>
                {snap.isFetching ? <Spinner /> : <RefreshCwIcon />} Refresh
              </Button>
            </WithTip>
            <WithTip tip="Back to the measurement view of this meter.">
              <Button asChild variant="outline">
                <a href={href("meters", id)}>
                  <GaugeIcon /> Meter
                </a>
              </Button>
            </WithTip>
          </>
        }
      />

      {device.error && <ErrorBox error={device.error} />}
      {snap.error && (
        <div className="mb-6">
          <ErrorBox error={snap.error} title="The encoder could not be read" />
        </div>
      )}

      <Tabs defaultValue="config">
        <TabsList className="mb-4">
          <TabsTrigger value="config">Configuration</TabsTrigger>
          <TabsTrigger value="map">Register map</TabsTrigger>
          <TabsTrigger value="raw">Raw access</TabsTrigger>
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
                  title="Device type ID"
                  hint="The encoder's slave type code (register 0x000A). Change it only if instructed for your installation."
                  current={s.device_type}
                  invalidate={invalidate}
                />
                <ValueCard
                  key={`${s.read_at}-threshold`}
                  base={base}
                  path="reverse-flow-threshold"
                  title="Reverse flow threshold"
                  hint="Threshold the encoder uses to detect reverse flow (register 0x000D), in the encoder's internal units."
                  current={s.reverse_flow_threshold}
                  invalidate={invalidate}
                />
                <Card className="lg:col-span-2">
                  <CardHeader>
                    <CardTitle className="flex items-center gap-1.5">
                      Diagnostics <Hint>Read-only health information reported by the encoder.</Hint>
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
                    <Field label="Battery status" hint="Battery status code (register 0x000E)." value={s.battery_status} />
                    <Field label="Failure" hint="Failure code (register 0x000F). 0 means no failure reported." value={s.failure === 0 ? "0 · none" : s.failure} />
                    <Field label="Firmware" hint="Software version date (registers 0x000B-0x000C)." value={s.firmware.text} />
                    <Field label="Debug bytes" hint="Six diagnostic bytes debugNum1-6 (registers 0x0019-0x001B)." value={s.debug_numbers?.join(" ") ?? "unavailable"} />
                  </CardContent>
                </Card>
              </div>
            )
          )}
        </TabsContent>

        <TabsContent value="map">
          <Card>
            <CardHeader>
              <CardTitle>Register map</CardTitle>
              <CardDescription>{s ? `Read ${fmtDateTime(s.read_at)}.` : "All documented holding registers."} 32-bit values span two registers, high word first.</CardDescription>
            </CardHeader>
            <CardContent>
              <QueryState query={map}>
                {(m) => (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Address</TableHead>
                        <TableHead>Register</TableHead>
                        <TableHead>Access</TableHead>
                        <TableHead>Raw</TableHead>
                        <TableHead>Value</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {m.registers.map((r) => {
                        const raws = s?.raw.filter((x: RegisterValue) => x.key === r.key) ?? []
                        return (
                          <TableRow key={r.key}>
                            <TableCell className="font-mono text-xs whitespace-nowrap">
                              {hex(r.address)}
                              {r.words > 1 ? `–${hex(r.address + r.words - 1)}` : ""}
                            </TableCell>
                            <TableCell>
                              <span className="inline-flex items-center gap-1.5 font-medium">
                                {r.title} <Hint>{r.description}</Hint>
                              </span>
                            </TableCell>
                            <TableCell>
                              <Badge variant={r.access === "rw" ? "default" : "outline"}>{r.access === "rw" ? "read/write" : "read only"}</Badge>
                            </TableCell>
                            <TableCell className="font-mono text-xs">{raws.map((x) => x.hex).join(" ") || "—"}</TableCell>
                            <TableCell className="tabular-nums">{s ? decoded(r.key, s) : "—"}</TableCell>
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
  const [slave, setSlave] = useState(current)
  const save = useAction(() => api.put(`${base}/slave-address`, { slave }), { success: `Slave address changed to ${slave}`, invalidate })
  return (
    <CardShell title="Slave address" hint="Re-addresses the encoder (register 0x0000) and updates this device's configuration to match." description={`Currently ${current}.`}>
      <div className="flex items-end gap-2">
        <FormField id="slave" label="New address" hint="1-247, unique on the bus. The encoder answers only to the new address afterwards." className="flex-1">
          <Input id="slave" type="number" min={1} max={247} value={slave} onChange={(e) => setSlave(Number(e.target.value))} />
        </FormField>
        <Confirm title={`Change slave address to ${slave}?`} description="The encoder will stop answering at its current address. The panel configuration is updated automatically." onConfirm={() => save.mutate(undefined)} tip="Write the new address to the encoder.">
          <Button disabled={slave === current || slave < 1 || slave > 247 || save.isPending}>{save.isPending && <Spinner />} Change</Button>
        </Confirm>
      </div>
    </CardShell>
  )
}

function ClockCard({ base, snap, tz, invalidate }: { base: string; snap: SnapshotResponse; tz: string; invalidate: unknown[][] }) {
  const [custom, setCustom] = useState(() => toLocalInput(new Date()))
  const sync = useAction((time?: string) => api.put<{ time: string }>(`${base}/clock`, time ? { time } : {}), { success: (r) => `Clock set to ${r.time}`, invalidate })
  const drift = snap.clock_drift_seconds
  return (
    <CardShell
      title="Real-time clock"
      hint="The encoder's clock (registers 0x0007-0x0009) holds local time without a time zone; the panel interprets it in the device's configured zone."
      description={
        snap.snapshot.clock.valid ? (
          <>
            {fmtDateTime(snap.clock_time)} ({tz}) · drift {drift !== undefined ? `${drift > 0 ? "+" : ""}${fmtNumber(drift, 0)} s` : "—"}
          </>
        ) : (
          "The encoder reports an invalid date."
        )
      }
    >
      <div className="grid gap-3">
        <WithTip tip="Write the server's current time, converted to the device's time zone.">
          <Button variant="outline" onClick={() => sync.mutate(undefined)} disabled={sync.isPending}>
            {sync.isPending && <Spinner />} Synchronise with server time
          </Button>
        </WithTip>
        <div className="flex items-end gap-2">
          <FormField id="clock-custom" label="Set specific time" hint="Time in your browser's zone; it is converted to the device's zone before writing." className="flex-1">
            <Input id="clock-custom" type="datetime-local" step={1} value={custom} onChange={(e) => setCustom(e.target.value)} />
          </FormField>
          <WithTip tip="Write the chosen time to the encoder.">
            <Button onClick={() => sync.mutate(new Date(custom).toISOString().replace(/\.\d+Z$/, "Z"))} disabled={sync.isPending || !custom}>
              Set
            </Button>
          </WithTip>
        </div>
      </div>
    </CardShell>
  )
}

function PulseCard({ base, s, expected, invalidate }: { base: string; s: Snapshot; expected: { base_liters_per_pulse: number; output_liters_per_unit: number }; invalidate: unknown[][] }) {
  const [vals, setVals] = useState({ base: s.base_pulse_liters, output: s.output_pulse_liters })
  const save = useAction(() => api.put(`${base}/pulse-equivalents`, { base_liters_per_pulse: vals.base, output_liters_per_unit: vals.output }), { success: "Pulse equivalents written", invalidate })
  const apply = useAction(() => api.post(`${base}/pulse-equivalents/apply`), { success: "Expected pulse equivalents applied", invalidate })
  const invalid = vals.base < 1 || vals.base > 10000 || vals.output < 1 || vals.output > 1000 || vals.output < vals.base
  const hasExpected = expected.base_liters_per_pulse > 0
  const mismatch = hasExpected && (expected.base_liters_per_pulse !== s.base_pulse_liters || expected.output_liters_per_unit !== s.output_pulse_liters)
  return (
    <CardShell
      title="Pulse equivalents"
      hint="How the encoder converts the meter's pulses into its counters (registers 0x0010-0x0011)."
      description={`Encoder: ${s.base_pulse_liters} L/pulse in, ${s.output_pulse_liters} L/unit out.${hasExpected ? ` Expected: ${expected.base_liters_per_pulse} / ${expected.output_liters_per_unit}.` : ""}`}
    >
      <form
        className="grid gap-3"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          save.mutate(undefined)
        }}
      >
        <div className="grid grid-cols-2 gap-3">
          <FormField id="pulse-base" label="Base (L/pulse)" hint="Liters per pulse emitted by the physical meter's sender, 1-10000. Must match the meter's nameplate.">
            <Input id="pulse-base" type="number" min={1} max={10000} value={vals.base} onChange={(e) => setVals({ ...vals, base: Number(e.target.value) })} />
          </FormField>
          <FormField id="pulse-out" label="Output (L/unit)" hint="Liters per counter unit reported over Modbus, 1-1000, not lower than the base value. Changing it changes the scale of all counters.">
            <Input id="pulse-out" type="number" min={1} max={1000} value={vals.output} onChange={(e) => setVals({ ...vals, output: Number(e.target.value) })} />
          </FormField>
        </div>
        {invalid && <p className="text-sm font-medium">Base must be 1-10000, output 1-1000 and output ≥ base.</p>}
        {mismatch && (
          <Alert>
            <AlertTitle>Differs from the configuration</AlertTitle>
            <AlertDescription>The encoder does not match the expected values set for this meter.</AlertDescription>
          </Alert>
        )}
        <div className="flex flex-wrap gap-2">
          <WithTip tip="Write both values to the encoder in one request.">
            <Button type="submit" disabled={invalid || save.isPending}>
              {save.isPending && <Spinner />} Write
            </Button>
          </WithTip>
          {hasExpected && (
            <WithTip tip="Write the expected values from this device's meter configuration.">
              <Button type="button" variant="outline" onClick={() => apply.mutate(undefined)} disabled={apply.isPending}>
                Apply expected
              </Button>
            </WithTip>
          )}
        </div>
      </form>
    </CardShell>
  )
}

function BaseReadingsCard({ base, s, invalidate }: { base: string; s: Snapshot; invalidate: unknown[][] }) {
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
    success: "Counters preset",
    invalidate,
    onSuccess: () => {
      setFwd("")
      setRev("")
    },
  })
  return (
    <CardShell
      title="Base readings"
      hint="Presets the forward and reverse counters (registers 0x0012-0x0015), e.g. to match the mechanical register of the meter when installing the encoder."
      description={`Forward ${fmtNumber(s.forward_base_reading)} · reverse ${fmtNumber(s.reverse_base_reading)} units.`}
    >
      <div className="grid gap-3">
        <div className="flex items-center gap-2">
          <ToggleGroup type="single" variant="outline" size="sm" value={mode} onValueChange={(v) => v && setMode(v as "liters" | "raw")}>
            <WithTip tip="Enter volumes in liters; they are converted with the meter's resolution and offset.">
              <ToggleGroupItem value="liters">Liters</ToggleGroupItem>
            </WithTip>
            <WithTip tip="Enter raw counter units exactly as stored in the encoder.">
              <ToggleGroupItem value="raw">Raw units</ToggleGroupItem>
            </WithTip>
          </ToggleGroup>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <FormField id="base-fwd" label={mode === "raw" ? "Forward (units)" : "Forward (L)"} hint="New forward reading. Leave empty to keep the current value.">
            <Input id="base-fwd" type="number" min={0} step="any" value={fwd} onChange={(e) => setFwd(e.target.value)} />
          </FormField>
          <FormField id="base-rev" label={mode === "raw" ? "Reverse (units)" : "Reverse (L)"} hint="New reverse reading. Leave empty to keep the current value.">
            <Input id="base-rev" type="number" min={0} step="any" value={rev} onChange={(e) => setRev(e.target.value)} />
          </FormField>
        </div>
        <Confirm title="Preset the counters?" description="The encoder's counters jump to the new values. Consumption computed across this moment is flagged as a counter reset." onConfirm={() => save.mutate(undefined)} tip="Write the new base readings to the encoder.">
          <Button disabled={(fwd === "" && rev === "") || save.isPending} className="justify-self-start">
            {save.isPending && <Spinner />} Write
          </Button>
        </Confirm>
      </div>
    </CardShell>
  )
}

function ValueCard({ base, path, title, hint, current, invalidate }: { base: string; path: string; title: string; hint: string; current: number; invalidate: unknown[][] }) {
  const [value, setValue] = useState(current)
  const save = useAction(() => api.put(`${base}/${path}`, { value }), { success: `${title} written`, invalidate })
  return (
    <CardShell title={title} hint={hint} description={`Currently ${current}.`}>
      <div className="flex items-end gap-2">
        <FormField id={path} label="New value" hint="Integer 0-65535." className="flex-1">
          <Input id={path} type="number" min={0} max={65535} value={value} onChange={(e) => setValue(Number(e.target.value))} />
        </FormField>
        <WithTip tip={`Write the value to the encoder.`}>
          <Button onClick={() => save.mutate(undefined)} disabled={value === current || value < 0 || value > 65535 || save.isPending}>
            {save.isPending && <Spinner />} Write
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
    { success: values.length === 1 ? "Register written (0x06)" : `${values.length} registers written (0x10)`, invalidate },
  )
  const badWrite = values.length === 0 || values.some((v) => !Number.isInteger(v) || v < 0 || v > 65535) || !Number.isInteger(parseNum(write.address))

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <CardShell title="Read holding registers" hint="Function 0x03. Reads up to 125 consecutive registers starting at any address, including undocumented ones." description="Addresses accept decimal or 0x-prefixed hex.">
        <div className="grid gap-3">
          <div className="flex items-end gap-2">
            <FormField id="raw-addr" label="Start address" hint="First register address, 0-65535." className="flex-1">
              <Input id="raw-addr" value={read.address} onChange={(e) => setRead({ ...read, address: e.target.value })} />
            </FormField>
            <FormField id="raw-count" label="Count" hint="Number of registers, 1-125." className="w-24">
              <Input id="raw-count" value={read.count} onChange={(e) => setRead({ ...read, count: e.target.value })} />
            </FormField>
            <WithTip tip="Send the read request to the encoder.">
              <Button onClick={() => doRead.mutate(undefined)} disabled={doRead.isPending}>
                {doRead.isPending && <Spinner />} Read
              </Button>
            </WithTip>
          </div>
          {result && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Address</TableHead>
                  <TableHead>Hex</TableHead>
                  <TableHead className="text-right">Decimal</TableHead>
                  <TableHead>Register</TableHead>
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
            Write registers <Hint>One value uses function 0x06 (write single register); several values use 0x10 (write multiple registers) starting at the address.</Hint>
          </CardTitle>
          <CardDescription>{allowWrites ? "Writes go straight to the encoder without validation. Prefer the Configuration tab." : "Disabled by the server (modbus.allow_raw_writes: false)."}</CardDescription>
          <CardAction>
            <Badge variant="outline">Advanced</Badge>
          </CardAction>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="grid grid-cols-3 gap-2">
            <FormField id="w-addr" label="Address" hint="Register address to write, 0-65535 (decimal or 0x hex).">
              <Input id="w-addr" value={write.address} disabled={!allowWrites} onChange={(e) => setWrite({ ...write, address: e.target.value })} />
            </FormField>
            <FormField id="w-vals" label="Values" hint="One or more 16-bit values (0-65535), separated by commas or spaces; up to 123." className="col-span-2">
              <Input id="w-vals" placeholder="e.g. 5 or 0x0001, 0x000A" value={write.values} disabled={!allowWrites} onChange={(e) => setWrite({ ...write, values: e.target.value })} />
            </FormField>
          </div>
          <Confirm title="Write raw registers?" description={`Write ${values.length} value(s) starting at ${write.address}. Writing the slave address register also updates the panel's configuration.`} onConfirm={() => doWrite.mutate(undefined)} tip="Send the write request to the encoder.">
            <Button disabled={!allowWrites || badWrite || doWrite.isPending} className="justify-self-start">
              {doWrite.isPending && <Spinner />} Write
            </Button>
          </Confirm>
        </CardContent>
      </Card>
    </div>
  )
}
