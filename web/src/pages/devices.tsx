import { useQuery } from "@tanstack/react-query"
import { ClockIcon, PencilIcon, PlusIcon, RadarIcon, RadioTowerIcon, SlidersHorizontalIcon, Trash2Icon } from "lucide-react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

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

function busDetail(t: TFunction, b: BusConfig) {
  switch (b.transport) {
    case "serial":
      return `${b.serial.device} · ${b.serial.baud_rate} ${b.serial.data_bits}${b.serial.parity}${b.serial.stop_bits}`
    case "tcp":
      return b.tcp.address
    default:
      return t("devices.simulatedSlaves", { slaves: (b.simulator.slaves ?? []).join(", ") })
  }
}

export function DevicesPage() {
  const { t } = useTranslation()
  const buses = useQuery({ queryKey: ["buses"], queryFn: () => api.get<{ buses: BusView[] }>("buses"), refetchInterval: 15_000 })
  const devices = useQuery({ queryKey: ["devices"], queryFn: () => api.get<{ devices: DeviceView[] }>("devices"), refetchInterval: 15_000 })
  const system = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system") })
  const busList = buses.data?.buses ?? []
  const allowBroadcast = system.data?.modbus.allow_broadcast ?? false

  const deleteBus = useAction((id: string) => api.del(`buses/${encodeURIComponent(id)}`), { success: t("devices.busDeleted"), invalidate: [["buses"], ["system"]] })
  const deleteDevice = useAction((id: string) => api.del(`devices/${encodeURIComponent(id)}`), { success: t("devices.deviceDeleted"), invalidate: [["devices"], ["meters"], ["buses"], ["system"]] })

  return (
    <>
      <PageHeader title={t("devices.title")} description={t("devices.description")} />
      {system.data && !system.data.persistent && (
        <Alert className="mb-6">
          <AlertTitle>{t("devices.notPersisted")}</AlertTitle>
          <AlertDescription>{t("devices.notPersistedDescription")}</AlertDescription>
        </Alert>
      )}

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="flex items-center gap-1.5">
            {t("devices.buses")} <Hint>{t("devices.busesHint")}</Hint>
          </CardTitle>
          <CardDescription>{t("devices.busesDescription")}</CardDescription>
          <CardAction>
            <BusDialog tip={t("devices.addBusTip")}>
              <Button size="sm">
                <PlusIcon /> {t("devices.addBus")}
              </Button>
            </BusDialog>
          </CardAction>
        </CardHeader>
        <CardContent>
          <QueryState query={buses}>
            {(d) =>
              d.buses.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("devices.noBuses")}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("devices.bus")}</TableHead>
                      <TableHead>{t("devices.connection")}</TableHead>
                      <TableHead className="text-right">{t("devices.devicesColumn")}</TableHead>
                      <TableHead className="text-right">
                        <span className="inline-flex items-center gap-1">
                          {t("devices.requests")} <Hint>{t("devices.requestsHint")}</Hint>
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
                            <Badge variant={b.stats.connected ? "default" : "outline"}>{b.stats.connected ? t("devices.open") : t("devices.closed")}</Badge>
                            <span className="text-sm">
                              {t(`transports.${b.config.transport}`)} · {busDetail(t, b.config)}
                            </span>
                          </div>
                          {b.stats.last_error && <div className="mt-1 max-w-md truncate text-xs">{t("devices.lastError", { error: b.stats.last_error })}</div>}
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{b.devices.length}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {t("devices.requestsValue", { requests: fmtNumber(b.stats.requests), failures: fmtNumber(b.stats.failures) })}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex justify-end whitespace-nowrap">
                            <BusDialog bus={b.config} tip={t("devices.editBusTip")}>
                              <Button variant="ghost" size="icon-sm" aria-label={t("devices.editBus")}>
                                <PencilIcon />
                              </Button>
                            </BusDialog>
                            <ScanDialog bus={b.config.id} buses={busList} tip={t("devices.scanTip")}>
                              <Button variant="ghost" size="icon-sm" aria-label={t("devices.scan")}>
                                <RadarIcon />
                              </Button>
                            </ScanDialog>
                            {allowBroadcast && (
                              <>
                                <BroadcastClockDialog bus={b.config.id} tip={t("devices.syncClocksTip")}>
                                  <Button variant="ghost" size="icon-sm" aria-label={t("devices.syncClocks")}>
                                    <ClockIcon />
                                  </Button>
                                </BroadcastClockDialog>
                                <BroadcastAddressDialog bus={b.config.id} tip={t("devices.broadcastAddressTip")}>
                                  <Button variant="ghost" size="icon-sm" aria-label={t("devices.broadcastAddress")}>
                                    <RadioTowerIcon />
                                  </Button>
                                </BroadcastAddressDialog>
                              </>
                            )}
                            <Confirm
                              title={t("devices.deleteBusTitle", { id: b.config.id })}
                              description={b.devices.length ? t("devices.deleteBusInUse", { count: b.devices.length }) : t("devices.deleteBusDescription")}
                              confirm={t("common.delete")}
                              tip={t("devices.deleteBusTip")}
                              onConfirm={() => deleteBus.mutate(b.config.id)}
                            >
                              <Button variant="ghost" size="icon-sm" aria-label={t("devices.deleteBus")}>
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
          <CardTitle>{t("devices.encoders")}</CardTitle>
          <CardDescription>{t("devices.encodersDescription")}</CardDescription>
          <CardAction>
            <DeviceDialog buses={busList} tip={t("devices.addDeviceTip")}>
              <Button size="sm" disabled={busList.length === 0}>
                <PlusIcon /> {t("devices.addDevice")}
              </Button>
            </DeviceDialog>
          </CardAction>
        </CardHeader>
        <CardContent>
          <QueryState query={devices}>
            {(d) =>
              d.devices.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("devices.noDevices")}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("devices.device")}</TableHead>
                      <TableHead>{t("devices.busSlave")}</TableHead>
                      <TableHead>{t("devices.status")}</TableHead>
                      <TableHead>
                        <span className="inline-flex items-center gap-1">
                          {t("devices.recording")} <Hint>{t("devices.recordingHint")}</Hint>
                        </span>
                      </TableHead>
                      <TableHead>
                        <span className="inline-flex items-center gap-1">
                          {t("devices.resolution")} <Hint>{t("devices.resolutionHint")}</Hint>
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
                        <TableCell>{v.recorded ? t("common.yes") : t("common.no")}</TableCell>
                        <TableCell className="text-sm">{v.config.meter.liters_per_unit > 0 ? t("devices.litersPerUnit", { value: fmtNumber(v.config.meter.liters_per_unit, 6) }) : t("devices.fromEncoder")}</TableCell>
                        <TableCell className="text-right whitespace-nowrap">
                          <WithTip tip={t("devices.registersTip")}>
                            <Button asChild variant="ghost" size="icon-sm" aria-label={t("devices.registers")}>
                              <a href={href("devices", v.config.id)}>
                                <SlidersHorizontalIcon />
                              </a>
                            </Button>
                          </WithTip>
                          <DeviceDialog device={v.config} buses={busList} tip={t("devices.editDeviceTip")}>
                            <Button variant="ghost" size="icon-sm" aria-label={t("devices.editDevice")}>
                              <PencilIcon />
                            </Button>
                          </DeviceDialog>
                          <Confirm
                            title={t("devices.deleteDeviceTitle", { id: v.config.id })}
                            description={t("devices.deleteDeviceDescription")}
                            confirm={t("common.delete")}
                            tip={t("devices.deleteDeviceTip")}
                            onConfirm={() => deleteDevice.mutate(v.config.id)}
                          >
                            <Button variant="ghost" size="icon-sm" aria-label={t("devices.deleteDevice")}>
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
