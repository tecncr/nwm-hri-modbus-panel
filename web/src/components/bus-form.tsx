import { WithTip } from "@/components/hint"
import { useState, type FormEvent, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { FormField, FormSection } from "@/components/form-field"
import { useAction } from "@/hooks/use-action"
import { api } from "@/lib/api"
import type { BusConfig } from "@/lib/types"

const EMPTY: BusConfig = {
  id: "",
  name: "",
  transport: "serial",
  serial: { device: "/dev/ttyUSB0", baud_rate: 9600, data_bits: 8, parity: "N", stop_bits: 1 },
  tcp: { address: "", connect_timeout: "5s" },
  simulator: { slaves: [1, 2, 3], flow_lpm: 10 },
  timeout: "0s",
  retries: null,
  frame_delay: "0s",
}

/** Create or edit a bus. */
export function BusDialog({ bus, children, tip }: { tip?: string; bus?: BusConfig; children: ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<BusConfig>(bus ?? EMPTY)
  const [slaves, setSlaves] = useState((bus ?? EMPTY).simulator.slaves?.join(", ") ?? "")
  const editing = !!bus
  const save = useAction((b: BusConfig) => (editing ? api.put(`buses/${encodeURIComponent(b.id)}`, b) : api.post("buses", b)), {
    success: editing ? t("busForm.updated") : t("busForm.created"),
    invalidate: [["buses"], ["devices"], ["system"]],
    onSuccess: () => setOpen(false),
  })

  function submit(e: FormEvent) {
    e.preventDefault()
    const b: BusConfig = { ...form, simulator: { ...form.simulator, slaves: slaves.split(/[\s,]+/).filter(Boolean).map(Number) } }
    save.mutate(b)
  }
  const set = <K extends keyof BusConfig>(k: K, v: BusConfig[K]) => setForm((f) => ({ ...f, [k]: v }))

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (o) {
          setForm(bus ?? EMPTY)
          setSlaves((bus ?? EMPTY).simulator.slaves?.join(", ") ?? "")
        }
      }}
    >
      <WithTip tip={tip}>
        <DialogTrigger asChild>{children}</DialogTrigger>
      </WithTip>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{editing ? t("busForm.titleEdit", { id: bus.id }) : t("busForm.titleAdd")}</DialogTitle>
          <DialogDescription>{t("busForm.description")}</DialogDescription>
        </DialogHeader>
        <form id="bus-form" onSubmit={submit} className="grid gap-4">
          <div className="grid grid-cols-2 gap-4">
            <FormField id="bus-id" label={t("busForm.id")} hint={t("busForm.idHint")}>
              <Input id="bus-id" value={form.id} disabled={editing} onChange={(e) => set("id", e.target.value)} required pattern="[a-z0-9][a-z0-9_\-]{0,63}" />
            </FormField>
            <FormField id="bus-name" label={t("busForm.name")} hint={t("busForm.nameHint")}>
              <Input id="bus-name" value={form.name} onChange={(e) => set("name", e.target.value)} />
            </FormField>
          </div>
          <FormField id="bus-transport" label={t("busForm.transport")} hint={t("busForm.transportHint")}>
            <Select value={form.transport} onValueChange={(v) => set("transport", v as BusConfig["transport"])}>
              <SelectTrigger id="bus-transport" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="serial">{t("busForm.transportSerial")}</SelectItem>
                <SelectItem value="tcp">{t("busForm.transportTcp")}</SelectItem>
                <SelectItem value="simulator">{t("busForm.transportSimulator")}</SelectItem>
              </SelectContent>
            </Select>
          </FormField>

          {form.transport === "serial" && (
            <FormSection title={t("busForm.serialLine")}>
              <FormField id="bus-dev" label={t("busForm.device")} hint={t("busForm.deviceHint")}>
                <Input id="bus-dev" value={form.serial.device} onChange={(e) => set("serial", { ...form.serial, device: e.target.value })} required />
              </FormField>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                <FormField id="bus-baud" label={t("busForm.baud")} hint={t("busForm.baudHint")}>
                  <Input id="bus-baud" type="number" min={300} max={921600} value={form.serial.baud_rate} onChange={(e) => set("serial", { ...form.serial, baud_rate: Number(e.target.value) })} />
                </FormField>
                <FormField id="bus-bits" label={t("busForm.dataBits")} hint={t("busForm.dataBitsHint")}>
                  <Input id="bus-bits" type="number" min={5} max={8} value={form.serial.data_bits} onChange={(e) => set("serial", { ...form.serial, data_bits: Number(e.target.value) })} />
                </FormField>
                <FormField id="bus-parity" label={t("busForm.parity")} hint={t("busForm.parityHint")}>
                  <Select value={form.serial.parity} onValueChange={(v) => set("serial", { ...form.serial, parity: v })}>
                    <SelectTrigger id="bus-parity" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="N">{t("busForm.parityNone")}</SelectItem>
                      <SelectItem value="E">{t("busForm.parityEven")}</SelectItem>
                      <SelectItem value="O">{t("busForm.parityOdd")}</SelectItem>
                    </SelectContent>
                  </Select>
                </FormField>
                <FormField id="bus-stop" label={t("busForm.stopBits")} hint={t("busForm.stopBitsHint")}>
                  <Select value={String(form.serial.stop_bits)} onValueChange={(v) => set("serial", { ...form.serial, stop_bits: Number(v) })}>
                    <SelectTrigger id="bus-stop" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">1</SelectItem>
                      <SelectItem value="2">2</SelectItem>
                    </SelectContent>
                  </Select>
                </FormField>
              </div>
            </FormSection>
          )}
          {form.transport === "tcp" && (
            <FormSection title={t("busForm.gateway")}>
              <div className="grid grid-cols-3 gap-4">
                <FormField id="bus-addr" label={t("busForm.address")} hint={t("busForm.addressHint")} className="col-span-2">
                  <Input id="bus-addr" placeholder="192.168.1.50:4001" value={form.tcp.address} onChange={(e) => set("tcp", { ...form.tcp, address: e.target.value })} required />
                </FormField>
                <FormField id="bus-ct" label={t("busForm.connectTimeout")} hint={t("busForm.connectTimeoutHint")}>
                  <Input id="bus-ct" value={form.tcp.connect_timeout} onChange={(e) => set("tcp", { ...form.tcp, connect_timeout: e.target.value })} />
                </FormField>
              </div>
            </FormSection>
          )}
          {form.transport === "simulator" && (
            <FormSection title={t("busForm.simulator")}>
              <div className="grid grid-cols-3 gap-4">
                <FormField id="bus-slaves" label={t("busForm.slaves")} hint={t("busForm.slavesHint")} className="col-span-2">
                  <Input id="bus-slaves" value={slaves} onChange={(e) => setSlaves(e.target.value)} />
                </FormField>
                <FormField id="bus-flow" label={t("busForm.flow")} hint={t("busForm.flowHint")}>
                  <Input id="bus-flow" type="number" min={0} step="any" value={form.simulator.flow_lpm} onChange={(e) => set("simulator", { ...form.simulator, flow_lpm: Number(e.target.value) })} />
                </FormField>
              </div>
            </FormSection>
          )}

          <FormSection title={t("busForm.timing")}>
            <div className="grid grid-cols-3 gap-4">
              <FormField id="bus-timeout" label={t("busForm.timeout")} hint={t("busForm.timeoutHint")}>
                <Input id="bus-timeout" value={form.timeout} onChange={(e) => set("timeout", e.target.value)} />
              </FormField>
              <FormField id="bus-retries" label={t("busForm.retries")} hint={t("busForm.retriesHint")}>
                <Input id="bus-retries" type="number" min={0} max={10} value={form.retries ?? ""} onChange={(e) => set("retries", e.target.value === "" ? null : Number(e.target.value))} />
              </FormField>
              <FormField id="bus-delay" label={t("busForm.frameDelay")} hint={t("busForm.frameDelayHint")}>
                <Input id="bus-delay" value={form.frame_delay} onChange={(e) => set("frame_delay", e.target.value)} />
              </FormField>
            </div>
          </FormSection>
        </form>
        <DialogFooter>
          <Button type="submit" form="bus-form" disabled={save.isPending}>
            {save.isPending && <Spinner />} {editing ? t("common.save") : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
