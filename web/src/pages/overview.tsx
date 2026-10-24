import { useMemo } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { RefreshCwIcon, TriangleAlertIcon } from "lucide-react"
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
import { fmtNumber, fmtRelative, fmtVolume } from "@/lib/format"
import type { ConsumptionSummary, Meter, SeriesPoint, SystemInfo } from "@/lib/types"
import { href } from "@/hooks/use-route"

export function OverviewPage() {
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
    if (failed.length) toast.error(`${failed.length} of ${results.length} meters did not answer`, { description: errorMessage((failed[0] as PromiseRejectedResult).reason) })
    else toast.success(`Read ${results.length} meters`)
  }

  const online = list.filter((m) => m.online).length
  const rec = system.data?.recorder

  return (
    <>
      <PageHeader
        title="Overview"
        description="Live state and consumption of every configured meter."
        actions={
          <WithTip tip="Reads every enabled meter over the bus right now, bypassing the live cache. Readings are not stored; the recorder stores them on its schedule.">
            <Button variant="outline" onClick={readAll} disabled={list.length === 0}>
              <RefreshCwIcon /> Read all now
            </Button>
          </WithTip>
        }
      />

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label="Meters online" value={meters.data ? `${online} / ${list.length}` : "—"} hint="Meters whose most recent request succeeded, out of all configured meters." />
        <Stat
          label="Net consumption"
          value={summary.data ? fmtVolume(summary.data.total_net_liters) : "—"}
          sub="In the selected range"
          hint="Forward minus reverse volume across all meters in the selected range, computed from recorded measurements."
        />
        <Stat
          label="Recorder"
          value={rec ? (rec.enabled ? (rec.last_run ? `${rec.last_run.recorded} ok` : "Waiting") : "Off") : "—"}
          sub={rec?.next_run ? `Next ${fmtRelative(rec.next_run)}` : rec?.schedule}
          hint="Result of the last scheduled recording pass and when the next one runs."
        />
        <Stat
          label="Stored rows"
          value={system.data ? fmtNumber(system.data.storage.stats.rows) : "—"}
          sub={system.data ? `${system.data.storage.driver}${system.data.storage.stats.estimated ? " · estimate" : ""}` : undefined}
          hint="Number of measurements in the database. On TimescaleDB this is a fast estimate."
        />
      </div>

      <div className="mb-6 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-1.5 text-lg font-bold">
          Consumption <Hint>Both charts are computed from recorded measurements. The range applies to both.</Hint>
        </h2>
        {control}
      </div>
      <div className="mb-6 grid gap-6 lg:grid-cols-5">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>By meter</CardTitle>
            <CardDescription>Net volume per meter in the range.</CardDescription>
          </CardHeader>
          <CardContent>
            <QueryState query={summary}>
              {(d) => <MeterBars data={d.meters.filter((m) => m.has_data).map((m) => ({ id: m.device_id, label: m.name || m.device_id, value: Math.max(m.net_liters, 0) }))} />}
            </QueryState>
          </CardContent>
        </Card>
        <Card className="lg:col-span-3">
          <CardHeader>
            <CardTitle>Over time</CardTitle>
            <CardDescription>
              Net volume per {range.bucket} bucket{list.length > MAX_SERIES ? ` · first ${MAX_SERIES} meters` : ""}.
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
          <CardTitle>Meters</CardTitle>
          <CardDescription>Last known reading of each meter. Open a meter for its history.</CardDescription>
          <CardAction>
            <Button asChild variant="outline" size="sm">
              <a href={href("devices")}>Manage devices</a>
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          <QueryState query={meters}>
            {(d) =>
              d.meters.length === 0 ? (
                <Empty className="border">
                  <EmptyHeader>
                    <EmptyTitle>No meters configured</EmptyTitle>
                    <EmptyDescription>Add a bus and a device, or scan a bus to discover encoders.</EmptyDescription>
                  </EmptyHeader>
                  <EmptyContent>
                    <Button asChild>
                      <a href={href("devices")}>Go to devices</a>
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
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Meter</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-right">
            <span className="inline-flex items-center gap-1">
              Reading <Hint>Net accumulated volume (forward minus reverse, plus the configured offset).</Hint>
            </span>
          </TableHead>
          <TableHead className="text-right">Forward</TableHead>
          <TableHead className="text-right">Reverse</TableHead>
          <TableHead>Updated</TableHead>
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
                      <TriangleAlertIcon className="size-4" aria-label="Warnings" />
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
