import { useState } from "react"
import { LanguagesIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { WithTip } from "@/components/hint"
import { autoLanguage, LANGUAGES, setLanguage, storedLanguage, type Language } from "@/i18n"

const AUTO = "auto"

/** Picks the dashboard language, or follows the browser (the default). */
export function LanguageSwitcher() {
  const { t } = useTranslation()
  const [choice, setChoice] = useState<Language | typeof AUTO>(() => storedLanguage() ?? AUTO)
  return (
    <DropdownMenu>
      <WithTip tip={t("language.tip")}>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon" aria-label={t("language.label")}>
            <LanguagesIcon />
          </Button>
        </DropdownMenuTrigger>
      </WithTip>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>{t("language.label")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuRadioGroup
          value={choice}
          onValueChange={(v) => {
            const lng = v === AUTO ? null : (v as Language)
            setChoice(lng ?? AUTO)
            setLanguage(lng)
          }}
        >
          <DropdownMenuRadioItem value={AUTO}>{t("language.auto", { language: t(`language.${autoLanguage()}`) })}</DropdownMenuRadioItem>
          {LANGUAGES.map((l) => (
            <DropdownMenuRadioItem key={l} value={l} lang={l}>
              {t(`language.${l}`)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
