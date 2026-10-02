import type { NodeLimits } from "./api"

/** Form state of node limits; empty strings stand for unset numbers. */
export interface LimitsForm {
  portMin: string
  portMax: string
  limitMemory: boolean
  memoryReserveMb: string
}

export function limitsForm(limits: NodeLimits): LimitsForm {
  return {
    portMin: limits.portMin?.toString() ?? "",
    portMax: limits.portMax?.toString() ?? "",
    limitMemory: limits.memoryReserveMb !== null,
    memoryReserveMb: (limits.memoryReserveMb ?? 1024).toString(),
  }
}

const number = (value: string) => (value.trim() === "" ? null : Number(value))

export function limitsOf(form: LimitsForm): NodeLimits {
  return {
    portMin: number(form.portMin),
    portMax: number(form.portMax),
    memoryReserveMb: form.limitMemory ? Number(form.memoryReserveMb) || 0 : null,
  }
}
