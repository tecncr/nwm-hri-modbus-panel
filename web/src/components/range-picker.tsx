import { useMemo, useState } from "react"

import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { Hint, WithTip } from "@/components/hint"
import { toLocalInput } from "@/lib/format"

export interface Range {
  from: string
  to: string
  bucket: string
}

const PRESETS = {
  "1h": { ms: 3600e3, bucket: "5m", label: "1H", tip: "Last hour, in 5-minute buckets." },
  "24h": { ms: 86400e3, bucket: "1h", label: "24H", tip: "Last 24 hours, in hourly buckets." },
  "7d": { ms: 7 * 86400e3, bucket: "6h", label: "7D", tip: "Last 7 days, in 6-hour buckets." },
  "30d": { ms: 30 * 86400e3, bucket: "24h", label: "30D", tip: "Last 30 days, in daily buckets (local midnight)." },
} as const

type Preset = keyof typeof PRESETS | "custom"

const BUCKETS = [
  ["1m", "1 minute"],
  ["5m", "5 minutes"],
  ["15m", "15 minutes"],
  ["1h", "1 hour"],
  ["6h", "6 hours"],
  ["24h", "1 day"],
  ["168h", "1 week"],
]

/** Time range selector with presets and an optional custom range. */
export function useRange(initial: keyof typeof PRESETS = "24h") {
  const [preset, setPreset] = useState<Preset>(initial)
  const [custom, setCustom] = useState(() => ({
    from: toLocalInput(new Date(Date.now() - 86400e3)),
    to: toLocalInput(new Date()),
    bucket: "1h",
  }))
  // Presets are relative to "now" when they are selected or refreshed.
  const [now, setNow] = useState(() => Date.now())

  const range: Range = useMemo(() => {
    if (preset === "custom") {
      return { from: new Date(custom.from).toISOString(), to: new Date(custom.to).toISOString(), bucket: custom.bucket }
    }
    const p = PRESETS[preset]
    return { from: new Date(now - p.ms).toISOString(), to: new Date(now).toISOString(), bucket: p.bucket }
  }, [preset, custom, now])

  const control = (
    <div className="flex flex-wrap items-center gap-2">
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={preset}
        onValueChange={(v) => {
          if (v) {
            setPreset(v as Preset)
            setNow(Date.now())
          }
        }}
      >
        {Object.entries(PRESETS).map(([k, p]) => (
          <WithTip key={k} tip={p.tip}>
            <ToggleGroupItem value={k} aria-label={p.tip}>
              {p.label}
            </ToggleGroupItem>
          </WithTip>
        ))}
        <WithTip tip="Choose start, end and bucket size yourself.">
          <ToggleGroupItem value="custom">Custom</ToggleGroupItem>
        </WithTip>
      </ToggleGroup>
      {preset === "custom" && (
        <div className="flex flex-wrap items-center gap-2">
          <Input type="datetime-local" className="h-7 w-auto" value={custom.from} max={custom.to} onChange={(e) => setCustom({ ...custom, from: e.target.value })} aria-label="From" />
          <span className="text-sm">to</span>
          <Input type="datetime-local" className="h-7 w-auto" value={custom.to} min={custom.from} onChange={(e) => setCustom({ ...custom, to: e.target.value })} aria-label="To" />
          <Select value={custom.bucket} onValueChange={(bucket) => setCustom({ ...custom, bucket })}>
            <SelectTrigger size="sm" className="w-32" aria-label="Bucket">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {BUCKETS.map(([v, l]) => (
                <SelectItem key={v} value={v}>
                  {l}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Hint>The bucket is the interval each bar or point summarises. The server rejects ranges that would produce too many buckets (query.max_buckets).</Hint>
        </div>
      )}
    </div>
  )

  return { range, control, refresh: () => setNow(Date.now()) }
}
