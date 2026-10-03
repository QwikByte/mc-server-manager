import "./index.css"
import { QueryClientProvider } from "@tanstack/react-query"
import { RouterProvider } from "@tanstack/react-router"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { setUpI18n } from "./lib/i18n"
import { queryClient, router } from "./router"

// The panel renders once its language is chosen. A top-level await instead would split it into many more chunks.
void setUpI18n().then(() =>
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </StrictMode>,
  ),
)
