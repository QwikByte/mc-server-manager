import { createContext } from "react"
import type { DataGroup } from "./tree"

/** The data that templates of the step being edited can name. */
export const DataContext = createContext<DataGroup[]>([])
