import { useEffect, useState } from "react"

/** The current time in milliseconds, renewed every interval while running. */
export function useNow(running: boolean, interval = 1_000) {
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    if (!running) return
    const timer = setInterval(() => setNow(Date.now()), interval)
    return () => clearInterval(timer)
  }, [running, interval])
  return now
}
