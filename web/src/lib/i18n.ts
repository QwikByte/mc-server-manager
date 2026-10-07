import i18next from "i18next"
import { initReactI18next } from "react-i18next"

// The panel's English texts are the keys of their translations. src/locales/en.json lists them
// (npm run i18n); a language is a copy of it with the texts translated, e.g. de.json. Texts that
// aren't translated stay English.
const files = import.meta.glob<Record<string, string>>(["../locales/*.json", "!../locales/en.json"], { import: "default" })

/** The codes of the panel's languages: English and those with a file in src/locales. */
export const languages = ["en", ...Object.keys(files).map((path) => path.slice(path.lastIndexOf("/") + 1, -".json".length))]

/** The name of a language in itself, e.g. Deutsch. */
export function languageName(code: string) {
  const name = new Intl.DisplayNames([code], { type: "language" }).of(code) ?? code
  return name[0].toUpperCase() + name.slice(1)
}

// The language chosen in this browser, so that the panel shows it before anyone signs in. The
// panel also stores the choice of the signed-in user, which then applies in every browser.
const storageKey = "noryx-language"

/** The language chosen in this browser, or "" to follow the browser's languages. */
export function chosenLanguage() {
  try {
    return localStorage.getItem(storageKey) ?? ""
  } catch {
    return "" // e.g. with site data blocked
  }
}

/** The language of a tag without its script or region, e.g. pt for pt-BR. */
const base = (tag: string) => tag.split("-")[0].toLowerCase()

/**
 * The language the panel shows for a choice: the chosen one, else the first of the browser's
 * languages the panel has, fully (pt-BR) or by its language alone (pt, then pt-PT), else English.
 */
function resolve(choice: string) {
  if (languages.includes(choice)) return choice
  for (const tag of navigator.languages) {
    const match =
      languages.find((l) => l.toLowerCase() === tag.toLowerCase()) ??
      languages.find((l) => l === base(tag)) ??
      languages.find((l) => base(l) === base(tag))
    if (match) return match
  }
  return "en"
}

/** Chooses a language, "" for the browser's, and reloads the panel if it shows another one. */
export function chooseLanguage(choice: string) {
  try {
    if (choice) localStorage.setItem(storageKey, choice)
    else localStorage.removeItem(storageKey)
  } catch {
    // Only this page then shows the language.
  }
  if (resolve(choice) !== language) location.reload()
}

// The language the panel shows, chosen before anything renders.
const language = resolve(chosenLanguage())

/**
 * The locale of dates and numbers: the panel's language, in the browser's region for it if there
 * is one, e.g. de-AT. A language with its own script or region, e.g. pt-BR, keeps it.
 */
export const locale = language.includes("-")
  ? language
  : (navigator.languages.find((l) => base(l) === language && valid(l)) ?? language)

/** Whether Intl takes a locale; some browsers report ones it doesn't, e.g. en-US@posix. */
function valid(l: string) {
  try {
    return Intl.getCanonicalLocales(l).length > 0
  } catch {
    return false
  }
}

/**
 * Marks a text outside of components for translation, which the components do with t. t itself
 * can't run there, as modules are loaded before the language is chosen.
 */
export const msg = <T extends string>(text: T) => text

/**
 * Chooses the language before the panel renders, so components can use i18next's t directly.
 * Changing it later means reloading the panel.
 */
export async function setUpI18n() {
  const resources = language === "en" ? {} : { [language]: { translation: await files[`../locales/${language}.json`]() } }
  document.documentElement.lang = language
  await i18next.use(initReactI18next).init({
    lng: language,
    fallbackLng: "en",
    // Texts not translated yet are empty in the files of the languages.
    returnEmptyString: false,
    resources,
    keySeparator: false,
    nsSeparator: false,
    // React escapes the texts. Values that users control go into components of <Trans>, never
    // into its values, which it would parse as markup.
    interpolation: { escapeValue: false },
    react: { transKeepBasicHtmlNodesFor: [] },
  })
}
