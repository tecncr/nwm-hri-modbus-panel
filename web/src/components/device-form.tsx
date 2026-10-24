import { WithTip } from "@/components/hint"
import { useState, type FormEvent, type ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { FormField, FormSection, SwitchField } from "@/components/form-field"
import { useAction } from "@/hooks/use-action"
import { api } from "@/lib/api"
import type { BusView, DeviceConfig } from "@/lib/types"

const EMPTY_DEVICE: DeviceConfig = {
  id: "",
  name: "",
  description: "",
  bus: "",
  slave: 1,
  enabled: null,
  record: null,
  timezone: "",
  grouped_reads: false,
  meter: { liters_per_unit: 0, offset_liters: 0, base_liters_per_pulse: 0, output_liters_per_unit: 0, model: "", serial_number: "" },
  tags: null,
}

const tagsToText = (t: Record<string, string> | null) =>
  Object.entries(t ?? {})
    .map(([k, v]) => `${k}=${v}`)
    .join("\n")

function textToTags(s: string): Record<string, string> | null {
  const out: Record<string, string> = {}
  for (const line of s.split("\n")) {
    const i = line.indexOf("=")
    if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + 1).trim()
  }
  return Object.keys(out).length ? out : null
}

/** Create or edit a device; `preset` pre-fills a new device (e.g. from a scan). */
export function DeviceDialog({ device, preset, buses, children, tip }: { tip?: string; device?: DeviceConfig; preset?: Partial<DeviceConfig>; buses: BusView[]; children: ReactNode }) {
  const editing = !!device
  const initial = () => device ?? { ...EMPTY_DEVICE, bus: buses[0]?.config.id ?? "", ...preset }
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<DeviceConfig>(initial)
  const [tags, setTags] = useState(tagsToText(initial().tags))
  const save = useAction((d: DeviceConfig) => (editing ? api.put(`devices/${encodeURIComponent(d.id)}`, d) : api.post("devices", d)), {
    success: editing ? "Device updated" : "Device created",
    invalidate: [["devices"], ["meters"], ["buses"], ["system"]],
    onSuccess: () => setOpen(false),
  })
  const set = <K extends keyof DeviceConfig>(k: K, v: DeviceConfig[K]) => setForm((f) => ({ ...f, [k]: v }))
  const setMeter = <K extends keyof DeviceConfig["meter"]>(k: K, v: DeviceConfig["meter"][K]) => setForm((f) => ({ ...f, meter: { ...f.meter, [k]: v } }))

  function submit(e: FormEvent) {
    e.preventDefault()
    save.mutate({ ...form, tags: textToTags(tags) })
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (o) {
          setForm(initial())
          setTags(tagsToText(initial().tags))
        }
      }}
    >
      <WithTip tip={tip}>
        <DialogTrigger asChild>{children}</DialogTrigger>
      </WithTip>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{editing ? `Edit device ${device.id}` : "Add device"}</DialogTitle>
          <DialogDescription>Defines how to reach an encoder and how its counters translate to volume. This only changes the panel's configuration, not the encoder.</DialogDescription>
        </DialogHeader>
        <form id="device-form" onSubmit={submit} className="grid gap-4">
          <div className="grid grid-cols-2 gap-4">
            <FormField id="dev-id" label="ID" hint="Unique identifier used in URLs, the database and CSV files. Lowercase letters, digits, - and _. Cannot be changed later.">
              <Input id="dev-id" value={form.id} disabled={editing} onChange={(e) => set("id", e.target.value)} required pattern="[a-z0-9][a-z0-9_\-]{0,63}" />
            </FormField>
            <FormField id="dev-name" label="Name" hint="Human-readable name shown in the dashboard and CSV exports.">
              <Input id="dev-name" value={form.name} maxLength={128} onChange={(e) => set("name", e.target.value)} />
            </FormField>
          </div>
          <FormField id="dev-desc" label="Description" hint="Free text, e.g. the meter's location.">
            <Input id="dev-desc" value={form.description} maxLength={1024} onChange={(e) => set("description", e.target.value)} />
          </FormField>

          <FormSection title="Connection">
            <div className="grid grid-cols-3 gap-4">
              <FormField id="dev-bus" label="Bus" hint="The RS-485 line the encoder is wired to." className="col-span-2">
                <Select value={form.bus} onValueChange={(v) => set("bus", v)}>
                  <SelectTrigger id="dev-bus" className="w-full">
                    <SelectValue placeholder="Select a bus" />
                  </SelectTrigger>
                  <SelectContent>
                    {buses.map((b) => (
                      <SelectItem key={b.config.id} value={b.config.id}>
                        {b.config.name || b.config.id} ({b.config.transport})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
              <FormField id="dev-slave" label="Slave address" hint="The encoder's Modbus address (1-247), unique on its bus. Changing it here does not re-address the encoder; use the Registers page for that.">
                <Input id="dev-slave" type="number" min={1} max={247} value={form.slave} onChange={(e) => set("slave", Number(e.target.value))} required />
              </FormField>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <FormField id="dev-tz" label="Clock time zone" hint="IANA time zone of the encoder's real-time clock (e.g. America/Lima). The encoder stores local time without a zone. Empty uses the server's local zone.">
                <Input id="dev-tz" placeholder="Server local time" value={form.timezone} onChange={(e) => set("timezone", e.target.value)} />
              </FormField>
            </div>
            <div className="flex flex-wrap gap-6">
              <SwitchField id="dev-enabled" label="Enabled" hint="When off, the panel never talks to this encoder and it is skipped by the recorder.">
                <Switch id="dev-enabled" checked={form.enabled !== false} onCheckedChange={(v) => set("enabled", v)} />
              </SwitchField>
              <SwitchField id="dev-record" label="Record" hint="Include this meter in periodic recordings to the database.">
                <Switch id="dev-record" checked={form.record !== false} onCheckedChange={(v) => set("record", v)} />
              </SwitchField>
              <SwitchField id="dev-grouped" label="Grouped reads" hint="Read registers in small groups instead of one block request. Only needed for firmware that rejects large reads; the panel also falls back automatically.">
                <Switch id="dev-grouped" checked={form.grouped_reads} onCheckedChange={(v) => set("grouped_reads", v)} />
              </SwitchField>
            </div>
          </FormSection>

          <FormSection title="Meter">
            <div className="grid grid-cols-2 gap-4">
              <FormField id="dev-lpu" label="Liters per unit" hint="Liters represented by one counter unit. 0 uses the encoder's output pulse equivalent register (recommended). Set a value to override it, e.g. for a fractional resolution.">
                <Input id="dev-lpu" type="number" min={0} step="any" value={form.meter.liters_per_unit} onChange={(e) => setMeter("liters_per_unit", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-offset" label="Offset (L)" hint="Added to the net and forward volume, e.g. the final reading of a meter this one replaced, so the series continues.">
                <Input id="dev-offset" type="number" step="any" value={form.meter.offset_liters} onChange={(e) => setMeter("offset_liters", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-base" label="Expected base L/pulse" hint="Liters per pulse of the physical meter's sender (1-10000). With the next field it can be pushed to the encoder, and differences are flagged as warnings. 0 disables the check.">
                <Input id="dev-base" type="number" min={0} max={10000} value={form.meter.base_liters_per_pulse} onChange={(e) => setMeter("base_liters_per_pulse", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-out" label="Expected output L/unit" hint="Desired counter resolution in liters (1-1000), not lower than the base value. Set both expected values or neither.">
                <Input id="dev-out" type="number" min={0} max={1000} value={form.meter.output_liters_per_unit} onChange={(e) => setMeter("output_liters_per_unit", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-model" label="Model" hint="Free-form model of the physical meter (e.g. DN50).">
                <Input id="dev-model" value={form.meter.model} onChange={(e) => setMeter("model", e.target.value)} />
              </FormField>
              <FormField id="dev-sn" label="Serial number" hint="Free-form serial number of the physical meter.">
                <Input id="dev-sn" value={form.meter.serial_number} onChange={(e) => setMeter("serial_number", e.target.value)} />
              </FormField>
            </div>
            <FormField id="dev-tags" label="Tags" hint="One key=value pair per line, for your own grouping (up to 32).">
              <Textarea id="dev-tags" rows={3} placeholder={"site=hq\nfloor=1"} value={tags} onChange={(e) => setTags(e.target.value)} />
            </FormField>
          </FormSection>
        </form>
        <DialogFooter>
          <Button type="submit" form="device-form" disabled={save.isPending || !form.bus}>
            {save.isPending && <Spinner />} {editing ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
