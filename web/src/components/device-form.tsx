import { WithTip } from "@/components/hint"
import { useState, type FormEvent, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()
  const editing = !!device
  const initial = () => device ?? { ...EMPTY_DEVICE, bus: buses[0]?.config.id ?? "", ...preset }
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<DeviceConfig>(initial)
  const [tags, setTags] = useState(tagsToText(initial().tags))
  const save = useAction((d: DeviceConfig) => (editing ? api.put(`devices/${encodeURIComponent(d.id)}`, d) : api.post("devices", d)), {
    success: editing ? t("deviceForm.updated") : t("deviceForm.created"),
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
          <DialogTitle>{editing ? t("deviceForm.titleEdit", { id: device.id }) : t("deviceForm.titleAdd")}</DialogTitle>
          <DialogDescription>{t("deviceForm.description")}</DialogDescription>
        </DialogHeader>
        <form id="device-form" onSubmit={submit} className="grid gap-4">
          <div className="grid grid-cols-2 gap-4">
            <FormField id="dev-id" label={t("deviceForm.id")} hint={t("deviceForm.idHint")}>
              <Input id="dev-id" value={form.id} disabled={editing} onChange={(e) => set("id", e.target.value)} required pattern="[a-z0-9][a-z0-9_\-]{0,63}" />
            </FormField>
            <FormField id="dev-name" label={t("deviceForm.name")} hint={t("deviceForm.nameHint")}>
              <Input id="dev-name" value={form.name} maxLength={128} onChange={(e) => set("name", e.target.value)} />
            </FormField>
          </div>
          <FormField id="dev-desc" label={t("deviceForm.descriptionLabel")} hint={t("deviceForm.descriptionHint")}>
            <Input id="dev-desc" value={form.description} maxLength={1024} onChange={(e) => set("description", e.target.value)} />
          </FormField>

          <FormSection title={t("deviceForm.connection")}>
            <div className="grid grid-cols-3 gap-4">
              <FormField id="dev-bus" label={t("deviceForm.bus")} hint={t("deviceForm.busHint")} className="col-span-2">
                <Select value={form.bus} onValueChange={(v) => set("bus", v)}>
                  <SelectTrigger id="dev-bus" className="w-full">
                    <SelectValue placeholder={t("deviceForm.busPlaceholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {buses.map((b) => (
                      <SelectItem key={b.config.id} value={b.config.id}>
                        {b.config.name || b.config.id} ({t(`transports.${b.config.transport}`)})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
              <FormField id="dev-slave" label={t("deviceForm.slave")} hint={t("deviceForm.slaveHint")}>
                <Input id="dev-slave" type="number" min={1} max={247} value={form.slave} onChange={(e) => set("slave", Number(e.target.value))} required />
              </FormField>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <FormField id="dev-tz" label={t("deviceForm.timezone")} hint={t("deviceForm.timezoneHint")}>
                <Input id="dev-tz" placeholder={t("deviceForm.timezonePlaceholder")} value={form.timezone} onChange={(e) => set("timezone", e.target.value)} />
              </FormField>
            </div>
            <div className="flex flex-wrap gap-6">
              <SwitchField id="dev-enabled" label={t("deviceForm.enabled")} hint={t("deviceForm.enabledHint")}>
                <Switch id="dev-enabled" checked={form.enabled !== false} onCheckedChange={(v) => set("enabled", v)} />
              </SwitchField>
              <SwitchField id="dev-record" label={t("deviceForm.record")} hint={t("deviceForm.recordHint")}>
                <Switch id="dev-record" checked={form.record !== false} onCheckedChange={(v) => set("record", v)} />
              </SwitchField>
              <SwitchField id="dev-grouped" label={t("deviceForm.grouped")} hint={t("deviceForm.groupedHint")}>
                <Switch id="dev-grouped" checked={form.grouped_reads} onCheckedChange={(v) => set("grouped_reads", v)} />
              </SwitchField>
            </div>
          </FormSection>

          <FormSection title={t("deviceForm.meter")}>
            <div className="grid grid-cols-2 gap-4">
              <FormField id="dev-lpu" label={t("deviceForm.litersPerUnit")} hint={t("deviceForm.litersPerUnitHint")}>
                <Input id="dev-lpu" type="number" min={0} step="any" value={form.meter.liters_per_unit} onChange={(e) => setMeter("liters_per_unit", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-offset" label={t("deviceForm.offset")} hint={t("deviceForm.offsetHint")}>
                <Input id="dev-offset" type="number" step="any" value={form.meter.offset_liters} onChange={(e) => setMeter("offset_liters", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-base" label={t("deviceForm.base")} hint={t("deviceForm.baseHint")}>
                <Input id="dev-base" type="number" min={0} max={10000} value={form.meter.base_liters_per_pulse} onChange={(e) => setMeter("base_liters_per_pulse", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-out" label={t("deviceForm.output")} hint={t("deviceForm.outputHint")}>
                <Input id="dev-out" type="number" min={0} max={1000} value={form.meter.output_liters_per_unit} onChange={(e) => setMeter("output_liters_per_unit", Number(e.target.value))} />
              </FormField>
              <FormField id="dev-model" label={t("deviceForm.model")} hint={t("deviceForm.modelHint")}>
                <Input id="dev-model" value={form.meter.model} onChange={(e) => setMeter("model", e.target.value)} />
              </FormField>
              <FormField id="dev-sn" label={t("deviceForm.serial")} hint={t("deviceForm.serialHint")}>
                <Input id="dev-sn" value={form.meter.serial_number} onChange={(e) => setMeter("serial_number", e.target.value)} />
              </FormField>
            </div>
            <FormField id="dev-tags" label={t("deviceForm.tags")} hint={t("deviceForm.tagsHint")}>
              <Textarea id="dev-tags" rows={3} placeholder={"site=hq\nfloor=1"} value={tags} onChange={(e) => setTags(e.target.value)} />
            </FormField>
          </FormSection>
        </form>
        <DialogFooter>
          <Button type="submit" form="device-form" disabled={save.isPending || !form.bus}>
            {save.isPending && <Spinner />} {editing ? t("common.save") : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
