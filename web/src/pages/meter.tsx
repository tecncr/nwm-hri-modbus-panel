import { useQuery, useQueryClient } from "@tanstack/react-query"
import { DownloadIcon, RefreshCwIcon, SlidersHorizontalIcon, TriangleAlertIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { PageHeader } from "@/components/app-shell"
import { ConsumptionBars, MeterLines } from "@/components/charts"
import { SERIES_COLORS } from "@/lib/series"
import { Hint, WithTip } from "@/components/hint"
import { ErrorBox, Loading, QueryState } from "@/components/query-state"
import { useRange } from "@/components/range-picker"
import { Stat } from "@/components/stat"
import { OnlineBadge } from "@/components/status"
import { api, download, errorMessage } from "@/lib/api"
import { fmtDate, fmtDateTime, fmtGoDuration, fmtNumber, fmtVolume } from "@/lib/format"
import type { Consumption, Meter, SeriesPoint } from "@/lib/types"
import { href } from "@/hooks/use-route"

export function MeterPage({ id }: { id: string }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const path = `meters/${encodeURIComponent(id)}`
  // Live reading through the server's short cache; refreshed periodically.
  const meter = useQuery({ queryKey: ["meter", id], queryFn: () => api.get<Meter>(path), refetchInterval: 30_000 })
  const { range, control } = useRange("24h")
  const consumption = useQuery({
    queryKey: ["meter-consumption", id, range.from, range.to],
    queryFn: () => api.get<Consumption>(`${path}/consumption`, { from: range.from, to: range.to }),
  })
  const series = useQuery({
    queryKey: ["meter-series", id, range.from, range.to, range.bucket],
    queryFn: () => api.get<{ points: SeriesPoint[] }>(`${path}/series`, { from: range.from, to: range.to, bucket: range.bucket }),
  })

  async function readNow() {
    try {
      const m = await api.get<Meter>(path, { max_age: "0s" })
      qc.setQueryData(["meter", id], m)
      toast.success(t("meter.read"))
    } catch (err) {
      toast.error(errorMessage(err))
      qc.invalidateQueries({ queryKey: ["meter", id] })
    }
  }

  const m = meter.data
  const r = m?.reading
  const points = series.data?.points ?? []

  return (
    <>
      <PageHeader
        title={m?.name || id}
        description={
          <span className="inline-flex flex-wrap items-center gap-2">
            {m && <OnlineBadge online={m.online} enabled={m.enabled} lastError={m.last_error} lastOk={r?.time} />}
            <span>
              {id}
              {m ? t("meter.subtitleBus", { bus: m.bus, slave: m.slave }) : ""}
              {m?.model ? ` · ${m.model}` : ""}
              {m?.serial_number ? t("meter.serial", { serial: m.serial_number }) : ""}
            </span>
          </span>
        }
        actions={
          <>
            <WithTip tip={t("meter.readNowTip")}>
              <Button variant="outline" onClick={readNow}>
                <RefreshCwIcon /> {t("meter.readNow")}
              </Button>
            </WithTip>
            <WithTip tip={t("meter.csvTip")}>
              <Button
                variant="outline"
                onClick={() => download("measurements/csv", { device: id, from: range.from, to: range.to }).catch((e) => toast.error(errorMessage(e)))}
              >
                <DownloadIcon /> {t("common.csv")}
              </Button>
            </WithTip>
            <WithTip tip={t("meter.registersTip")}>
              <Button asChild>
                <a href={href("devices", id)}>
                  <SlidersHorizontalIcon /> {t("meter.registers")}
                </a>
              </Button>
            </WithTip>
          </>
        }
      />

      {meter.error && (
        <div className="mb-6">
          <ErrorBox error={meter.error} title={t("meter.readFailed")} />
        </div>
      )}
      {r?.warnings?.length ? (
        <Alert className="mb-6">
          <TriangleAlertIcon />
          <AlertTitle>{t("meter.attention")}</AlertTitle>
          <AlertDescription>
            <ul className="list-disc pl-4">
              {r.warnings.map((w) => (
                <li key={w}>{w}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}

      {meter.isPending ? (
        <Loading className="mb-6 h-28" />
      ) : (
        <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
          <Stat label={t("meter.reading")} value={fmtVolume(r?.total_liters, "m3")} sub={r ? fmtDateTime(r.time) : undefined} hint={t("meter.readingHint")} />
          <Stat label={t("meter.forward")} value={fmtVolume(r?.forward_liters, "m3")} sub={r ? t("meter.units", { value: fmtNumber(r.raw.forward) }) : undefined} hint={t("meter.forwardHint")} />
          <Stat label={t("meter.reverse")} value={fmtVolume(r?.reverse_liters, "L")} sub={r ? t("meter.units", { value: fmtNumber(r.raw.reverse) }) : undefined} hint={t("meter.reverseHint")} />
          <Stat
            label={t("meter.resolution")}
            value={r ? t("meter.resolutionValue", { value: fmtNumber(r.liters_per_unit, 3) }) : "—"}
            sub={r ? (r.liters_per_unit_source === "config" ? t("meter.fromConfig") : r.liters_per_unit_source === "encoder" ? t("meter.fromEncoder") : t("meter.assumed")) : undefined}
            hint={t("meter.resolutionHint")}
          />
        </div>
      )}

      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-1.5 text-lg font-bold">
          {t("meter.history")} <Hint>{t("meter.historyHint")}</Hint>
        </h2>
        {control}
      </div>

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <QueryState query={consumption} skeleton="h-28 col-span-4">
          {(c) => (
            <>
              <Stat label={t("meter.consumed")} value={c.has_data ? fmtVolume(c.net_liters) : "—"} sub={c.counter_reset ? t("meter.counterReset") : t("meter.netInRange")} hint={t("meter.consumedHint")} />
              <Stat label={t("meter.forward")} value={c.has_data ? fmtVolume(c.forward_liters) : "—"} hint={t("meter.forwardRangeHint")} />
              <Stat label={t("meter.reverse")} value={c.has_data ? fmtVolume(c.reverse_liters) : "—"} hint={t("meter.reverseRangeHint")} />
              <Stat label={t("meter.span")} value={c.start_time ? fmtDate(c.start_time) : "—"} sub={c.end_time ? t("meter.spanTo", { time: fmtDateTime(c.end_time) }) : undefined} hint={t("meter.spanHint")} />
            </>
          )}
        </QueryState>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t("meter.perBucket", { bucket: fmtGoDuration(range.bucket) })}</CardTitle>
            <CardDescription>{t("meter.perBucketDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            {series.isPending ? <Loading className="h-72" /> : series.error ? <ErrorBox error={series.error} /> : <ConsumptionBars bucket={range.bucket} data={points.map((p) => ({ start: p.start, value: Math.max(p.net_liters, 0) }))} />}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("meter.averageFlow")}</CardTitle>
            <CardDescription>{t("meter.averageFlowDescription")}</CardDescription>
          </CardHeader>
          <CardContent>
            {series.isPending ? (
              <Loading className="h-80" />
            ) : (
              <MeterLines bucket={range.bucket} unit="L/min" rows={points.map((p) => ({ start: p.start, flow: p.avg_flow_lpm }))} series={[{ key: "flow", label: t("meter.averageFlow"), color: SERIES_COLORS[0] }]} />
            )}
          </CardContent>
        </Card>
      </div>

      {(m?.description || (m?.tags && Object.keys(m.tags).length > 0)) && (
        <Card className="mt-6">
          <CardHeader>
            <CardTitle>{t("meter.details")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-2 text-sm">
            {m.description && <p>{m.description}</p>}
            {m.tags && (
              <div className="flex flex-wrap gap-2">
                {Object.entries(m.tags).map(([k, v]) => (
                  <span key={k} className="rounded-sm border px-2 py-0.5 text-xs">
                    {k}: {v}
                  </span>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </>
  )
}
