import { useMemo } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { RefreshCwIcon, TriangleAlertIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { PageHeader } from "@/components/app-shell"
import { MeterBars, MeterLines } from "@/components/charts"
import { MAX_SERIES, SERIES_COLORS } from "@/lib/series"
import { Hint, WithTip } from "@/components/hint"
import { Loading, QueryState } from "@/components/query-state"
import { useRange } from "@/components/range-picker"
import { Stat } from "@/components/stat"
import { OnlineBadge } from "@/components/status"
import { api, errorMessage } from "@/lib/api"
import { fmtGoDuration, fmtNumber, fmtRelative, fmtSchedule, fmtVolume } from "@/lib/format"
import type { ConsumptionSummary, Meter, SeriesPoint, SystemInfo } from "@/lib/types"
import { href } from "@/hooks/use-route"

export function OverviewPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const meters = useQuery({ queryKey: ["meters"], queryFn: () => api.get<{ meters: Meter[] }>("meters"), refetchInterval: 15_000 })
  const system = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system"), refetchInterval: 30_000 })
  const { range, control } = useRange("24h")
  const summary = useQuery({
    queryKey: ["consumption", range.from, range.to],
    queryFn: () => api.get<ConsumptionSummary>("consumption", { from: range.from, to: range.to }),
    enabled: system.data?.storage.enabled !== false,
  })

  const list = meters.data?.meters ?? []
  // Colour follows the meter's position in the inventory, not its rank.
  const charted = list.slice(0, MAX_SERIES)
  const series = useQuery({
    queryKey: ["aggregate", charted.map((m) => m.id).join(","), range.from, range.to, range.bucket],
    queryFn: () =>
      api.get<{ series: Record<string, SeriesPoint[]> }>("measurements/aggregate", {
        device: charted.map((m) => m.id).join(","),
        from: range.from,
        to: range.to,
        bucket: range.bucket,
      }),
    enabled: charted.length > 0 && system.data?.storage.enabled !== false,
  })

  const lineRows = useMemo(() => {
    const byStart = new Map<string, Record<string, number | string>>()
    for (const [id, points] of Object.entries(series.data?.series ?? {})) {
      for (const p of points) {
        const row = byStart.get(p.start) ?? { start: p.start }
        row[id] = p.net_liters
        byStart.set(p.start, row)
      }
    }
    return [...byStart.values()].sort((a, b) => String(a.start).localeCompare(String(b.start)))
  }, [series.data])

  async function readAll() {
    const results = await Promise.allSettled(list.filter((m) => m.enabled).map((m) => api.get(`meters/${encodeURIComponent(m.id)}`, { max_age: "0s" })))
    const failed = results.filter((r) => r.status === "rejected")
    qc.invalidateQueries({ queryKey: ["meters"] })
    if (failed.length) toast.error(t("overview.notAnswered", { failed: failed.length, count: results.length }), { description: errorMessage((failed[0] as PromiseRejectedResult).reason) })
    else toast.success(t("overview.readAllDone", { count: results.length }))
  }

  const online = list.filter((m) => m.online).length
  const rec = system.data?.recorder

  return (
    <>
      <PageHeader
        title={t("overview.title")}
        description={t("overview.description")}
        actions={
          <WithTip tip={t("overview.readAllTip")}>
            <Button variant="outline" onClick={readAll} disabled={list.length === 0}>
              <RefreshCwIcon /> {t("overview.readAll")}
            </Button>
          </WithTip>
        }
      />

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label={t("overview.online")} value={meters.data ? `${online} / ${list.length}` : "—"} hint={t("overview.onlineHint")} />
        <Stat
          label={t("overview.net")}
          value={summary.data ? fmtVolume(summary.data.total_net_liters) : "—"}
          sub={t("overview.netSub")}
          hint={t("overview.netHint")}
        />
        <Stat
          label={t("overview.recorder")}
          value={rec ? (rec.enabled ? (rec.last_run ? t("overview.recorderOk", { count: rec.last_run.recorded }) : t("overview.waiting")) : t("common.off")) : "—"}
          sub={rec?.next_run ? t("overview.next", { when: fmtRelative(rec.next_run) }) : rec && fmtSchedule(rec.schedule)}
          hint={t("overview.recorderHint")}
        />
        <Stat
          label={t("overview.rows")}
          value={system.data ? fmtNumber(system.data.storage.stats.rows) : "—"}
          sub={system.data ? `${system.data.storage.driver}${system.data.storage.stats.estimated ? t("overview.estimate") : ""}` : undefined}
          hint={t("overview.rowsHint")}
        />
      </div>

      <div className="mb-6 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-1.5 text-lg font-bold">
          {t("overview.consumption")} <Hint>{t("overview.consumptionHint")}</Hint>
        </h2>
        {control}
      </div>
      <div className="mb-6 grid gap-6 lg:grid-cols-5">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>{t("overview.byMeter")}</CardTitle>
            <CardDescription>{t("overview.byMeterDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            <QueryState query={summary}>
              {(d) => <MeterBars data={d.meters.filter((m) => m.has_data).map((m) => ({ id: m.device_id, label: m.name || m.device_id, value: Math.max(m.net_liters, 0) }))} />}
            </QueryState>
          </CardContent>
        </Card>
        <Card className="lg:col-span-3">
          <CardHeader>
            <CardTitle>{t("overview.overTime")}</CardTitle>
            <CardDescription>
              {list.length > MAX_SERIES ? t("overview.firstMeters", { bucket: fmtGoDuration(range.bucket), count: MAX_SERIES }) : t("overview.overTimeDescription", { bucket: fmtGoDuration(range.bucket) })}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {series.isPending && charted.length > 0 ? (
              <Loading className="h-80" />
            ) : (
              <MeterLines
                rows={lineRows}
                bucket={range.bucket}
                series={charted.flatMap((m, i) => (series.data?.series[m.id]?.length ? [{ key: m.id, label: m.name || m.id, color: SERIES_COLORS[i] }] : []))}
              />
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("overview.meters")}</CardTitle>
          <CardDescription>{t("overview.metersDescription")}</CardDescription>
          <CardAction>
            <Button asChild variant="outline" size="sm">
              <a href={href("devices")}>{t("overview.manage")}</a>
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <QueryState query={meters}>
            {(d) =>
              d.meters.length === 0 ? (
                <Empty className="border">
                  <EmptyHeader>
                    <EmptyTitle>{t("overview.noMeters")}</EmptyTitle>
                    <EmptyDescription>{t("overview.noMetersDescription")}</EmptyDescription>
                  </EmptyHeader>
                  <EmptyContent>
                    <Button asChild>
                      <a href={href("devices")}>{t("overview.goDevices")}</a>
                    </Button>
                  </EmptyContent>
                </Empty>
              ) : (
                <MeterTable meters={d.meters} />
              )
            }
          </QueryState>
        </CardContent>
      </Card>
    </>
  )
}

function MeterTable({ meters }: { meters: Meter[] }) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("overview.meter")}</TableHead>
          <TableHead>{t("overview.status")}</TableHead>
          <TableHead className="text-right">
            <span className="inline-flex items-center gap-1">
              {t("overview.reading")} <Hint>{t("overview.readingHint")}</Hint>
            </span>
          </TableHead>
          <TableHead className="text-right">{t("overview.forward")}</TableHead>
          <TableHead className="text-right">{t("overview.reverse")}</TableHead>
          <TableHead>{t("overview.updated")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {meters.map((m) => {
          const r = m.reading
          const total = r?.total_liters ?? m.last_recorded?.total_liters
          const time = r?.time ?? m.last_recorded?.time
          return (
            <TableRow key={m.id}>
              <TableCell>
                <a href={href("meters", m.id)} className="font-semibold hover:underline">
                  {m.name || m.id}
                </a>
                <div className="text-xs">
                  {m.id} · {m.bus} #{m.slave}
                </div>
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  <OnlineBadge online={m.online} enabled={m.enabled} lastError={m.last_error} lastOk={r?.time} />
                  {r?.warnings?.length ? (
                    <WithTip tip={r.warnings.join(" · ")}>
                      <TriangleAlertIcon className="size-4" aria-label={t("overview.warnings")} />
                    </WithTip>
                  ) : null}
                </div>
              </TableCell>
              <TableCell className="text-right font-semibold tabular-nums">{fmtVolume(total, "m3")}</TableCell>
              <TableCell className="text-right tabular-nums">{fmtVolume(r?.forward_liters ?? m.last_recorded?.forward_liters, "m3")}</TableCell>
              <TableCell className="text-right tabular-nums">{fmtVolume(r?.reverse_liters ?? m.last_recorded?.reverse_liters, "L")}</TableCell>
              <TableCell className="text-sm whitespace-nowrap">{time ? fmtRelative(time) : "—"}</TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
