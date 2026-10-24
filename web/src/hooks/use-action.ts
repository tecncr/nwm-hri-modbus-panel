import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { errorMessage } from "@/lib/api"
import type { InventoryResult } from "@/lib/types"

/**
 * A mutation that reports success/failure as a toast and refreshes the
 * given query keys afterwards.
 */
export function useAction<TArgs, TResult>(fn: (args: TArgs) => Promise<TResult>, opts: { success?: string | ((r: TResult) => string); invalidate?: unknown[][]; onSuccess?: (r: TResult) => void } = {}) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: (r) => {
      const inv = (r as { inventory?: InventoryResult } | undefined)?.inventory
      if (opts.success) toast.success(typeof opts.success === "function" ? opts.success(r) : opts.success, inv?.warning ? { description: inv.warning } : undefined)
      opts.onSuccess?.(r)
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => {
      for (const key of opts.invalidate ?? []) qc.invalidateQueries({ queryKey: key })
    },
  })
}
