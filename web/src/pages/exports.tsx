import { useState, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { DownloadIcon, PlayIcon, RefreshCwIcon, Trash2Icon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { PageHeader } from "@/components/app-shell"
import { Confirm } from "@/components/confirm"
import { FormField } from "@/components/form-field"
import { WithTip } from "@/components/hint"
import { QueryState } from "@/components/query-state"
import { Stat } from "@/components/stat"
import { useAction } from "@/hooks/use-action"
import { api, download, errorMessage } from "@/lib/api"
import { fmtBytes, fmtDateTime, fmtGoDuration, fmtNumber, fmtRelative, parseSchedule, toLocalInput } from "@/lib/format"
import type { ExportFile, ExportRegeneration, ExportResult, ExportStatus } from "@/lib/types"

export function ExportsPage() {
  const { t } = useTranslation()
  const data = useQuery({ queryKey: ["exports"], queryFn: () => api.get<{ status: ExportStatus; files: ExportFile[] }>("exports"), refetchInterval: 30_000 })
  const run = useAction(() => api.post<ExportResult>("exports/run"), {
    success: (r) => t("exports.exported", { rows: fmtNumber(r.rows), count: r.files.length }),
    invalidate: [["exports"]],
  })
  const del = useAction((name: string) => api.del(`exports/files/${encodeURIComponent(name)}`), { success: t("exports.fileDeleted"), invalidate: [["exports"]] })
  const st = data.data?.status
  const last = st?.last_run
  const schedule = st && parseSchedule(st.schedule)

  return (
    <>
      <PageHeader
        title={t("exports.title")}
        description={t("exports.description")}
        actions={
          <>
            <RegenerateDialog tip={t("exports.regenerateTip")}>
              <Button variant="outline">
                <RefreshCwIcon /> {t("exports.regenerate")}
              </Button>
            </RegenerateDialog>
            <WithTip tip={t("exports.runTip")}>
              <Button onClick={() => run.mutate(undefined)} disabled={run.isPending}>
                {run.isPending ? <Spinner /> : <PlayIcon />} {t("exports.run")}
              </Button>
            </WithTip>
          </>
        }
      />
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label={t("exports.schedule")} value={st ? (st.enabled ? schedule!.text : t("common.off")) : "—"} sub={st?.enabled && schedule!.aligned ? t("exports.aligned") : undefined} hint={t("exports.scheduleHint")} />
        <Stat label={t("exports.window")} value={fmtGoDuration(st?.window)} hint={t("exports.windowHint")} />
        <Stat label={t("exports.nextRun")} value={st?.next_run ? fmtRelative(st.next_run) : "—"} sub={st?.next_run ? fmtDateTime(st.next_run) : undefined} hint={t("exports.nextRunHint")} />
        <Stat
          label={t("exports.lastRun")}
          value={last ? (last.error ? t("exports.failed") : t("exports.rows", { count: last.rows, formatted: fmtNumber(last.rows) })) : "—"}
          sub={last ? (last.error ?? fmtRelative(last.tick)) : undefined}
          hint={t("exports.lastRunHint")}
        />
      </div>
      <Card>
        <CardHeader>
          <CardTitle>{t("exports.files")}</CardTitle>
          <CardDescription>{st ? t("exports.directory", { directory: st.directory }) : ""}</CardDescription>
        </CardHeader>
        <CardContent>
          <QueryState query={data}>
            {(d) =>
              d.files.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("exports.noFiles")}</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("exports.file")}</TableHead>
                      <TableHead className="text-right">{t("exports.size")}</TableHead>
                      <TableHead>{t("exports.created")}</TableHead>
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
                          <WithTip tip={t("exports.downloadTip")}>
                            <Button variant="ghost" size="icon-sm" aria-label={t("common.download")} onClick={() => download(`exports/files/${encodeURIComponent(f.name)}`, undefined, f.name).catch((e) => toast.error(errorMessage(e)))}>
                              <DownloadIcon />
                            </Button>
                          </WithTip>
                          <Confirm title={t("exports.deleteTitle", { name: f.name })} description={t("exports.deleteDescription")} confirm={t("common.delete")} onConfirm={() => del.mutate(f.name)} tip={t("exports.deleteTip")}>
                            <Button variant="ghost" size="icon-sm" aria-label={t("common.delete")}>
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

/** Rewrites the periodic files of a past range from the database. */
function RegenerateDialog({ tip, children }: { tip: string; children: ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [range, setRange] = useState(() => ({ from: toLocalInput(new Date(Date.now() - 7 * 86400e3)), to: toLocalInput(new Date()) }))
  const from = new Date(range.from)
  const to = new Date(range.to)
  const valid = !isNaN(from.getTime()) && !isNaN(to.getTime()) && from < to
  const regen = useAction(() => api.post<ExportRegeneration>("exports/regenerate", { from: from.toISOString(), to: to.toISOString() }), {
    success: (r) => t("exports.regenerated", { windows: fmtNumber(r.windows), rows: fmtNumber(r.rows), count: r.files.length }),
    invalidate: [["exports"]],
    onSuccess: (r) => {
      if (r.errors.length > 0) toast.error(t("exports.regenerateErrors", { count: r.errors.length }), { description: r.errors[0] })
      setOpen(false)
    },
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <WithTip tip={tip}>
        <DialogTrigger asChild>{children}</DialogTrigger>
      </WithTip>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("exports.regenerateTitle")}</DialogTitle>
          <DialogDescription>{t("exports.regenerateDescription")}</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField id="regen-from" label={t("exports.regenerateFrom")} hint={t("exports.regenerateFromHint")}>
            <Input id="regen-from" type="datetime-local" value={range.from} onChange={(e) => setRange({ ...range, from: e.target.value })} />
          </FormField>
          <FormField id="regen-to" label={t("exports.regenerateTo")} hint={t("exports.regenerateToHint")}>
            <Input id="regen-to" type="datetime-local" value={range.to} onChange={(e) => setRange({ ...range, to: e.target.value })} />
          </FormField>
        </div>
        {!valid && <p className="text-sm text-destructive">{t("exports.regenerateInvalid")}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={() => regen.mutate(undefined)} disabled={!valid || regen.isPending}>
            {regen.isPending ? <Spinner /> : <RefreshCwIcon />} {t("exports.regenerate")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
