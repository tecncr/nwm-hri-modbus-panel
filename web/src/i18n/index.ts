import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import en from "./en"
import es from "./es"

// The language is, in order: the viewer's explicit choice (stored in this
// browser), the first supported language in the browser's preferences, and
// finally the server's default (server.ui_language, injected into a meta tag
// of index.html), or English.

export const LANGUAGES = ["en", "es"] as const
export type Language = (typeof LANGUAGES)[number]

/** Formatting locale used when the browser prefers no variant of the language. */
const REGIONS: Record<Language, string> = { en: "en-US", es: "es-PE" }

const STORAGE_KEY = "nwm-panel.language"

const supported = (v: string | null | undefined): Language | undefined => {
  const base = v?.trim().toLowerCase().split("-")[0]
  return LANGUAGES.find((l) => l === base)
}

function browserLanguages(): readonly string[] {
  return navigator.languages?.length ? navigator.languages : [navigator.language]
}

/** The language configured on the server, used when the browser gives no match. */
export function defaultLanguage(): Language {
  return supported(document.querySelector<HTMLMetaElement>('meta[name="nwm-panel-default-language"]')?.content) ?? "en"
}

/** The language picked from the browser preferences, ignoring any stored choice. */
export function autoLanguage(): Language {
  for (const l of browserLanguages()) {
    const s = supported(l)
    if (s) return s
  }
  return defaultLanguage()
}

/** The viewer's explicit choice, or null when following the browser. */
export function storedLanguage(): Language | null {
  try {
    return supported(localStorage.getItem(STORAGE_KEY)) ?? null
  } catch {
    return null
  }
}

/** Switches the language; null follows the browser again. */
export function setLanguage(lng: Language | null) {
  try {
    if (lng) localStorage.setItem(STORAGE_KEY, lng)
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Storage unavailable: the choice lasts until the page is reloaded.
  }
  void i18n.changeLanguage(lng ?? autoLanguage())
}

/** BCP 47 locale for number and date formatting in the current language. */
export function locale(): string {
  const lng = supported(i18n.language) ?? "en"
  return browserLanguages().find((l) => supported(l) === lng) ?? REGIONS[lng]
}

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en }, es: { translation: es } },
  lng: storedLanguage() ?? autoLanguage(),
  fallbackLng: "en",
  supportedLngs: LANGUAGES,
  // React escapes interpolated values itself.
  interpolation: { escapeValue: false },
  returnNull: false,
})

const syncDocument = (lng: string) => document.documentElement.setAttribute("lang", lng)
syncDocument(i18n.language)
i18n.on("languageChanged", syncDocument)

export default i18n
