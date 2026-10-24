import type { ReactNode } from "react"

import { Card, CardContent } from "@/components/ui/card"
import { Hint } from "@/components/hint"

/** A headline number with its label and an explanation. */
export function Stat({ label, value, sub, hint }: { label: string; value: ReactNode; sub?: ReactNode; hint: ReactNode }) {
  return (
    <Card className="py-4">
      <CardContent className="px-4">
        <div className="flex items-center gap-1.5 text-xs font-semibold tracking-wide uppercase">
          {label}
          <Hint>{hint}</Hint>
        </div>
        <div className="mt-2 text-2xl font-bold tracking-tight tabular-nums">{value}</div>
        {sub && <div className="mt-1 text-xs">{sub}</div>}
      </CardContent>
    </Card>
  )
}
