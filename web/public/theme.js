// Applies the saved colour theme before the first paint, so that the panel doesn't flash.
// It is a file rather than an inline script because the Content Security Policy forbids inline scripts.
try {
  const theme = localStorage.getItem("theme")
  const dark = theme === "dark" || (theme !== "light" && matchMedia("(prefers-color-scheme: dark)").matches)
  document.documentElement.classList.toggle("dark", dark)
} catch {
  // Storage can be blocked; the light theme is used then.
}
