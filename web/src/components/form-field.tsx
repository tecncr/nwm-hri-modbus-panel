import type { ReactNode } from "react"

import { HintLabel } from "@/components/hint"
import { cn } from "@/lib/utils"

/** Label, explanatory tooltip and control, stacked. */
export function FormField({ id, label, hint, children, className }: { id: string; label: string; hint: ReactNode; children: ReactNode; className?: string }) {
  return (
    <div className={cn("grid gap-1.5", className)}>
      <HintLabel htmlFor={id} hint={hint}>
        {label}
      </HintLabel>
      {children}
    </div>
  )
}

/** A switch row: control on the left, label with tooltip on the right. */
export function SwitchField({ id, label, hint, children }: { id: string; label: string; hint: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      {children}
      <HintLabel htmlFor={id} hint={hint}>
        {label}
      </HintLabel>
    </div>
  )
}

export function FormSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset className="grid gap-4 border-t pt-4">
      <legend className="pr-2 text-sm font-medium">{title}</legend>
      {children}
    </fieldset>
  )
}
