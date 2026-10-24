import { WithTip } from "@/components/hint"
import { useState, type FormEvent, type ReactNode } from "react"

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
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<BusConfig>(bus ?? EMPTY)
  const [slaves, setSlaves] = useState((bus ?? EMPTY).simulator.slaves?.join(", ") ?? "")
  const editing = !!bus
  const save = useAction((b: BusConfig) => (editing ? api.put(`buses/${encodeURIComponent(b.id)}`, b) : api.post("buses", b)), {
    success: editing ? "Bus updated" : "Bus created",
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
          <DialogTitle>{editing ? `Edit bus ${bus.id}` : "Add bus"}</DialogTitle>
          <DialogDescription>A bus is one RS-485 line (or serial gateway) shared by one or more encoders. Saving reconnects it.</DialogDescription>
        </DialogHeader>
        <form id="bus-form" onSubmit={submit} className="grid gap-4">
          <div className="grid grid-cols-2 gap-4">
            <FormField id="bus-id" label="ID" hint="Unique identifier used in URLs and in device definitions. Lowercase letters, digits, - and _. Cannot be changed later.">
              <Input id="bus-id" value={form.id} disabled={editing} onChange={(e) => set("id", e.target.value)} required pattern="[a-z0-9][a-z0-9_\-]{0,63}" />
            </FormField>
            <FormField id="bus-name" label="Name" hint="Optional human-readable name.">
              <Input id="bus-name" value={form.name} onChange={(e) => set("name", e.target.value)} />
            </FormField>
          </div>
          <FormField id="bus-transport" label="Transport" hint="Serial: a local RS-485 adapter. TCP: a transparent serial-to-Ethernet gateway that forwards raw RTU frames. Simulator: in-memory encoders for testing without hardware.">
            <Select value={form.transport} onValueChange={(v) => set("transport", v as BusConfig["transport"])}>
              <SelectTrigger id="bus-transport" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="serial">Serial (RS-485)</SelectItem>
                <SelectItem value="tcp">RTU over TCP gateway</SelectItem>
                <SelectItem value="simulator">Simulator</SelectItem>
              </SelectContent>
            </Select>
          </FormField>

          {form.transport === "serial" && (
            <FormSection title="Serial line">
              <FormField id="bus-dev" label="Device" hint="Serial port path, e.g. /dev/ttyUSB0 or /dev/serial/by-id/… (stable across reboots). The server user needs read/write access (dialout group).">
                <Input id="bus-dev" value={form.serial.device} onChange={(e) => set("serial", { ...form.serial, device: e.target.value })} required />
              </FormField>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                <FormField id="bus-baud" label="Baud" hint="Line speed. NWM-HRI encoders use 9600 baud by default.">
                  <Input id="bus-baud" type="number" min={300} max={921600} value={form.serial.baud_rate} onChange={(e) => set("serial", { ...form.serial, baud_rate: Number(e.target.value) })} />
                </FormField>
                <FormField id="bus-bits" label="Data bits" hint="Bits per character. Encoders use 8.">
                  <Input id="bus-bits" type="number" min={5} max={8} value={form.serial.data_bits} onChange={(e) => set("serial", { ...form.serial, data_bits: Number(e.target.value) })} />
                </FormField>
                <FormField id="bus-parity" label="Parity" hint="Parity bit. Encoders use none (N).">
                  <Select value={form.serial.parity} onValueChange={(v) => set("serial", { ...form.serial, parity: v })}>
                    <SelectTrigger id="bus-parity" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="N">None</SelectItem>
                      <SelectItem value="E">Even</SelectItem>
                      <SelectItem value="O">Odd</SelectItem>
                    </SelectContent>
                  </Select>
                </FormField>
                <FormField id="bus-stop" label="Stop bits" hint="Stop bits per character. Encoders use 1.">
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
            <FormSection title="Gateway">
              <div className="grid grid-cols-3 gap-4">
                <FormField id="bus-addr" label="Address" hint="host:port of the serial server. It must forward raw RTU frames (transparent mode), not translate to Modbus TCP." className="col-span-2">
                  <Input id="bus-addr" placeholder="192.168.1.50:4001" value={form.tcp.address} onChange={(e) => set("tcp", { ...form.tcp, address: e.target.value })} required />
                </FormField>
                <FormField id="bus-ct" label="Connect timeout" hint="How long to wait for the TCP connection, as a duration (e.g. 5s).">
                  <Input id="bus-ct" value={form.tcp.connect_timeout} onChange={(e) => set("tcp", { ...form.tcp, connect_timeout: e.target.value })} />
                </FormField>
              </div>
            </FormSection>
          )}
          {form.transport === "simulator" && (
            <FormSection title="Simulator">
              <div className="grid grid-cols-3 gap-4">
                <FormField id="bus-slaves" label="Slave addresses" hint="Comma-separated addresses (1-247) of the simulated encoders." className="col-span-2">
                  <Input id="bus-slaves" value={slaves} onChange={(e) => setSlaves(e.target.value)} />
                </FormField>
                <FormField id="bus-flow" label="Flow (L/min)" hint="Mean simulated flow of each encoder; actual flow varies with a daily pattern.">
                  <Input id="bus-flow" type="number" min={0} step="any" value={form.simulator.flow_lpm} onChange={(e) => set("simulator", { ...form.simulator, flow_lpm: Number(e.target.value) })} />
                </FormField>
              </div>
            </FormSection>
          )}

          <FormSection title="Timing">
            <div className="grid grid-cols-3 gap-4">
              <FormField id="bus-timeout" label="Timeout" hint="Response timeout for this bus as a duration (e.g. 500ms). 0s uses the global modbus.timeout.">
                <Input id="bus-timeout" value={form.timeout} onChange={(e) => set("timeout", e.target.value)} />
              </FormField>
              <FormField id="bus-retries" label="Retries" hint="Retries after a timeout or CRC error. Leave empty to use the global modbus.retries.">
                <Input id="bus-retries" type="number" min={0} max={10} value={form.retries ?? ""} onChange={(e) => set("retries", e.target.value === "" ? null : Number(e.target.value))} />
              </FormField>
              <FormField id="bus-delay" label="Frame delay" hint="Silent interval enforced between frames. 0s uses the Modbus RTU standard of 3.5 character times for the baud rate (about 4 ms at 9600).">
                <Input id="bus-delay" value={form.frame_delay} onChange={(e) => set("frame_delay", e.target.value)} />
              </FormField>
            </div>
          </FormSection>
        </form>
        <DialogFooter>
          <Button type="submit" form="bus-form" disabled={save.isPending}>
            {save.isPending && <Spinner />} {editing ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
