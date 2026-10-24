import { useQuery } from "@tanstack/react-query"
import { PlayIcon } from "lucide-react"

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
import { fmtDateTime, fmtDuration, fmtGoDuration, fmtNumber, fmtRelative } from "@/lib/format"
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
  const info = useQuery({ queryKey: ["system"], queryFn: () => api.get<SystemInfo>("system"), refetchInterval: 15_000 })
  const config = useQuery({ queryKey: ["system-config"], queryFn: () => api.get<unknown>("system/config") })
  const record = useAction(() => api.post<RecorderRun>("recorder/run"), {
    success: (r) => `Recorded ${r.recorded} meter(s)${r.failed ? `, ${r.failed} failed` : ""}`,
    invalidate: [["system"], ["meters"], ["measurements"]],
  })
  const s = info.data
  const run = s?.recorder.last_run

  return (
    <>
      <PageHeader
        title="System"
        description="Server, recorder and storage status."
        actions={
          <WithTip tip="Read every recordable meter now and store the measurements, outside the schedule.">
            <Button onClick={() => record.mutate(undefined)} disabled={record.isPending || !s?.storage.enabled}>
              {record.isPending ? <Spinner /> : <PlayIcon />} Record now
            </Button>
          </WithTip>
        }
      />
      <QueryState query={info} skeleton="h-96">
        {(s) => (
          <>
            <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
              <Stat label="Version" value={s.version} sub={s.build.commit !== "n/a" ? s.build.commit.slice(0, 12) : `${s.build.os}/${s.build.arch}`} hint="Build of the running server." />
              <Stat label="Uptime" value={fmtDuration(s.uptime_seconds)} sub={`since ${fmtDateTime(s.started_at)}`} hint="Time since the server started." />
              <Stat label="Inventory" value={`${s.buses} bus${s.buses === 1 ? "" : "es"} · ${s.devices} meter${s.devices === 1 ? "" : "s"}`} sub={s.persistent ? "Saved to config file" : "In memory only"} hint="Configured buses and encoders. Changes made in the dashboard are written back to the config file when one is in use." />
              <Stat label="Database" value={fmtNumber(s.storage.stats.rows)} sub={`${s.storage.driver}${s.storage.stats.estimated ? " · estimated" : ""}`} hint="Rows in the measurement database and the storage driver in use." />
            </div>
            <div className="grid gap-6 lg:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>Recorder</CardTitle>
                  <CardDescription>
                    {s.recorder.enabled ? s.recorder.schedule : "Disabled"}
                    {s.recorder.next_run ? ` · next ${fmtRelative(s.recorder.next_run)}` : ""}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {run ? (
                    <>
                      <p className="mb-3 text-sm">
                        Last pass {fmtDateTime(run.tick)} took {run.duration_ms} ms: {run.recorded} stored, {run.failed} failed.
                        {run.store_error && <span className="block font-semibold">Database error: {run.store_error}</span>}
                      </p>
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Meter</TableHead>
                            <TableHead>Result</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {(run.devices ?? []).map((d) => (
                            <TableRow key={d.device_id}>
                              <TableCell>{d.device_id}</TableCell>
                              <TableCell>{d.ok ? <Badge>OK</Badge> : <span className="text-xs">{d.error}</span>}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </>
                  ) : (
                    <p className="text-sm">No recording pass has run yet.</p>
                  )}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>Server</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableBody>
                      <Row label="Config file" hint="The YAML file the server loaded; inventory changes are saved there.">
                        {s.config_file || "none"}
                      </Row>
                      <Row label="Storage" hint="Measurement database driver and the time span of stored data.">
                        {s.storage.driver}
                        {s.storage.stats.oldest ? ` · ${fmtDateTime(s.storage.stats.oldest)} → ${fmtDateTime(s.storage.stats.newest)}` : ""}
                        {s.storage.error ? ` · ${s.storage.error}` : ""}
                      </Row>
                      <Row label="Modbus timeout / retries" hint="Default response timeout and retries; buses can override them.">
                        {fmtGoDuration(s.modbus.timeout)} / {s.modbus.retries}
                      </Row>
                      <Row label="Live cache" hint="Live readings younger than this are served from memory instead of the bus.">
                        {fmtGoDuration(s.modbus.live_cache_ttl)}
                      </Row>
                      <Row label="Raw writes / broadcast" hint="Whether the API allows raw register writes and broadcast writes.">
                        {s.modbus.allow_raw_writes ? "allowed" : "disabled"} / {s.modbus.allow_broadcast ? "allowed" : "disabled"}
                      </Row>
                      <Row label="Query limits" hint="Bounds applied to database queries to protect the server.">
                        range ≤ {fmtGoDuration(s.query.max_range)}, page ≤ {s.query.max_page_size}, CSV ≤ {fmtNumber(s.query.max_export_rows)} rows, ≤ {s.query.max_buckets} buckets
                      </Row>
                      <Row label="Build" hint="Detailed build information.">
                        {s.build.version} · {s.build.commit} · {s.build.date || "n/a"} · {s.build.go}
                      </Row>
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
              <Card className="lg:col-span-2">
                <CardHeader>
                  <CardTitle className="flex items-center gap-1.5">
                    Effective configuration <Hint>The merged result of defaults, config file, environment variables and flags. Secrets are redacted. Edit the config file to change it; buses and devices can also be edited on the Devices page.</Hint>
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
