import { useQuery, useQueryClient } from "@tanstack/react-query"
import { DownloadIcon, RefreshCwIcon, SlidersHorizontalIcon, TriangleAlertIcon } from "lucide-react"
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
import { fmtDateTime, fmtNumber, fmtVolume } from "@/lib/format"
import type { Consumption, Meter, SeriesPoint } from "@/lib/types"
import { href } from "@/hooks/use-route"

export function MeterPage({ id }: { id: string }) {
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
      toast.success("Meter read")
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
              {m ? ` · bus ${m.bus} · slave ${m.slave}` : ""}
              {m?.model ? ` · ${m.model}` : ""}
              {m?.serial_number ? ` · S/N ${m.serial_number}` : ""}
            </span>
          </span>
        }
        actions={
          <>
            <WithTip tip="Read the encoder over the bus now instead of using the cached reading.">
              <Button variant="outline" onClick={readNow}>
                <RefreshCwIcon /> Read now
              </Button>
            </WithTip>
            <WithTip tip="Download the recorded measurements of this meter in the selected range as CSV.">
              <Button
                variant="outline"
                onClick={() => download("measurements/csv", { device: id, from: range.from, to: range.to }).catch((e) => toast.error(errorMessage(e)))}
              >
                <DownloadIcon /> CSV
              </Button>
            </WithTip>
            <WithTip tip="Advanced: configure the encoder and read or write its registers.">
              <Button asChild>
                <a href={href("devices", id)}>
                  <SlidersHorizontalIcon /> Registers
                </a>
              </Button>
            </WithTip>
          </>
        }
      />

      {meter.error && (
        <div className="mb-6">
          <ErrorBox error={meter.error} title="The meter could not be read" />
        </div>
      )}
      {r?.warnings?.length ? (
        <Alert className="mb-6">
          <TriangleAlertIcon />
          <AlertTitle>Attention</AlertTitle>
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
          <Stat label="Reading" value={fmtVolume(r?.total_liters, "m3")} sub={r ? fmtDateTime(r.time) : undefined} hint="Net accumulated volume: forward minus reverse, plus the configured offset. This is the value a meter register would display." />
          <Stat label="Forward" value={fmtVolume(r?.forward_liters, "m3")} sub={r ? `${fmtNumber(r.raw.forward)} units` : undefined} hint="Volume that flowed in the normal direction (plus offset). Units are the encoder's raw counter value." />
          <Stat label="Reverse" value={fmtVolume(r?.reverse_liters, "L")} sub={r ? `${fmtNumber(r.raw.reverse)} units` : undefined} hint="Volume that flowed backwards through the meter." />
          <Stat
            label="Resolution"
            value={r ? `${fmtNumber(r.liters_per_unit, 3)} L/unit` : "—"}
            sub={r ? (r.liters_per_unit_source === "config" ? "From configuration" : r.liters_per_unit_source === "encoder" ? "From encoder register" : "Assumed") : undefined}
            hint="Liters represented by one counter unit. Taken from the device's meter.liters_per_unit setting, or from the encoder's output pulse equivalent register when that setting is 0."
          />
        </div>
      )}

      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <h2 className="flex items-center gap-1.5 text-lg font-bold">
          History <Hint>Computed from recorded measurements; live reads are not stored.</Hint>
        </h2>
        {control}
      </div>

      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <QueryState query={consumption} skeleton="h-28 col-span-4">
          {(c) => (
            <>
              <Stat label="Consumed" value={c.has_data ? fmtVolume(c.net_liters) : "—"} sub={c.counter_reset ? "Counter was reset in range" : "Net, in range"} hint="Net volume used between the first and last measurement of the range." />
              <Stat label="Forward" value={c.has_data ? fmtVolume(c.forward_liters) : "—"} hint="Forward volume in the range." />
              <Stat label="Reverse" value={c.has_data ? fmtVolume(c.reverse_liters) : "—"} hint="Backflow volume in the range." />
              <Stat label="Span" value={c.start_time ? fmtDateTime(c.start_time).split(",")[0] : "—"} sub={c.end_time ? `to ${fmtDateTime(c.end_time)}` : undefined} hint="First and last measurement used for the computation." />
            </>
          )}
        </QueryState>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Consumption per {range.bucket}</CardTitle>
            <CardDescription>Net volume used in each bucket.</CardDescription>
          </CardHeader>
          <CardContent>
            {series.isPending ? <Loading className="h-72" /> : series.error ? <ErrorBox error={series.error} /> : <ConsumptionBars bucket={range.bucket} data={points.map((p) => ({ start: p.start, value: Math.max(p.net_liters, 0) }))} />}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Average flow</CardTitle>
            <CardDescription>Forward volume per minute, averaged over each bucket.</CardDescription>
          </CardHeader>
          <CardContent>
            {series.isPending ? (
              <Loading className="h-80" />
            ) : (
              <MeterLines bucket={range.bucket} unit="L/min" rows={points.map((p) => ({ start: p.start, flow: p.avg_flow_lpm }))} series={[{ key: "flow", label: "Average flow", color: SERIES_COLORS[0] }]} />
            )}
          </CardContent>
        </Card>
      </div>

      {(m?.description || (m?.tags && Object.keys(m.tags).length > 0)) && (
        <Card className="mt-6">
          <CardHeader>
            <CardTitle>Details</CardTitle>
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
