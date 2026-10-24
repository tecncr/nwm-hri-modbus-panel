import { WithTip } from "@/components/hint"
import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"

/** Wraps a trigger with a confirmation step for consequential actions. */
export function Confirm({ title, description, confirm, onConfirm, children, tip }: { tip?: string; title: string; description: ReactNode; confirm?: string; onConfirm: () => void; children: ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <WithTip tip={tip}>
        <AlertDialogTrigger asChild>{children}</AlertDialogTrigger>
      </WithTip>
      <AlertDialogContent className="data-[size=default]:sm:max-w-md">
        <AlertDialogHeader className="min-w-0">
          <AlertDialogTitle className="wrap-anywhere">{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
          <AlertDialogAction
            onClick={() => {
              onConfirm()
              setOpen(false)
            }}
          >
            {confirm ?? t("common.confirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
