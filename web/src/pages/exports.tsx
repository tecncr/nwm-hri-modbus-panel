import { useQuery } from "@tanstack/react-query"
import { DownloadIcon, PlayIcon, Trash2Icon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PageHeader } from "@/components/app-shell"
import { Confirm } from "@/components/confirm"
import { WithTip } from "@/components/hint"
import { QueryState } from "@/components/query-state"
import { Stat } from "@/components/stat"
import { useAction } from "@/hooks/use-action"
import { api, download, errorMessage } from "@/lib/api"
import { fmtBytes, fmtDateTime, fmtGoDuration, fmtRelative } from "@/lib/format"
import type { ExportFile, ExportResult, ExportStatus } from "@/lib/types"

export function ExportsPage() {
  const data = useQuery({ queryKey: ["exports"], queryFn: () => api.get<{ status: ExportStatus; files: ExportFile[] }>("exports"), refetchInterval: 30_000 })
  const run = useAction(() => api.post<ExportResult>("exports/run"), {
    success: (r) => `Exported ${r.rows} rows to ${r.files.length} file(s)`,
    invalidate: [["exports"]],
  })
  const del = useAction((name: string) => api.del(`exports/files/${encodeURIComponent(name)}`), { success: "File deleted", invalidate: [["exports"]] })
  const st = data.data?.status
  const last = st?.last_run

  return (
    <>
      <PageHeader
        title="Exports"
        description="CSV files generated periodically from the recorded measurements."
        actions={
          <WithTip tip="Generate the export now for the configured window ending at the current time. Scheduled exports are unaffected.">
            <Button onClick={() => run.mutate(undefined)} disabled={run.isPending}>
              {run.isPending ? <Spinner /> : <PlayIcon />} Export now
            </Button>
          </WithTip>
        }
      />
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label="Schedule" value={st ? (st.enabled ? st.schedule.replace(" (aligned)", "") : "Off") : "—"} sub={st?.enabled ? "aligned to the clock" : undefined} hint="When periodic exports run (export.interval or export.cron). Interval schedules are aligned to the wall clock, e.g. 1h runs on the hour." />
        <Stat label="Window" value={fmtGoDuration(st?.window)} hint="Time span covered by each file, ending at the scheduled time (export.window)." />
        <Stat label="Next run" value={st?.next_run ? fmtRelative(st.next_run) : "—"} sub={st?.next_run ? fmtDateTime(st.next_run) : undefined} hint="When the next scheduled export runs." />
        <Stat
          label="Last run"
          value={last ? (last.error ? "Failed" : `${last.rows} rows`) : "—"}
          sub={last ? (last.error ?? fmtRelative(last.tick)) : undefined}
          hint="Outcome of the most recent export, scheduled or manual."
        />
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Files</CardTitle>
          <CardDescription>{st ? `In ${st.directory} on the server.` : ""}</CardDescription>
        </CardHeader>
        <CardContent>
          <QueryState query={data}>
            {(d) =>
              d.files.length === 0 ? (
                <p className="text-sm text-muted-foreground">No export files yet.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>File</TableHead>
                      <TableHead className="text-right">Size</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {d.files.map((f) => (
                      <TableRow key={f.name}>
                        <TableCell className="font-mono text-xs">{f.name}</TableCell>
                        <TableCell className="text-right tabular-nums">{fmtBytes(f.size)}</TableCell>
                        <TableCell className="whitespace-nowrap">{fmtDateTime(f.modified)}</TableCell>
                        <TableCell className="text-right whitespace-nowrap">
                          <WithTip tip="Download this file.">
                            <Button variant="ghost" size="icon-sm" aria-label="Download" onClick={() => download(`exports/files/${encodeURIComponent(f.name)}`, undefined, f.name).catch((e) => toast.error(errorMessage(e)))}>
                              <DownloadIcon />
                            </Button>
                          </WithTip>
                          <Confirm title={`Delete ${f.name}?`} description="The file is removed from the export directory. The measurements stay in the database." confirm="Delete" onConfirm={() => del.mutate(f.name)} tip="Delete this file from the server.">
                            <Button variant="ghost" size="icon-sm" aria-label="Delete">
                              <Trash2Icon />
                            </Button>
                          </Confirm>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )
            }
          </QueryState>
        </CardContent>
      </Card>
    </>
  )
}
