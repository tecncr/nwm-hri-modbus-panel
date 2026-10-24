import { useState } from "react"
import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { ChevronDownIcon, ChevronLeftIcon, ChevronRightIcon, DownloadIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PageHeader } from "@/components/app-shell"
import { FormField } from "@/components/form-field"
import { WithTip } from "@/components/hint"
import { ErrorBox, Loading } from "@/components/query-state"
import { api, download, errorMessage } from "@/lib/api"
import { fmtDateTime, fmtGoDuration, fmtNumber, toLocalInput } from "@/lib/format"
import type { DeviceView, MeasurementPage, RegisterMap, SystemInfo } from "@/lib/types"

/** A multi-select of items rendered as a checkbox menu. */
function MultiSelect({ id, label, options, value, onChange, tip, compact }: { compact?: boolean; id: string; label: string; options: { value: string; label: string }[]; value: string[]; onChange: (v: string[]) => void; tip: string }) {
  const { t } = useTranslation()
  return (
    <DropdownMenu>
      <WithTip tip={tip}>
        <DropdownMenuTrigger asChild>
          <Button id={id} variant="outline" className={compact ? "w-40 justify-between font-normal" : "w-full justify-between font-normal"}>
            <span className="truncate">{value.length === 0 ? t("measurements.all", { label }) : value.length === 1 ? (options.find((o) => o.value === value[0])?.label ?? value[0]) : t("measurements.selected", { count: value.length, label })}</span>
            <ChevronDownIcon />
          </Button>
        </DropdownMenuTrigger>
      </WithTip>
      <DropdownMenuContent className="max-h-80 w-64 overflow-y-auto" align="start">
        <DropdownMenuLabel>{t("measurements.select", { label })}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {options.map((o) => (
          <DropdownMenuCheckboxItem
            key={o.value}
            checked={value.includes(o.value)}
            onSelect={(e) => e.preventDefault()}
            onCheckedChange={(c) => onChange(c ? [...value, o.value] : value.filter((v) => v !== o.value))}
          >
            {o.label}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function MeasurementsPage() {
  const { t } = useTranslation()
  const devices = useQuery({ queryKey: ["devices"], queryFn: () => api.get<{ devices: DeviceView[] }>("devices") })
  const system = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system") })
  const regmap = useQuery({ queryKey: ["register-map"], queryFn: () => api.get<RegisterMap>("registers"), staleTime: Infinity })
  const [filter, setFilter] = useState(() => ({
    devices: [] as string[],
    from: toLocalInput(new Date(Date.now() - 86400e3)),
    to: toLocalInput(new Date(Date.now() + 60e3)),
    order: "desc",
    pageSize: 100,
  }))
  const [page, setPage] = useState(1)
  const [columns, setColumns] = useState<string[]>([])
  const [downloading, setDownloading] = useState(false)

  const params = {
    device: filter.devices.join(","),
    from: new Date(filter.from).toISOString(),
    to: new Date(filter.to).toISOString(),
    order: filter.order,
    page_size: filter.pageSize,
    page,
  }
  const data = useQuery({
    queryKey: ["measurements", params],
    queryFn: () => api.get<MeasurementPage>("measurements", params),
    placeholderData: keepPreviousData,
  })
  const update = (patch: Partial<typeof filter>) => {
    setFilter((f) => ({ ...f, ...patch }))
    setPage(1)
  }
  const names = Object.fromEntries((devices.data?.devices ?? []).map((d) => [d.config.id, d.config.name || d.config.id]))
  const maxPage = system.data?.query.max_page_size ?? 1000
  const pageSizes = [25, 50, 100, 250, 500, 1000].filter((n) => n <= maxPage)
  const d = data.data

  async function csv() {
    setDownloading(true)
    try {
      await download("measurements/csv", { device: params.device, from: params.from, to: params.to, order: "asc", columns: columns.join(",") })
    } catch (e) {
      toast.error(errorMessage(e))
    } finally {
      setDownloading(false)
    }
  }

  return (
    <>
      <PageHeader title={t("measurements.title")} description={t("measurements.description")} />
      <Card className="mb-6">
        <CardHeader>
          <CardTitle>{t("measurements.filters")}</CardTitle>
          <CardDescription>
            {t("measurements.limits", { range: system.data ? fmtGoDuration(system.data.query.max_range) : t("measurements.configuredMaximum"), rows: fmtNumber(maxPage) })}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
          <FormField id="m-devices" label={t("measurements.meters")} hint={t("measurements.metersHint")}>
            <MultiSelect id="m-devices" label={t("measurements.metersLower")} tip={t("measurements.metersTip")} options={(devices.data?.devices ?? []).map((v) => ({ value: v.config.id, label: v.config.name || v.config.id }))} value={filter.devices} onChange={(v) => update({ devices: v })} />
          </FormField>
          <FormField id="m-from" label={t("measurements.from")} hint={t("measurements.fromHint")}>
            <Input id="m-from" type="datetime-local" value={filter.from} onChange={(e) => update({ from: e.target.value })} />
          </FormField>
          <FormField id="m-to" label={t("measurements.to")} hint={t("measurements.toHint")}>
            <Input id="m-to" type="datetime-local" value={filter.to} onChange={(e) => update({ to: e.target.value })} />
          </FormField>
          <FormField id="m-order" label={t("measurements.order")} hint={t("measurements.orderHint")}>
            <Select value={filter.order} onValueChange={(v) => update({ order: v })}>
              <SelectTrigger id="m-order" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="desc">{t("measurements.newest")}</SelectItem>
                <SelectItem value="asc">{t("measurements.oldest")}</SelectItem>
              </SelectContent>
            </Select>
          </FormField>
          <FormField id="m-size" label={t("measurements.pageSize")} hint={t("measurements.pageSizeHint")}>
            <Select value={String(filter.pageSize)} onValueChange={(v) => update({ pageSize: Number(v) })}>
              <SelectTrigger id="m-size" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {pageSizes.map((n) => (
                  <SelectItem key={n} value={String(n)}>
                    {n}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormField>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{d ? t("measurements.count", { count: d.total, formatted: fmtNumber(d.total) }) : t("measurements.title")}</CardTitle>
          <CardDescription>{d && d.total > 0 ? t("measurements.page", { page: fmtNumber(d.page), pages: fmtNumber(d.total_pages) }) : t("measurements.noRows")}</CardDescription>
          <CardAction className="flex items-center gap-2">
            <MultiSelect id="m-cols" compact label={t("measurements.columns")} tip={t("measurements.columnsTip")} options={(regmap.data?.export_columns ?? []).map((c) => ({ value: c, label: c }))} value={columns} onChange={setColumns} />
            <WithTip tip={t("measurements.csvTip", { rows: fmtNumber(system.data?.query.max_export_rows) })}>
              <Button variant="outline" onClick={csv} disabled={downloading || !d?.total}>
                {downloading ? <Spinner /> : <DownloadIcon />} {t("common.csv")}
              </Button>
            </WithTip>
          </CardAction>
        </CardHeader>
        <CardContent>
          {data.isPending ? (
            <Loading className="h-96" />
          ) : data.error ? (
            <ErrorBox error={data.error} />
          ) : (
            d && (
              <>
                <div className="overflow-x-auto">
                  <Table className={data.isFetching ? "opacity-60" : undefined}>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("measurements.time")}</TableHead>
                        <TableHead>{t("measurements.meter")}</TableHead>
                        <TableHead className="text-right">{t("measurements.net")}</TableHead>
                        <TableHead className="text-right">{t("measurements.forward")}</TableHead>
                        <TableHead className="text-right">{t("measurements.reverse")}</TableHead>
                        <TableHead className="text-right">{t("measurements.rawNet")}</TableHead>
                        <TableHead className="text-right">{t("measurements.litersPerUnit")}</TableHead>
                        <TableHead className="text-right">{t("measurements.battery")}</TableHead>
                        <TableHead className="text-right">{t("measurements.failure")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {d.measurements.map((m) => (
                        <TableRow key={`${m.device_id}-${m.time}`}>
                          <TableCell className="whitespace-nowrap tabular-nums">{fmtDateTime(m.time)}</TableCell>
                          <TableCell>{names[m.device_id] ?? m.device_id}</TableCell>
                          <TableCell className="text-right font-semibold tabular-nums">{fmtNumber(m.total_liters / 1000, 3)}</TableCell>
                          <TableCell className="text-right tabular-nums">{fmtNumber(m.forward_liters / 1000, 3)}</TableCell>
                          <TableCell className="text-right tabular-nums">{fmtNumber(m.reverse_liters, 1)}</TableCell>
                          <TableCell className="text-right tabular-nums">{fmtNumber(m.total_raw)}</TableCell>
                          <TableCell className="text-right tabular-nums">{fmtNumber(m.liters_per_unit, 3)}</TableCell>
                          <TableCell className="text-right tabular-nums">{m.battery_status}</TableCell>
                          <TableCell className="text-right tabular-nums">{m.failure}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
                {d.total_pages > 1 && (
                  <div className="mt-4 flex items-center justify-end gap-2">
                    <WithTip tip={t("measurements.previous")}>
                      <Button variant="outline" size="icon-sm" aria-label={t("measurements.previous")} disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                        <ChevronLeftIcon />
                      </Button>
                    </WithTip>
                    <span className="text-sm tabular-nums">
                      {page} / {d.total_pages}
                    </span>
                    <WithTip tip={t("measurements.next")}>
                      <Button variant="outline" size="icon-sm" aria-label={t("measurements.next")} disabled={page >= d.total_pages} onClick={() => setPage((p) => p + 1)}>
                        <ChevronRightIcon />
                      </Button>
                    </WithTip>
                  </div>
                )}
              </>
            )
          )}
        </CardContent>
      </Card>
    </>
  )
}
