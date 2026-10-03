import i18next from "i18next"
import { initReactI18next } from "react-i18next"

// The panel's English texts are the keys of their translations. src/locales/en.json lists them
// (npm run i18n); a language is a copy of it with the texts translated, e.g. de.json, which
// browsers that prefer German then get. Texts that aren't translated stay English.
const languages = import.meta.glob<Record<string, string>>(["../locales/*.json", "!../locales/en.json"], { import: "default" })

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
  const lng = navigator.languages.map((l) => l.split("-")[0]).find((l) => `../locales/${l}.json` in languages) ?? "en"
  const resources = lng === "en" ? {} : { [lng]: { translation: await languages[`../locales/${lng}.json`]() } }
  document.documentElement.lang = lng
  await i18next.use(initReactI18next).init({
    lng,
    fallbackLng: "en",
    resources,
    keySeparator: false,
    nsSeparator: false,
    // React escapes the texts. Values that users control go into components of <Trans>, never
    // into its values, which it would parse as markup.
    interpolation: { escapeValue: false },
    react: { transKeepBasicHtmlNodesFor: [] },
  })
}
