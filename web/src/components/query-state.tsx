import type { ReactNode } from "react"
import { TriangleAlertIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { errorMessage } from "@/lib/api"

export function ErrorBox({ error, title = "Request failed" }: { error: unknown; title?: string }) {
  return (
    <Alert>
      <TriangleAlertIcon />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{errorMessage(error)}</AlertDescription>
    </Alert>
  )
}

export function Loading({ className = "h-40" }: { className?: string }) {
  return <Skeleton className={`w-full border ${className}`} />
}

/** Renders loading and error states around query data. */
export function QueryState<T>({ query, children, skeleton }: { query: { data?: T; error: unknown; isPending: boolean }; children: (data: T) => ReactNode; skeleton?: string }) {
  if (query.isPending) return <Loading className={skeleton} />
  if (query.error) return <ErrorBox error={query.error} />
  return <>{children(query.data as T)}</>
}
