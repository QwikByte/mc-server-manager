import { CheckIcon, CopyIcon } from "@phosphor-icons/react"
import { t } from "i18next"
import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** Copies a value, e.g. an address, and shows for a moment that it did. label names the value. */
export function CopyButton({ value, label, className }: { value: string; label: string; className?: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t("The browser didn't allow copying."))
    }
  }

  return (
    <Button
      type="button"
      size="icon-xs"
      variant="ghost"
      aria-label={copied ? t("Copied") : t("Copy {{label}}", { label })}
      title={t("Copy {{label}}", { label })}
      onClick={copy}
      className={cn("text-muted-foreground", className)}
    >
      {copied ? <CheckIcon className="text-success" /> : <CopyIcon />}
    </Button>
  )
}
