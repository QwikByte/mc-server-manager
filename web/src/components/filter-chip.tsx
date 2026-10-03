import { XIcon } from "@phosphor-icons/react"
import { t } from "i18next"

/** An active filter that can be removed. */
export function FilterChip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 py-0.5 pr-1 pl-3 text-xs font-medium text-primary">
      {label}
      <button
        type="button"
        aria-label={t("Remove the filter: {{filter}}", { filter: label })}
        onClick={onRemove}
        className="grid size-5 place-items-center rounded-full outline-none hover:bg-primary/15 focus-visible:ring-2 focus-visible:ring-ring"
      >
        <XIcon className="size-3" weight="bold" />
      </button>
    </span>
  )
}
