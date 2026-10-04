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

/** The language the panel shows for a choice: the chosen one, else the browser's first one the panel has, else English. */
function resolve(choice: string) {
  if (languages.includes(choice)) return choice
  return navigator.languages.map((l) => l.split("-")[0]).find((l) => languages.includes(l)) ?? "en"
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

/** The locale of dates and numbers: the panel's language, in the browser's region for it if there is one, e.g. de-AT. */
export const locale = navigator.languages.find((l) => l.split("-")[0] === language) ?? language

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
