import { Button } from "@/components/ui/button"
import { href } from "@/hooks/use-route"

export function NotFound() {
  return (
    <div className="py-24 text-center">
      <h1 className="text-2xl font-bold">Page not found</h1>
      <Button asChild variant="outline" className="mt-6">
        <a href={href("overview")}>Back to overview</a>
      </Button>
    </div>
  )
}
