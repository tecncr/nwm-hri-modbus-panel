import { CircleCheckIcon, CircleOffIcon, CircleSlashIcon, TriangleAlertIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { WithTip } from "@/components/hint"
import { fmtRelative } from "@/lib/format"

/** Device health shown with an icon and a word, never color alone. */
export function OnlineBadge({ online, enabled = true, lastOk, lastError }: { online: boolean; enabled?: boolean; lastOk?: string; lastError?: string }) {
  const { t } = useTranslation()
  if (!enabled) {
    return (
      <WithTip tip={t("status.disabledTip")}>
        <Badge variant="outline">
          <CircleSlashIcon /> {t("status.disabled")}
        </Badge>
      </WithTip>
    )
  }
  if (online) {
    return (
      <WithTip tip={t("status.onlineTip", { when: fmtRelative(lastOk) })}>
        <Badge>
          <CircleCheckIcon /> {t("status.online")}
        </Badge>
      </WithTip>
    )
  }
  if (!lastError) {
    return (
      <WithTip tip={t("status.unknownTip")}>
        <Badge variant="outline">
          <CircleOffIcon /> {t("status.unknown")}
        </Badge>
      </WithTip>
    )
  }
  return (
    <WithTip tip={t("status.offlineTip", { error: lastError })}>
      <Badge variant="outline" className="border-dashed">
        <TriangleAlertIcon /> {t("status.offline")}
      </Badge>
    </WithTip>
  )
}
