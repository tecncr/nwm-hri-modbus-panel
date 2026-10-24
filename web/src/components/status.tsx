import { CircleCheckIcon, CircleOffIcon, CircleSlashIcon, TriangleAlertIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { WithTip } from "@/components/hint"
import { fmtRelative } from "@/lib/format"

/** Device health shown with an icon and a word, never color alone. */
export function OnlineBadge({ online, enabled = true, lastOk, lastError }: { online: boolean; enabled?: boolean; lastOk?: string; lastError?: string }) {
  if (!enabled) {
    return (
      <WithTip tip="Polling is disabled for this device in its configuration.">
        <Badge variant="outline">
          <CircleSlashIcon /> Disabled
        </Badge>
      </WithTip>
    )
  }
  if (online) {
    return (
      <WithTip tip={`The last request to the encoder succeeded (${fmtRelative(lastOk)}).`}>
        <Badge>
          <CircleCheckIcon /> Online
        </Badge>
      </WithTip>
    )
  }
  if (!lastError) {
    return (
      <WithTip tip="The encoder has not been contacted since the server started.">
        <Badge variant="outline">
          <CircleOffIcon /> Unknown
        </Badge>
      </WithTip>
    )
  }
  return (
    <WithTip tip={`Last error: ${lastError}`}>
      <Badge variant="outline" className="border-dashed">
        <TriangleAlertIcon /> Offline
      </Badge>
    </WithTip>
  )
}
