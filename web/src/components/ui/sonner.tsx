import { t } from "i18next"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { CheckCircleIcon, InfoIcon, WarningIcon, XCircleIcon, SpinnerIcon } from "@phosphor-icons/react"
import { useTheme } from "@/lib/theme"

const Toaster = ({ ...props }: ToasterProps) => {
  const theme = useTheme()

  return (
    <Sonner
      theme={theme}
      containerAriaLabel={t("Notifications")}
      className="toaster group"
      icons={{
        success: (
          <CheckCircleIcon className="size-4 text-success" weight="fill" />
        ),
        info: (
          <InfoIcon className="size-4 text-info" weight="fill" />
        ),
        warning: (
          <WarningIcon className="size-4 text-warning" weight="fill" />
        ),
        error: (
          <XCircleIcon className="size-4 text-destructive" weight="fill" />
        ),
        loading: (
          <SpinnerIcon className="size-4 animate-spin text-muted-foreground" />
        ),
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast shadow-lg",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
