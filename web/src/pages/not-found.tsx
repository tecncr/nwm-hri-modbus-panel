import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { href } from "@/hooks/use-route"

export function NotFound() {
  const { t } = useTranslation()
  return (
    <div className="py-24 text-center">
      <h1 className="text-2xl font-bold">{t("notFound.title")}</h1>
      <Button asChild variant="outline" className="mt-6">
        <a href={href("overview")}>{t("notFound.back")}</a>
      </Button>
    </div>
  )
}
