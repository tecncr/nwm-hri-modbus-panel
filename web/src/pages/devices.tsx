import { useQuery } from "@tanstack/react-query"
import { ClockIcon, PencilIcon, PlusIcon, RadarIcon, RadioTowerIcon, SlidersHorizontalIcon, Trash2Icon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PageHeader } from "@/components/app-shell"
import { BroadcastAddressDialog, BroadcastClockDialog, ScanDialog } from "@/components/bus-tools"
import { BusDialog } from "@/components/bus-form"
import { Confirm } from "@/components/confirm"
import { DeviceDialog } from "@/components/device-form"
import { Hint, WithTip } from "@/components/hint"
import { QueryState } from "@/components/query-state"
import { OnlineBadge } from "@/components/status"
import { useAction } from "@/hooks/use-action"
import { href } from "@/hooks/use-route"
import { api } from "@/lib/api"
import { fmtNumber } from "@/lib/format"
import type { BusConfig, BusView, DeviceView, SystemInfo } from "@/lib/types"

function busDetail(b: BusConfig) {
  switch (b.transport) {
    case "serial":
      return `${b.serial.device} · ${b.serial.baud_rate} ${b.serial.data_bits}${b.serial.parity}${b.serial.stop_bits}`
    case "tcp":
      return b.tcp.address
    default:
      return `simulated slaves ${(b.simulator.slaves ?? []).join(", ")}`
  }
}

export function DevicesPage() {
  const buses = useQuery({ queryKey: ["buses"], queryFn: () => api.get<{ buses: BusView[] }>("buses"), refetchInterval: 15_000 })
  const devices = useQuery({ queryKey: ["devices"], queryFn: () => api.get<{ devices: DeviceView[] }>("devices"), refetchInterval: 15_000 })
  const system = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system") })
  const busList = buses.data?.buses ?? []
  const allowBroadcast = system.data?.modbus.allow_broadcast ?? false

  const deleteBus = useAction((id: string) => api.del(`buses/${encodeURIComponent(id)}`), { success: "Bus deleted", invalidate: [["buses"], ["system"]] })
  const deleteDevice = useAction((id: string) => api.del(`devices/${encodeURIComponent(id)}`), { success: "Device deleted", invalidate: [["devices"], ["meters"], ["buses"], ["system"]] })

  return (
    <>
      <PageHeader title="Devices" description="Buses and NWM-HRI encoders managed by this panel." />
      {system.data && !system.data.persistent && (
        <Alert className="mb-6">
          <AlertTitle>Changes are not persisted</AlertTitle>
          <AlertDescription>The server runs without a config file, so inventory changes are lost on restart. Start it with --config to persist them.</AlertDescription>
        </Alert>
      )}

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-1.5">
            Buses <Hint>Each bus is a half-duplex RS-485 line; requests to encoders on the same bus are serialised.</Hint>
          </CardTitle>
          <CardDescription>Serial lines and gateways.</CardDescription>
          <CardAction>
            <BusDialog tip="Add an RS-485 serial line, an RTU-over-TCP gateway or a simulated bus.">
              <Button size="sm">
                <PlusIcon /> Add bus
              </Button>
            </BusDialog>
          </CardAction>
        </CardHeader>
        <CardContent>
          <QueryState query={buses}>
            {(d) =>
              d.buses.length === 0 ? (
                <p className="text-sm">No buses yet. Add one to connect encoders.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Bus</TableHead>
                      <TableHead>Connection</TableHead>
                      <TableHead className="text-right">Devices</TableHead>
                      <TableHead className="text-right">
                        <span className="inline-flex items-center gap-1">
                          Requests <Hint>Modbus requests since start, and how many failed (timeouts, CRC errors, exceptions).</Hint>
                        </span>
                      </TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {d.buses.map((b) => (
                      <TableRow key={b.config.id}>
                        <TableCell>
                          <div className="font-semibold">{b.config.name || b.config.id}</div>
                          <div className="text-xs">{b.config.id}</div>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Badge variant={b.stats.connected ? "default" : "outline"}>{b.stats.connected ? "Open" : "Closed"}</Badge>
                            <span className="text-sm">
                              {b.config.transport} · {busDetail(b.config)}
                            </span>
                          </div>
                          {b.stats.last_error && <div className="mt-1 max-w-md truncate text-xs">Last error: {b.stats.last_error}</div>}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{b.devices.length}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {fmtNumber(b.stats.requests)} / {fmtNumber(b.stats.failures)} failed
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end whitespace-nowrap">
                            <BusDialog bus={b.config} tip="Edit the bus connection settings. Saving reconnects the bus.">
                              <Button variant="ghost" size="icon-sm" aria-label="Edit bus">
                                <PencilIcon />
                              </Button>
                            </BusDialog>
                            <ScanDialog bus={b.config.id} buses={busList} tip="Probe slave addresses on this bus to discover encoders.">
                              <Button variant="ghost" size="icon-sm" aria-label="Scan for encoders">
                                <RadarIcon />
                              </Button>
                            </ScanDialog>
                            {allowBroadcast && (
                              <>
                                <BroadcastClockDialog bus={b.config.id} tip="Set the clock of every encoder on this bus to the server time.">
                                  <Button variant="ghost" size="icon-sm" aria-label="Synchronise all clocks">
                                    <ClockIcon />
                                  </Button>
                                </BroadcastClockDialog>
                                <BroadcastAddressDialog bus={b.config.id} tip="Assign a slave address to every encoder on this bus (commissioning a single unknown encoder).">
                                  <Button variant="ghost" size="icon-sm" aria-label="Broadcast slave address">
                                    <RadioTowerIcon />
                                  </Button>
                                </BroadcastAddressDialog>
                              </>
                            )}
                            <Confirm
                              title={`Delete bus ${b.config.id}?`}
                              description={b.devices.length ? `It is used by ${b.devices.length} device(s); delete or move them first.` : "The bus is removed from the configuration."}
                              confirm="Delete"
                              tip="Remove this bus from the configuration."
                              onConfirm={() => deleteBus.mutate(b.config.id)}
                            >
                              <Button variant="ghost" size="icon-sm" aria-label="Delete bus">
                                <Trash2Icon />
                              </Button>
                            </Confirm>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )
            }
          </QueryState>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Encoders</CardTitle>
          <CardDescription>Each encoder is attached to a physical meter.</CardDescription>
          <CardAction>
            <DeviceDialog buses={busList} tip="Add an encoder at a known slave address on one of the buses.">
              <Button size="sm" disabled={busList.length === 0}>
                <PlusIcon /> Add device
              </Button>
            </DeviceDialog>
          </CardAction>
        </CardHeader>
        <CardContent>
          <QueryState query={devices}>
            {(d) =>
              d.devices.length === 0 ? (
                <p className="text-sm">No devices yet. Add one, or scan a bus to discover encoders.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Device</TableHead>
                      <TableHead>Bus / slave</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>
                        <span className="inline-flex items-center gap-1">
                          Recording <Hint>Whether the recorder stores this meter's measurements.</Hint>
                        </span>
                      </TableHead>
                      <TableHead>
                        <span className="inline-flex items-center gap-1">
                          Resolution <Hint>Liters per counter unit: a fixed value from the configuration, or read from the encoder.</Hint>
                        </span>
                      </TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {d.devices.map((v) => (
                      <TableRow key={v.config.id}>
                        <TableCell>
                          <a href={href("meters", v.config.id)} className="font-semibold hover:underline">
                            {v.config.name || v.config.id}
                          </a>
                          <div className="text-xs">{v.config.id}</div>
                        </TableCell>
                        <TableCell className="tabular-nums">
                          {v.config.bus} · {v.config.slave}
                        </TableCell>
                        <TableCell>
                          <OnlineBadge online={v.state.online} enabled={v.enabled} lastError={v.state.last_error} lastOk={v.state.last_ok_at} />
                        </TableCell>
                        <TableCell>{v.recorded ? "Yes" : "No"}</TableCell>
                        <TableCell className="text-sm">{v.config.meter.liters_per_unit > 0 ? `${v.config.meter.liters_per_unit} L/unit` : "From encoder"}</TableCell>
                        <TableCell className="text-right whitespace-nowrap">
                          <WithTip tip="Read and write the encoder's registers.">
                            <Button asChild variant="ghost" size="icon-sm" aria-label="Registers">
                              <a href={href("devices", v.config.id)}>
                                <SlidersHorizontalIcon />
                              </a>
                            </Button>
                          </WithTip>
                          <DeviceDialog device={v.config} buses={busList} tip="Edit name, connection and meter conversion settings.">
                            <Button variant="ghost" size="icon-sm" aria-label="Edit device">
                              <PencilIcon />
                            </Button>
                          </DeviceDialog>
                          <Confirm
                            title={`Delete device ${v.config.id}?`}
                            description="The device is removed from the configuration. Its recorded measurements stay in the database."
                            confirm="Delete"
                            tip="Remove this device from the configuration. Recorded data is kept."
                            onConfirm={() => deleteDevice.mutate(v.config.id)}
                          >
                            <Button variant="ghost" size="icon-sm" aria-label="Delete device">
                              <Trash2Icon />
                            </Button>
                          </Confirm>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )
            }
          </QueryState>
        </CardContent>
      </Card>
    </>
  )
}
