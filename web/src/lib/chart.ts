/** Rounds up to 1, 2, 2.5 or 5 times a power of ten, for the top of an axis. */
export function niceMax(value: number) {
  const power = 10 ** Math.floor(Math.log10(Math.max(value, 1)))
  return [1, 2, 2.5, 5, 10].map((m) => m * power).find((v) => v >= value) ?? value
}

/** Like niceMax, in the binary unit of the value, so that formatBytes shows round numbers. */
export function niceBytes(value: number) {
  const unit = 1024 ** Math.max(0, Math.floor(Math.log(Math.max(value, 1)) / Math.log(1024)))
  return niceMax(value / unit) * unit
}
