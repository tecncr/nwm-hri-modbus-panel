import type { ReactNode } from "react"
import { InfoIcon } from "lucide-react"

import { Label } from "@/components/ui/label"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

/** An info icon that explains what an option does on hover or focus. */
export function Hint({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" className={cn("inline-flex size-4 shrink-0 text-muted-foreground hover:text-foreground items-center justify-center align-middle", className)} aria-label="More information">
          <InfoIcon className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-72 text-xs leading-relaxed">
        {children}
      </TooltipContent>
    </Tooltip>
  )
}

/** A form label followed by an explanatory hint. */
export function HintLabel({ htmlFor, children, hint }: { htmlFor?: string; children: ReactNode; hint: ReactNode }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label htmlFor={htmlFor}>{children}</Label>
      <Hint>{hint}</Hint>
    </div>
  )
}

/** Wraps any control with a tooltip explaining it (no-op without a tip). */
export function WithTip({ tip, children }: { tip?: ReactNode; children: ReactNode }) {
  if (!tip) return <>{children}</>
  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side="bottom" className="max-w-72 text-xs leading-relaxed">
        {tip}
      </TooltipContent>
    </Tooltip>
  )
}
