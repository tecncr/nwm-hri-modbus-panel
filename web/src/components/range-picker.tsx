import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

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
  "1h": { ms: 3600e3, bucket: "5m" },
  "24h": { ms: 86400e3, bucket: "1h" },
  "7d": { ms: 7 * 86400e3, bucket: "6h" },
  "30d": { ms: 30 * 86400e3, bucket: "24h" },
} as const

type Preset = keyof typeof PRESETS | "custom"

const BUCKETS = ["1m", "5m", "15m", "1h", "6h", "24h", "168h"] as const

/** Time range selector with presets and an optional custom range. */
export function useRange(initial: keyof typeof PRESETS = "24h") {
  const { t } = useTranslation()
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
        {(Object.keys(PRESETS) as (keyof typeof PRESETS)[]).map((k) => (
          <WithTip key={k} tip={t(`range.presets.${k}.tip`)}>
            <ToggleGroupItem value={k} aria-label={t(`range.presets.${k}.tip`)}>
              {t(`range.presets.${k}.label`)}
            </ToggleGroupItem>
          </WithTip>
        ))}
        <WithTip tip={t("range.customTip")}>
          <ToggleGroupItem value="custom">{t("range.custom")}</ToggleGroupItem>
        </WithTip>
      </ToggleGroup>
      {preset === "custom" && (
        <div className="flex flex-wrap items-center gap-2">
          <Input type="datetime-local" className="h-7 w-auto" value={custom.from} max={custom.to} onChange={(e) => setCustom({ ...custom, from: e.target.value })} aria-label={t("range.from")} />
          <span className="text-sm">{t("range.to")}</span>
          <Input type="datetime-local" className="h-7 w-auto" value={custom.to} min={custom.from} onChange={(e) => setCustom({ ...custom, to: e.target.value })} aria-label={t("range.toLabel")} />
          <Select value={custom.bucket} onValueChange={(bucket) => setCustom({ ...custom, bucket })}>
            <SelectTrigger size="sm" className="w-32" aria-label={t("range.bucket")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {BUCKETS.map((v) => (
                <SelectItem key={v} value={v}>
                  {t(`range.buckets.${v}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Hint>{t("range.bucketHint")}</Hint>
        </div>
      )}
    </div>
  )

  return { range, control, refresh: () => setNow(Date.now()) }
}
