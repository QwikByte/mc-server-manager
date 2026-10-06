import { useEffect, useReducer, useRef } from "react"

const trends = new Map<string, number[]>()

/**
 * The values a figure had while the panel was open, e.g. the players online: the latest
 * `size` of them, taken every `every` milliseconds. They outlast the component, so that a
 * page shows the trend again when it opens once more.
 */
export function useTrend(key: string, value: number | undefined, size = 60, every = 5_000) {
  const [, update] = useReducer((n: number) => n + 1, 0)
  const latest = useRef(value)
  useEffect(() => {
    latest.current = value
  })
  useEffect(() => {
    const take = () => {
      if (latest.current === undefined) return
      trends.set(key, [...(trends.get(key) ?? []), latest.current].slice(-size))
      update()
    }
    take()
    const timer = setInterval(take, every)
    return () => clearInterval(timer)
  }, [key, size, every])
  return trends.get(key) ?? []
}
