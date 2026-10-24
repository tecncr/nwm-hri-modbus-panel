import { useTranslation } from "react-i18next"
import { Bar, BarChart, CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts"

import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { locale } from "@/i18n"
import { fmtDateTime, fmtNumber } from "@/lib/format"
import { SERIES_COLORS } from "@/lib/series"

function tickTime(bucketHours: number) {
  return (v: string) => {
    const d = new Date(v)
    return bucketHours >= 24
      ? d.toLocaleDateString(locale(), { month: "short", day: "numeric" })
      : bucketHours >= 6
        ? d.toLocaleString(locale(), { month: "short", day: "numeric", hour: "2-digit" })
        : d.toLocaleTimeString(locale(), { hour: "2-digit", minute: "2-digit" })
  }
}

function bucketHours(bucket: string) {
  const m = /^(\d+)(m|h)$/.exec(bucket)
  if (!m) return 1
  return m[2] === "h" ? Number(m[1]) : Number(m[1]) / 60
}

const fmtLiters = (v: number) => (Math.abs(v) >= 10000 ? `${fmtNumber(v / 1000, 1)} m³` : `${fmtNumber(v, 0)} L`)

export function NoData({ children }: { children?: string }) {
  const { t } = useTranslation()
  return (
    <Empty className="h-64 border">
      <EmptyHeader>
        <EmptyTitle>{t("charts.noData")}</EmptyTitle>
        <EmptyDescription>{children ?? t("charts.noDataInRange")}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

/** Consumption per time bucket for one meter (single series: no legend). */
export function ConsumptionBars({ data, bucket, label: labelProp }: { data: { start: string; value: number }[]; bucket: string; label?: string }) {
  const { t } = useTranslation()
  if (data.length === 0) return <NoData />
  const label = labelProp ?? t("charts.consumption")
  const config = { value: { label, color: SERIES_COLORS[0] } } satisfies ChartConfig
  return (
    <ChartContainer config={config} className="aspect-auto h-72 w-full">
      <BarChart data={data} barCategoryGap={2} margin={{ left: 4, right: 4, top: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="start" tickLine={false} axisLine={false} tickMargin={8} minTickGap={32} tickFormatter={tickTime(bucketHours(bucket))} />
        <YAxis tickLine={false} axisLine={false} width={64} tickFormatter={fmtLiters} />
        <ChartTooltip
          cursor={{ fill: "var(--foreground)", fillOpacity: 0.06 }}
          content={
            <ChartTooltipContent
              labelFormatter={(_, p) => fmtDateTime(p?.[0]?.payload?.start)}
              formatter={(v) => (
                <span className="flex w-full justify-between gap-4">
                  <span>{label}</span>
                  <span className="font-mono font-semibold tabular-nums">{fmtLiters(Number(v))}</span>
                </span>
              )}
            />
          }
        />
        <Bar dataKey="value" fill="var(--color-value)" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ChartContainer>
  )
}

/** One line per meter (legend always present for two or more series). */
export function MeterLines({ rows, series, bucket, unit = "L" }: { rows: Record<string, number | string>[]; series: { key: string; label: string; color: string }[]; bucket: string; unit?: string }) {
  if (rows.length === 0 || series.length === 0) return <NoData />
  const config: ChartConfig = Object.fromEntries(series.map((s) => [s.key, { label: s.label, color: s.color }]))
  return (
    <ChartContainer config={config} className="aspect-auto h-80 w-full">
      <LineChart data={rows} margin={{ left: 4, right: 12, top: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="start" tickLine={false} axisLine={false} tickMargin={8} minTickGap={32} tickFormatter={tickTime(bucketHours(bucket))} />
        <YAxis tickLine={false} axisLine={false} width={64} tickFormatter={unit === "L" ? fmtLiters : (v) => fmtNumber(v, 1)} />
        <ChartTooltip
          content={
            <ChartTooltipContent
              indicator="line"
              labelFormatter={(_, p) => fmtDateTime(p?.[0]?.payload?.start)}
              formatter={(v, name) => (
                <span className="flex w-full items-center justify-between gap-4">
                  <span className="flex items-center gap-1.5">
                    <span className="size-2.5 rounded-[2px]" style={{ background: config[String(name)]?.color }} />
                    {config[String(name)]?.label}
                  </span>
                  <span className="font-mono font-semibold tabular-nums">{unit === "L" ? fmtLiters(Number(v)) : `${fmtNumber(Number(v), 1)} ${unit}`}</span>
                </span>
              )}
            />
          }
        />
        {series.length > 1 && <ChartLegend content={<ChartLegendContent />} />}
        {series.map((s) => (
          <Line key={s.key} dataKey={s.key} type="monotone" stroke={`var(--color-${s.key})`} strokeWidth={2} dot={false} activeDot={{ r: 4, strokeWidth: 2, stroke: "var(--background)" }} connectNulls />
        ))}
      </LineChart>
    </ChartContainer>
  )
}

/** Consumption by meter: one measure across categories, so a single colour. */
export function MeterBars({ data }: { data: { id: string; label: string; value: number }[] }) {
  const { t } = useTranslation()
  if (data.length === 0) return <NoData />
  const config = { value: { label: t("charts.netConsumption"), color: SERIES_COLORS[0] } } satisfies ChartConfig
  return (
    <ChartContainer config={config} className="aspect-auto w-full" style={{ height: Math.max(160, data.length * 36 + 40) }}>
      <BarChart data={data} layout="vertical" barCategoryGap={2} margin={{ left: 4, right: 48 }}>
        <CartesianGrid horizontal={false} />
        <XAxis type="number" tickLine={false} axisLine={false} tickFormatter={fmtLiters} />
        <YAxis type="category" dataKey="label" tickLine={false} axisLine={false} width={140} />
        <ChartTooltip
          cursor={{ fill: "var(--foreground)", fillOpacity: 0.06 }}
          content={
            <ChartTooltipContent
              hideLabel
              formatter={(v, _n, item) => (
                <span className="flex w-full justify-between gap-4">
                  <span>{item.payload.label}</span>
                  <span className="font-mono font-semibold tabular-nums">{fmtLiters(Number(v))}</span>
                </span>
              )}
            />
          }
        />
        <Bar dataKey="value" fill="var(--color-value)" radius={[0, 4, 4, 0]} label={{ position: "right", formatter: (v: unknown) => fmtLiters(Number(v)), className: "fill-foreground text-xs" }} />
      </BarChart>
    </ChartContainer>
  )
}
