import { useQuery } from "@tanstack/react-query"
import { PlayIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PageHeader } from "@/components/app-shell"
import { Hint, WithTip } from "@/components/hint"
import { QueryState } from "@/components/query-state"
import { Stat } from "@/components/stat"
import { useAction } from "@/hooks/use-action"
import { api } from "@/lib/api"
import { fmtDateTime, fmtDuration, fmtGoDuration, fmtNumber, fmtRelative, fmtSchedule } from "@/lib/format"
import type { RecorderRun, SystemInfo } from "@/lib/types"

function Row({ label, hint, children }: { label: string; hint: string; children: React.ReactNode }) {
  return (
    <TableRow>
      <TableCell className="w-1/2">
        <span className="inline-flex items-center gap-1.5 font-medium">
          {label} <Hint>{hint}</Hint>
        </span>
      </TableCell>
      <TableCell className="font-mono text-xs break-all whitespace-normal">{children}</TableCell>
    </TableRow>
  )
}

export function SystemPage() {
  const { t } = useTranslation()
  const info = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system"), refetchInterval: 15_000 })
  const config = useQuery({ queryKey: ["system-config"], queryFn: () => api.get<unknown>("system/config") })
  const record = useAction(() => api.post<RecorderRun>("recorder/run"), {
    success: (r) => {
      const message = t("system.recorded", { count: r.recorded })
      return r.failed ? t("system.recordedFailed", { message, failed: r.failed }) : message
    },
    invalidate: [["system"], ["meters"], ["measurements"]],
  })
  const s = info.data
  // Placeholders the server reports for development builds.
  const version = (v: string) => (v === "development build" ? t("system.devBuild") : v)
  const notAvailable = (v: string) => (!v || v === "n/a" ? t("system.notAvailable") : v)
  const run = s?.recorder.last_run

  return (
    <>
      <PageHeader
        title={t("system.title")}
        description={t("system.description")}
        actions={
          <WithTip tip={t("system.recordTip")}>
            <Button onClick={() => record.mutate(undefined)} disabled={record.isPending || !s?.storage.enabled}>
              {record.isPending ? <Spinner /> : <PlayIcon />} {t("system.record")}
            </Button>
          </WithTip>
        }
      />
      <QueryState query={info} skeleton="h-96">
        {(s) => (
          <>
            <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
              <Stat label={t("system.version")} value={version(s.version)} sub={s.build.commit !== "n/a" ? s.build.commit.slice(0, 12) : `${s.build.os}/${s.build.arch}`} hint={t("system.versionHint")} />
              <Stat label={t("system.uptime")} value={fmtDuration(s.uptime_seconds)} sub={t("system.since", { time: fmtDateTime(s.started_at) })} hint={t("system.uptimeHint")} />
              <Stat label={t("system.inventory")} value={`${t("system.buses", { count: s.buses })} · ${t("system.meters", { count: s.devices })}`} sub={s.persistent ? t("system.savedToFile") : t("system.inMemory")} hint={t("system.inventoryHint")} />
              <Stat label={t("system.database")} value={fmtNumber(s.storage.stats.rows)} sub={`${s.storage.driver}${s.storage.stats.estimated ? t("system.estimated") : ""}`} hint={t("system.databaseHint")} />
            </div>
            <div className="grid gap-6 lg:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>{t("system.recorder")}</CardTitle>
                  <CardDescription>
                    {s.recorder.enabled ? fmtSchedule(s.recorder.schedule) : t("system.recorderDisabled")}
                    {s.recorder.next_run ? t("system.next", { when: fmtRelative(s.recorder.next_run) }) : ""}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {run ? (
                    <>
                      <p className="mb-3 text-sm">
                        {t("system.lastPass", { time: fmtDateTime(run.tick), duration: fmtNumber(run.duration_ms), recorded: run.recorded, failed: run.failed })}
                        {run.store_error && <span className="block font-semibold">{t("system.databaseError", { error: run.store_error })}</span>}
                      </p>
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>{t("system.meter")}</TableHead>
                            <TableHead>{t("system.result")}</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {(run.devices ?? []).map((d) => (
                            <TableRow key={d.device_id}>
                              <TableCell>{d.device_id}</TableCell>
                              <TableCell>{d.ok ? <Badge>{t("common.ok")}</Badge> : <span className="text-xs">{d.error}</span>}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </>
                  ) : (
                    <p className="text-sm text-muted-foreground">{t("system.noPass")}</p>
                  )}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>{t("system.server")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableBody>
                      <Row label={t("system.configFile")} hint={t("system.configFileHint")}>
                        {s.config_file || t("common.none")}
                      </Row>
                      <Row label={t("system.storage")} hint={t("system.storageHint")}>
                        {s.storage.driver}
                        {s.storage.stats.oldest ? ` · ${fmtDateTime(s.storage.stats.oldest)} → ${fmtDateTime(s.storage.stats.newest)}` : ""}
                        {s.storage.error ? ` · ${s.storage.error}` : ""}
                      </Row>
                      <Row label={t("system.modbus")} hint={t("system.modbusHint")}>
                        {fmtGoDuration(s.modbus.timeout)} / {s.modbus.retries}
                      </Row>
                      <Row label={t("system.liveCache")} hint={t("system.liveCacheHint")}>
                        {fmtGoDuration(s.modbus.live_cache_ttl)}
                      </Row>
                      <Row label={t("system.writes")} hint={t("system.writesHint")}>
                        {s.modbus.allow_raw_writes ? t("common.allowed") : t("common.disabled")} / {s.modbus.allow_broadcast ? t("common.allowed") : t("common.disabled")}
                      </Row>
                      <Row label={t("system.limits")} hint={t("system.limitsHint")}>
                        {t("system.limitsValue", { range: fmtGoDuration(s.query.max_range), page: fmtNumber(s.query.max_page_size), rows: fmtNumber(s.query.max_export_rows), buckets: fmtNumber(s.query.max_buckets) })}
                      </Row>
                      <Row label={t("system.build")} hint={t("system.buildHint")}>
                        {version(s.build.version)} · {notAvailable(s.build.commit)} · {notAvailable(s.build.date)} · {s.build.go}
                      </Row>
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
              <Card className="lg:col-span-2">
                <CardHeader>
                  <CardTitle className="flex items-center gap-1.5">
                    {t("system.effective")} <Hint>{t("system.effectiveHint")}</Hint>
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <QueryState query={config}>
                    {(c) => (
                      <ScrollArea className="h-96 rounded-sm border">
                        <pre className="p-4 font-mono text-xs">{JSON.stringify(c, null, 2)}</pre>
                      </ScrollArea>
                    )}
                  </QueryState>
                </CardContent>
              </Card>
            </div>
          </>
        )}
      </QueryState>
    </>
  )
}
