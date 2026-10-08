/** Colours for icons and status: emerald for servers, sky for nodes, violet for networks. */
export type Tone = "success" | "info" | "violet" | "warning" | "destructive" | "neutral"

export const toneClasses: Record<Tone, string> = {
  success: "bg-success/10 text-success ring-success/20",
  info: "bg-info/10 text-info ring-info/20",
  violet: "bg-violet/10 text-violet ring-violet/20",
  warning: "bg-warning/10 text-warning ring-warning/20",
  destructive: "bg-destructive/10 text-destructive ring-destructive/20",
  neutral: "bg-muted text-muted-foreground ring-border",
}

export const toneDots: Record<Tone, string> = {
  success: "bg-success",
  info: "bg-info",
  violet: "bg-violet",
  warning: "bg-warning",
  destructive: "bg-destructive",
  neutral: "bg-muted-foreground/60",
}

export const toneText: Record<Tone, string> = {
  success: "text-success",
  info: "text-info",
  violet: "text-violet",
  warning: "text-warning",
  destructive: "text-destructive",
  neutral: "text-muted-foreground",
}
