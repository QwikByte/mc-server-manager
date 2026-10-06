/** Lets an item of a list fade in and rise, a little after the one before it. */
export const rise = (index = 0) =>
  ({
    initial: { opacity: 0, y: 10 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.3, ease: "easeOut", delay: Math.min(index, 12) * 0.04 },
  }) as const
