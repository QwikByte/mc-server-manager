import { formatBytes } from "@/lib/format"

const decimal = new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 })

/** Formats thousandths of a CPU core, e.g. "0.42 cores". */
export function formatCores(millis: number) {
  const cores = millis / 1000
  return `${decimal.format(cores)} ${cores === 1 ? "core" : "cores"}`
}

/** Formats bytes per second, e.g. "12.5 KB/s". */
export const formatRate = (bytes: number) => `${formatBytes(Math.round(bytes))}/s`

export const formatNumber = (n: number) => decimal.format(n)
